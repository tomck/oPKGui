#!/bin/sh
# oPKGui: grant its unprivileged service account (sc-opkgui) exactly the
# elevated access it needs, since DSM 7 won't let the .spk itself request
# any root privilege (see DESIGN.md's Round 5).
#
# Installed the same way Entware itself is bootstrapped on DSM (see
# https://github.com/entware/entware/wiki/Install-on-Synology-NAS):
# Control Panel > Task Scheduler > Create > Triggered Task > User-defined
# script, User: root, Event: Boot-up. Paste this script in as the task's
# "Run command" (or point it at wherever this file lives on your NAS).
#
# Safe to re-run.

# 1. Read access to opkg.conf and the package status file (both 600
#    root:root by default -- opkg refuses to run at all otherwise, even
#    for read-only listing).
for f in /opt/etc/opkg.conf /opt/lib/opkg/status; do
    if [ -e "${f}" ]; then
        # Note: DSM names the account "sc-opkgui" but its group "opkgui"
        # (no sc- prefix) -- confirmed via `id sc-opkgui` on the test NAS.
        chown root:opkgui "${f}"
        chmod 640 "${f}"
    fi
done

# 2. A narrow sudo grant so the (still fully unprivileged) service can run
#    exactly opkg install/remove/upgrade as root -- nothing else. The Go
#    binary itself validates package names before this ever runs (see
#    main.go's packageNameRE + isKnownPackage), so this is defense in
#    depth, not the only check.
cat > /etc/sudoers.d/opkgui <<'EOF'
sc-opkgui ALL=(root) NOPASSWD: /opt/bin/opkg install *
sc-opkgui ALL=(root) NOPASSWD: /opt/bin/opkg remove *
sc-opkgui ALL=(root) NOPASSWD: /opt/bin/opkg upgrade *
Defaults!/opt/bin/opkg !requiretty
EOF
chown root:root /etc/sudoers.d/opkgui
chmod 440 /etc/sudoers.d/opkgui
