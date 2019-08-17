package plugin

import "knative.dev/serving/pkg/autoscaler"

type Autoscaler struct {
	autoscaler *autoscaler.Autoscaler
	collector *autoscaler.MetricCollector
}

func NewAutoscaler() &Autoscaler {
	// TODO: construct Knative Autoscaler and MetricCollector.
	return &Autoscaler{}
}

func (a *Autoscaler) Stat(stats []*proto.Stat) error {
	for _, s := range stats {
		if s.Type != proto.
		t := time.Unix(0, s.Time)
		stat := autoscaler.Stat{
			Time: &t,
			PodName: s.PodName,
			AverageConcurrentRequests: s.Value,
		}
		collector.Record(types.NamespacedName{
			Namespace: "default",
			Name: "autoscaler",
		}, stat)
	}
}

func (a *Autoscaler) Scale(now int64) (int32, error) {
	desiredPodCount, _, err := a.autoscaler.Scale(context.TODO(), time.Unix(0, now))
	return desiredPodCount, err
}

