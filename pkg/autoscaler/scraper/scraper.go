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
	"fmt"
	"sync"
	"time"

	autoscaling "github.com/knative/serving/pkg/apis/autoscaling/v1alpha1"
	"github.com/knative/serving/pkg/autoscaler/types"
	_ "github.com/prometheus/prometheus/pkg/textparse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

type Scraper struct {
	sync.Mutex
	deploymentLister appsv1listers.DeploymentLister
	podInformer      corev1informers.PodInformer
	targets          map[string]*target
}

type target struct {
	selector  labels.Selector
	record    func(context.Context, types.Stat)
	endpoints map[string]*endpoint
	stopCh    chan<- struct{}
}

type endpoint struct {
	ip   string
	port int
	path string
}

func New(deploymentInformer appsv1informers.DeploymentInformer, podInformer corev1informers.PodInformer) *Scraper {
	return &Scraper{
		deploymentLister: deploymentInformer.Lister(),
		podInformer:      podInformer,
		targets:          make(map[string]*target),
	}
}

func (s *Scraper) Run(stopCh <-chan struct{}) error {
	s.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.addPod,
		UpdateFunc: s.updatePod,
		DeleteFunc: s.deletePod,
	})

	ticker := time.NewTicker(time.Second).C
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

func (s *Scraper) addPod(obj interface{}) {
	yoloPod := obj.(*corev1.Pod)
	s.Lock()
	defer s.Unlock()
	for _, target := range s.targets {
		if target.selector.Matches(labels.Set(yoloPod.Labels)) {
			if endpoint, ok := endpointFromPod(yoloPod); ok {
				target.endpoints[yoloPod.Name] = endpoint
			}
		}
	}
}

func (s *Scraper) updatePod(oldObj, newObj interface{}) {
	s.addPod(newObj)
}

func (s *Scraper) deletePod(obj interface{}) {
	yoloPod := obj.(*corev1.Pod)
	s.Lock()
	defer s.Unlock()
	for _, target := range s.targets {
		if target.selector.Matches(labels.Set(yoloPod.Labels)) {
			if _, ok := target.endpoints[yoloPod.Name]; ok {
				delete(target.endpoints, yoloPod.Name)
			}
		}
	}
}

func (s *Scraper) Add(kpa *autoscaling.PodAutoscaler, record func(context.Context, types.Stat), stopCh chan<- struct{}) error {

	// Get the Deployment's label selectors.
	// TODO: use the objectRef instead.
	dep, err := s.deploymentLister.Deployments(kpa.Namespace).Get(kpa.Name)
	if err != nil {
		return err
	}
	selector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return err
	}

	// Create the initial list of endpoints.
	pods, err := s.podInformer.Lister().List(selector)
	if err != nil {
		return err
	}
	endpoints := make(map[string]*endpoint)
	for _, pod := range pods {
		if endpoint, ok := endpointFromPod(pod); ok {
			endpoints[pod.Name] = endpoint
		}
	}

	// Register the target.
	s.Lock()
	defer s.Unlock()
	s.targets[kpa.Namespace+"/"+kpa.Name] = &target{
		selector:  selector,
		record:    record,
		endpoints: endpoints,
		stopCh:    stopCh,
	}
	return nil
}

func (s *Scraper) scrape() error {
	for key, target := range s.targets {
		fmt.Printf("endpoints for %v: %v\n", key, target.endpoints)
	}
	// For each target set of labels:
	// 1. list pods matching label selector
	// 2. chose 5 pods at random
	// 3. scrape pods and call record for each
	return nil
}

func endpointFromPod(pod *corev1.Pod) (*endpoint, bool) {
	if pod.Status.PodIP == "" {
		return nil, false
	}
	return &endpoint{
		ip:   pod.Status.PodIP,
		port: 8080,
		path: "/metrics",
	}, true
}
