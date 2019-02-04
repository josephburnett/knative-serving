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

package autoscaler

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/knative/pkg/logging"
	autoscalerConfig "github.com/knative/serving/pkg/autoscaler/config"
)

const (
	// ActivatorPodName defines the pod name of the activator
	// as defined in the metrics it sends.
	ActivatorPodName string = "activator"

	// If the latest received stat from a pod is in the last activeThreshold duration,
	// assume the pod is still active. Otherwise, the active status of a pod is
	// unknown.
	activeThreshold time.Duration = time.Second

	// Activator pod weight is always 1
	activatorPodWeight float64 = 1

	approximateZero = 1e-8
)

// Stat defines a single measurement at a point in time
type Stat struct {
	// The time the data point was collected on the pod.
	Time *time.Time

	// The unique identity of this pod.  Used to count how many pods
	// are contributing to the metrics.
	PodName string

	// Average number of requests currently being handled by this pod.
	AverageConcurrentRequests float64

	// Number of requests received since last Stat (approximately QPS).
	RequestCount int32

	// Lameduck indicates this Pod has received a shutdown signal.
	// Deprecated and no longer used by newly created Pods.
	LameDuck bool
}

// StatMessage wraps a Stat with identifying information so it can be routed
// to the correct receiver.
type StatMessage struct {
	Key  string
	Stat Stat
}

type statKey struct {
	podName string
	time    time.Time
}

// Creates a new totalAggregation
func newTotalAggregation() *totalAggregation {
	return &totalAggregation{
		perPodAggregations:  make(map[string]*perPodAggregation),
		activatorsContained: make(map[string]struct{}),
	}
}

// Holds an aggregation across all pods
type totalAggregation struct {
	perPodAggregations  map[string]*perPodAggregation
	probeCount          int32
	activatorsContained map[string]struct{}
}

// Aggregates a given stat to the correct pod-aggregation
func (agg *totalAggregation) aggregate(stat Stat) {
	current, exists := agg.perPodAggregations[stat.PodName]
	if !exists {
		current = &perPodAggregation{
			isActivator: isActivator(stat.PodName),
		}
		agg.perPodAggregations[stat.PodName] = current
	}
	current.aggregate(stat)
	if current.isActivator {
		agg.activatorsContained[stat.PodName] = struct{}{}
	}
	agg.probeCount++
}

// The number of pods that are observable via stats
// Subtracts the activator pod if its not the only pod reporting stats
func (agg *totalAggregation) observedPods(now time.Time, window time.Duration) float64 {
	podCount := float64(0.0)
	for _, pod := range agg.perPodAggregations {
		podCount += pod.podWeight(now, window)
	}

	activatorsCount := len(agg.activatorsContained)
	// Discount the activators in the pod count.
	if activatorsCount > 0 {
		discountedPodCount := podCount - float64(activatorsCount)
		// Report a minimum of 1 pod if the activators are sending metrics.
		if discountedPodCount < 1.0 {
			return 1.0
		}
		return discountedPodCount
	}
	return podCount
}

// The observed concurrency of a revision (sum of all average concurrencies of
// the observed pods)
// Ignores activator sent metrics if its not the only pod reporting stats
func (agg *totalAggregation) observedConcurrency(now time.Time, window time.Duration) float64 {
	accumulatedConcurrency := float64(0)
	activatorConcurrency := float64(0)
	for podName, perPod := range agg.perPodAggregations {
		if isActivator(podName) {
			activatorConcurrency += perPod.calculateAverage(now, window)
		} else {
			accumulatedConcurrency += perPod.calculateAverage(now, window)
		}
	}
	if accumulatedConcurrency < approximateZero {
		return activatorConcurrency
	}
	return accumulatedConcurrency
}

// The observed concurrency per pod (sum of all average concurrencies
// distributed over the observed pods)
// Ignores activator sent metrics if its not the only pod reporting stats
func (agg *totalAggregation) observedConcurrencyPerPod(now time.Time, window time.Duration) float64 {
	return divide(agg.observedConcurrency(now, window), agg.observedPods(now, window))
}

