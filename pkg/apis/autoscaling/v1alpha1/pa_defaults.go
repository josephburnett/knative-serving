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

package v1alpha1

import (
	"strconv"

	"github.com/knative/serving/pkg/apis/autoscaling"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/autoscaler/config"
)

// github.com/knative/pkg/webhook doesn't pass a configmap through to
// SetDefaults so we use package-level defaults set by main.go as a
// side-channel.
var (
	AutoscalerConfig *config.DynamicConfig
)

func (r *PodAutoscaler) SetDefaults() {
	r.Spec.SetDefaults()
	if r.Annotations == nil {
		r.Annotations = make(map[string]string)
	}
	if _, ok := r.Annotations[autoscaling.ClassAnnotationKey]; !ok {
		// Default class to KPA.
		r.Annotations[autoscaling.ClassAnnotationKey] = autoscaling.KPA
	}
	// Default metric per class.
	switch r.Class() {
	case autoscaling.KPA:
		if _, ok := r.Annotations[autoscaling.MetricAnnotationKey]; !ok {
			r.Annotations[autoscaling.MetricAnnotationKey] = autoscaling.Concurrency
		}
		// KPA specific defaults.
		if AutoscalerConfig != nil {
			current := AutoscalerConfig.Current()
			window := autoscaling.WindowAnnotationKey
			target := autoscaling.TargetAnnotationKey
			wpanic := autoscaling.WindowPanicPercentageAnnotationKey
			tpanic := autoscaling.TargetPanicPercentageAnnotationKey
			if _, ok := r.Annotations[window]; !ok && AutoscalerConfig != nil {
				r.Annotations[window] = current.StableWindow.String()
			}
			if _, ok := r.Annotations[target]; !ok && AutoscalerConfig != nil {
				r.Annotations[target] = asString(current.ContainerConcurrencyTargetDefault)
			}
			if _, ok := r.Annotations[wpanic]; !ok && AutoscalerConfig != nil {
				r.Annotations[wpanic] = asString(current.WindowPanicPercentage)
			}
			if _, ok := r.Annotations[tpanic]; !ok && AutoscalerConfig != nil {
				r.Annotations[tpanic] = asString(current.TargetPanicPercentage)
			}
		}
	case autoscaling.HPA:
		if _, ok := r.Annotations[autoscaling.MetricAnnotationKey]; !ok {
			r.Annotations[autoscaling.MetricAnnotationKey] = autoscaling.CPU
		}
	}
}

func asString(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (rs *PodAutoscalerSpec) SetDefaults() {
	// When ConcurrencyModel is specified but ContainerConcurrency
	// is not (0), use the ConcurrencyModel value.
	if rs.ConcurrencyModel == servingv1alpha1.RevisionRequestConcurrencyModelSingle && rs.ContainerConcurrency == 0 {
		rs.ContainerConcurrency = 1
	}
}
