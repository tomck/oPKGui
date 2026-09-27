# oPKGui v0 — Design

Written after a research pass on (a) iPKGui's surviving traces and (b) current
`spksrc`/DSM 7 conventions for WebUI-backed packages. See `AGENT-HANDOFF.md` for the
project's origin and distribution strategy.

## Research findings

**iPKGui:** nothing recoverable. Its host (cphub.net) is dead, no GitHub repo or archived
`.spk` surfaced, and every secondary mention just namechecks it without describing the UI.
Treated as a dead lead — v0's UI is designed fresh, not ported.

**Current DSM 7 web-package convention** (studied via `spksrc`'s own reference package,
[`demowebservice`](https://github.com/SynoCommunity/spksrc/tree/master/spk/demowebservice)):
a DSM 7 web-type package is Apache+PHP served by WebStation, running as a dedicated,
**unprivileged** per-package account (`sc-<pkgname>`), provisioned automatically by DSM
itself from the `conf/resource` file's `webservice.services[].php.user` field — no custom
account-creation script needed, no `conf/privilege` escalation file involved. Confirmed by
inspecting `demowebservice`'s actual `Makefile`, `src/conf_php8.2/resource`, and
`src/service-setup.sh`.

**Important negative finding:** there is no `spksrc`-provided pattern for "safely broker a
privileged CLI command from a web frontend." Packages that need root-level actions (like
`opkg install`) get no help from the framework — that part is on us, same conclusion the
original handoff doc anticipated.

## The v0 scope decision this unlocks

`opkg list-installed` and `opkg list-upgradable` are **read-only** operations, so the
*intent* was for v0 to need no privilege boundary at all. That assumption turned out to be
wrong on the user's real NAS (see "Round 3" below) — but the mutation problem
(`opkg install/remove`) is still deferred to v1; v0 only had to solve *read* access.

## v0 architecture (current)

- **Package type:** a self-contained DSM service package (`STARTABLE = yes`,
  `SERVICE_USER = auto`, own embedded HTTP server on port `18890`) — no WebStation/PHP
  dependency. See "Round 2" below for why this replaced the original web-type design.
- **Backend/frontend:** a single static Go binary (`pkgsrc/opkgui/cmd/opkgui/main.go`,
  cross-compiled `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`) serving an embedded HTML/JS page
  and a `/api?action=installed|upgradable` JSON endpoint. `action` only ever selects a key
  into a fixed Go map of exact argv slices (`exec.Command("/opt/bin/opkg", "list-installed")`
  etc.) — never concatenated into a shell string, so there's no injection surface by
  construction, not by sanitization.
- **Packaging:** built through SynoCommunity's actual `spksrc` framework (via their
  published `ghcr.io/synocommunity/spksrc` Docker image), not hand-assembled. `pkgsrc/opkgui/`
  holds the package source; `build.sh` cross-compiles the Go binary, clones `spksrc` into a
  gitignored `.spksrc/`, syncs `pkgsrc/opkgui` into its `spk/opkgui`, and runs `make` inside
  the container.
- **Privilege model:** `conf/privilege` declares `defaults.run-as: package` (the running
  service is unprivileged, as `sc-opkgui`) with `ctrl-script` overrides making only the
  install/uninstall/upgrade lifecycle scripts run as root — modeled directly on real
  `spksrc` packages (`dnscrypt-proxy`, `ntopng`, `saltpad`) that need the same split. The
  root-run `postinst` grants `sc-opkgui` group-read access to Entware's `opkg.conf` and
  `status` files (`chown root:sc-opkgui` + `chmod 640`); `postuninst` reverts it. See
  "Round 3" for why this is needed at all.
- **Noarch caveat:** packaged as `override ARCH=noarch` even though the binary is real
  x86_64 — see "Round 4." This means DSM won't block installing on a non-x86_64 model at
  the Package Center level (it would just fail to start there, wrong binary format). Fine
  for solo use on the user's one confirmed x86_64 NAS; **must be fixed before any wider
  distribution.**
- **Testing loop:** manual sideload via DSM's Package Center → Manual Install, pointing at
  the built `.spk` file in `dist/`.

## Round 1: use the framework, not a hand-rolled tar

The first v0 attempt hand-assembled the `.spk` tar structure directly instead of using
`spksrc`, reasoning that a pure-`noarch` package didn't need the cross-compilation
toolchain. That repeated exactly the mistake this project already knows to avoid elsewhere
([[SynoVPNSocks]] uses Entware/`opkg` instead of reinventing package management) — `spksrc`
isn't just a cross-compiler, it's accumulated knowledge of DSM packaging pitfalls (like the
`conf/privilege` requirement that blocked the hand-built attempt outright: "Unable to
install because it runs with root privileges"). Switched to building through the real
framework from this point on. (The original design was also PHP+WebStation-based; see
Round 2 for why that changed too, independent of this tooling fix.)

## Round 2: drop WebStation/PHP for a self-contained service

Testing directly against the user's real NAS over SSH (read-only commands only) showed
**WebStation isn't installed there at all**, and none of the NAS's other web-facing
packages (Transmission, Jackett, Sonarr, Radarr, xbvr) depend on it — they all ship their
own embedded server. Requiring WebStation+Apache+PHP just for oPKGui would have been a
heavier dependency footprint than anything else on the box, and didn't match the pattern
already in use. Rebuilt as a single static Go binary with its own HTTP server instead,
packaged the way `spksrc`'s own `filebrowser`/`dnscrypt-proxy`/`ntopng` packages are
(`STARTABLE=yes`, `SERVICE_USER=auto`, `SERVICE_PORT`) rather than the `demowebservice`
web-type pattern.

## Round 3: the "no privilege needed" assumption was wrong

Also confirmed live via SSH: `/opt/etc/opkg.conf` and `/opt/lib/opkg/status` are both `600
root:root` on the user's NAS. Running `opkg list-installed` as an unprivileged user fails
outright (`opkg_conf_parse_file: Failed to open /opt/etc/opkg.conf: Permission denied`) —
even read-only listing needs *some* elevated step, exactly the fallback this doc's original
draft flagged as possible. `setfacl`/`getfacl` aren't available on this DSM (no ACL
package), so the grant uses plain POSIX group permissions instead of ACLs: `postinst` (runs
as root per `conf/privilege`'s `ctrl-script` override) does
`chown root:sc-opkgui opkg.conf status && chmod 640 opkg.conf status`, scoping read access
to exactly this package's own dedicated account rather than loosening the files for every
user on the box. `postuninst` reverts to `600 root:root`.

## Round 4: noarch vs. real arch metadata

Tried packaging with the real architecture (`ARCH=x64`, which covers the user's `avoton`
Atom C2538 model) so DSM would properly restrict install to compatible models. This
requires `spksrc`'s C cross-toolchain machinery even though we don't compile any C code
(the toolchain-extraction stage is unconditional for any non-`noarch` `ARCH`), and hit a
Docker-Desktop-on-macOS bind-mount permission failure extracting it — plus the user's Mac
had only ~5.4GB free at the time, which made further toolchain-download attempts a bad
trade for what's currently solo testing on one confirmed-working machine. Decided (with the
user) to ship as `noarch` for now and revisit real per-arch packaging only if this is ever
distributed beyond the user's own NAS.

## Non-goals for v0 (unchanged from the handoff doc)

- No install/remove/upgrade actions.
- No architecture-detection logic beyond "does `/opt/bin/opkg` exist."