// Holds an aggregation per pod
type perPodAggregation struct {
	accumulatedConcurrency float64
	probeCount             int32
	latestStatTime         *time.Time
	isActivator            bool
}

// Aggregates the given concurrency
func (agg *perPodAggregation) aggregate(stat Stat) {
	agg.accumulatedConcurrency += stat.AverageConcurrentRequests
	agg.probeCount++
	if agg.latestStatTime == nil || agg.latestStatTime.Before(*stat.Time) {
		agg.latestStatTime = stat.Time
	}
}

// Calculates the average concurrency over all values given
func (agg *perPodAggregation) calculateAverage(now time.Time, window time.Duration) float64 {
	if agg.probeCount == 0 {
		return 0.0
	}
	return agg.accumulatedConcurrency / float64(agg.probeCount) * agg.podWeight(now, window)
}

// Calculates the pod weight. Assuming the latest stat time is the point when
// pod became out of service.
func (agg *perPodAggregation) podWeight(now time.Time, window time.Duration) float64 {
	if agg.isActivator {
		return activatorPodWeight
	}

	gapToNow := now.Sub(*agg.latestStatTime)
	// Less than activeThreshold means the pod is active, give 1 weight
	if gapToNow <= activeThreshold {
		return 1.0
	}
	return 1.0 - (float64(gapToNow) / float64(window))
}

// Autoscaler stores current state of an instance of an autoscaler
type Autoscaler struct {
	*autoscalerConfig.DynamicConfig
	key          string
	spec         MetricSpec
	specMutex    sync.RWMutex
	stats        map[statKey]Stat
	statsMutex   sync.Mutex
	panicking    bool
	panicTime    *time.Time
	maxPanicPods float64
	reporter     StatsReporter
}

// New creates a new instance of autoscaler
func New(dynamicConfig *autoscalerConfig.DynamicConfig, spec MetricSpec, reporter StatsReporter) *Autoscaler {
	return &Autoscaler{
		DynamicConfig: dynamicConfig,
		spec:          spec,
		stats:         make(map[statKey]Stat),
		reporter:      reporter,
	}
}

// Update reconfigures the UniScaler according to the MetricSpec.
func (a *Autoscaler) Update(spec MetricSpec) error {
	a.specMutex.Lock()
	defer a.specMutex.Unlock()
	a.spec = spec
	return nil
}

// Record a data point.
func (a *Autoscaler) Record(ctx context.Context, stat Stat) {
	if stat.Time == nil {
		logger := logging.FromContext(ctx)
		logger.Errorf("Missing time from stat: %+v", stat)
		return
	}
	a.statsMutex.Lock()
	defer a.statsMutex.Unlock()

	key := statKey{
		podName: stat.PodName,
		time:    *stat.Time,
	}
	a.stats[key] = stat
}

