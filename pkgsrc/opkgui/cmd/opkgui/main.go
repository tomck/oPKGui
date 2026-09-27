// oPKGui: a web viewer and manager for Entware (opkg) packages, packaged as
// a self-contained DSM service (own embedded HTTP server, no WebStation/PHP
// dependency).
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

//go:embed static/index.html
var staticFS embed.FS

const (
	opkgBin = "/opt/bin/opkg"
	sudoBin = "/usr/bin/sudo"
)

// action never reaches the shell string -- it only selects a key into this
// fixed command table, so there is no injection surface to defend against.
var readCommands = map[string][]string{
	"installed":  {opkgBin, "list-installed"},
	"upgradable": {opkgBin, "list-upgradable"},
	"available":  {opkgBin, "list"},
}

// Mutating actions run via sudo (a narrow sudoers rule scopes sc-opkgui to
// exactly these three opkg subcommands -- see opkgui-grant-permissions.sh).
// pkg is never passed through a shell: it becomes one argv element to
// exec.Command, and is additionally validated against packageNameRE and
// cross-checked against opkg's own current listing before being used at
// all, so even a bug in one layer doesn't hand opkg an arbitrary string.
var mutateSubcommands = map[string]string{
	"install": "install",
	"remove":  "remove",
	"upgrade": "upgrade",
}

var packageNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]*$`)

// systemBinDirs are checked to flag packages that likely duplicate a
// command DSM/BusyBox already provides outside of Entware's own /opt tree
// (e.g. installing Entware's `bash` alongside the system's own /bin/bash).
// This is a name-only heuristic: it catches a package whose name matches
// the command it provides, but not a multi-binary package (coreutils,
// findutils, ...) whose name differs from the binaries it would actually
// shadow.
var systemBinDirs = []string{
	"/bin", "/sbin", "/usr/bin", "/usr/sbin",
	"/usr/syno/bin", "/usr/syno/sbin", "/usr/local/bin", "/usr/local/sbin",
}

func systemHasCommand(name string) bool {
	for _, dir := range systemBinDirs {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// entwareArchFor maps `uname -m` to Entware's own install-script naming
// scheme (https://github.com/entware/entware/wiki/Install-on-Synology-NAS)
// -- confirmed live: architecture alone determines the URL, not the
// specific Synology platform/model.
type entwareArch struct {
	label        string
	installerURL string
}

var entwareArchFor = map[string]entwareArch{
	"x86_64":   {"x64", "https://bin.entware.net/x64-k3.2/installer/generic.sh"},
	"aarch64":  {"aarch64", "https://bin.entware.net/aarch64-k3.10/installer/generic.sh"},
	"armv7l":   {"armv7", "https://bin.entware.net/armv7sf-k3.2/installer/generic.sh"},
	"armv5tel": {"armv5", "https://bin.entware.net/armv5sf-k3.2/installer/generic.sh"},
}

// entwareBootScript is Entware's own documented boot-time mount/init script,
// verbatim from their install wiki -- not something we invented. Assumes
// /volume1; the user may need to adjust this for their own setup.
const entwareBootScript = `#!/bin/sh
# Mount/Start Entware
mkdir -p /opt
mount -o bind "/volume1/@Entware/opt" /opt
/opt/etc/init.d/rc.unslung start

# Add Entware Profile in Global Profile
if grep -qF '/opt/etc/profile' /etc/profile; then
	echo "Confirmed: Entware Profile in Global Profile"
else
	echo "Adding: Entware Profile in Global Profile"
cat >> /etc/profile <<"EOF"

# Load Entware Profile
[ -r "/opt/etc/profile" ] && . /opt/etc/profile
EOF
fi

# Update Entware List
/opt/bin/opkg update
`

// statusHandler reports whether opkg exists at all, and if not, everything
// needed to bootstrap Entware: detected architecture (oPKGui can never do
// this install itself -- it needs the same root access DSM blocks the
// package from ever requesting, per DESIGN.md Round 5 -- so this only ever
// guides the user, mirroring Entware's own documented steps).
func statusHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(opkgBin); err == nil {
		writeOK(w, map[string]interface{}{"opkg_installed": true})
		return
	}

	unameOut, _ := exec.Command("uname", "-m").CombinedOutput()
	unameM := strings.TrimSpace(string(unameOut))

	body := map[string]interface{}{
		"opkg_installed": false,
		"uname_m":        unameM,
		"boot_script":    entwareBootScript,
	}
	if arch, ok := entwareArchFor[unameM]; ok {
		body["arch"] = arch.label
		body["install_cmd"] = "wget -O - " + arch.installerURL + " | /bin/sh"
	}
	writeOK(w, body)
}

type pkg struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	NewVersion string `json:"new_version,omitempty"`
	Desc       string `json:"desc,omitempty"`
	SystemDup  bool   `json:"system_dup,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg string, extra ...string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]interface{}{"error": msg}
	if len(extra) > 0 {
		body["output"] = extra
	}
	json.NewEncoder(w).Encode(body)
}

