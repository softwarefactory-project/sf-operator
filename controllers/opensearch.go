// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the opensearch configuration.
package controllers

import (
	_ "embed"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

// TODO
// ====
//
// - Fix usage of securityadmin.sh command
// When internal_users.yml change then securityadmin.sh command must be run to update the security index.const
// However we were unable to make it work
// ./securityadmin.sh -cacert /usr/share/opensearch/config/certs/ca.crt -cert /usr/share/opensearch/config/certs/tls.crt -h opensearch
// Will connect to opensearch:9200 ... done
// Connected as null
// ERR: null is not an admin user
// Seems you use a client certificate but this one is not registered as admin_dn
// Make sure opensearch.yml on all nodes contains:
// plugins.security.authcz.admin_dn:
//   - "null"
//

//go:embed static/opensearch/opensearch.yml
var os_opensearch_objs string

//go:embed static/opensearch/auth-config.yml
var os_auth_config string

//go:embed static/opensearch/log4j2.properties
var os_log4j string

//go:embed static/opensearch/nodes_dn.yml
var os_nodes_dn string

//go:embed static/opensearch/whitelist.yml
var os_whitelist string

//go:embed static/opensearch/roles.yml
var os_roles string

const OPENSEARCH_PORT = 9200
const OPENSEARCH_TRANSPORT_PORT = 9300
const OPENSEARCH_PORT_NAME = "os"
const OPENSEARCH_TRANSPORT_PORT_NAME = "os-transport"

type OSHeader struct {
	Type    string `json:"type"`
	Version int    `json:"config_version"`
}

type OSUser struct {
	Hash         string   `json:"hash"`
	Reserved     bool     `json:"reserved"`
	BackendRoles []string `json:"backend_roles,omitempty"`
	Description  string   `json:"description"`
}

func (r *SFController) DeployOpensearch(enabled bool) bool {
	if enabled {

		server_cert := r.create_client_certificate(r.ns, "opensearch-server", "ca-issuer", "opensearch-server-tls", "opensearch")
		r.GetOrCreate(&server_cert)

		// generate password
		users := []string{"admin", "kibanaserver", "kibanaro", "logstash", "readall"}
		users_hash := make(map[string]string)
		for _, user := range users {
			current_user := "opensearch-" + user + "-password"
			uuid_pass := string(r.GenerateSecretUUID(current_user).Data[current_user])
			users_hash[user] = gen_bcrypt_pass(uuid_pass)
		}

		// create internal_users.yml file
		es_users := map[string]interface{}{
			"_meta": OSHeader{
				Type:    "internalusers",
				Version: 2,
			},
			"admin": OSUser{
				Hash:        users_hash["admin"],
				Reserved:    true,
				Description: "OpenSearch Admin user",
				BackendRoles: []string{
					"admin",
				},
			},
			"kibanaserver": OSUser{
				Hash:         users_hash["kibanaserver"],
				Reserved:     true,
				Description:  "OpenSearch Dashboards user",
				BackendRoles: []string{"opensearch_dashboards_user"},
			},
			"guest": OSUser{
				Hash:         users_hash["kibanaro"],
				Reserved:     false,
				Description:  "OpenSearch Dashboards read only user",
				BackendRoles: []string{"opensearch_dashboards_read_only"},
			},
			// "logstash": OSUser{
			// 	Hash:         users_hash["logstash"],
			// 	Reserved:     false,
			// 	Description:  "OpenSearch Dashboards Logstash user",
			// 	BackendRoles: []string{"logstash"},
			// },
		}

		data, err := yaml.Marshal(es_users)
		if err != nil {
			panic(err)
		}

		plugin_data := make(map[string]string)
		plugin_data["internal_users.yml"] = string(data)
		plugin_data["config.yml"] = string(os_auth_config)
		r.EnsureConfigMap("opensearch-internal-users", plugin_data)

		// config maps
		cm_data := make(map[string]string)
		cm_data["opensearch.yml"] = os_opensearch_objs
		cm_data["log4j2.properties"] = os_log4j
		cm_data["nodes_dn.yml"] = os_nodes_dn
		cm_data["whitelist.yml"] = os_whitelist
		cm_data["roles.yml"] = os_roles
		r.EnsureConfigMap("opensearch", cm_data)

		dep := create_statefulset(r.ns, "opensearch", "quay.io/software-factory/opensearch:2.2.0-1")
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"/bin/bash", "-x", "/usr/share/opensearch/opensearch-docker-entrypoint.sh"}

		dep.Spec.Template.Spec.Hostname = "opensearch-master"
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(OPENSEARCH_PORT)

		user := int64(1000)
		securitycontext := &apiv1.PodSecurityContext{RunAsUser: &user, RunAsGroup: &user, FSGroup: &user}
		dep.Spec.Template.Spec.SecurityContext = securitycontext

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(OPENSEARCH_PORT, OPENSEARCH_PORT_NAME),
			create_container_port(OPENSEARCH_TRANSPORT_PORT, OPENSEARCH_TRANSPORT_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      "opensearch",
				MountPath: "/usr/share/opensearch/data",
			},
			// Only mount the required file to not override the directory content
			{
				Name:      "os-config",
				MountPath: "/usr/share/opensearch/config/opensearch.yml",
				SubPath:   "opensearch.yml",
				ReadOnly:  true,
			},
			// Only mount the required file to not override the directory content
			// {
			// 	Name:      "os-config",
			// 	MountPath: "/usr/share/opensearch/config/log4j2.properties",
			// 	SubPath:   "log4j2.properties",
			// 	ReadOnly:  true,
			// },
			// Only mount the required file to not override the directory content
			// {
			// 	Name:      "os-config",
			// 	MountPath: "/usr/share/opensearch/config/nodes_dn.yml",
			// 	SubPath:   "nodes_dn.yml",
			// 	ReadOnly:  true,
			// },
			// Only mount the required file to not override the directory content
			// {
			// 	Name:      "os-config",
			// 	MountPath: "/usr/share/opensearch/config/whitelist.yml",
			// 	SubPath:   "whitelist.yml",
			// 	ReadOnly:  true,
			// },
			// Only mount the required file to not override the directory content
			{
				Name:      "os-config",
				MountPath: "/usr/share/opensearch/config/roles.yml",
				SubPath:   "roles.yml",
				ReadOnly:  true,
			},
			{
				Name:      "opensearch-server-certs",
				MountPath: "/usr/share/opensearch/config/certs",
				ReadOnly:  true,
			},
			// Only mount the required file to not override the directory content
			{
				Name:      "os-plugin",
				MountPath: "/usr/share/opensearch/config/opensearch-security/internal_users.yml",
				SubPath:   "internal_users.yml",
				ReadOnly:  true,
			},
			// Only mount the required file to not override the directory content
			{
				Name:      "os-plugin",
				MountPath: "/usr/share/opensearch/config/opensearch-security/config.yml",
				SubPath:   "config.yml",
				ReadOnly:  true,
			},
		}
		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm("os-config", "opensearch-config-map"),
			// can be done with function create_volume_secret, but
			// if secret name and volume name are same, secret will get
			// a random string suffix, which will freeze opensearch deployment.
			{
				Name: "opensearch-server-certs",
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{
						SecretName: "opensearch-server-tls",
					},
				},
			},
			create_volume_cm("os-plugin", "opensearch-internal-users-config-map"),
		}

		// Expose env vars
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_env("discovery.type", "single-node"),
			create_env("node.name", "opensearch-master"),
			create_env("network.host", "0.0.0.0"),
			create_env("OPENSEARCH_JAVA_OPTS", "-Xmx512M -Xms512M"),
			create_env("DISABLE_INSTALL_DEMO_CONFIG", "true"),
		}

		r.GetOrCreate(&dep)
		srv := create_service(r.ns, "opensearch", "opensearch", OPENSEARCH_PORT, OPENSEARCH_PORT_NAME)
		r.GetOrCreate(&srv)
		return r.IsStatefulSetReady(&dep)
	} else {
		r.DeleteStatefulSet("opensearch")
		r.DeleteService("opensearch")
		return true
	}
}

func (r *SFController) IngressOpensearch() netv1.IngressRule {
	return create_ingress_rule("opensearch."+r.cr.Spec.FQDN, "opensearch", OPENSEARCH_PORT)
}