// Scale calculates the desired scale based on current statistics given the current time.
func (a *Autoscaler) Scale(ctx context.Context, now time.Time) (int32, bool) {
	logger := logging.FromContext(ctx)

	a.specMutex.RLock()
	defer a.specMutex.RUnlock()

	a.statsMutex.Lock()
	defer a.statsMutex.Unlock()

	// 60 second window
	stableData := newTotalAggregation()

	// 6 second window
	panicData := newTotalAggregation()

	// Last stat per Pod
	lastStat := make(map[string]Stat)

	// accumulate stats into their respective buckets
	for key, stat := range a.stats {
		instant := key.time
		if instant.Add(a.spec.WindowPanic).After(now) {
			panicData.aggregate(stat)
		}
		if instant.Add(a.spec.Window).After(now) {
			stableData.aggregate(stat)

			// If there's no last stat for this pod, set it
			if _, ok := lastStat[stat.PodName]; !ok {
				lastStat[stat.PodName] = stat
			}
			// If the current last stat is older than the new one, override
			if lastStat[stat.PodName].Time.Before(*stat.Time) {
				lastStat[stat.PodName] = stat
			}
		} else {
			// Drop metrics after 60 seconds
			delete(a.stats, key)
		}
	}

	observedStablePods := stableData.observedPods(now, a.spec.Window)
	// Do nothing when we have no data.
	if observedStablePods < 1.0 {
		logger.Debug("No data to scale on.")
		return 0, false
	}

	// Log system totals
	totalCurrentQPS := int32(0)
	totalCurrentConcurrency := float64(0)
	for _, stat := range lastStat {
		totalCurrentQPS = totalCurrentQPS + stat.RequestCount
		totalCurrentConcurrency = totalCurrentConcurrency + stat.AverageConcurrentRequests
	}
	logger.Debugf("Current QPS: %v  Current concurrent clients: %v", totalCurrentQPS, totalCurrentConcurrency)

	observedPanicPods := panicData.observedPods(now, a.spec.WindowPanic)
	observedStableConcurrency := stableData.observedConcurrency(now, a.spec.Window)
	observedPanicConcurrency := panicData.observedConcurrency(now, a.spec.WindowPanic)
	observedStableConcurrencyPerPod := stableData.observedConcurrencyPerPod(now, a.spec.Window)
	observedPanicConcurrencyPerPod := panicData.observedConcurrencyPerPod(now, a.spec.WindowPanic)
	// Desired pod count is observed concurrency of revision over desired (stable) concurrency per pod.
	// The scaling up rate limited to within MaxScaleUpRate.
	desiredStablePodCount := a.podCountLimited(observedStableConcurrency/a.spec.TargetConcurrency, observedStablePods)
	desiredPanicPodCount := a.podCountLimited(observedPanicConcurrency/a.spec.TargetConcurrency, observedStablePods)

	a.reporter.ReportObservedPodCount(observedStablePods)
	a.reporter.ReportStableRequestConcurrency(observedStableConcurrencyPerPod)
	a.reporter.ReportPanicRequestConcurrency(observedPanicConcurrencyPerPod)
	a.reporter.ReportTargetRequestConcurrency(a.spec.TargetConcurrency)

	logger.Debugf("STABLE: Observed average %0.3f concurrency over %v seconds over %v samples over %v pods.",
		observedStableConcurrencyPerPod, a.spec.Window, stableData.probeCount, observedStablePods)
	logger.Debugf("PANIC: Observed average %0.3f concurrency over %v seconds over %v samples over %v pods.",
		observedPanicConcurrencyPerPod, a.spec.WindowPanic, panicData.probeCount, observedPanicPods)

	// Stop panicking after the surge has made its way into the stable metric.
	if a.panicking && a.panicTime.Add(a.spec.Window).Before(now) {
		logger.Info("Un-panicking.")
		a.panicking = false
		a.panicTime = nil
		a.maxPanicPods = 0
	}

	// Begin panicking when we cross the 6 second concurrency threshold.
	if !a.panicking && observedPanicPods > 0.0 && observedPanicConcurrencyPerPod >= (a.spec.TargetConcurrencyPanic) {
		logger.Info("PANICKING")
		a.panicking = true
		a.panicTime = &now
	}

	var desiredPodCount int32

	if a.panicking {
		logger.Debug("Operating in panic mode.")
		a.reporter.ReportPanic(1)
		if desiredPanicPodCount > a.maxPanicPods {
			logger.Infof("Increasing pods from %v to %v.", observedPanicPods, int(desiredPanicPodCount))
			a.panicTime = &now
			a.maxPanicPods = desiredPanicPodCount
		}
		desiredPodCount = int32(math.Ceil(a.maxPanicPods))
	} else {
		logger.Debug("Operating in stable mode.")
		a.reporter.ReportPanic(0)
		desiredPodCount = int32(math.Ceil(desiredStablePodCount))
	}

	a.reporter.ReportDesiredPodCount(int64(desiredPodCount))
	return desiredPodCount, true
}

func (a *Autoscaler) podCountLimited(desiredPodCount, currentPodCount float64) float64 {
	return math.Min(desiredPodCount, a.Current().MaxScaleUpRate*currentPodCount)
}

func isActivator(podName string) bool {
	// TODO(#2282): This can cause naming collisions.
	return strings.HasPrefix(podName, ActivatorPodName)
}

func divide(a, b float64) float64 {
	if math.Abs(b) < approximateZero {
		return 0
	}
	return a / b
}