func writeOK(w http.ResponseWriter, body map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(body)
}

// opkg keeps its own lock file (/opt/tmp/opkg.lock) and fails immediately
// rather than waiting when it's already held elsewhere -- including by
// another invocation from this same process (the frontend loads all three
// tabs concurrently on page load). Serializing every opkg/sudo call through
// this mutex is what actually avoids that, not just reducing how often it
// happens.
var opkgMu sync.Mutex

func runOpkg(args ...string) (lines []string, err error) {
	opkgMu.Lock()
	defer opkgMu.Unlock()
	out, err := exec.Command(opkgBin, args...).CombinedOutput()
	lines = strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return lines, err
}

func runOpkgAsRoot(args ...string) (lines []string, err error) {
	opkgMu.Lock()
	defer opkgMu.Unlock()
	fullArgs := append([]string{"-n", opkgBin}, args...)
	out, err := exec.Command(sudoBin, fullArgs...).CombinedOutput()
	lines = strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return lines, err
}

// parsePackageLines handles opkg's " - "-delimited line formats:
//
//	list-installed: "name - version"
//	list-upgradable: "name - old_version - new_version"
//	list:            "name - version - description"
func parsePackageLines(lines []string, thirdField string) []pkg {
	var packages []pkg
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " - ", 3)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) < 2 {
			continue
		}
		p := pkg{Name: parts[0], Version: parts[1]}
		if len(parts) >= 3 {
			switch thirdField {
			case "new_version":
				p.NewVersion = parts[2]
			case "desc":
				p.Desc = parts[2]
			}
		}
		packages = append(packages, p)
	}
	if packages == nil {
		packages = []pkg{}
	}
	return packages
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	cmd, ok := readCommands[action]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}

	if _, err := os.Stat(opkgBin); err != nil {
		writeError(w, http.StatusServiceUnavailable, "opkg not found at "+opkgBin+" -- is Entware installed?")
		return
	}

	lines, err := runOpkg(cmd[1:]...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "opkg failed: "+err.Error(), lines...)
		return
	}

	thirdField := map[string]string{"upgradable": "new_version", "available": "desc"}[action]
	packages := parsePackageLines(lines, thirdField)
	if action == "available" {
		for i := range packages {
			packages[i].SystemDup = systemHasCommand(packages[i].Name)
		}
	}
	writeOK(w, map[string]interface{}{"packages": packages})
}

// isKnownPackage cross-checks a name against opkg's own current listing
// (list for install targets, list-installed for remove/upgrade targets)
// rather than trusting packageNameRE alone.
func isKnownPackage(name string, installed bool) bool {
	args := []string{"list"}
	if installed {
		args = []string{"list-installed"}
	}
	lines, err := runOpkg(args...)
	if err != nil {
		return false
	}
	prefix := name + " - "
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func mutateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	// Minimal CSRF guard: browsers don't attach custom headers to plain
	// cross-site form submissions, only to same-origin fetch()/XHR calls
	// like this app's own frontend makes.
	if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		writeError(w, http.StatusForbidden, "missing required header")
		return
	}

	action := r.URL.Query().Get("action")
	subcommand, ok := mutateSubcommands[action]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action")
		return
	}

	name := r.URL.Query().Get("pkg")
	if !packageNameRE.MatchString(name) {
		writeError(w, http.StatusBadRequest, "invalid package name")
		return
	}
	// install targets come from the available list; remove/upgrade targets
	// must already be installed.
	if !isKnownPackage(name, action != "install") {
		writeError(w, http.StatusBadRequest, "unknown package: "+name)
		return
	}

	lines, err := runOpkgAsRoot(subcommand, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "opkg "+subcommand+" failed: "+err.Error(), lines...)
		return
	}

	writeOK(w, map[string]interface{}{"output": lines})
}

func main() {
	port := flag.String("port", "18890", "port to listen on")
	flag.Parse()

	index, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		log.Fatalf("embedded index.html missing: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
	mux.HandleFunc("/api/status", statusHandler)
	mux.HandleFunc("/api", readHandler)
	mux.HandleFunc("/api/action", mutateHandler)

	addr := "0.0.0.0:" + *port
	log.Printf("oPKGui listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
