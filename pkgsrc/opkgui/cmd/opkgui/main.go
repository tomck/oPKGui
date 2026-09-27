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
	"regexp"
	"strings"
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

type pkg struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	NewVersion string `json:"new_version,omitempty"`
	Desc       string `json:"desc,omitempty"`
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

func runOpkg(args ...string) (lines []string, err error) {
	out, err := exec.Command(opkgBin, args...).CombinedOutput()
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

	out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "opkg failed: "+err.Error(), lines...)
		return
	}

	thirdField := map[string]string{"upgradable": "new_version", "available": "desc"}[action]
	writeOK(w, map[string]interface{}{"packages": parsePackageLines(lines, thirdField)})
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

	out, err := exec.Command(sudoBin, "-n", opkgBin, subcommand, name).CombinedOutput()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
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
	mux.HandleFunc("/api", readHandler)
	mux.HandleFunc("/api/action", mutateHandler)

	addr := "0.0.0.0:" + *port
	log.Printf("oPKGui listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
