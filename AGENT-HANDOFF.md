# oPKGui — Agent Handoff

Status: **idea stage, not started**. No code, no packaging, nothing installed anywhere.
This file is written by a past agent session for whichever agent (or human) picks this up next.

## The idea

Entware (the `opkg` package manager, from the OpenWrt/embedded-Linux world) is the standard
way to get software onto a Synology NAS that isn't packaged as a proper DSM `.spk` — e.g.
`tinyproxy`, which has no SynoCommunity equivalent. But Entware is CLI-only: you bootstrap it
over SSH and manage it entirely with `opkg install`/`opkg remove`/`opkg list-upgradable`. There
is no GUI for it inside DSM.

There used to be a GUI for the *predecessor* system — "iPKGui", for `ipkg`/Optware — but that's
been dead since roughly the DSM 5/6 era and is very unlikely to install on DSM 7's packaging
model. As of this writing (checked via web search, Sept 2026), nothing current fills that gap.

So: a SynoCommunity package that gives DSM's Package Center-style UI a way to browse, install,
update, and remove Entware packages, instead of dropping to SSH every time.

## Why this folder exists

Came up as a tangent while working on a *different* project, `~/Claude/SynoVPNSocks` (a VPN +
HTTP proxy setup for a DS1815+, which itself needs Entware to install `tinyproxy`). The user
does normal SynoCommunity-repo package management and was surprised Entware has no GUI. This
folder is the stub for that as its own, separate project — deliberately not folded into
SynoVPNSocks, since they solve unrelated problems and have very different risk profiles.

## Why this is a bigger lift than it sounds

1. **It's a real SynoCommunity submission, not a quick script.** That means building against
   their `spksrc` framework (https://github.com/SynoCommunity/spksrc), multi-architecture builds
   (this NAS is x86_64, but SynoCommunity packages typically need to build for ARM too), and
   going through their contribution/review process
   (https://github.com/SynoCommunity/spksrc/wiki/Frequently-Asked-Questions) — worth opening a
   discussion/issue with the maintainers *before* investing real build time, partly to gauge
   whether they'd even accept a GUI for an unofficial/unsanctioned package manager riding
   alongside their own official packages.

2. **Privilege/blast-radius problem.** A DSM WebUI package needs a backend process (typically
   PHP-CGI, per how most SynoCommunity webapps are structured) that can shell out to `opkg`,
   which means it needs meaningful privilege on the box. Naively exposing "run arbitrary opkg
   command" through a web form is a real security hole on a NAS (remote code execution surface
   if the WebUI has any auth/CSRF weakness). Needs to be scoped carefully:
   - Read-only listing/searching of available + installed packages should be low-risk.
   - Install/remove/upgrade actions need real gating (confirmation, maybe DSM's own admin-auth
     checks reused rather than reinvented, no free-text command construction from user input).
   - Should probably shell out to `opkg` with a fixed, validated set of arguments only — never
     interpolate raw user text into a shell command.

3. **Entware itself isn't Synology-sanctioned.** SynoCommunity maintainers may be cool with
   packaging a *manager for* an unofficial system, or may not — this is a real open question,
   not just a technicality. Worth an early conversation with them.

4. **DSM version/architecture matrix.** Entware itself has separate builds per architecture
   (x86_64, aarch64, armv7, etc. — see https://github.com/Entware/Entware/wiki/Install-on-Synology-NAS).
   A GUI wrapper needs to detect what's installed correctly and not assume a single arch.

## Distribution strategy (revised 2026-09-27)

Originally this doc suggested opening a SynoCommunity discussion *before* writing any code, to
ask "should this exist." On reflection that's the wrong order and the wrong question:
SynoCommunity's process (per their CONTRIBUTING/FAQ) is a gate for *inclusion in their official
repo*, not a gate for permission to build something. The DSM/Synology ecosystem has a long
history of independent, self-hosted third-party package repos (single-maintainer `.spk` feeds
added as a custom Package Center source) that never went through SynoCommunity at all —
see https://synopackage.com (a search engine/aggregator that indexes exactly these independent
repos, e.g. https://synopackage.com/sources for the list) and https://synocommunity.com itself
being just one such source among several DSM supports concurrently.

So the plan is: build and ship oPKGui as our own self-hosted Package Center source first — same
mechanism any independent Synology package maintainer uses, no third-party gatekeeping required.
Once it's real and working, *then* it's worth deciding whether to also pursue inclusion in
SynoCommunity's official repo (via their Discord https://discord.gg/nnN9fgE7EF or a `spksrc`
GitHub issue) — that's a distribution-reach decision made from a working artifact, not a
pre-approval step blocking day one.

## Real embedded DSM window: done (Round 9), lighter than first planned

A `~/Claude/DSMjs` side investigation (triggered by wanting oPKGui's icon to open as a real
DSM window instead of a new tab) found Synology's private `SYNO.SDS.*` ExtJS framework and,
initially, concluded it'd require a compiled `.so` webapi backend (three components: ExtJS
JS + a `.lib` API definition + a C/C++ `.so` wrapping `opkg`) — modeled on copying the real
Git app's files. That plan is **superseded**: once actual SynoCommunity ExtJS documentation
was found (a `spksrc` wiki page + a real template repo,
[`DigitalBox98/SimpleExtJSApp`](https://github.com/DigitalBox98/SimpleExtJSApp) — see
`DESIGN.md`'s Round 9), it turned out no compiled backend is needed at all: a real
`SYNO.SDS.AppWindow` can just `Ext.Ajax.request`/embed an iframe pointing at any URL,
including oPKGui's own existing Go HTTP service. Implemented as: a small `opkgui.js`
(`SYNOCOMMUNITY.OPKGui.*` namespace, extending `SYNO.SDS.AppInstance`/`AppWindow`) whose
window is just an iframe onto `http://<hostname>:18890/` — reusing the existing, already-
tested frontend unchanged, no new backend at all. See `pkgsrc/opkgui/src/ui/`.

Also worth correcting: the DSMjs doc's original claim that third-party `.so` calls happen
"without restriction or sandboxing" turned out to be incomplete — checking Synology's own
official headers (`webapi-DSM5/APIRequest.h`) showed auth is checked against the *logged-in
DSM session* (`IsAdmin()`, `GetLoginUID()`), not package identity. The earlier test succeeding
was explained by the tester already being an admin, not by an absence of access control.

## Non-goals for v0 (still true)

- Don't try to replace Entware's `opkg` semantics with something custom — just wrap it.
- Don't try to support every possible Entware package category (e.g. arbitrary daemons with init
  scripts) on day one — start with simple, stateless CLI tools.

## Related

- `DESIGN.md` — full round-by-round history, including Round 9 (the embedded window that
  actually worked) and the corrected DSMjs claim above.
- **DSMjs** (`~/Claude/DSMjs/AGENT-HANDOFF.md`) — the side investigation that surfaced the
  real ExtJS pattern. Its own conclusions evolved significantly over the investigation; read
  its top "Resolved" section, not its middle sections, for the current understanding.
- [[SynoVPNSocks]] — the project that surfaced this gap (`~/Claude/SynoVPNSocks`), not otherwise
  related in implementation.
