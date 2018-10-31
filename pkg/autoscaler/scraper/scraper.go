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

package scraper

import (
	"context"
	"sync"
	"time"

	"github.com/knative/serving/pkg/autoscaler"
	_ "github.com/prometheus/prometheus/pkg/textparse"
	corev1informers "k8s.io/client-go/informers/core/v1"
)

type Scraper struct {
	sync.Mutex
	podInformer corev1informers.PodInformer
	targets     map[string]*target
}

type target struct {
	selector *metav1.LabelSelector
	record   func(context.Context, autoscaler.Stat)
	stopCh   chan<- struct{}
}

func New(podInformer corev1informers.PodInformer) *Scraper {
	return &Scraper{
		podInformer: podInformer,
		targets:     make(map[string]*target),
	}
}

func (s *Scraper) Run(stopCh <-chan struct{}) error {
	ticker := time.NewTicker(time.Second).T
	for {
		select {
		case <-ticker:
			if err := s.scrape(); err != nil {
				return err
			}
		case <-stopCh:
			break
		}
	}
	return nil
}

func (s *Scraper) Add(kpa *autoscaling.PodAutoscaler, scaler *autoscaler.UniScaler, stopCh chan<- struct{}) error {
	s.Lock()
	defer s.Unlock()
	targets[autoscaling.NewKpaKey(kpa.Namespace, kpa.Name)] = &target{
		// TODO: deref kpa and get deployment selector
		record: scaler.Record,
		stopCh: stopCh,
	}
}

func (s *Scraper) scrape() error {
	// For each target set of labels:
	// 1. list pods matching label selector
	// 2. chose 5 pods at random
	// 3. scrape pods and call record for each
}
