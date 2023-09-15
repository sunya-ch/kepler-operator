/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package estimator

import (
	"fmt"
	"strings"

	"github.com/sustainable.computing.io/kepler-operator/pkg/api/v1alpha1"
	"github.com/sustainable.computing.io/kepler-operator/pkg/utils/k8s"

	corev1 "k8s.io/api/core/v1"
)

const (
	prefix = "estimator-"
	// exported
	PVCName = prefix + "pvc"

	StableImage          = "quay.io/sustainable_computing_io/kepler_model_server:v0.6"
	waitForSocketCommand = "until [ -e /tmp/estimator.sock ]; do sleep 1; done && %s"
)

// Config that will be set from outside
var (
	Config = struct {
		Image string
	}{
		Image: StableImage,
	}
)

var (
	bashCommand = []string{"/bin/sh", "-c"}
)

// NeedsEstimatorSidecar returns true if any of estimator config has sidecar enabled
func NeedsEstimatorSidecar(es *v1alpha1.EstimatorSpec) bool {

	if es.Node.Total != nil && es.Node.Total.SidecarEnabled {
		return true
	}
	if es.Node.Components != nil && es.Node.Components.SidecarEnabled {
		return true
	}
	if es.Container.Total != nil && es.Container.Total.SidecarEnabled {
		return true
	}
	if es.Container.Components != nil && es.Container.Components.SidecarEnabled {
		return true
	}

	return false
}

// NewContainer returns sidecar container
func NewContainer() corev1.Container {
	mounts := []corev1.VolumeMount{{
		Name:      "cfm",
		MountPath: "/etc/kepler/kepler.config",
		ReadOnly:  true,
	}, {
		Name:      "mnt",
		MountPath: "/mnt",
	}, {
		Name:      "tmp",
		MountPath: "/tmp",
	}}

	return corev1.Container{
		Image:           Config.Image,
		ImagePullPolicy: corev1.PullAlways,
		Name:            "estimator",
		VolumeMounts:    mounts,
		Command:         []string{"python3.8"},
		Args:            []string{"-u", "src/estimate/estimator.py"},
	}
}

// NewContainer returns sidecar additional volumes
func NewVolumes() []corev1.Volume {
	return []corev1.Volume{
		k8s.VolumeFromEmptyDir("mnt"),
		k8s.VolumeFromEmptyDir("tmp"),
	}
}

func addTmpMount(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	return append(volumeMounts, corev1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	})
}

func addSocketWaitCmd(exporterContainer *corev1.Container) *corev1.Container {
	cmd := exporterContainer.Command
	exporterContainer.Command = bashCommand
	exporterContainer.Args = []string{fmt.Sprintf(waitForSocketCommand, strings.Join(cmd, " "))}
	return exporterContainer
}

func AddEstimatorDependency(exporterContainer *corev1.Container) *corev1.Container {
	exporterContainer = addSocketWaitCmd(exporterContainer)
	exporterContainer.VolumeMounts = addTmpMount(exporterContainer.VolumeMounts)
	return exporterContainer
}

func estimatorConfig(prefix string, spec v1alpha1.EstimatorConfig) string {
	config := fmt.Sprintf("%s_ESTIMATOR=%v\n", prefix, spec.SidecarEnabled)
	if spec.InitUrl != "" {
		config += fmt.Sprintf("%s_INIT_URL=%s\n", prefix, spec.InitUrl)
	}
	return config
}

func ModelConfig(es *v1alpha1.EstimatorSpec) string {
	config := ""

	if es.Node.Total != nil {
		config += estimatorConfig("NODE_TOTAL", *es.Node.Total)
	}
	if es.Node.Components != nil {
		config += estimatorConfig("NODE_COMPONENTS", *es.Node.Components)
	}
	if es.Container.Total != nil {
		config += estimatorConfig("CONTAINER_TOTAL", *es.Container.Total)
	}
	if es.Container.Components != nil {
		config += estimatorConfig("CONTAINER_COMPONENTS", *es.Node.Components)
	}

	return config
}
