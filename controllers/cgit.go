// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the cgit configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const CGIT_IDENT string = "cgit"
const CGIT_IMAGE string = "quay.io/software-factory/cgit:1.2.3-1"

const CGIT_PORT = 37920
const CGIT_PORT_NAME = "cgit-port"

func (r *SFController) CgitRc() string {
	templatefile := "controllers/static/cgit/cgitrc.tmpl"

	template, err := parse_template(templatefile, CGIT_IDENT+"."+r.cr.Spec.FQDN)
	if err != nil {
		r.log.V(1).Error(err, "Template parsing failed")
	}
	return template
}

func (r *SFController) CgitConfig() string {

	templatefile := "controllers/static/cgit/cgit.conf.tmpl"

	// Anonymous structure
	cgitconfig := struct {
		Port int
		Fqdn string
	}{CGIT_PORT, CGIT_IDENT + "." + r.cr.Spec.FQDN}

	template, err := parse_template(templatefile, cgitconfig)
	if err != nil {
		r.log.V(1).Error(err, "Template parsing failed")
	}
	return template
}

func (r *SFController) CgitHttpConfig() string {
	templatefile := "controllers/static/cgit/httpd.conf.tmpl"

	template, err := parse_template(templatefile, CGIT_PORT)
	if err != nil {
		r.log.V(1).Error(err, "Template parsing failed")
	}
	return template
}

func (r *SFController) DeployCgit(enabled bool) bool {

	if enabled {
		// Creating cgit config.json file
		conf_data := make(map[string]string)

		conf_data["httpd.conf"] = r.CgitHttpConfig()
		conf_data["cgit.conf"] = r.CgitConfig()
		conf_data["cgitrc"] = r.CgitRc()
		r.EnsureConfigMap(CGIT_IDENT, conf_data)

		dep := create_deployment(r.ns, CGIT_IDENT, CGIT_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"httpd", "-DFOREGROUND"}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(CGIT_PORT, CGIT_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      CGIT_IDENT + "-configrc-vol",
				MountPath: "/etc/cgitrc",
				SubPath:   "cgitrc",
			},
			{
				Name:      CGIT_IDENT + "-httpd-vol",
				MountPath: "/etc/httpd/conf/httpd.conf",
				SubPath:   "httpd.conf",
			},
			{
				Name:      CGIT_IDENT + "-httpd-cgit-vol",
				MountPath: "/etc/httpd/conf.d/cgit.conf",
				SubPath:   "cgit.conf",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm_keys(CGIT_IDENT+"-configrc-vol", CGIT_IDENT+"-config-map", []apiv1.KeyToPath{{Key: "cgitrc", Path: "cgitrc"}}),
			create_volume_cm_keys(CGIT_IDENT+"-httpd-vol", CGIT_IDENT+"-config-map", []apiv1.KeyToPath{{Key: "httpd.conf", Path: "httpd.conf"}}),
			create_volume_cm_keys(CGIT_IDENT+"-httpd-cgit-vol", CGIT_IDENT+"-config-map", []apiv1.KeyToPath{{Key: "cgit.conf", Path: "cgit.conf"}}),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", CGIT_PORT)
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(CGIT_PORT)

		r.GetOrCreate(&dep)

		srv := create_service(r.ns, CGIT_IDENT, CGIT_IDENT, CGIT_PORT, CGIT_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(CGIT_IDENT)
		r.DeleteService(CGIT_PORT_NAME)
		r.DeleteConfigMap("cgit-config-map")
		return true
	}
}

func (r *SFController) IngressCgit() netv1.IngressRule {
	return create_ingress_rule(CGIT_IDENT+"."+r.cr.Spec.FQDN, CGIT_IDENT, CGIT_PORT)
}
