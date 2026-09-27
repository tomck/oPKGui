# Testing oPKGui v0

Built through SynoCommunity's actual `spksrc` framework now (see DESIGN.md's "round 1
correction" section for why that replaced an initial hand-rolled `.spk`). Still expect
iteration — this is the package's first real run through the framework and hasn't touched a
live DSM 7 installer yet.

## Build

Requires Docker Desktop running.

```
./build.sh
```

First run clones `spksrc` (shallow, gitignored at `.spksrc/`) and pulls
`ghcr.io/synocommunity/spksrc`, which is a few hundred MB. Produces
`dist/opkgui_noarch-dsm72_0.1.0-1.spk` (exact filename depends on `TCVERSION`, default
`7.2` — override with `TCVERSION=7.0 ./build.sh` if your DSM is older).

## Install

1. Copy the `.spk` from `dist/` to a machine that can reach your NAS's DSM web UI.
2. DSM → **Package Center** → **Manual Install** → browse to the `.spk` file.
3. DSM will likely warn it's from an unidentified publisher (expected — unsigned, not from
   a trusted source yet). Proceed anyway.
4. If it installs: **Package Center → Installed** should show "oPKGui". It's a port-based
   web portal on `18890`, so try `http://<nas-ip>:18890/` directly.

## What "success" looks like for v0

- Two tabs: **Installed** (all `opkg`-managed packages) and **Updates available**.
- If Entware isn't installed, or lives somewhere other than `/opt`, you should see a clean
  error message (`"opkg not found at /opt/bin/opkg"`) rather than a crash — that's the
  `is_executable()` check in `api.php`.

## Things likely to go wrong (and what they'd tell us)

- **Install fails outright with an INFO/manifest error.** `spksrc`'s own Makefile system
  generates `INFO` for us now, so this is less likely than round 1, but the custom bits
  (`SPK_DEPENDS`, `CONF_DIR` version-gating) are still hand-adapted from `demowebservice` and
  could be off.
- **Installs, but no `sc-opkgui` account gets created, or the web portal 404s / won't bind
  port 18890.** That'd mean something in `conf_php8.2/resource`'s `webservice` block is off
  (the `backend` numeric codes were copied from `demowebservice`'s example rather than from
  documented DSM constants — should be right since PHP version and web-server type match,
  but this is the least-verified part of the whole package).
- **Page loads, but tables show a permissions/exec error from `api.php`.** That would mean
  the assumption that `/opt/var/opkg/*` is readable by an unprivileged `sc-opkgui` account
  doesn't hold on your NAS — worth checking with `ls -la /opt/var/opkg/` over SSH as a
  sanity check if this happens. If so, v0 needs a minimal privileged read-only helper after
  all (see DESIGN.md's fallback note), still much simpler than the install/remove privilege
  design deferred to v1.
- **Everything works.** Then the two real "v1" design questions become live: the
  install/remove privilege boundary (the one genuinely open problem `spksrc` gives no
  pattern for), and packaging this as a self-hosted Package Center source for easier
  reinstall/updates instead of manual sideload.

## Iteration log

- **v0, round 1 (hand-built `.spk`, since abandoned):** installer refused with "Unable to
  install because it runs with root privileges." Added `conf/privilege` with
  `{"defaults": {"run-as": "package"}}` per Synology's dev docs as a fix — but checking
  `spksrc`'s own framework afterward showed its reference web package (`demowebservice`)
  ships with **no** `conf/privilege` file at all and presumably still installs, so the true
  root cause of round 1's error was never fully confirmed. Rather than keep guessing at
  hand-rolled SPK internals, switched to building through the actual `spksrc` framework
  (this file's current state) so packaging follows a tested pattern instead of one
  reconstructed from documentation alone. The privilege file is kept in the new package too
  (`pkgsrc/opkgui/src/conf_php8.2/privilege`) since it's harmless and documented, just no
  longer assumed to be the whole fix.
- **v0, round 2 (spksrc-built):** not yet tested on real hardware.

Report back whatever DSM actually says/does and we'll iterate from there.
