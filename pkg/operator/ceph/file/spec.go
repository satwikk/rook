/*
Copyright 2016 The Rook Authors. All rights reserved.

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

package file

import (
	"strconv"

	"github.com/rook/rook/pkg/operator/ceph/config"
	opspec "github.com/rook/rook/pkg/operator/ceph/spec"
	"github.com/rook/rook/pkg/operator/k8sutil"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mdsDaemonCommand = "ceph-mds"
)

func (c *cluster) makeDeployment(mdsConfig *mdsConfig) *apps.Deployment {
	podSpec := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        mdsConfig.ResourceName,
			Labels:      c.podLabels(mdsConfig),
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{},
			Containers: []v1.Container{
				c.makeMdsDaemonContainer(mdsConfig),
			},
			RestartPolicy: v1.RestartPolicyAlways,
			Volumes:       opspec.DaemonVolumes(mdsConfig.DataPathMap, mdsConfig.ResourceName),
			HostNetwork:   c.HostNetwork,
		},
	}
	if c.HostNetwork {
		podSpec.Spec.DNSPolicy = v1.DNSClusterFirstWithHostNet
	}
	c.fs.Spec.MetadataServer.Placement.ApplyToPodSpec(&podSpec.Spec)

	replicas := int32(1)
	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mdsConfig.ResourceName,
			Namespace: c.fs.Namespace,
			Labels:    c.podLabels(mdsConfig),
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: podSpec.Labels,
			},
			Template: podSpec,
			Replicas: &replicas,
			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
		},
	}
	k8sutil.SetOwnerRefs(c.context.Clientset, c.fs.Namespace, &d.ObjectMeta, c.ownerRefs)
	return d
}

func (c *cluster) makeMdsDaemonContainer(mdsConfig *mdsConfig) v1.Container {
	return v1.Container{
		Name: "mds",
		Command: []string{
			"ceph-mds",
		},
		Args: append(
			opspec.DaemonFlags(c.clusterInfo, config.MdsType, mdsConfig.DaemonID),
			"--foreground",
			config.NewFlag("mds-standby-for-fscid", c.fsID),
			config.NewFlag("mds-standby-replay", strconv.FormatBool(c.fs.Spec.MetadataServer.ActiveStandby)),
		),
		Image:        c.cephVersion.Image,
		VolumeMounts: opspec.DaemonVolumeMounts(mdsConfig.DataPathMap, mdsConfig.ResourceName),
		Env: append(
			opspec.DaemonEnvVars(c.cephVersion.Image),
		),
		Resources: c.fs.Spec.MetadataServer.Resources,
	}
}

func (c *cluster) podLabels(mdsConfig *mdsConfig) map[string]string {
	labels := opspec.PodLabels(AppName, c.fs.Namespace, "mds", mdsConfig.DaemonID)
	labels["rook_file_system"] = c.fs.Name
	return labels
}
