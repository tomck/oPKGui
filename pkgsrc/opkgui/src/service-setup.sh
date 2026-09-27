# oPKGui service setup
#
# Runs fully unprivileged (conf/privilege: run-as package, no ctrl-script
# overrides) -- DSM 7 blocks any Package Center-installed package that
# requests root for any script when it isn't from a signed/trusted
# publisher. The read-access grant this service needs onto Entware's
# opkg.conf/status (both 600 root:root by default) is done separately, via
# a DSM Task Scheduler root script -- see opkgui-grant-permissions.sh.

OPKGUI="${SYNOPKG_PKGDEST}/bin/opkgui"

SERVICE_COMMAND="${OPKGUI} -port ${SERVICE_PORT}"
SVC_BACKGROUND=y
SVC_WRITE_PID=y
