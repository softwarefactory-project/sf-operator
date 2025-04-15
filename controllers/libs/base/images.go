// Copyright (C) 2023 Red Hat
// SPDX-License-Identifier: Apache-2.0
//

package base

import (
	_ "embed"
	"sort"

	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
)

//go:embed static/images.yaml
var imagesYAML string

type ContainerImages struct {
	Images []Image `yaml:"images"`
}

type Image struct {
	Name      string `yaml:"name"`
	Container string `yaml:"container"`
	Version   string `yaml:"version"`
	Source    string `yaml:"source,omitempty"`
}

func loadImages() ContainerImages {
	var images ContainerImages
	if err := yaml.UnmarshalStrict([]byte(imagesYAML), &images); err != nil {
		panic(err)
	}
	return images
}

func getImage(name string) string {
	images := loadImages()
	for _, image := range images.Images {
		if image.Name == name {
			return image.Container + ":" + image.Version

		}
	}
	panic("Unknown container image: " + name)
}

func GetSelfManagedImages() []Image {
	imagesByName := make(map[string]Image)
	ret := []Image{}
	images := loadImages()
	for _, image := range images.Images {
		if image.Source != "" {
			imagesByName[image.Name] = image
		}
	}
	imageNames := maps.Keys(imagesByName)
	sort.Strings(imageNames)
	for _, imageName := range imageNames {
		ret = append(ret, imagesByName[imageName])
	}
	return ret
}

func ZuulExecutorImage() string {
	return getImage("zuul-executor")
}

func ZuulMergerImage() string {
	return getImage("zuul-merger")
}

func ZuulSchedulerImage() string {
	return getImage("zuul-scheduler")
}

func ZuulWebImage() string {
	return getImage("zuul-web")
}

func NodepoolBuilderImage() string {
	return getImage("nodepool-builder")
}

func NodepoolLauncherImage() string {
	return getImage("nodepool-launcher")
}

func BusyboxImage() string {
	return getImage("busybox")
}

func GitServerImage() string {
	return getImage("git-server")
}

func SSHDImage() string {
	return getImage("sshd")
}

func PurgelogsImage() string {
	return getImage("purgelogs")
}

func MariaDBImage() string {
	return getImage("mariadb")
}

func ZookeeperImage() string {
	return getImage("zookeeper")
}

func ZuulCapacityImage() string {
	return getImage("zuul-capacity")
}

func ZuulWeederImage() string {
	return getImage("zuul-weeder")
}

func LogJuicerImage() string {
	return getImage("logjuicer")
}

func HTTPDImage() string {
	return getImage("httpd")
}

func NodeExporterImage() string {
	return getImage("node-exporter")
}

func StatsdExporterImage() string {
	return getImage("statsd-exporter")
}

func FluentBitImage() string {
	return getImage("fluentbit")
}
