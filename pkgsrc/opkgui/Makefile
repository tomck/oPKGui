SPK_NAME = opkgui
SPK_VERS = 0.1.0
SPK_REV = 1
SPK_ICON = src/opkgui.png

DEPENDS =

# Targeting DSM 7 only for v0.
REQUIRED_MIN_SRM = 3.0

override ARCH=noarch

MAINTAINER = tomkoch
DESCRIPTION = Read-only browser for installed Entware (opkg) packages and available updates.
DISPLAY_NAME = oPKGui
CHANGELOG = "v0: read-only package list, no install/remove yet"

LICENSE = MIT

include ../../mk/spksrc.common.mk
ifeq ($(call version_ge, ${TCVERSION}, 7.0),1)
PHP_VERSION = 8.2
endif

# DSM 7 dependencies are declared in conf/resource instead (see CONF_DIR below).
ifeq ($(call version_ge, ${TCVERSION}, 7.0),1)
SPK_DEPENDS = WebStation:PHP$(PHP_VERSION):Apache2.4
endif

# DSM_UI_DIR/DSM_UI_CONFIG must stay set even though NO_SERVICE_SHORTCUT
# suppresses the app/config shortcut on DSM 7 -- spksrc's icon.mk only
# generates app/images/* icons when both are non-empty.
DSM_UI_DIR = app
DSM_UI_CONFIG = src/app/config

ifeq ($(call version_ge, ${TCVERSION}, 7.0),1)
CONF_DIR = src/conf_php$(PHP_VERSION)/
NO_SERVICE_SHORTCUT = y
SERVICE_PORT = 18890
endif

SERVICE_SETUP = src/service-setup.sh
STARTABLE = no

COPY_TARGET = opkgui_install
POST_ICON_TARGET = opkgui_post_icon

include ../../mk/spksrc.spk.mk

.PHONY: opkgui_install
opkgui_install:
	@$(MSG) Install the web application
	@install -d -m 755 $(STAGING_DIR)/web/$(SPK_NAME)/
	@install -m 755 src/web/index.php $(STAGING_DIR)/web/$(SPK_NAME)/
	@install -m 755 src/web/api.php $(STAGING_DIR)/web/$(SPK_NAME)/

.PHONY: opkgui_post_icon
opkgui_post_icon:
	@$(MSG) Add package icons to the web application
	@install -d -m 755 $(STAGING_DIR)/web/$(SPK_NAME)/images
	@for image_size in 16 32; do \
	  install -m 755 $(STAGING_DIR)/app/images/$(SPK_NAME)-$${image_size}.png $(STAGING_DIR)/web/$(SPK_NAME)/images/ ; \
	done
