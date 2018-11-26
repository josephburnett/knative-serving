/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"github.com/knative/serving/pkg/apis/autoscaling/v1alpha1"
	"github.com/knative/serving/pkg/reconciler/v1alpha1/revision/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta1"
)

func MakeVPA(pa *v1alpha1.PodAutoscaler) *v1beta1.VerticalPodAutoscaler {
	updateMode := v1beta1.UpdateModeAuto
	scalingMode := v1beta1.ContainerScalingModeAuto
	return &v1beta1.VerticalPodAutoscaler{
		ObjectMeta: pa.ObjectMeta,
		Spec: v1beta1.VerticalPodAutoscalerSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: pa.Labels,
			},
			UpdatePolicy: &v1beta1.PodUpdatePolicy{
				UpdateMode: &updateMode,
			},
			ResourcePolicy: &v1beta1.PodResourcePolicy{
				ContainerPolicies: []v1beta1.ContainerResourcePolicy{
					{
						ContainerName: resources.UserContainerName,
						Mode:          &scalingMode,
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resources.UserContainerMaxCPU,
							corev1.ResourceMemory: resources.UserContainerMaxMemory,
						},
					},
					{
						ContainerName: resources.FluentdContainerName,
						Mode:          &scalingMode,
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resources.FluentdContainerMaxCPU,
							corev1.ResourceMemory: resources.FluentdContainerMaxMemory,
						},
					},
					{
						ContainerName: resources.EnvoyContainerName,
						Mode:          &scalingMode,
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resources.EnvoyContainerMaxCPU,
							corev1.ResourceMemory: resources.EnvoyContainerMaxMemory,
						},
					},
					{
						ContainerName: resources.QueueContainerName,
						Mode:          &scalingMode,
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resources.QueueContainerMaxCPU,
							corev1.ResourceMemory: resources.QueueContainerMaxMemory,
						},
					},
				},
			},
		},
	}
}
