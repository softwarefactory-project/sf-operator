// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the postfix configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
)

const POSTFIX_IDENT string = "postfix"
const POSTFIX_IMAGE string = "quay.io/software-factory/postfix:3.5.8-1"

const POSTFIX_PORT = 25
const POSTFIX_PORT_NAME = "postfix-port"

func PostfixMailname(r *SFController) string {
	return POSTFIX_IDENT + "." + r.cr.Spec.FQDN + "\n"
}

func PostfixVirtual(r *SFController) string {
	virtual := "root      " + r.cr.Spec.Postfix.ForwardEmail + "\n" +
		"backup      " + r.cr.Spec.Postfix.ForwardEmail + "\n" +
		"admin      " + r.cr.Spec.Postfix.ForwardEmail + "\n" +
		"dev-robot      " + r.cr.Spec.Postfix.ForwardEmail + "\n"

	if r.cr.Spec.Gerrit.Enabled {
		virtual += "gerrit      " + r.cr.Spec.Postfix.ForwardEmail + "\n" +
			"keycloak      " + r.cr.Spec.Postfix.ForwardEmail + "\n"
	}
	return virtual
}

func PostfixMain(r *SFController) string {
	maincf := ""

	rootsection := ""

	inifile := r.LoadConfigINI(maincf)

	inifile.NewSection(rootsection)
	inifile.Section(rootsection).NewKey("smtpd_banner", "$myhostname ESMTP $mail_name")
	inifile.Section(rootsection).NewKey("biff", "no")

	inifile.Section(rootsection).NewKey("append_dot_mydomain", "no")

	inifile.Section(rootsection).NewKey("myhostname", r.cr.Spec.FQDN)
	inifile.Section(rootsection).NewKey("mydomain", r.cr.Spec.FQDN)
	inifile.Section(rootsection).NewKey("mydestination", "$myhostname, localhost")

	inifile.Section(rootsection).NewKey("alias_maps", "hash:/etc/aliases")
	inifile.Section(rootsection).NewKey("transport_maps", "hash:/etc/postfix/transport")

	inifile.Section(rootsection).NewKey("virtual_alias_maps", "hash:/etc/postfix/virtual")

	inifile.Section(rootsection).NewKey("myorigin", r.cr.Spec.FQDN)

	// TODO: Check if this option is necessary
	// From the original SF main.cf file
	// {% if network.smtp_relay %}
	//relayhost = {{ network.smtp_relay }}
	//{% endif %}

	inifile.Section(rootsection).NewKey("mynetworks", "127.0.0.0/8 [::ffff:127.0.0.0]/104 [::1]/128")
	inifile.Section(rootsection).NewKey("mailbox_size_limit", "0")
	inifile.Section(rootsection).NewKey("recipient_delimiter", "+")
	inifile.Section(rootsection).NewKey("inet_interfaces", "loopback-only")
	inifile.Section(rootsection).NewKey("inet_interfaces", "loopback-only")

	// Option add due to postfix version 3.5.8-4
	inifile.Section(rootsection).NewKey("compatibility_level", "2")

	maincf = r.DumpConfigINI(inifile)

	return maincf
}

func PostfixTransport(r *SFController) string {
	return "# Drop mails sent to Zuul by Gerrit each time a review is updated\n" +
		"zuul@" + r.cr.Spec.FQDN + " discard:silently\n"
}

func (r *SFController) DeployPostfix() bool {

	if r.cr.Spec.Postfix.Enabled {
		// Creating postfix config.json file
		conf_data := make(map[string]string)

		conf_data["transport"] = PostfixTransport(r)
		conf_data["main.cf"] = PostfixMain(r)
		conf_data["virtual"] = PostfixVirtual(r)
		conf_data["mailname"] = PostfixMailname(r)
		r.EnsureConfigMap(POSTFIX_IDENT, conf_data)

		dep := create_deployment(r.ns, POSTFIX_IDENT, POSTFIX_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"sudo", "postfix", "start-fg"}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(POSTFIX_PORT, POSTFIX_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      POSTFIX_IDENT + "-config-vol",
				MountPath: "/etc/postfix2",
			},
			{
				Name:      POSTFIX_IDENT + "-mailname-vol",
				MountPath: "/etc/mailname",
				SubPath:   "mailname",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm_keys(POSTFIX_IDENT+"-config-vol", POSTFIX_IDENT+"-config-map",
				[]apiv1.KeyToPath{
					{
						Key:  "transport",
						Path: "transport",
					},
					{
						Key:  "main.cf",
						Path: "main.cf",
					},
					{
						Key:  "virtual",
						Path: "virtual",
					},
				}),
			create_volume_cm_keys(POSTFIX_IDENT+"-mailname-vol", POSTFIX_IDENT+"-config-map",
				[]apiv1.KeyToPath{
					{
						Key:  "mailname",
						Path: "mailname",
					},
				}),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"sudo", "postfix", "status"})

		r.GetOrCreate(&dep)

		srv := create_service(r.ns, POSTFIX_IDENT, POSTFIX_IDENT, POSTFIX_PORT, POSTFIX_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(POSTFIX_IDENT)
		r.DeleteService(POSTFIX_PORT_NAME)
		r.DeleteConfigMap("postfix-config-map")
		return true
	}
}
