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
- **Privilege model:** `conf/privilege` declares `defaults.run-as: package` only — no
  `ctrl-script` overrides at all. The package requests zero root access, anywhere. The
  read-access grant this service needs onto Entware's `opkg.conf`/`status` is done entirely
  *outside* the package instead, via a DSM Task Scheduler root script
  (`opkgui-grant-permissions.sh`), the same mechanism Entware itself is bootstrapped with.
  See "Round 5" for why the original `ctrl-script`-root design (modeled on `dnscrypt-proxy`/
  `ntopng`/`saltpad`) had to be abandoned.
- **Noarch caveat:** packaged as `override ARCH=noarch` even though the binary is real
  x86_64 — see "Round 4." This means DSM won't block installing on a non-x86_64 model at
  the Package Center level (it would just fail to start there, wrong binary format). Fine
  for solo use on the user's one confirmed x86_64 NAS; **must be fixed before any wider
  distribution.**
- **Testing loop:** manual sideload via DSM's Package Center → Manual Install, pointing at
  the built `.spk` file in `dist/`.

## Round 7: install/remove/upgrade (v1)

Extended the read-only v0 into a real package manager: an **Available** tab (`opkg list`,
already reachable read-only since it only needs the already-world-readable
`/opt/var/opkg-lists/entware` cache plus `opkg.conf` — no new permission needed beyond what
Round 5 already granted) with Install buttons, and Install/Remove/Upgrade buttons on the
other two tabs.

