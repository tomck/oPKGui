# Testing oPKGui

**v0 and v1 both confirmed working end-to-end on the user's real NAS**, including a real
install (`opkg install cal`) through the web UI. Self-contained Go service, built through
SynoCommunity's actual `spksrc` framework, all privilege grants done via DSM Task Scheduler
rather than from the package itself. See DESIGN.md's "Round 1–7" notes for the full decision
trail.

**Task Scheduler's "Run command" box handles multi-line scripts fine** — paste the whole
script, comments and all, directly into it. (A round of confusion happened here purely
because of a tool-display miss on the agent's side, not any real limitation of that field —
see the v1 log entry below. No indirection through a separate script file is needed.)

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
5. Set up (or update) the permission grant via **Control Panel → Task Scheduler**:
   - If you already have the `oPKGui permissions` task from v0: edit it, replace the Run
     command with the current contents of `pkgsrc/opkgui/opkgui-grant-permissions.sh`
     (v1 adds a `sudoers.d` rule to the same script), save, right-click → **Run**.
   - If not: **Create → Triggered Task → User-defined script** — General tab: Task name
     `oPKGui permissions`, User: **root**; Schedule tab: Event = **Boot-up**; Task Settings
     tab → Run command: paste the script contents. Save, then right-click → **Run** once
     immediately (don't wait for a reboot).
   - **This edits `/etc/sudoers.d/opkgui` system-wide.** Right after running it, sanity-check
     with `sudo -l` (as your own admin user, over SSH) that sudo still works normally at all
     — a malformed sudoers file can break sudo entirely. Rollback if anything looks wrong:
     `rm -f /etc/sudoers.d/opkgui` (as root).
6. Refresh the page — all three tabs should now show real data, and Install/Remove/Upgrade
   buttons should work.

## What "success" looks like

- Three tabs: **Installed** (with Remove buttons), **Updates available** (with Upgrade
  buttons), **Available** (all Entware packages, with a name filter and Install buttons).
- If Entware isn't installed, or lives somewhere other than `/opt`, you should see a clean
  error message rather than a crash.
- `ls -la /opt/etc/opkg.conf /opt/lib/opkg/status` should show `640 root:opkgui` after the
  Task Scheduler grant runs (group is `opkgui`, **not** `sc-opkgui` — see round 5 below).
- Installing/removing/upgrading a package via the buttons should actually change what
  `opkg list-installed` reports, reflected in the Installed tab after the action completes.

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

- **v0 → v1:** Added install/remove/upgrade. The privilege boundary uses the same
  Task-Scheduler-as-root pattern as the read-only permission grant, extended with a narrow
  `sudoers.d` rule (`sc-opkgui` may run exactly `opkg install/remove/upgrade <arg>` as root,
  nothing else) instead of trying to get the package itself any privilege — DSM 7 already
  established (round 5) that it won't allow that. Package names are validated twice before
  ever reaching `sudo`: a regex, then cross-checked against opkg's own current listing.
- **v1, first real test:** hit a lock-contention bug first (all three tabs fetching
  concurrently on page load raced for `opkg`'s own lock file, ~2/3 failing with a 255) —
  fixed with a mutex serializing every `opkg`/`sudo` call in the Go server. Then the actual
  sudoers grant didn't take effect (`sudo: a password is required` trying to install `cal`).
  Turned out the agent had called a tool to `cat` the script into chat rather than writing
  its content directly into the reply, so nothing was ever actually shown to copy — but this
  got misdiagnosed as a DSM Task Scheduler UI limitation (multi-line paste not working in
  the "Run command" field) instead of being recognized as the agent's own display miss,
  leading to an unnecessary workaround (writing the script to a file on the NAS and pointing
  Task Scheduler at it with `sh /path/to/script.sh`) before the user corrected this directly:
  **the Run command field handles a full multi-line script pasted in as-is, no indirection
  needed.** Once the actual script was in there, `opkg install cal` worked immediately —
  confirmed via SSH (`/opt/bin/cal` present) and in the web UI's Installed tab.

## What's next (v2?)

- Packaging this as a self-hosted Package Center source (a JSON feed + hosted `.spk`) for
  reinstall/updates without manual sideload.
- The `noarch` architecture-metadata gap (DESIGN.md Round 4) before any distribution beyond
  the user's own NAS.
