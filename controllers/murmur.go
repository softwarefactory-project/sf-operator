// Copyright (C) 2022 Red Hat
// SPDX-License-Identifier: Apache-2.0
//
// This package contains the etherpad configuration.
package controllers

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	sfv1 "github.com/softwarefactory-project/sf-operator/api/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

const MURMUR_IDENT string = "murmur"
const MURMUR_IMAGE string = "quay.io/software-factory/murmur:0.2.20-1"

//go:embed static/murmur/cmdProbe.sh
var murmurCmdProbe string

const MURMUR_PORT = 64738
const MURMUR_PORT_NAME = "murmur"

func murmur_channel_config(r *SFController, channel sfv1.MurmurChannelSpec) string {

	channelFormatted := fmt.Sprintf("\n% 25s{\n", " ")
	channelFormatted = channelFormatted + fmt.Sprintf("% 26sname = \"%s\";\n", " ", channel.Name)
	channelFormatted = channelFormatted + fmt.Sprintf("% 26sdescription = \"%s\";\n", " ", channel.Description)

	if (channel.Password != sfv1.SecretRef{}) {
		channelsecret, err := getValueFromSecret(r, channel.Password)
		if err != nil {
			r.log.V(1).Info("Murmur Channel Password will be set to default")
		} else {
			channelFormatted = channelFormatted + fmt.Sprintf("% 26spassword = \"%s\";\n", " ", string(channelsecret))
		}
	}

	channelFormatted = channelFormatted + fmt.Sprintf("% 25s}", " ")

	return channelFormatted
}

func getValueFromKeySecret(secret apiv1.Secret, keyname string) ([]byte, error) {
	keyvalue := secret.Data[keyname]
	if len(keyvalue) == 0 {
		return []byte{}, fmt.Errorf("key named %s not found in Secret %s at namespace %s", keyname, secret.Name, secret.Namespace)
	}

	return keyvalue, nil
}

func getSecret(r *SFController, secret sfv1.SecretRef) (apiv1.Secret, error) {
	if (secret.SecretKeyRef == sfv1.Secret{}) {
		return apiv1.Secret{}, fmt.Errorf("secretKeyRef must be defined")
	}

	if secret.SecretKeyRef.Name == "" || secret.SecretKeyRef.Key == "" {
		return apiv1.Secret{}, fmt.Errorf("secretKeyRef.name or secretKeyRef.key must be defined")
	}

	return r.GetSecretbyNameRef(secret.SecretKeyRef.Name)
}

func getValueFromSecret(r *SFController, secretref sfv1.SecretRef) ([]byte, error) {
	secret, err := getSecret(r, secretref)
	if err != nil {
		r.log.V(1).Error(err, "Secret not found")
		return []byte{}, err
	}
	secretvalue, err := getValueFromKeySecret(secret, secretref.SecretKeyRef.Key)
	if err != nil {
		r.log.V(1).Error(err, "Key not found")
		return []byte{}, err
	}
	return secretvalue, nil
}

func GenerateConfigFile(r *SFController, spec sfv1.MurmurSpec) string {
	MURMUR_CONF_HEADER := `
	max_bandwidth = 48000;
	welcometext = "{{ murmur_welcome_text }}";
	certificate = "/var/lib/umurmurd/tls.crt";
	private_key = "/var/lib/umurmurd/tls.key";
	password = "{{ murmur_password }}";
	max_users = {{ murmur_max_users }};

	logfile = "/var/log/umurmurd/umurmurd.log";
	channels = ( {
			 name = "Root";
			 parent = "";
			 description = "Root channel. No entry.";
			 noenter = true;
			 },
			 {
			 name = "Lobby";
			 parent = "Root";
			 description = "Lobby channel";
			 },
			 {
			 name = "Silent";
			 parent = "Root";
			 description = "Silent channel";
			 silent = true; # Optional. Default is false
			 }`

	configText := ""

	welcometext := "Welcome to SF Murmur"
	if spec.WelcomeText != "" {
		welcometext = spec.WelcomeText
	}
	configText = strings.Replace(MURMUR_CONF_HEADER, "{{ murmur_welcome_text }}", welcometext, 1)

	password := ""
	if (spec.Password != sfv1.SecretRef{}) {
		secretvalue, err := getValueFromSecret(r, spec.Password)
		if err != nil {
			r.log.V(1).Error(err, "Wrong Secret definition")
			r.log.V(1).Info("Murmur Server Password will be set to default")
		} else {
			password = string(secretvalue)
		}
	}
	configText = strings.Replace(configText, "{{ murmur_password }}", password, 1)

	maxusers := 42
	if spec.Maxusers != 0 {
		maxusers = spec.Maxusers
	}
	configText = strings.Replace(configText, "{{ murmur_max_users }}", strconv.FormatInt(int64(maxusers), 10), 1)

	if spec.Channels != nil {
		for _, channel := range spec.Channels {
			configText = configText + " , " + murmur_channel_config(r, channel)
		}
	}

	configText = configText + `
	);

      default_channel = "Lobby";
	`

	return configText
}

