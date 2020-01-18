#!/bin/sh -e
INPUT=$(yaml-to-dhall "(./conf/sf/applications/SoftwareFactory.dhall).Input.Type" < ci/cr_spec.yaml)
dhall-to-yaml --omit-empty --explain <<< "./conf/operator/deploy/Kubernetes.dhall ((./conf/sf/applications/SoftwareFactory.dhall).Application ($INPUT))"
