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
DESCRIPTION = Browse, install, remove and upgrade Entware (opkg) packages.
DISPLAY_NAME = oPKGui
CHANGELOG = "v1: install/remove/upgrade, system-command-conflict warnings"

LICENSE = MIT

CONF_DIR = src/conf/

STARTABLE = yes
SERVICE_USER = auto
SERVICE_SETUP = src/service-setup.sh
SERVICE_PORT = 18890
ADMIN_PORT = $(SERVICE_PORT)
SERVICE_PORT_TITLE = oPKGui (HTTP)
# Round 6 tried getting a true embedded DSM desktop window
# (SERVICE_TYPE=legacy, and a hand-authored SYNO.SDS._ThirdParty.App.* +
# appWindow config) -- neither worked cleanly. Round 8 is trying the older
# webman/3rdparty application.cfg mechanism (type=embedded) instead -- see
# DESIGN.md's "DSM desktop window" note for both.

COPY_TARGET = opkgui_install

include ../../mk/spksrc.spk.mk

.PHONY: opkgui_install
opkgui_install:
	@$(MSG) Install the opkgui binary
	@install -d -m 755 $(STAGING_DIR)/bin
	@install -m 755 src/bin/opkgui-$(ARCH) $(STAGING_DIR)/bin/opkgui
	@$(MSG) Install application.cfg for the webman/3rdparty embedded-window experiment
	@install -d -m 755 $(STAGING_DIR)/app
	@install -m 644 src/application.cfg $(STAGING_DIR)/app/application.cfg
