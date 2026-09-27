# Testing oPKGui v0

Self-contained Go service, built through SynoCommunity's actual `spksrc` framework. See
DESIGN.md's "Round 1–4" notes for how it got to this shape.

## Build

Requires Docker Desktop running, and a local Go toolchain (`go build` cross-compiles the
binary before handing off to `spksrc`).

```
./build.sh
```

First run clones `spksrc` (shallow, gitignored at `.spksrc/`) and pulls
`ghcr.io/synocommunity/spksrc`. Produces `dist/opkgui_noarch-dsm7_0.1.0-1.spk`. Override
`TCVERSION=7.2 ./build.sh` etc. if targeting a different DSM major/minor floor (default
`7.1`, matching the test NAS).

## Install

1. Copy the `.spk` from `dist/` to a machine that can reach your NAS's DSM web UI.
2. DSM → **Package Center** → **Manual Install** → browse to the `.spk` file.
3. DSM will likely warn it's from an unidentified publisher (expected — unsigned). Proceed
   anyway.
4. If it installs: **Package Center → Installed** should show "oPKGui", running on port
   `18890` — try `http://<nas-ip>:18890/` directly.

## What "success" looks like for v0

- Two tabs: **Installed** (all `opkg`-managed packages) and **Updates available**.
- If Entware isn't installed, or lives somewhere other than `/opt`, you should see a clean
  error message rather than a crash.
- If everything installed correctly, `sc-opkgui` should now have group-read access to
  `/opt/etc/opkg.conf` and `/opt/lib/opkg/status` (verify with `ls -la` over SSH if curious)
  — that's the `postinst` permission grant, not something Entware does on its own.

## Things likely to go wrong (and what they'd tell us)

- **Service doesn't start / port 18890 doesn't respond.** Check DSM's package log
  (Package Center → Installed → oPKGui → Log, or `/var/log/synopkg.log` over SSH) for what
  `start-stop-status` reported. `SERVICE_COMMAND` in `service-setup.sh` is the first thing
  to sanity-check.
- **Tables show a permission/exec error from `/api`.** Means the `postinst` `chown`/`chmod`
  grant didn't take (check `ls -la /opt/etc/opkg.conf /opt/lib/opkg/status` over SSH —
  should show group `sc-opkgui`, mode `640`). This is the least-tested part of the package;
  the chown/chmod commands themselves were never dry-run against the real box beforehand
  (only confirmed `chown`/`chmod` binaries exist, not that a root-context DSM ctrl-script
  invocation of them behaves identically to running them by hand over SSH).
- **Package installs on the wrong architecture entirely.** Known gap — see DESIGN.md's
  "Round 4" noarch caveat. Not expected to bite on the user's own NAS (confirmed x86_64),
  but flagging so it's not mistaken for a surprise later.
- **Everything works.** Then the two real "v1" design questions become live: the
  install/remove privilege boundary, and packaging this as a self-hosted Package Center
  source for easier reinstall/updates instead of manual sideload.

## Iteration log

- **v0, round 1 (hand-built `.spk`, abandoned):** installer refused with "Unable to install
  because it runs with root privileges" — missing `conf/privilege`. Root-caused only
  partially; see round 2 below for why this whole approach was replaced rather than just
  patched.
- **v0, round 2 (spksrc-built, PHP+WebStation, abandoned):** build succeeded after fixing a
  dropped `DSM_UI_DIR`/`DSM_UI_CONFIG` (needed for `spksrc`'s icon generation regardless of
  the app-shortcut feature they otherwise control). Never install-tested — superseded before
  that step by the WebStation-isn't-installed finding (DESIGN.md Round 2).
- **v0, round 3:** wrong `TCVERSION` (defaulted to 7.2; test NAS is DSM 7.1.1) — fixed.
  Live SSH testing against the real NAS then found (a) no WebStation, (b) `opkg.conf` and
  `status` are `600 root:root`, unreadable even for read-only listing. Rebuilt as a
  self-contained Go service with a `postinst` permission grant (DESIGN.md Rounds 2–3).
  Cross-compiled binary was tested directly on the NAS over SSH (outside the SPK) and
  correctly reproduced the permission error with a clean JSON response, confirming the Go
  server logic works before wrapping it in a package.
- **v0, round 4:** tried real arch metadata (`ARCH=x64`), hit a Docker-Desktop/macOS
  toolchain-extraction bug plus low disk space; reverted to `ARCH=noarch` with the caveat
  documented in DESIGN.md. Current `dist/opkgui_noarch-dsm7_0.1.0-1.spk` not yet installed
  on real hardware.

Report back whatever DSM actually says/does and we'll iterate from there.