func (r *SFController) DeployMurmur(spec sfv1.MurmurSpec) bool {

	if spec.Enabled {

		// Creating Murmur Probe script as a ConfigMap
		cm_probe_data := make(map[string]string)
		cm_probe_data["cmdProbe.sh"] = murmurCmdProbe
		r.EnsureConfigMap(MURMUR_IDENT+"-probe", cm_probe_data)

		// Creating Murmur Configuration file as a ConfigMap
		cm_config := make(map[string]string)
		cm_config["umurmur.conf"] = GenerateConfigFile(r, spec)
		r.EnsureConfigMap(MURMUR_IDENT+"-umurmurd", cm_config)

		// Generating murmur Passwords
		r.GenerateSecretUUID("murmur-session-key")

		dep := create_deployment(r.ns, MURMUR_IDENT, MURMUR_IMAGE)
		dep.Spec.Template.Spec.Containers[0].Command = []string{
			"umurmurd", "-d", "-c", "/etc/umurmur/umurmur.conf"}
		dep.Spec.Template.Spec.Containers[0].Env = []apiv1.EnvVar{
			create_secret_env("MURMUR_SESSION_KEY", "murmur-session-key",
				"murmur-session-key"),
		}

		dep.Spec.Template.Spec.Containers[0].Ports = []apiv1.ContainerPort{
			create_container_port(MURMUR_PORT, MURMUR_PORT_NAME),
		}

		dep.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      MURMUR_IDENT + "-config-vol",
				MountPath: "/etc/umurmur",
			},
			{
				Name:      MURMUR_IDENT + "-probe",
				MountPath: "/home/umurmurd/bin/",
			},
		}

		dep.Spec.Template.Spec.Volumes = []apiv1.Volume{
			create_volume_cm(MURMUR_IDENT+"-config-vol", MURMUR_IDENT+"-umurmurd-config-map"),
			create_volume_cm(MURMUR_IDENT+"-probe", MURMUR_IDENT+"-probe-config-map"),
		}

		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_http_probe("/", MURMUR_PORT)
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_tcp_probe(MURMUR_PORT)
		dep.Spec.Template.Spec.Containers[0].ReadinessProbe = create_readiness_cmd_probe([]string{
			"bash",
			"/home/umurmurd/bin/cmdProbe.sh",
		})

		r.GetOrCreate(&dep)
		srv := create_service(r.ns, MURMUR_IDENT, MURMUR_IDENT, MURMUR_PORT, MURMUR_PORT_NAME)
		r.GetOrCreate(&srv)

		return r.IsDeploymentReady(&dep)
	} else {
		r.DeleteDeployment(MURMUR_IDENT)
		r.DeleteService(MURMUR_PORT_NAME)
		r.DeleteSecret("murmur-session-key")
		r.DeleteSecret("murmur-db-password")
		r.DeleteConfigMap("murmur-umurmurd-config-map")
		r.DeleteConfigMap("murmur-probe-config-map")
		return true
	}
}

func (r *SFController) IngressMurmur() netv1.IngressRule {
	fmt.Println(MURMUR_IDENT + "." + r.cr.Spec.FQDN)
	return create_ingress_rule(MURMUR_IDENT+"."+r.cr.Spec.FQDN, MURMUR_PORT_NAME, MURMUR_PORT)
}
