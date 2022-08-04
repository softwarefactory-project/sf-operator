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

//go:embed templates/opensearch/opensearch.yml
var os_opensearch_objs string

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
		r.log.V(1).Info("Opensearch deploy not found")

		server_cert := r.create_client_certificate(r.ns, "opensearch-server", "ca-issuer", "opensearch-server-tls")
		r.GetOrCreate(&server_cert)

		// generate password
		users := []string{"admin", "kibanaserver", "kibanaro", "logstash", "readall"}
		users_hash := make(map[string]string)
		for _, user := range users {
			current_user := "opensearch-" + user + "-password"
			bcrpt_pass := string(r.GenerateBCRYPTPassword(current_user).Data[current_user])
			users_hash[user] = bcrpt_pass
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
			},
			"kibanaserver": OSUser{
				Hash:        users_hash["kibanaserver"],
				Reserved:    true,
				Description: "OpenSearch Dashboards user",
			},
			"kibanaro": OSUser{
				Hash:         users_hash["kibanaro"],
				Reserved:     false,
				Description:  "OpenSearch Dashboards read only user",
				BackendRoles: []string{"kibanauser", "readall"},
			},
			"logstash": OSUser{
				Hash:         users_hash["logstash"],
				Reserved:     false,
				Description:  "OpenSearch Dashboards Logstash user",
				BackendRoles: []string{"logstash"},
			},
			"readall": OSUser{
				Hash:         users_hash["readall"],
				Reserved:     false,
				Description:  "OpenSearch Dashboards readall user",
				BackendRoles: []string{"readall"},
			},
		}

		data, err := yaml.Marshal(es_users)
		if err != nil {
			panic(err)
		}

		plugin_data := make(map[string]string)
		plugin_data["internal_users.yml"] = string(data)
		r.EnsureConfigMap("opensearch-internal-users", plugin_data)

		// config maps
		cm_data := make(map[string]string)
		cm_data["opensearch.yml"] = os_opensearch_objs
		r.EnsureConfigMap("opensearch", cm_data)

		dep := create_statefulset(r.ns, "opensearch", "quay.io/software-factory/opensearch:1.3.1-1")
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
				Name:      "os-data",
				MountPath: "/usr/share/opensearch/data",
			},
			{
				// mount just an opensearch.yml file, due there is a lot
				// other configuration files there.
				Name:      "os-config",
				MountPath: "/usr/share/opensearch/config/opensearch.yml",
				SubPath:   "opensearch.yml",
				ReadOnly:  true,
			},
			{
				Name:      "opensearch-server-certs",
				MountPath: "/usr/share/opensearch/config/certs",
				ReadOnly:  true,
			},
			{
				// mount just internal_users.yml file
				Name:      "os-plugin",
				MountPath: "/usr/share/opensearch/plugins/opensearch-security/securityconfig/internal_users.yml",
				SubPath:   "internal_users.yml",
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

		dep.Spec.VolumeClaimTemplates = append(
			dep.Spec.VolumeClaimTemplates,
			create_pvc(r.ns, "os-data"),
		)

		// Expose env vars
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_env("node.name", "opensearch-master"),
			create_env("cluster.initial_master_nodes", "opensearch-master"),
			create_env("discovery.seed_hosts", `opensearch.`+r.ns+`:9200`),
			create_env("cluster.name", "opensearch-cluster"),
			create_env("network.host", "0.0.0.0"),
			create_env("OPENSEARCH_JAVA_OPTS", "-Xmx512M -Xms512M"),
			create_env("node.roles", "master,ingest,data,remote_cluster_client"),
			create_env("DISABLE_INSTALL_DEMO_CONFIG", "true"),
			// additional
			create_env("OPENSEARCH_PATH_CONF", "/usr/share/opensearch/config"),
			create_env("LD_LIBRARY_PATH", ":/usr/share/opensearch/plugins/opensearch-knn/lib"),
			create_env("JAVA_HOME", "/usr/share/opensearch/jdk"),
			create_env("OPENSEARCH_HOME", "/usr/share/opensearch"),
			create_env("HOME", "/usr/share/opensearch"),
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
