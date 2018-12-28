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
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/serving/pkg/apis/autoscaling"
	"github.com/knative/serving/pkg/apis/autoscaling/v1alpha1"
	serving "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/knative/serving/pkg/autoscaler"
	autoscalerConfig "github.com/knative/serving/pkg/autoscaler/config"
	. "github.com/knative/serving/pkg/reconciler/v1alpha1/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeMetric(t *testing.T) {
	cases := []struct {
		name string
		pa   *v1alpha1.PodAutoscaler
		want *autoscaler.Metric
	}{{
		name: "defaults",
		pa:   pa(),
		want: metric(),
	}, {
		name: "with container concurrency 1",
		pa: pa(
			WithContainerConcurrency(1)),
		want: metric(
			withTarget(1.0),
			withTargetPanic(2.0)),
	}, {
		name: "with target annotation 1",
		pa: pa(
			WithTargetAnnotation("1")),
		want: metric(
			withTarget(1.0),
			withTargetPanic(2.0),
			withTargetAnnotation("1")),
	}, {
		name: "with container concurrency greater than target annotation (ok)",
		pa: pa(
			WithContainerConcurrency(10),
			WithTargetAnnotation("1")),
		want: metric(
			withTarget(1.0),
			withTargetPanic(2.0),
			withTargetAnnotation("1")),
	}, {
		name: "with target annotation greater than container concurrency (ignore annotation for safety)",
		pa: pa(
			WithContainerConcurrency(1),
			WithTargetAnnotation("10")),
		want: metric(
			withTarget(1.0),
			withTargetPanic(2.0),
			withTargetAnnotation("10")),
	}, {
		name: "with longer window, no panic percentage (defaults to backward-compatible 6s)",
		pa: pa(
			WithWindowAnnotation("120s")),
		want: metric(
			withWindow(120*time.Second),
			withWindowPanic(6*time.Second),
			withWindowAnnotation("120s")),
	}, {
		name: "with longer window, default panic percentage of 10%",
		pa: pa(
			WithWindowAnnotation("120s"),
			WithWindowPanicPercentageAnnotation("10.0")),
		want: metric(
			withWindow(120*time.Second),
			withWindowPanic(12*time.Second),
			withWindowAnnotation("120s"),
			withWindowPanicPercentageAnnotation("10.0")),
	}, {
		name: "with higher panic target",
		pa: pa(
			WithTargetAnnotation("1"),
			WithTargetPanicPercentageAnnotation("400.0")),
		want: metric(
			withTarget(1.0),
			withTargetPanic(4.0),
			withTargetAnnotation("1"),
			withTargetPanicPercentageAnnotation("400.0")),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, MakeMetric(context.TODO(), tc.pa, config)); diff != "" {
				t.Errorf("%q (-want, +got):\n%v", tc.name, diff)
			}
		})
	}
}

func pa(options ...PodAutoscalerOption) *v1alpha1.PodAutoscaler {
	p := &v1alpha1.PodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-name",
			Annotations: map[string]string{
				autoscaling.ClassAnnotationKey: autoscaling.KPA,
			},
		},
		Spec: v1alpha1.PodAutoscalerSpec{
			ContainerConcurrency: serving.RevisionContainerConcurrencyType(0),
		},
		Status: v1alpha1.PodAutoscalerStatus{},
	}
	for _, fn := range options {
		fn(p)
	}
	return p
}

func metric(options ...MetricOption) *autoscaler.Metric {
	m := &autoscaler.Metric{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
			Name:      "test-name",
			Annotations: map[string]string{
				autoscaling.ClassAnnotationKey: autoscaling.KPA,
			},
		},
		Spec: autoscaler.MetricSpec{
			TargetConcurrency:      float64(100),
			TargetConcurrencyPanic: float64(200),
			Window:                 time.Second * 60,
			WindowPanic:            time.Second * 6,
		},
	}
	for _, fn := range options {
		fn(m)
	}
	return m
}

type MetricOption func(*autoscaler.Metric)

func withTarget(target float64) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Spec.TargetConcurrency = target
	}
}

func withWindow(window time.Duration) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Spec.Window = window
	}
}

func withTargetPanic(target float64) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Spec.TargetConcurrencyPanic = target
	}
}

func withWindowPanic(window time.Duration) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Spec.WindowPanic = window
	}
}

func withTargetAnnotation(target string) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Annotations[autoscaling.TargetAnnotationKey] = target
	}
}

func withWindowAnnotation(window string) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Annotations[autoscaling.WindowAnnotationKey] = window
	}
}

func withTargetPanicPercentageAnnotation(percentage string) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Annotations[autoscaling.TargetPanicPercentageAnnotationKey] = percentage
	}
}

func withWindowPanicPercentageAnnotation(percentage string) MetricOption {
	return func(metric *autoscaler.Metric) {
		metric.Annotations[autoscaling.WindowPanicPercentageAnnotationKey] = percentage
	}
}

var config = &autoscalerConfig.Config{
	EnableScaleToZero:                    true,
	ContainerConcurrencyTargetPercentage: 1.0,
	ContainerConcurrencyTargetDefault:    100.0,
	MaxScaleUpRate:                       10.0,
	StableWindow:                         60 * time.Second,
	PanicWindow:                          6 * time.Second,
	WindowPanicPercentage:                10.0,
	TargetPanicPercentage:                200.0,
	TickInterval:                         2 * time.Second,
	ScaleToZeroGracePeriod:               30 * time.Second,
}
