#!/bin/sh
# oPKGui: grant its unprivileged service account (sc-opkgui) read-only
# access to Entware's opkg.conf and package status file, both 600 root:root
# by default -- opkg refuses to run at all otherwise, even for read-only
# listing.
#
# Installed the same way Entware itself is bootstrapped on DSM (see
# https://github.com/entware/entware/wiki/Install-on-Synology-NAS):
# Control Panel > Task Scheduler > Create > Triggered Task > User-defined
# script, User: root, Event: Boot-up. Paste this script in as the task's
# "Run command" (or point it at wherever this file lives on your NAS).
#
# Safe to re-run: skips files that don't exist or are already granted.

for f in /opt/etc/opkg.conf /opt/lib/opkg/status; do
    if [ -e "${f}" ]; then
        # Note: DSM names the account "sc-opkgui" but its group "opkgui"
        # (no sc- prefix) -- confirmed via `id sc-opkgui` on the test NAS.
        chown root:opkgui "${f}"
        chmod 640 "${f}"
    fi
done