The mutation privilege boundary uses the exact same escape hatch as Round 5's read grant —
Task Scheduler running as root, since the package itself still can't request any privilege
(that constraint didn't change). `opkgui-grant-permissions.sh` now also drops a narrow
`/etc/sudoers.d/opkgui` rule: `sc-opkgui` may run `sudo -n /opt/bin/opkg install|remove|
upgrade <arg>` as root, and nothing else. Defense in depth on top of that narrow grant: the
Go backend validates any package name against a regex *and* cross-checks it against opkg's
own current `list`/`list-installed` output before it's ever passed to `sudo`/`opkg` — never
trusting a single layer, consistent with the original handoff doc's stated requirement
("no free-text command construction from user input"). Mutating requests also require
`POST` + a custom header, a lightweight CSRF guard (no login system exists to protect
otherwise).

**Not yet tested on real hardware** — unlike every other round in this doc, which were each
validated live on the NAS before being called done. `visudo` isn't available on this DSM to
pre-validate the sudoers syntax, so the first real test doubles as syntax validation; the
plan is to sanity-check `sudo -l` immediately after running the updated grant script, with
`rm -f /etc/sudoers.d/opkgui` as the documented rollback if anything looks wrong.

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

## Round 5: DSM 7 blocks root ctrl-scripts for unsigned packages, full stop

The `dnscrypt-proxy`/`ntopng`/`saltpad`-style `conf/privilege` (unprivileged service,
`ctrl-script` root override for install/uninstall/upgrade only) kept failing Manual Install
with the same "Unable to install because it runs with root privileges" error round 1 hit —
even with a syntactically valid, narrowly-scoped privilege file. DSM 6 had a Package Center
"Trust Level" setting that could be loosened to "Any publisher" for exactly this kind of
sideloaded package; **DSM 7 removed that setting entirely** (confirmed both by the user's
own Package Center UI, which has no such option, and independently via search). The working
theory: those packages' `ctrl-script` root requests are only honored because they're
installed through SynoCommunity's own signed repository — an unsigned, manually sideloaded
`.spk` can't get root for *any* script, no matter how narrowly it's scoped in
`conf/privilege`.

Confirmed empirically: stripped `ctrl-script` out entirely (pure `defaults.run-as: package`,
zero root requests) and that version installed and ran cleanly. So the permission grant
can't live in the package at all under this distribution model (self-hosted/sideloaded,
not going through SynoCommunity's signed repo — see [[feedback-distribution-strategy]]).

The user's fix: this is exactly the situation Entware's own bootstrap is already in —
Entware isn't a signed package either, so [its own install instructions](https://github.com/entware/entware/wiki/Install-on-Synology-NAS)
use a DSM Task Scheduler "Triggered Task → User-defined script" (User: root, Event:
Boot-up) to get root access outside Package Center's signing model entirely. oPKGui now
does the same thing for its one root-requiring step: `opkgui-grant-permissions.sh` is a
standalone script (not part of the `.spk`) that the user adds as a root Task Scheduler
task, doing the `chown`/`chmod` grant Round 3 originally tried to do from `postinst`.

One gotcha hit setting this up: DSM's `SERVICE_USER=auto` created the account `sc-opkgui`
but its **group** is just `opkgui` (no `sc-` prefix) — confirmed via `id sc-opkgui` on the
test NAS. `chown root:sc-opkgui` silently failed (nonexistent group) while the `chmod 640`
in the same script succeeded independently, which briefly looked like a partial fix. Fixed
by using `opkgui` as the group name.

**v0 confirmed fully working end-to-end** on the user's real NAS after this: install,
service start, permission grant, both the Installed and Updates-available tabs populate
correctly (the latter briefly errored once right after the permissions fix, most likely a
transient race with the grant script's `Run`, and resolved on its own by the next request).

## Round 6: DSM desktop window (post-v0 exploration, reverted)

The user wanted oPKGui to feel like a real DSM desktop app rather than "a separate webapp on
a port" — clicking an icon and getting a window inside DSM's own interface, not a new browser
tab. Investigated whether this is achievable at all for our architecture (a standalone Go
service on its own port, not CGI, not behind WebStation).

Found `spksrc`'s `app/config` schema supports `"type": "legacy"` as an alternative to the
default `"type": "url"`, and found a real working example — [MODS Web Console](https://github.com/vletroye/SynoPackages/tree/master/DSM%207.x/MODS%20Web%20Console%207.x),
a genuinely third-party, embedded-in-a-DSM-window webapp — to model it on. Three escalating
experiments, each tested live on the real NAS:

1. **`SERVICE_TYPE = legacy`, default URL (`/`).** Got further than expected — DSM did show
   an "Open" button and let the icon be added to the desktop. But the window it opened
   reloaded DSM's own root page inside itself ("nested DSM"), not oPKGui.
2. **Same, with a fully-qualified URL** (`http://<nas-ip>:18890/`) baked into the config,
   theorizing `legacy` used the `url` field as a literal iframe target and ignored the
   separate `protocol`/`port` fields (consistent with a bare `/` loading DSM's own root).
   Identical nesting result — ruled that theory out.
3. **Hand-authored config matching MODS's exact proven shape**: an id in the
   `SYNO.SDS._ThirdParty.App.*` namespace plus a matching `appWindow` key (neither of which
   `spksrc`'s generic template generates at all). This *regressed*: no icon, no Open button,
   nothing in the Main Menu — worse than either previous attempt, despite matching the JSON
   shape of a package that's confirmed to work.

Conclusion: DSM's `legacy`/`appWindow` third-party desktop-window mechanism almost certainly
needs more than JSON config — MODS's own tool is a substantial Windows app that likely
registers additional client-side plumbing we don't have and haven't identified, and/or
depends on same-origin serving through DSM's own web server (WebStation or raw CGI, both of
which we deliberately moved away from in Round 2 and Round 5's underlying reasoning). Not
pursuing further for now: the cost of continuing to guess outweighs the payoff, and the
result of chasing it was getting *worse*, not closer. **Reverted to the default `"type":
"url"` config** — a normal Main Menu icon that opens the app in a new tab, which is also how
other third-party Package Center webapps (Sonarr, Transmission, etc.) actually behave.

## Non-goals for v0 (unchanged from the handoff doc)

- No install/remove/upgrade actions.
- No architecture-detection logic beyond "does `/opt/bin/opkg` exist."
