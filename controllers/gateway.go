// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the gateway configuration.
package controllers

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
	"text/template"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const GATEWAY_IDENT string = "gateway"
const GATEWAY_IMAGE string = "quay.io/software-factory/gateway:1.0.0-1"

const GATEWAY_PORT = 80
const GATEWAY_PORT_NAME = "gateway-port"

//go:embed static/gateway/gateway.common
var gateway_common string

// Service Mapping
//
// Defines a map with <service name> : <service path>
//
// Return key and value based on the service_name
//
// Wrapping the map here, the map is not on global scope
func serviceMapping(r *SFController, service_name string) (key string, value string) {
	services := map[string]string{
		GERRIT_IDENT:            "/gerrit",
		"keycloak":              "/keycloak",
		"zuul":                  "/zuul",
		"nodepool":              "/nodepool",
		"opensearch-dashboards": "/opensearchdashboards",
		GRAFANA_IDENT:           "/" + GRAFANA_IDENT,
		"etherpad":              "/etherpad",
		LODGEIT_IDENT:           "/lodgeit",
		HOUND_IDENT:             "/hound",
		CGIT_IDENT:              "/" + CGIT_IDENT,
		MURMUR_IDENT:            "mumble://" + MURMUR_IDENT + "." + r.cr.Spec.FQDN + "/?version=1.2.0",
	}

	for key, value := range services {
		if key == service_name {
			return key, value
		}
	}

	return "", ""
}

// Get Service Fqdn
//
// Returns a string with with following format:
//
// <service_name><fqdn>
//
// e.g: ( "softwarefactory", "sf.com" )
//
// softwarefactory.sf.com
func getServiceFqdn(service_name string, fqdn string) string {
	return service_name + "." + fqdn
}

// Get Service Https Url
//
// Returns a string with with following format:
//
// https://<protocol><service_name><fqdn>
//
// e.g: ( "https://", "softwarefactory", "sf.com" )
//
// http://softwarefactory.sf.com
func getServiceHttpsUrl(service_name string, fqdn string) string {
	return getServiceUrl("https://", service_name, fqdn)
}

// Get Service Url
//
// Returns a string with with following format:
//
// <protocol><service_name><fqdn>
//
// e.g: ( "http://", "softwarefactory", "sf.com" )
//
// http://softwarefactory.sf.com
func getServiceUrl(protocol string, service_name string, fqdn string) string {
	return protocol + getServiceFqdn(service_name, fqdn)
}

// service.conf generator
//
// This function generates service.conf, mounted at
// /etc/httpd/conf.d/
func gatewayServiceConfGenerator(r *SFController) string {

	template_frame := `
	{{ if .Services }}
	{{ range $idx, $elem := .Services }}
	# {{ $elem.Title }} Ingres Redirect
	Redirect "{{ $elem.UrlPath }}" "{{ $elem.RedirectUrl }}"
	{{ end }}
	{{ end }}
	`

	type Services struct {
		Title       string
		UrlPath     string
		RedirectUrl string
	}

	data := struct {
		Services []Services
	}{}

	// lambda function
	appendingService := func(service_name string) {
		key, value := serviceMapping(r, service_name)
		data.Services = append(data.Services,
			Services{
				strings.ToTitle(key),
				value,
				getServiceHttpsUrl(key, r.cr.Spec.FQDN)},
		)
	}

	if r.cr.Spec.Gerrit.Enabled {
		appendingService(GERRIT_IDENT)
		appendingService("keycloak")
	}

	if r.cr.Spec.Zuul.Enabled {
		appendingService("zuul")
	}

	if r.cr.Spec.OpensearchDashboards.Enabled {
		appendingService("opensearch-dashboards")
	}

	if r.cr.Spec.Grafana.Enabled {
		appendingService(GRAFANA_IDENT)
	}

	if r.cr.Spec.Etherpad.Enabled {
		appendingService("etherpad")
	}

	if r.cr.Spec.Lodgeit.Enabled {
		appendingService(LODGEIT_IDENT)
	}

	if r.cr.Spec.Hound.Enabled {
		appendingService(HOUND_IDENT)
	}

	if r.cr.Spec.Cgit.Enabled {
		appendingService(CGIT_IDENT)
	}

	// TODO: Develop an elegant way of retrieving both zuul and node pool versions.
	data.Services = append(data.Services,
		Services{
			"Zuul Documentation",
			"/docs/zuul",
			"https://zuul-ci.org/docs/zuul/"},
		Services{
			"Nodepool Documentation",
			"/docs/nodepool",
			"https://zuul-ci.org/docs/nodepool/"},
	)

	tmpl := template.Must(template.New("test").Parse(template_frame))
	var file bytes.Buffer
	if err := tmpl.Execute(&file, data); err != nil {
		r.log.V(1).Error(err, "could execute tempalte")
		return ""
	}
	return file.String()
}

