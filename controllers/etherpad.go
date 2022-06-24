// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the etherpad configuration.
package controllers

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const (
	etherpadSettingsTemplate = `
{
  "title": "${TITLE:SF - Etherpad}",
  "favicon": "${FAVICON:null}",
  "skinName": "${SKIN_NAME:colibris}",
  "skinVariants": "${SKIN_VARIANTS:super-light-toolbar super-light-editor light-background}",
  "ip": "${IP:0.0.0.0}",
  "port": "${PORT:8080}",
  "showSettingsInAdminPage": "${SHOW_SETTINGS_IN_ADMIN_PAGE:true}",
  "dbType": "${DB_TYPE:mysql}",
  "dbSettings": {
    "host":     "${DB_HOST:mariadb}",
    "port":     "${DB_PORT:3306}",
    "database": "${DB_NAME:etherpad}",
    "user":     "${DB_USER:etherpad}",
    "password": "${DB_PASS:%s}",
    "charset":  "${DB_CHARSET:undefined}",
    "filename": "${DB_FILENAME:var/dirty.db}",
    "collection": "${DB_COLLECTION:undefined}",
    "url":      "${DB_URL:undefined}"
  },
  "users": {
    "admin": {
      "password": "${ADMIN_PASSWORD:%s}",
      "is_admin": true
    },
  },

  "defaultPadText" : "${DEFAULT_PAD_TEXT:Welcome to Software Factory Etherpad!\n\nThis pad text is synchronized as you type, so that everyone viewing this page sees the same text. This allows you to collaborate seamlessly on documents!\n\nGet involved with Etherpad at https:\/\/etherpad.org\n}",

  "padOptions": {
    "noColors":         "${PAD_OPTIONS_NO_COLORS:false}",
    "showControls":     "${PAD_OPTIONS_SHOW_CONTROLS:true}",
    "showChat":         "${PAD_OPTIONS_SHOW_CHAT:true}",
    "showLineNumbers":  "${PAD_OPTIONS_SHOW_LINE_NUMBERS:true}",
    "useMonospaceFont": "${PAD_OPTIONS_USE_MONOSPACE_FONT:false}",
    "userName":         "${PAD_OPTIONS_USER_NAME:false}",
    "userColor":        "${PAD_OPTIONS_USER_COLOR:false}",
    "rtl":              "${PAD_OPTIONS_RTL:false}",
    "alwaysShowChat":   "${PAD_OPTIONS_ALWAYS_SHOW_CHAT:false}",
    "chatAndUsers":     "${PAD_OPTIONS_CHAT_AND_USERS:false}",
    "lang":             "${PAD_OPTIONS_LANG:en-gb}"
  },
  "padShortcutEnabled" : {
    "altF9":     "${PAD_SHORTCUTS_ENABLED_ALT_F9:true}",      /* focus on the File Menu and/or editbar */
    "altC":      "${PAD_SHORTCUTS_ENABLED_ALT_C:true}",       /* focus on the Chat window */
    "cmdShift2": "${PAD_SHORTCUTS_ENABLED_CMD_SHIFT_2:true}", /* shows a gritter popup showing a line author */
    "delete":    "${PAD_SHORTCUTS_ENABLED_DELETE:true}",
    "return":    "${PAD_SHORTCUTS_ENABLED_RETURN:true}",
    "esc":       "${PAD_SHORTCUTS_ENABLED_ESC:true}",         /* in mozilla versions 14-19 avoid reconnecting pad */
    "cmdS":      "${PAD_SHORTCUTS_ENABLED_CMD_S:true}",       /* save a revision */
    "tab":       "${PAD_SHORTCUTS_ENABLED_TAB:true}",         /* indent */
    "cmdZ":      "${PAD_SHORTCUTS_ENABLED_CMD_Z:true}",       /* undo/redo */
    "cmdY":      "${PAD_SHORTCUTS_ENABLED_CMD_Y:true}",       /* redo */
    "cmdI":      "${PAD_SHORTCUTS_ENABLED_CMD_I:true}",       /* italic */
    "cmdB":      "${PAD_SHORTCUTS_ENABLED_CMD_B:true}",       /* bold */
    "cmdU":      "${PAD_SHORTCUTS_ENABLED_CMD_U:true}",       /* underline */
    "cmd5":      "${PAD_SHORTCUTS_ENABLED_CMD_5:true}",       /* strike through */
    "cmdShiftL": "${PAD_SHORTCUTS_ENABLED_CMD_SHIFT_L:true}", /* unordered list */
    "cmdShiftN": "${PAD_SHORTCUTS_ENABLED_CMD_SHIFT_N:true}", /* ordered list */
    "cmdShift1": "${PAD_SHORTCUTS_ENABLED_CMD_SHIFT_1:true}", /* ordered list */
    "cmdShiftC": "${PAD_SHORTCUTS_ENABLED_CMD_SHIFT_C:true}", /* clear authorship */
    "cmdH":      "${PAD_SHORTCUTS_ENABLED_CMD_H:true}",       /* backspace */
    "ctrlHome":  "${PAD_SHORTCUTS_ENABLED_CTRL_HOME:true}",   /* scroll to top of pad */
    "pageUp":    "${PAD_SHORTCUTS_ENABLED_PAGE_UP:true}",
    "pageDown":  "${PAD_SHORTCUTS_ENABLED_PAGE_DOWN:true}"
  },
  "suppressErrorsInPadText": "${SUPPRESS_ERRORS_IN_PAD_TEXT:false}",
  "requireSession": "${REQUIRE_SESSION:false}",
  "editOnly": "${EDIT_ONLY:false}",
  "minify": "${MINIFY:true}",
  "maxAge": "${MAX_AGE:21600}", // 60 * 60 * 6 = 6 hours
  "abiword": "${ABIWORD:null}",
  "soffice": "${SOFFICE:null}",
  "tidyHtml": "${TIDY_HTML:null}",
  "allowUnknownFileEnds": "${ALLOW_UNKNOWN_FILE_ENDS:true}",
  "requireAuthentication": "${REQUIRE_AUTHENTICATION:false}",
  "requireAuthorization": "${REQUIRE_AUTHORIZATION:false}",
  "trustProxy": "${TRUST_PROXY:false}",
  "cookie": {
    "sameSite": "${COOKIE_SAME_SITE:Lax}"
  },
  "disableIPlogging": "${DISABLE_IP_LOGGING:false}",
  "automaticReconnectionTimeout": "${AUTOMATIC_RECONNECTION_TIMEOUT:0}",
  "scrollWhenFocusLineIsOutOfViewport": {
    "percentage": {
      "editionAboveViewport": "${FOCUS_LINE_PERCENTAGE_ABOVE:0}",
      "editionBelowViewport": "${FOCUS_LINE_PERCENTAGE_BELOW:0}"
    },
    "duration": "${FOCUS_LINE_DURATION:0}",
    "scrollWhenCaretIsInTheLastLineOfViewport": "${FOCUS_LINE_CARET_SCROLL:false}",
    "percentageToScrollWhenUserPressesArrowUp": "${FOCUS_LINE_PERCENTAGE_ARROW_UP:0}"
  },
  "socketTransportProtocols" : ["xhr-polling", "jsonp-polling", "htmlfile"],
  "socketIo": {
    "maxHttpBufferSize": "${SOCKETIO_MAX_HTTP_BUFFER_SIZE:10000}"
  },
  "loadTest": "${LOAD_TEST:false}",
  "dumpOnUncleanExit": "${DUMP_ON_UNCLEAN_EXIT:false}",
  "importExportRateLimiting": {
    "windowMs": "${IMPORT_EXPORT_RATE_LIMIT_WINDOW:90000}",
    "max": "${IMPORT_EXPORT_MAX_REQ_PER_IP:10}"
  },
  "importMaxFileSize": "${IMPORT_MAX_FILE_SIZE:52428800}", // 50 * 1024 * 1024
  "commitRateLimiting": {
    "duration": "${COMMIT_RATE_LIMIT_DURATION:1}",
    "points": "${COMMIT_RATE_LIMIT_POINTS:10}"
  },
  "exposeVersion": "${EXPOSE_VERSION:false}",
  "loglevel": "${LOGLEVEL:INFO}",
  "customLocaleStrings": {}
}
`
)

