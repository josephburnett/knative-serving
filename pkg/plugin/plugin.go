package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/josephburnett/sk-plugin/pkg/skplug"
	"github.com/josephburnett/sk-plugin/pkg/skplug/proto"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/serving/pkg/apis/autoscaling/v1alpha1"
	av1alpha1 "knative.dev/serving/pkg/apis/autoscaling/v1alpha1"
	"knative.dev/serving/pkg/autoscaler"
	"knative.dev/serving/pkg/resources"
)

type Autoscaler struct {
	autoscaler *autoscaler.Autoscaler
	collector  *autoscaler.MetricCollector
	pods       map[string]*skplug.Pod
}

func NewAutoscaler(yaml string) (*Autoscaler, error) {
	pods := make(map[string]*skplug.Pod)
	c := autoscaler.NewMetricCollector(fakeStatsScraperFactory, zap.NewNop().Sugar())
	// TODO: create Metric from PodAutoscaler.
	metric := &av1alpha1.Metric{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "autoscaler",
		},
		Spec: v1alpha1.MetricSpec{
			StableWindow: 60 * time.Second,
			PanicWindow:  6 * time.Second,
			ScrapeTarget: "service",
		},
	}
	err := c.CreateOrUpdate(metric)
	if err != nil {
		return nil, err
	}
	a, err := autoscaler.New(
		"default",
		"autoscaler",
		c,
		&fakeReadyPodCounter{
			pods: pods,
		}, // TODO: provide count of pods
		autoscaler.DeciderSpec{
			TickInterval:      2 * time.Second,
			MaxScaleUpRate:    1000.0,
			TargetConcurrency: 10.0,
			TotalConcurrency:  10.0,
			PanicThreshold:    20.0,
			StableWindow:      60 * time.Second,
			ServiceName:       "service",
		}, // TODO: create Decider from PodAutoscaler.
		&fakeStatsReporter{},
	)
	if err != nil {
		return nil, err
	}
	return &Autoscaler{
		autoscaler: a,
		collector:  c,
		pods:       pods,
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

var _ autoscaler.StatsScraper = &fakeStatsScraper{}

type fakeStatsScraper struct{}

func (f *fakeStatsScraper) Scrape() (*autoscaler.StatMessage, error) {
	// Don't scrape. Empty messages are ignored.
	// We record stats when provided by the plugin interface.
	return nil, nil
}

func fakeStatsScraperFactory(*av1alpha1.Metric) (autoscaler.StatsScraper, error) {
	return &fakeStatsScraper{}, nil
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
	pods map[string]*skplug.Pod
}

func (f *fakeReadyPodCounter) ReadyCount() (int, error) {
	// TODO: count ready pods.
	return len(f.pods), nil
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
