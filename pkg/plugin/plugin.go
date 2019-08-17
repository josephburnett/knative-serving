package plugin

import (
	"time"
	"context"
	
	"github.com/josephburnett/sk-plugin/pkg/skplug/proto"
	"knative.dev/serving/pkg/autoscaler"
	"k8s.io/apimachinery/pkg/types"
)

type Autoscaler struct {
	autoscaler *autoscaler.Autoscaler
	collector *autoscaler.MetricCollector
}

func NewAutoscaler() *Autoscaler {
	// TODO: construct Knative Autoscaler and MetricCollector.
	return &Autoscaler{}
}

func (a *Autoscaler) Stat(stats []*proto.Stat) error {
	for _, s := range stats {
		if s.Type != proto.MetricType_CPU_MILLIS {
			continue
		}
		t := time.Unix(0, s.Time)
		stat := autoscaler.Stat{
			Time: &t,
			PodName: s.PodName,
			AverageConcurrentRequests: float64(s.Value),
		}
		a.collector.Record(types.NamespacedName{
			Namespace: "default",
			Name: "autoscaler",
		}, stat)
	}
	return nil
}

func (a *Autoscaler) Scale(now int64) (int32, error) {
	desiredPodCount, _, _ := a.autoscaler.Scale(context.TODO(), time.Unix(0, now))
	return desiredPodCount, nil
}