// gateway.conf generator
//
// This function generates gateway.conf, mounted at
// /etc/httpd/conf.d/
func gatewayConfigGenerator(r *SFController) string {

	template_path := "controllers/static/gateway/gateway.conf.tmpl"

	file, err := parse_template(template_path, r.cr.Spec.FQDN)
	if err != nil {
		r.log.V(1).Error(err, "Failed while parsing template: "+template_path)
	}
	return file
}

// Checks Enabled Services
//
// Populates info.json "services" property depending on the enalbed service
func checkEnabledServices(r *SFController) []map[string]string {
	services := []map[string]string{}

	key, value := "", ""

	// lambda function
	appendingServices := func(service_name string) {
		key, value = serviceMapping(r, service_name)
		services = append(services, map[string]string{"name": key, "path": value})
	}

	if r.cr.Spec.Gerrit.Enabled {
		appendingServices(GERRIT_IDENT)
		appendingServices("keycloak")
	}

	if r.cr.Spec.Zuul.Enabled {
		appendingServices("zuul")
		appendingServices("nodepool")
	}

	if r.cr.Spec.OpensearchDashboards.Enabled {
		appendingServices("opensearch-dashboards")
	}

	if r.cr.Spec.Grafana.Enabled {
		appendingServices(GRAFANA_IDENT)
	}

	if r.cr.Spec.Etherpad.Enabled {
		appendingServices("etherpad")
	}

	if r.cr.Spec.Lodgeit.Enabled {
		appendingServices(LODGEIT_IDENT)
	}

	if r.cr.Spec.Hound.Enabled {
		appendingServices(HOUND_IDENT)
	}

	if r.cr.Spec.Cgit.Enabled {
		appendingServices(CGIT_IDENT)
	}

	if r.cr.Spec.Murmur.Enabled {
		appendingServices(MURMUR_IDENT)
	}

	return services
}

func logoToBase64(r *SFController, image_path string) string {

	logo_encoded, err := r.ImageToBase64(image_path)
	if err != nil {
		r.log.V(1).Error(err, "Failed to encode image "+image_path)
		logo_encoded = ""
	}
	return logo_encoded
}

// info.json generator
//
// This function generates info.json, mounted at
// /var/www/api
func gatewayInfoJsonGenerator(r *SFController) string {

	header_logo_path := "controllers/static/gateway/logo-topmenu.png"

	top_logo_path := "controllers/static/gateway/logo-splash.png"

	data := map[string]interface{}{
		"header_logo_b64data":  logoToBase64(r, header_logo_path),
		"splash_image_b64data": logoToBase64(r, top_logo_path),
		"version":              r.cr.APIVersion,
		"links": map[string]interface{}{
			"status":  []string{},
			"contact": []string{},
			"documentation": []map[string]interface{}{
				{
					"link": "/docs",
					"name": "Software Factory",
				},
				{
					"link": "/docs/zuul",
					"name": "Zuul",
				},
				{
					"link": "/docs/nodepool",
					"name": "Nodepool",
				},
			},
		},
		"services": checkEnabledServices(r),
		// TODO: Check how auths is genereted in sf config
		"auths": map[string]interface{}{
			"oauth": []string{},
			"other": []map[string]interface{}{},
		},
	}

	jsonData, err := json.MarshalIndent(data, "    ", "  ")
	if err != nil {
		r.log.V(1).Error(err, "could not marshal json")
		return ""
	}

	return string(jsonData)
}

