// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the grafana configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const GRAFANA_IDENT string = "grafana"
const GRAFANA_IMAGE string = "quay.io/software-factory/grafana:7.5.7"

const GRAFANA_PORT = 3000
const GRAFANA_PORT_NAME = "grafana-port"

func GrafanaMailname(r *SFController) string {
	return GRAFANA_IDENT + "." + r.cr.Spec.FQDN + "\n"
}

func GrafanaVirtual(r *SFController) string {
	virtual := ""
	return virtual
}

func GrafanaIni(r *SFController, dbsecret apiv1.Secret, adminsecret apiv1.Secret, keycloakcliensecret apiv1.Secret) string {
	maincf := ""

	inifile := r.LoadConfigINI(maincf)

	// OAuth Section
	oauthsection := "auth"
	inifile.NewSection(oauthsection)
	inifile.Section(oauthsection).NewKey("oauth_auto_login", "true")

	// Server Section
	serversection := "server"
	inifile.NewSection(serversection)
	inifile.Section(serversection).NewKey("root_url", "https://"+GRAFANA_IDENT+"."+r.cr.Spec.FQDN)

	// Database Section
	databasesection := "database"
	inifile.Section(databasesection).NewKey("type", "mysql")
	inifile.Section(databasesection).NewKey("host", "mariadb")
	inifile.Section(databasesection).NewKey("name", GRAFANA_IDENT)
	inifile.Section(databasesection).NewKey("user", GRAFANA_IDENT)
	inifile.Section(databasesection).NewKey("password", string(dbsecret.Data[GRAFANA_IDENT+"-db-password"]))

	// Auth.Anonymous Section
	authanonymoussection := "auth.anonymous"
	inifile.Section(authanonymoussection).NewKey("enabled", "True")
	inifile.Section(authanonymoussection).NewKey("org_role", "Viewer")

	// Security Section
	securitysection := "security"
	inifile.Section(securitysection).NewKey("admin_password", string(adminsecret.Data[GRAFANA_IDENT+"-admin-password"]))

	// Auth Generic Oauth Section
	authgenericauthsection := "auth.generic_oauth"
	inifile.Section(authgenericauthsection).NewKey("enabled", "true")
	inifile.Section(authgenericauthsection).NewKey("name", "Software Factory SSO")
	inifile.Section(authgenericauthsection).NewKey("client_id", "grafana")
	inifile.Section(authgenericauthsection).NewKey("client_secret", string(keycloakcliensecret.Data[GRAFANA_IDENT+"-kc-client-password"]))
	inifile.Section(authgenericauthsection).NewKey("scopes", "openid profile")
	inifile.Section(authgenericauthsection).NewKey("auth_url", "https://keycloak."+r.cr.Spec.FQDN+"/realms/"+KC_DEFAULT_REALM+"/protocol/openid-connect/auth")
	inifile.Section(authgenericauthsection).NewKey("token_url", "https://keycloak."+r.cr.Spec.FQDN+"/realms/"+KC_DEFAULT_REALM+"/protocol/openid-connect/token")
	inifile.Section(authgenericauthsection).NewKey("api_url", "https://keycloak."+r.cr.Spec.FQDN+"/realms/"+KC_DEFAULT_REALM+"/protocol/openid-connect/userinfo")
	inifile.Section(authgenericauthsection).NewKey("tls_client_ca", "/etc/pki/ca-trust/extracted/pem/ca.crt")
	inifile.Section(authgenericauthsection).NewKey("tls_client_cert", "/etc/pki/ca-trust/extracted/pem/tls.crt")
	inifile.Section(authgenericauthsection).NewKey("tls_client_key", "/etc/pki/ca-trust/extracted/pem/tls.key")
	inifile.Section(authgenericauthsection).NewKey("allow_sign_up", "true")
	//inifile.Section(authgenericauthsection).NewKey("tls_skip_verify_insecure", "true")

	logsection := "log"
	inifile.Section(logsection).NewKey("level", "debug info warn error")

	maincf = r.DumpConfigINI(inifile)

	return maincf
}

func GrafanaTransport(r *SFController) string {
	return "# Drop mails sent to Zuul by Gerrit each time a review is updated\n" +
		"zuul@" + r.cr.Spec.FQDN + " discard:silently\n"
}

func (r *SFController) DeployGrafana(enabled bool) bool {

	if enabled {
		// Creating DB
		initContainers, grafanapassword := r.EnsureDBInit(GRAFANA_IDENT)

		// Creating Certificate
		cert := r.create_client_certificate(
			r.ns, GRAFANA_IDENT+"-client", "ca-issuer", GRAFANA_IDENT+"-client-tls", GRAFANA_IDENT)
		r.GetOrCreate(&cert)

		// Generate Grafana Admin Secret
		grafanaadminpassword := r.GenerateSecretUUID(GRAFANA_IDENT + "-admin-password")

		// Generate Grafana Keycloak Password
		grafanakeycloakclientpassword := r.GenerateSecretUUID(GRAFANA_IDENT + "-kc-client-password")

		// Creating grafana config.json file
		conf_data := make(map[string]string)
		conf_data["grafana.ini"] = GrafanaIni(r, grafanapassword, grafanaadminpassword, grafanakeycloakclientpassword)
		r.EnsureConfigMap(GRAFANA_IDENT, conf_data)

		dep := create_deployment(r.ns, GRAFANA_IDENT, GRAFANA_IMAGE)

		dep.Spec.Template.Spec.InitContainers = initContainers

		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"/run.sh"}

		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_env("GF_INSTALL_PLUGINS", "grafana-clock-panel"),
		}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GRAFANA_PORT, GRAFANA_PORT_NAME),
		}

		kc_ip := r.get_service_ip("keycloak")
		if kc_ip == "" {
			return false
		}
		dep.Spec.Template.Spec.HostAliases = []apiv1.HostAlias{{
			IP:        kc_ip,
			Hostnames: []string{"keycloak." + r.cr.Spec.FQDN},
		}}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      GRAFANA_IDENT + "-client-tls",
				MountPath: "/etc/pki/ca-trust/extracted/pem/",
				ReadOnly:  true,
			},
			{
				Name:      GRAFANA_IDENT + "-config-vol",
				MountPath: "/etc/grafana/",
			},
			{
				Name:      GRAFANA_IDENT + "-lib-vol",
				MountPath: "/var/lib/grafana",
			},
			{
				Name:      GRAFANA_IDENT + "-log-vol",
				MountPath: "/var/log/grafana",
			},
			{
				Name:      GRAFANA_IDENT + "-run-vol",
				MountPath: "/var/run/grafana",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(GRAFANA_IDENT+"-config-vol", GRAFANA_IDENT+"-config-map"),
			create_empty_dir(GRAFANA_IDENT + "-lib-vol"),
			create_empty_dir(GRAFANA_IDENT + "-log-vol"),
			create_empty_dir(GRAFANA_IDENT + "-run-vol"),
			create_volume_secret(GRAFANA_IDENT + "-client-tls"),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{"id"})

		r.GetOrCreate(&dep)

		srv := create_service(r.ns, GRAFANA_IDENT, GRAFANA_IDENT, GRAFANA_PORT, GRAFANA_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(GRAFANA_IDENT)
		r.DeleteService(GRAFANA_PORT_NAME)
		r.DeleteConfigMap("grafana-config-map")
		return true
	}
}

func (r *SFController) IngressGrafana() netv1.IngressRule {
	return create_ingress_rule(GRAFANA_IDENT+"."+r.cr.Spec.FQDN, GRAFANA_IDENT, GRAFANA_PORT)
}
