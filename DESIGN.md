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

**Confirmed working end-to-end**: `opkg install cal` through the web UI actually installed
it (`/opt/bin/cal` present, Installed tab shows it). Two real bugs surfaced along the way:

1. **Lock contention.** The page fires all three tabs' fetches concurrently on load; `opkg`
   keeps its own lock file and fails immediately (not queued/retried) when it's already
   held, so roughly 2/3 of concurrent calls failed with a 255. Fixed with a mutex
   serializing every `opkg`/`sudo` call inside the Go server.
2. **A process failure, not a design failure.** After fixing the lock issue, installing
   still failed (`sudo: a password is required`) because the sudoers rule had never
   actually been written — the agent had called a tool to `cat` the grant script into the
   conversation rather than writing its content directly into a reply, so there was nothing
   for the user to actually copy into Task Scheduler. This got misdiagnosed as a DSM UI
   limitation (guessing the "Run command" field couldn't handle multi-line/pasted script
   text) and "fixed" with an unnecessary workaround — writing the script to a file on the
   NAS and having Task Scheduler call `sh /path/to/script.sh` instead of holding the script
   directly. The user corrected this directly: **Task Scheduler's Run command field handles
   a full multi-line script (comments included) pasted in as-is, no indirection needed** —
   confirmed by pasting the real script straight from an SSH session's `cat` output. The
   file-on-NAS workaround was removed once this was clear.

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

**Round 8 (later): one more lead, also closed.** A separate investigation (`~/Claude/DSMjs`)
turned up `/usr/syno/synoman/webman/3rdparty/`, whose `README` (genuine Synology docs,
copyright/version-stamped 2010) documents a completely different, older mechanism: a plain
`key = value` `application.cfg` with `type = embedded` or `popup` and its own
`protocol`/`address`/`port`/`path` fields — distinct from the JSON `app/config` scheme Round
6 exhausted, and with an example that was almost exactly our use case (point at a service's
own port, leave `address` unset so DSM fills in whatever host the admin is currently using,
solving the earlier hardcoded-IP problem too). Shipped it (coexists fine alongside the
framework's own auto-generated `app/config`/`app/opkgui.sc` — confirmed in the built
package, different filenames, same directory) and tested live: **nothing rendered anywhere**
in DSM 7 — not the Main Menu, not Control Panel, "Open" still just opened a new tab. The
README's own language ("left frame of management UI," a tree-nav paradigm) describes a DSM
version that predates the modern Desktop; the backend code reading this file evidently
hasn't been fully retired, but nothing in DSM 7's frontend appears to render whatever it once
fed. Reverted (`application.cfg` removed). **Not revisiting the embedded-window question
again without a genuinely new, concrete lead** — three real mechanisms tried, three dead
ends, diminishing returns each time.

## Round 9: the genuinely new lead — and it worked

Round 6 closed with "this almost certainly needs more than JSON config... and/or same-origin
serving." Both guesses were half-right and half-wrong, and the actual missing piece was much
simpler: real documentation existed the whole time, just not where the earlier searches
looked. `~/Claude/DSMjs` (a side investigation into the same question) initially concluded
"no public SDK, proprietary, reverse-engineering required" after reverse-engineering
first-party packages' compiled files — a real, careful process, but one that skipped
searching for existing community docs/templates first. The user pushed back directly on
that "nobody bothered" framing, which prompted an actual web search rather than more binary
archaeology, and immediately turned up:
- A `spksrc` wiki page, ["UI Develop"](https://github.com/SynoCommunity/spksrc/wiki/UI-Develop).
- A real template repo, [`DigitalBox98/SimpleExtJSApp`](https://github.com/DigitalBox98/SimpleExtJSApp),
  whose `/docs` folder ships a full ExtJS API doc/guide tarball (`synoextjsdocs.tar.gz` +
  a `-source` tarball with the actual guide markdown and framework source).

The getting-started guide's own example is the exact piece Round 6's third attempt got
wrong: third-party apps use their **own** namespace (the guide's example:
`SYNOCOMMUNITY.SimpleExtJSApp.*`), not `SYNO.SDS.*` or a guessed
`SYNO.SDS._ThirdParty.App.*` — while *extending* `SYNO.SDS.AppInstance`/`SYNO.SDS.AppWindow`
as base classes. Round 6's attempt also never shipped any actual `.js` file implementing
those classes at all, just a config guessing at the shape — this is the difference between
"guessing at a JSON shape" and "using the documented pattern with real code behind it."

Separately, pulling Synology's own official dev headers (from the same toolkit `spksrc`
uses — `archive.synology.com`'s DSM 7.1 toolkit, `avoton` platform match for this NAS)
confirmed something that had been ambiguous since the `uitest` test in `~/Claude/DSMjs`:
webapi auth (`APIRequest::IsAdmin()`, `GetLoginUID()`) is checked against the *logged-in DSM
session*, not package identity — so "a copied `.so` still worked under a new package name"
was never evidence of "no sandboxing," just evidence that the tester was already an admin.

**Implemented:** `pkgsrc/opkgui/src/ui/opkgui.js` — `SYNOCOMMUNITY.OPKGui.AppInstance`/
`AppWindow`, following the documented pattern exactly, whose window body is just an iframe
onto oPKGui's own existing `http://<hostname>:18890/` (via `Ext.Ajax`'s sibling pattern of
just hitting a plain URL — no compiled `.so` needed, since oPKGui already has a working
HTTP+JSON backend). This reuses the entire existing, already-tested frontend unchanged and
only adds real DSM window chrome around it. `Makefile`'s `DSM_UI_CONFIG` now points at a
hand-authored `src/ui/config` instead of the framework's auto-generated one. **Built, not
yet installed/tested live** — that's the next step.

One known risk not yet tested: if DSM is accessed over HTTPS, the iframe's plain-`http://`
`src` may get blocked as mixed content by the browser. Deliberately not pre-solving this
(e.g. adding TLS to the Go server, or a reverse-proxy rule) until a real test shows whether
it's actually a problem — consistent with how every other round in this doc has worked.

## Non-goals for v0 (unchanged from the handoff doc)

- No install/remove/upgrade actions.
- No architecture-detection logic beyond "does `/opt/bin/opkg` exist."
