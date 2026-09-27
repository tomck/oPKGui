SPK_NAME = opkgui
SPK_VERS = 0.1.0
SPK_REV = 1
SPK_ICON = src/opkgui.png

DEPENDS =

# Ships a prebuilt static Go binary (cross-compiled outside spksrc, see
# build.sh) rather than compiling via spksrc's own toolchain, so no real
# ARCH/toolchain is needed. This does mean DSM won't block installing on a
# non-x86_64 model at the Package Center level -- see DESIGN.md's "noarch
# caveat" note. Revisit before any wider distribution.
override ARCH=noarch

MAINTAINER = tomkoch
DESCRIPTION = Read-only browser for installed Entware (opkg) packages and available updates.
DISPLAY_NAME = oPKGui
CHANGELOG = "v0: read-only package list, self-contained service (no WebStation dependency)"

LICENSE = MIT

CONF_DIR = src/conf/

STARTABLE = yes
SERVICE_USER = auto
SERVICE_SETUP = src/service-setup.sh
SERVICE_PORT = 18890
ADMIN_PORT = $(SERVICE_PORT)
SERVICE_PORT_TITLE = oPKGui (HTTP)

COPY_TARGET = opkgui_install

include ../../mk/spksrc.spk.mk

.PHONY: opkgui_install
opkgui_install:
	@$(MSG) Install the opkgui binary
	@install -d -m 755 $(STAGING_DIR)/bin
	@install -m 755 src/bin/opkgui-$(ARCH) $(STAGING_DIR)/bin/opkgui
