// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the etherpad configuration.
package controllers

import (
	_ "embed"
	"strconv"
	"strings"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	apiv1 "k8s.io/api/core/v1"
)

const GERRITBOT_IDENT string = "gerritbot"
const GERRITBOT_IMAGE string = "quay.io/software-factory/gerritbot:0.4.0-1"

const GERRITBOT_PORT = 6667
const GERRITBOT_PORT_NAME = "gerritbot-port"

//go:embed static/gerritbot/logging.conf
var gerritbotLogging string

func yamlliststructure(list []string, level int) string {
	yamllist := ""
	for _, elem := range list {
		yamllist += strings.Repeat(" ", level) + "- " + elem + "\n"
	}
	return yamllist
}

func GerritBotConfig(r *SFController, spec sfv1.GerritBotIRCBotSpec) string {
	password := ""
	if (spec.Password != sfv1.SecretRef{}) {
		secretvalue, err := getValueFromSecret(r, spec.Password)
		if err != nil {
			r.log.V(1).Error(err, "Wrong Secret definition")
			r.log.V(1).Info("IRC Bot Server Password will be set to default")
		} else {
			password = string(secretvalue)
		}
	}

	port := 6667
	if spec.Port != 0 {
		port = spec.Port
	}

	// Starting ini file
	inifile := ""

	cfg_ini := r.LoadConfigINI(inifile)

	section := "ircbot"
	cfg_ini.NewSection(section)
	cfg_ini.Section(section).NewKey("nick", spec.Nick)
	cfg_ini.Section(section).NewKey("pass", password)
	cfg_ini.Section(section).NewKey("server", spec.Server)
	cfg_ini.Section(section).NewKey("port", strconv.Itoa(port))
	cfg_ini.Section(section).NewKey("channel_config", "/etc/gerritbot/channels.yaml")
	cfg_ini.Section(section).NewKey("pid", "/var/run/gerritbot/gerritbot.pid")

	section = "gerrit"
	cfg_ini.NewSection(section)
	cfg_ini.Section(section).NewKey("user", "zuul")
	cfg_ini.Section(section).NewKey("key", "/var/lib/gerritbot/.ssh/id_rsa")
	cfg_ini.Section(section).NewKey("host", "gerrit-sshd")
	cfg_ini.Section(section).NewKey("port", "29418")

	inifile = r.DumpConfigINI(cfg_ini)

	return inifile
}

func GerritBotConfigChannels(r *SFController, channels []sfv1.GerritBotChannelsSpec) string {
	channelsconfig := ""
	if len(channels) != 0 {
		for _, channel := range channels {
			channelsconfig += channel.Name + ":\n" +
				" events:\n" +
				yamlliststructure(channel.Events, 3) +
				" projects:\n" +
				yamlliststructure(channel.Projects, 3) +
				" branches:\n" +
				yamlliststructure(channel.Branches, 3) + "\n"
		}
	}
	return channelsconfig
}

func (r *SFController) DeployGerritBot(enabled bool) bool {

	if enabled {
		// Creating gerritbot config.json file
		conf_data := make(map[string]string)
		conf_data["gerritbot.conf"] = GerritBotConfig(r, r.cr.Spec.GerritBot.IRCbot)
		conf_data["logging.conf"] = gerritbotLogging
		conf_data["channels.yaml"] = GerritBotConfigChannels(r, r.cr.Spec.GerritBot.IRCbot.Channel)
		r.EnsureConfigMap(GERRITBOT_IDENT, conf_data)

		dep := create_deployment(r.ns, GERRITBOT_IDENT, GERRITBOT_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"gerritbot", "--no-daemon", "/etc/gerritbot/gerritbot.conf"}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GERRITBOT_PORT, GERRITBOT_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      GERRITBOT_IDENT + "-config-vol",
				MountPath: "/etc/gerritbot",
			},
			{
				Name:      GERRITBOT_IDENT + "-lib-vol",
				MountPath: "/var/lib/gerritbot",
			},
			{
				Name:      GERRITBOT_IDENT + "-ssh-vol",
				MountPath: "/var/lib/gerritbot/.ssh",
			},
		}

		var mod int32 = 256 // decimal for 0400 octal
		var modpub int32 = 292
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(GERRITBOT_IDENT+"-config-vol", GERRITBOT_IDENT+"-config-map"),
			create_empty_dir(GERRITBOT_IDENT + "-lib-vol"),
			{
				Name: GERRITBOT_IDENT + "-ssh-vol",
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName:  "zuul-ssh-key",
						DefaultMode: &mod,
						Items: []apiv1.KeyToPath{
							{
								Key:  "pub",
								Path: "id_rsa.pub",
								Mode: &modpub,
							},
							{
								Key:  "priv",
								Path: "id_rsa",
								Mode: &modpub,
							},
						},
					},
				},
			},
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{
			"cat",
			"/proc/1/cmdline",
		})

		r.GetOrCreate(&dep)

		srv := create_service(r.ns, GERRITBOT_IDENT, GERRITBOT_IDENT, GERRITBOT_PORT, GERRITBOT_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(GERRITBOT_IDENT)
		r.DeleteService(GERRITBOT_PORT_NAME)
		r.DeleteConfigMap("gerritbot-config-map")
		return true
	}
}
