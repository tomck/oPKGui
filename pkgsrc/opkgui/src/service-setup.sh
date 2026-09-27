# oPKGui service setup
#
# The service itself runs unprivileged (see conf/privilege). It needs to
# read Entware's opkg.conf and package status file, which are 600 root:root
# by default -- opkg refuses to run at all otherwise, even for `list-installed`.
# Rather than loosening those files for every user on the box, grant group
# read access to this package's own dedicated service account only.

OPKGUI="${SYNOPKG_PKGDEST}/bin/opkgui"

SERVICE_COMMAND="${OPKGUI} -port ${SERVICE_PORT}"
SVC_BACKGROUND=y
SVC_WRITE_PID=y

ENTWARE_GRANT_FILES="/opt/etc/opkg.conf /opt/lib/opkg/status"

service_postinst()
{
    for f in ${ENTWARE_GRANT_FILES}; do
        if [ -e "${f}" ]; then
            chown root:sc-opkgui "${f}" 2>/dev/null
            chmod 640 "${f}" 2>/dev/null
        fi
    done
}

service_postuninst()
{
    for f in ${ENTWARE_GRANT_FILES}; do
        if [ -e "${f}" ]; then
            chown root:root "${f}" 2>/dev/null
            chmod 600 "${f}" 2>/dev/null
        fi
    done
}
