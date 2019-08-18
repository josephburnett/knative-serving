package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/josephburnett/sk-plugin/pkg/skplug"
	"github.com/josephburnett/sk-plugin/pkg/skplug/proto"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/serving/pkg/autoscaler"
	"knative.dev/serving/pkg/resources"
)

type Autoscaler struct {
	autoscaler *autoscaler.Autoscaler
	collector  *autoscaler.MetricCollector
	pods       map[string]*skplug.Pod
}

func NewAutoscaler(yaml string) (*Autoscaler, error) {
	// TODO: construct metric collector
	a, err := autoscaler.New(
		"default",
		"autoscaler",
		&fakeMetricClient{},      // TODO: provide metric collector
		&fakeReadyPodCounter{},   // TODO: provide count of pods
		autoscaler.DeciderSpec{}, // TODO: parse PodAutoscaler
		&fakeStatsReporter{},
	)
	if err != nil {
		return nil, err
	}
	return &Autoscaler{
		autoscaler: a,
		pods:       make(map[string]*skplug.Pod),
	}, nil
}

func (a *Autoscaler) Stat(stats []*proto.Stat) error {
	for _, s := range stats {
		if s.Type != proto.MetricType_CPU_MILLIS {
			continue
		}
		t := time.Unix(0, s.Time)
		stat := autoscaler.Stat{
			Time:                      &t,
			PodName:                   s.PodName,
			AverageConcurrentRequests: float64(s.Value),
		}
		a.collector.Record(types.NamespacedName{
			Namespace: "default",
			Name:      "autoscaler",
		}, stat)
	}
	return nil
}

func (a *Autoscaler) Scale(now int64) (int32, error) {
	desiredPodCount, _, _ := a.autoscaler.Scale(context.TODO(), time.Unix(0, now))
	return desiredPodCount, nil
}

func (a *Autoscaler) CreatePod(p *skplug.Pod) error {
	if _, ok := a.pods[p.Name]; ok {
		return fmt.Errorf("duplicate create pod: %v", p.Name)
	}
	a.pods[p.Name] = p
	return nil
}

func (a *Autoscaler) UpdatePod(p *skplug.Pod) error {
	if _, ok := a.pods[p.Name]; !ok {
		return fmt.Errorf("update non-existant pod: %v", p.Name)
	}
	a.pods[p.Name] = p
	return nil
}

func (a *Autoscaler) DeletePod(p *skplug.Pod) error {
	if _, ok := a.pods[p.Name]; !ok {
		return fmt.Errorf("delete non-existant pod: %v", p.Name)
	}
	delete(a.pods, p.Name)
	return nil
}

var _ autoscaler.MetricClient = &fakeMetricClient{}

type fakeMetricClient struct {
	collector *autoscaler.MetricCollector
}

func (f *fakeMetricClient) StableAndPanicConcurrency(key types.NamespacedName, now time.Time) (float64, float64, error) {
	// TODO: report concurrency metrics collected by collector.
	return 1.0, 1.0, nil
}

func (f *fakeMetricClient) StableAndPanicOPS(key types.NamespacedName, now time.Time) (float64, float64, error) {
	return 1.0, 1.0, nil
}

var _ resources.ReadyPodCounter = &fakeReadyPodCounter{}

type fakeReadyPodCounter struct {
}

func (f *fakeReadyPodCounter) ReadyCount() (int, error) {
	// TODO: keep track of pods.
	return 0, nil
}

var _ autoscaler.StatsReporter = &fakeStatsReporter{}

type fakeStatsReporter struct{}

func (f *fakeStatsReporter) ReportDesiredPodCount(v int64) error {
	return nil
}

func (f *fakeStatsReporter) ReportRequestedPodCount(v int64) error {
	return nil
}

func (f *fakeStatsReporter) ReportActualPodCount(v int64) error {
	return nil
}

func (f *fakeStatsReporter) ReportStableRequestConcurrency(v float64) error {
	return nil
}

func (f *fakeStatsReporter) ReportPanicRequestConcurrency(v float64) error {
	return nil
}

func (f *fakeStatsReporter) ReportTargetRequestConcurrency(v float64) error {
	return nil
}

func (f *fakeStatsReporter) ReportExcessBurstCapacity(v float64) error {
	return nil
}

func (f *fakeStatsReporter) ReportPanic(v int64) error {
	return nil
}
