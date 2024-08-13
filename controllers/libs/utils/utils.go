// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0

// Package utils provides various utility functions
package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"gopkg.in/ini.v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

// GetEnvVarValue returns the value of the named env var. Return an empty string when not found.
func GetEnvVarValue(varName string) (string, error) {
	ns, found := os.LookupEnv(varName)
	if !found {
		return "", fmt.Errorf("%s unable to find env var", varName)
	}
	return ns, nil
}

func Int32Ptr(i int32) *int32 { return &i }
func BoolPtr(b bool) *bool    { return &b }

var Execmod int32 = 493 // decimal for 0755 octal
var Readmod int32 = 256 // decimal for 0400 octal

// LogI logs a message with the INFO log Level
func LogI(msg string) {
	ctrl.Log.Info(msg)
}

// LogD logs a message with the DEBUG log Level
func LogD(msg string) {
	ctrl.Log.V(1).Info(msg)
}

// LogE logs a message with the Error log Level
func LogE(err error, msg string) {
	ctrl.Log.Error(err, msg)
}

// NewUUIDString produce a UUID
func NewUUIDString() string {
	return uuid.New().String()
}

// Qty1Gi produces a Quantity of 1GB value
func Qty1Gi() resource.Quantity {
	q, _ := resource.ParseQuantity("1Gi")
	return q
}

// Checksum returns a SHA256 checksum as string
func Checksum(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// ParseString function to easilly use templated string.
//
// Pass the template text.
// And the data structure to be applied to the template
func ParseString(text string, data any) (string, error) {
	// Create Template object
	templateBody, err := template.New("StringtoParse").Parse(text)
	if err != nil {
		return "", fmt.Errorf("Text not in the right format: " + text)
	}

	// Parsing Template
	var buf bytes.Buffer
	err = templateBody.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failure while parsing template %s", text)
	}

	return buf.String(), nil
}

func MapEquals(m1 *map[string]string, m2 *map[string]string) bool {
	return reflect.DeepEqual(m1, m2)
}

type SSHKey struct {
	Pub  []byte
	Priv []byte
}

func MkSSHKey() SSHKey {
	bitSize := 4096

	generatePrivateKey := func(bitSize int) (*rsa.PrivateKey, error) {
		// Private Key generation
		privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
		if err != nil {
			return nil, err
		}
		// Validate Private Key
		err = privateKey.Validate()
		if err != nil {
			return nil, err
		}
		return privateKey, nil
	}

	generatePublicKey := func(privatekey *rsa.PublicKey) ([]byte, error) {
		publicRsaKey, err := ssh.NewPublicKey(privatekey)
		if err != nil {
			return nil, err
		}

		pubKeyBytes := ssh.MarshalAuthorizedKey(publicRsaKey)

		return pubKeyBytes, nil
	}

	encodePrivateKeyToPEM := func(privateKey *rsa.PrivateKey) []byte {
		// Get ASN.1 DER format
		privDER := x509.MarshalPKCS1PrivateKey(privateKey)

		// pem.Block
		privBlock := pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   privDER,
		}

		// Private key in PEM format
		privatePEM := pem.EncodeToMemory(&privBlock)

		return privatePEM
	}

	privateKey, err := generatePrivateKey(bitSize)
	if err != nil {
		panic(err.Error())
	}

	publicKeyBytes, err := generatePublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(err.Error())
	}

	privateKeyBytes := encodePrivateKeyToPEM(privateKey)

	return SSHKey{
		Pub:  publicKeyBytes,
		Priv: privateKeyBytes,
	}
}

// IniSectionsChecksum takes one or more section name and compute a checkum
func IniSectionsChecksum(cfg *ini.File, names []string) string {

	var IniGetSectionBody = func(cfg *ini.File, section *ini.Section) string {
		var s = ""
		keys := section.KeyStrings()
		sort.Strings(keys)
		for _, k := range keys {
			s = s + k + section.Key(k).String()
		}
		return s
	}

	var data = ""
	for _, name := range names {
		section, err := cfg.GetSection(name)
		if err != nil {
			panic("No such ini section: " + name)
		}
		data += IniGetSectionBody(cfg, section)
	}

	return Checksum([]byte(data))
}

// IniGetSectionNamesByPrefix gets Ini section names filtered by prefix
func IniGetSectionNamesByPrefix(cfg *ini.File, prefix string) []string {
	filteredNames := []string{}
	names := cfg.SectionStrings()
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			filteredNames = append(filteredNames, n)
		}
	}
	return filteredNames
}
