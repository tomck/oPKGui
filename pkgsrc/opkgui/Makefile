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
# Round 9: a real embedded DSM desktop window, via SynoCommunity's actual
# documented ExtJS pattern (SYNOCOMMUNITY.<App>.* extending SYNO.SDS.
# AppInstance/AppWindow -- found via docs/templates search, not guessed;
# see DESIGN.md's Round 9). Three earlier guesses (SERVICE_TYPE=legacy, a
# hand-authored SYNO.SDS._ThirdParty.App.* config, and a webman/3rdparty
# application.cfg) all failed because none of them shipped real JS at all.
DSM_UI_CONFIG = src/ui/config

COPY_TARGET = opkgui_install

include ../../mk/spksrc.spk.mk

.PHONY: opkgui_install
opkgui_install:
	@$(MSG) Install the opkgui binary
	@install -d -m 755 $(STAGING_DIR)/bin
	@install -m 755 src/bin/opkgui-$(ARCH) $(STAGING_DIR)/bin/opkgui
	@$(MSG) Install the ExtJS desktop window
	@install -d -m 755 $(STAGING_DIR)/app
	@install -m 644 src/ui/opkgui.js $(STAGING_DIR)/app/opkgui.js