const ETHERPAD_PORT = 8080
const ETHERPAD_PORT_NAME = "etherpad-port"

func (r *SFController) DeployEtherpad(enabled bool) bool {
	var dep appsv1.Deployment
	found := r.GetM("etherpad", &dep)
	if !found && enabled {
		r.log.V(1).Info("Etherpad deploy not found")
		db_key_name := "etherpad-db-password"
		db_password, db_ready := r.EnsureDB("etherpad")
		admin_password := r.EnsureSecret("etherpad-admin-password")
		settings := makeEtherpatSettings(db_password.Data[db_key_name], admin_password.Data["etherpad-admin-password"])
		r.EnsureConfigMap("etherpad", "settings.json", settings)
		if db_ready {
			r.log.V(1).Info("Etherpad DB is ready, deploying the service now!")
			dep = create_deployment(r.ns, "etherpad", "quay.io/software-factory/sf-etherpad:1.8.17-1")
			dep.Spec.Template.Spec.Containers[0].Command = []string{
				"node", "./src/node/server.js", "--settings", "/etc/etherpad/settings.json"}
			dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
				create_container_port(ETHERPAD_PORT, ETHERPAD_PORT_NAME),
			}
			dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
				{
					Name:      "config-volume",
					MountPath: "/etc/etherpad",
				},
			}
			dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
				create_volume_cm("config-volume", "etherpad-config-map"),
			}
			r.CreateR(&dep)
			srv := create_service(r.ns, "etherpad", "etherpad", ETHERPAD_PORT, ETHERPAD_PORT_NAME)
			r.CreateR(&srv)
		}
	} else if found {
		if !enabled {
			r.log.V(1).Info("Etherpad deployment found, but it's not enabled, deleting it now")
			if err := r.Delete(r.ctx, &dep); err != nil {
				panic(err.Error())
			}
		}
	}
	if enabled {
		// Wait for the service to be ready.
		return (dep.Status.ReadyReplicas > 0)
	} else {
		// The service is not enabled, so it is always ready.
		return true
	}
}

func makeEtherpatSettings(db_password []byte, admin_password []byte) string {
	return fmt.Sprintf(etherpadSettingsTemplate, db_password, admin_password)
}

func (r *SFController) IngressEtherpad() netv1.IngressRule {
	return create_ingress_rule("etherpad."+r.cr.Spec.FQDN, "etherpad", ETHERPAD_PORT)
}
