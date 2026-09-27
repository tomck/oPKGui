// oPKGui: a read-only web viewer for Entware (opkg) packages, packaged as a
// self-contained DSM service (own embedded HTTP server, no WebStation/PHP
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
	"strings"
)

//go:embed static/index.html
var staticFS embed.FS

const opkgBin = "/opt/bin/opkg"

// action never reaches the shell string -- it only selects a key into this
// fixed command table, so there is no injection surface to defend against.
var commands = map[string][]string{
	"installed":  {opkgBin, "list-installed"},
	"upgradable": {opkgBin, "list-upgradable"},
}

type pkg struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	NewVersion string `json:"new_version,omitempty"`
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

func apiHandler(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	cmd, ok := commands[action]
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

	var packages []pkg
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " - ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) < 2 {
			continue
		}
		p := pkg{Name: parts[0], Version: parts[1]}
		if len(parts) >= 3 {
			p.NewVersion = parts[2]
		}
		packages = append(packages, p)
	}
	if packages == nil {
		packages = []pkg{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"packages": packages})
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
	mux.HandleFunc("/api", apiHandler)

	addr := "0.0.0.0:" + *port
	log.Printf("oPKGui listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
