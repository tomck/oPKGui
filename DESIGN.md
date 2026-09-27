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

`opkg list-installed` and `opkg list-upgradable` are **read-only** operations — they just
read Entware's package status/lists files under `/opt/var/opkg/`, which are world-readable
by default (standard Entware install, root-owned files at typical `644`/`755` perms). That
means v0 needs **no privilege boundary at all**: it can run entirely as the unprivileged
`sc-opkgui` account DSM provisions automatically, with zero elevated access.

This lets v0 sidestep the one open, hard design problem (safely brokering *mutating*
`opkg install/remove` commands from an unprivileged web process to root) entirely, and defer
it to v1. If it turns out some Entware install has tighter permissions than expected, the
fallback is a tiny root-owned read-only helper — still much simpler than the install/remove
case — but the default assumption is that no helper is needed yet. **This assumption gets
verified during real testing on the user's NAS, not before.**

## v0 architecture

- **Package type:** DSM 7 web-type SPK (`ARCH=noarch` — pure PHP + static assets, no
  compiled binaries, no cross-toolchain needed).
- **Backend:** a single PHP endpoint (`api.php`) that runs exactly two fixed, hardcoded
  shell commands — `opkg list-installed` and `opkg list-upgradable` — via `shell_exec()`
  with **no user-supplied input anywhere in the command string** (v0 takes no parameters,
  so there is no injection surface to defend by construction, not by sanitization).
- **Frontend:** one static HTML/JS page (`index.php` per the demowebservice convention,
  so DSM's variable substitution still applies) that fetches `api.php` and renders two
  tables: installed packages, and packages with available updates. No install/remove
  buttons in v0.
- **Packaging:** built through SynoCommunity's actual `spksrc` framework (via their
  published `ghcr.io/synocommunity/spksrc` Docker image), not hand-assembled. `pkgsrc/opkgui/`
  holds the package source (a `Makefile` modeled directly on `spksrc`'s own
  `demowebservice` reference package); `build.sh` clones `spksrc` into a gitignored
  `.spksrc/`, syncs `pkgsrc/opkgui` into its `spk/opkgui`, and runs `make` inside the
  container. This is a course-correction from the original v0 approach — see the
  round-1 note below.
- **Testing loop:** manual sideload via DSM's Package Center → Manual Install, pointing at
  the built `.spk` file in `dist/`. No package-source hosting needed yet for this stage.

## Round 1 correction: use the framework, not a hand-rolled tar

The first v0 attempt hand-assembled the `.spk` tar structure directly instead of using
`spksrc`, reasoning that a pure-`noarch`-PHP package didn't need the cross-compilation
toolchain. That reasoning wasn't wrong on its own terms, but it repeated exactly the
mistake this project already knows to avoid elsewhere ([[SynoVPNSocks]] uses Entware/`opkg`
instead of reinventing package management) — `spksrc` isn't just a cross-compiler, it's
accumulated knowledge of DSM packaging pitfalls (like the `conf/privilege` requirement that
blocked the first hand-built attempt outright: "Unable to install because it runs with root
privileges"). Distribution strategy (self-host vs. submit to SynoCommunity) and build
tooling (use `spksrc`'s Makefiles vs. not) are separate decisions — using the framework
locally to produce a `.spk` doesn't commit us to ever submitting it upstream. Switched to
building through the real framework from this point on.

## Non-goals for v0 (unchanged from the handoff doc)

- No install/remove/upgrade actions.
- No architecture-detection logic beyond "does `/opt/bin/opkg` exist."
