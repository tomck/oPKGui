# Testing oPKGui v0

This is a first-pass, hand-built `.spk` — expect at least one round of fixes based on
whatever DSM's package installer actually complains about. That feedback loop is the point:
we don't have a way to validate SPK internals other than a real DSM 7 install.

## Build

```
./build-spk.sh
```

Produces `opkgui-0.1.0-1.spk` in the repo root (gitignored — it's a build artifact, rebuild
any time from `spk/`).

## Install

1. Copy the `.spk` to a machine that can reach your NAS's DSM web UI (or straight onto the
   NAS).
2. DSM → **Package Center** → the **Manual Install** option (⚙ icon top-right, or a button
   depending on DSM version) → browse to `opkgui-0.1.0-1.spk`.
3. DSM will likely warn it's from an unidentified publisher (expected — it's unsigned and
   not from a trusted source yet). Proceed anyway.
4. If it installs: **Package Center → Installed** should show "oPKGui". It's a `webportal`
   type package on port `18890`, so try `http://<nas-ip>:18890/` directly.

## What "success" looks like for v0

- Two tabs: **Installed** (all `opkg`-managed packages) and **Updates available**.
- If Entware isn't installed, or lives somewhere other than `/opt`, you should see a clean
  error message (`"opkg not found at /opt/bin/opkg"`) rather than a crash — that's the
  `is_executable()` check in `api.php`.

## Things likely to go wrong (and what they'd tell us)

- **Install fails outright with an INFO/manifest error.** DSM's error message will usually
  name the bad field — send it back and I'll fix the `INFO` file. The exact required/allowed
  `INFO` keys for DSM 7 weren't independently verified against Synology's own spec, just
  inferred from `spksrc`'s generated output for a comparable package.
- **Installs, but no `sc-opkgui` account gets created, or the web portal 404s / won't bind
  port 18890.** That'd mean something in `conf/resource`'s `webservice` block is off (the
  `backend` numeric codes were copied from `spksrc`'s `demowebservice` example rather than
  from documented DSM constants — they should be right since PHP versions and web-server
  type match, but this is the least-verified part of the whole package).
- **Page loads, but tables show a permissions/exec error from `api.php`.** That would mean
  the assumption that `/opt/var/opkg/*` is readable by an unprivileged `sc-opkgui` account
  doesn't hold on your NAS — worth checking with `ls -la /opt/var/opkg/` over SSH as a
  sanity check if this happens. If so, v0 needs a minimal privileged read-only helper after
  all (see DESIGN.md's fallback note), which is still a much smaller problem than the
  install/remove privilege design deferred to v1.
- **Everything works.** Then the two real "v1" design questions become live: the
  install/remove privilege boundary (flagged in `AGENT-HANDOFF.md` and `DESIGN.md` as the
  one genuinely open problem `spksrc` gives no pattern for), and packaging this as a
  self-hosted Package Center source for easier reinstall/updates instead of manual sideload.

## Iteration log

- **v0, round 1:** installer refused with "Unable to install because it runs with root
  privileges." Cause: DSM 7 requires an explicit `conf/privilege` declaration that a package
  doesn't need root, normally injected automatically by `spksrc`'s build framework — since
  this SPK is hand-built rather than built via `spksrc`, that default never got added. Fixed
  by adding `spk/conf/privilege` with `{"defaults": {"run-as": "package"}}`. Rebuilt; not yet
  retested.

Report back whatever DSM actually says/does and we'll iterate from there.
