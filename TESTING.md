# Testing oPKGui v0

**Status: confirmed working end-to-end on the user's real NAS.** Self-contained Go service,
built through SynoCommunity's actual `spksrc` framework, permission grant done via DSM Task
Scheduler rather than from the package itself. See DESIGN.md's "Round 1–5" notes for the
full decision trail.

## Build

Requires Docker Desktop running, and a local Go toolchain (`go build` cross-compiles the
binary before handing off to `spksrc`).

```
./build.sh
```

First run clones `spksrc` (shallow, gitignored at `.spksrc/`) and pulls
`ghcr.io/synocommunity/spksrc`. Produces `dist/opkgui_noarch-dsm7_0.1.0-1.spk`. Override
`TCVERSION=7.2 ./build.sh` etc. if targeting a different DSM major/minor floor (default
`7.1`).

## Install

1. Copy the `.spk` from `dist/` to a machine that can reach your NAS's DSM web UI.
2. DSM → **Package Center** → **Manual Install** → browse to the `.spk` file.
3. DSM will likely warn it's from an unidentified publisher (expected — unsigned). Proceed
   anyway.
4. **Package Center → Installed** should show "oPKGui", running on port `18890` — try
   `http://<nas-ip>:18890/` directly. At this point both tabs will show a permission error
   (expected — see next step).
5. Set up the one-time permission grant via **Control Panel → Task Scheduler → Create →
   Triggered Task → User-defined script**:
   - General tab: Task name `oPKGui permissions`, User: **root**
   - Schedule tab: Event = **Boot-up**
   - Task Settings tab → Run command: paste the contents of
     `pkgsrc/opkgui/opkgui-grant-permissions.sh`
   - Save, then right-click the task and **Run** once immediately (don't wait for a reboot)
6. Refresh the page — both tabs should now show real data.

## What "success" looks like for v0

- Two tabs: **Installed** (all `opkg`-managed packages) and **Updates available**.
- If Entware isn't installed, or lives somewhere other than `/opt`, you should see a clean
  error message rather than a crash.
- `ls -la /opt/etc/opkg.conf /opt/lib/opkg/status` should show `640 root:opkgui` after the
  Task Scheduler grant runs (group is `opkgui`, **not** `sc-opkgui` — see round 5 below).

## Iteration log

- **v0, round 1 (hand-built `.spk`, abandoned):** installer refused with "Unable to install
  because it runs with root privileges" — missing `conf/privilege`.
- **v0, round 2 (spksrc-built, PHP+WebStation, abandoned):** build succeeded after fixing a
  dropped `DSM_UI_DIR`/`DSM_UI_CONFIG`. Never install-tested — superseded before that step
  by the WebStation-isn't-installed finding (DESIGN.md Round 2).
- **v0, round 3:** wrong `TCVERSION` (defaulted to 7.2; test NAS is DSM 7.1.1) — fixed. Live
  SSH testing then found (a) no WebStation, (b) `opkg.conf`/`status` are `600 root:root`,
  unreadable even for read-only listing. Rebuilt as a self-contained Go service with a
  `postinst` permission grant (DESIGN.md Rounds 2–3).
- **v0, round 4:** tried real arch metadata (`ARCH=x64`), hit a Docker-Desktop/macOS
  toolchain-extraction bug plus low disk space; reverted to `ARCH=noarch` with the caveat
  documented in DESIGN.md.
- **v0, round 5:** the `postinst`-root design from round 3 kept failing with the same
  "runs with root privileges" error even with a correct `conf/privilege` — DSM 7 turns out
  to block root `ctrl-script` requests entirely for unsigned/sideloaded packages (it's only
  honored for SynoCommunity's own signed repo installs; DSM 6's "Trust Level" bypass for
  this was removed in DSM 7). Confirmed by testing a fully unprivileged build, which
  installed and ran cleanly. Moved the permission grant out of the package entirely, onto a
  DSM Task Scheduler root script — the same mechanism Entware itself is bootstrapped with
  (user's suggestion, following Entware's own install wiki). Hit one gotcha getting it
  working: the service account is `sc-opkgui` but its **group** is `opkgui` (no `sc-`
  prefix) — `chown root:sc-opkgui` silently failed while a same-script `chmod 640` still
  succeeded, briefly looking like a partial fix. Fixed the group name and confirmed both
  tabs populate correctly on the real NAS.

## What's next (v1)

- The install/remove privilege boundary — still the one genuinely open, harder problem;
  the Task Scheduler pattern used here for a one-time permission grant doesn't obviously
  extend to arbitrary mutating `opkg` commands from a web request.
- Packaging this as a self-hosted Package Center source (a JSON feed + hosted `.spk`) for
  reinstall/updates without manual sideload.
- The `noarch` architecture-metadata gap (DESIGN.md Round 4) before any distribution beyond
  the user's own NAS.