func IsToDeployGateway(r *SFController) bool {
	if r.cr.Spec.Gerrit.Enabled || r.cr.Spec.Zuul.Enabled ||
		r.cr.Spec.OpensearchDashboards.Enabled || r.cr.Spec.Grafana.Enabled ||
		r.cr.Spec.Etherpad.Enabled || r.cr.Spec.Lodgeit.Enabled ||
		r.cr.Spec.Hound.Enabled || r.cr.Spec.Cgit.Enabled ||
		r.cr.Spec.Murmur.Enabled {
		return true
	}
	return false
}

func (r *SFController) DeployGateway() bool {

	if IsToDeployGateway(r) {
		// Generate Gateway Keycloak Password
		r.GenerateSecretUUID(GATEWAY_IDENT + "-kc-client-password")

		// Creating Map for /var/www/api files
		api_data := make(map[string]string)

		// TODO: Other operator should modify/update these ConfigMaps
		// or other operator must create these files, and gateway
		// operator just have to mount as a volume
		api_data["resources.json"] = ""
		api_data["status.json"] = ""
		// TODO: This file was generated by sfconfig from sf-install-server and sf-gateway components on version sf3.8.
		// The current implementation is the bare minimum for the gateway to work
		api_data["info.json"] = gatewayInfoJsonGenerator(r)
		r.EnsureConfigMap(GATEWAY_IDENT+"-api", api_data)

		// Creating Map for /etc/httpd/conf.d/ files
		http_data := make(map[string]string)
		http_data["gateway.conf"] = gatewayConfigGenerator(r)
		http_data["gateway.common"] = gateway_common
		http_data["service.conf"] = gatewayServiceConfGenerator(r)

		r.EnsureConfigMap(GATEWAY_IDENT+"-httpd", http_data)

		// Creating Certificate
		cert := r.create_client_certificate(
			r.ns, GATEWAY_IDENT+"-client", "ca-issuer", GATEWAY_IDENT+"-client-tls", GATEWAY_IDENT)
		r.GetOrCreate(&cert)

		dep := create_deployment(r.ns, GATEWAY_IDENT, GATEWAY_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"httpdforeground.sh"}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(GATEWAY_PORT, GATEWAY_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      GATEWAY_IDENT + "-api-vol",
				MountPath: "/var/www/api",
			},
			{
				Name:      GATEWAY_IDENT + "-client-tls",
				MountPath: "/etc/pki/tls/certs/gateway",
				ReadOnly:  true,
			},
			// NOTE: Setting file by file, does not delete files in the mounting destination point
			{
				Name:      GATEWAY_IDENT + "-httpd-vol",
				MountPath: "/etc/httpd/conf.d/gateway.conf",
				SubPath:   "gateway.conf",
			},
			{
				Name:      GATEWAY_IDENT + "-httpd-vol",
				MountPath: "/etc/httpd/conf.d/gateway.common",
				SubPath:   "gateway.common",
			},
			{
				Name:      GATEWAY_IDENT + "-httpd-vol",
				MountPath: "/etc/httpd/conf.d/service.conf",
				SubPath:   "service.conf",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(GATEWAY_IDENT+"-api-vol", GATEWAY_IDENT+"-api"+"-config-map"),
			create_volume_cm(GATEWAY_IDENT+"-httpd-vol", GATEWAY_IDENT+"-httpd"+"-config-map"),
			create_volume_secret(GATEWAY_IDENT + "-client-tls"),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", GATEWAY_PORT)
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(GATEWAY_PORT)

		r.GetOrCreate(&dep)

		srv := create_service(r.ns, GATEWAY_IDENT, GATEWAY_IDENT, GATEWAY_PORT, GATEWAY_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(GATEWAY_IDENT)
		r.DeleteService(GATEWAY_PORT_NAME)
		r.DeleteConfigMap(GATEWAY_IDENT + "-api" + "-config-map")
		r.DeleteConfigMap(GATEWAY_IDENT + "-httpd" + "-config-map")
		r.DeleteSecret(GATEWAY_IDENT + "-kc-client-password")
		return true
	}
}

func (r *SFController) IngressGateway() netv1.IngressRule {
	return create_ingress_rule(r.cr.Spec.FQDN, GATEWAY_IDENT, GATEWAY_PORT)
}
