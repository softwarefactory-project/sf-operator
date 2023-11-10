// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//

package base

type Image struct {
	Path    string
	Version string
}

func ImageToString(i Image) string {
	return i.Path + ":" + i.Version
}

const (
	nodepoolImageVersion = "9.0.0-6"
	zuulImageVersion     = "9.2.0-1"
)

var (
	BusyboxImage          = ImageToString(Image{Path: "quay.io/software-factory/sf-op-busybox", Version: "1.5-3"})
	GerritImage           = ImageToString(Image{Path: "quay.io/software-factory/gerrit", Version: "3.6.4-8"})
	GitServerImage        = ImageToString(Image{Path: "quay.io/software-factory/git-deamon", Version: "2.39.1-3"})
	SSHDImage             = ImageToString(Image{Path: "quay.io/software-factory/sshd", Version: "0.1-2"})
	PurgeLogsImage        = ImageToString(Image{Path: "quay.io/software-factory/purgelogs", Version: "0.2.3-2"})
	NodepoolLauncherImage = ImageToString(Image{Path: "quay.io/software-factory/nodepool-launcher", Version: nodepoolImageVersion})
	NodepoolBuilderImage  = ImageToString(Image{Path: "quay.io/software-factory/nodepool-builder", Version: nodepoolImageVersion})
	MariabDBImage         = ImageToString(Image{Path: "quay.io/software-factory/mariadb", Version: "10.5.16-4"})
	ZookeeperImage        = ImageToString(Image{Path: "quay.io/software-factory/zookeeper", Version: "3.8.3-1"})
	// https://catalog.redhat.com/software/containers/ubi8/httpd-24/6065b844aee24f523c207943?q=httpd&architecture=amd64&image=651f274c8ce9242f7bb3e011
	HTTPDImage          = ImageToString(Image{Path: "registry.access.redhat.com/ubi8/httpd-24", Version: "1-284.1696531168"})
	NodeExporterImage   = ImageToString(Image{Path: "quay.io/prometheus/node-exporter", Version: "v1.6.1"})
	StatsdExporterImage = ImageToString(Image{Path: "quay.io/prometheus/statsd-exporter", Version: "v0.24.0"})
)

func ZuulImage(service string) string {
	return ImageToString(Image{Path: "quay.io/software-factory/" + service, Version: zuulImageVersion})
}
