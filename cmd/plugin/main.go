package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/go-plugin"
	"github.com/josephburnett/sk-plugin/pkg/skplug"
	"github.com/josephburnett/sk-plugin/pkg/skplug/proto"
	knativeplugin "knative.dev/serving/pkg/plugin"
)

const (
	pluginType = "podautoscaler.v1alpha1.knative.dev"
)

var _ skplug.Plugin = &pluginServer{}

type pluginServer struct {
	mux         sync.RWMutex
	autoscalers map[string]*knativeplugin.Autoscaler
}

func newPluginServer() *pluginServer {
	return &pluginServer{
		autoscalers: make(map[string]*knativeplugin.Autoscaler),
	}
}

func (p *pluginServer) Event(part string, time int64, typ proto.EventType, object skplug.Object) error {
	switch o := object.(type) {
	case *skplug.Autoscaler:
		switch typ {
		case proto.EventType_CREATE:
			return p.createAutoscaler(part, o)
		case proto.EventType_UPDATE:
			return fmt.Errorf("update autoscaler event not supported")
		case proto.EventType_DELETE:
			return p.deleteAutoscaler(part)
		default:
			return fmt.Errorf("unhandled event type: %v for object of type: %T", typ, object)
		}
	case *skplug.Pod:
		switch typ {
		case proto.EventType_CREATE:
			return p.createPod(part, o)
		case proto.EventType_UPDATE:
			return p.updatePod(part, o)
		case proto.EventType_DELETE:
			return p.deletePod(part, o)
		default:
			return fmt.Errorf("unhandled event type: %v for object of type: %T", typ, object)
		}
	default:
		return fmt.Errorf("unhandled object type: %T", object)
	}
}

func (p *pluginServer) Stat(part string, stat []*proto.Stat) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	autoscaler, ok := p.autoscalers[part]
	if !ok {
		return fmt.Errorf("stat for non-existant partition: %v", part)
	}
	return autoscaler.Stat(stat)
}

func (p *pluginServer) Scale(part string, now int64) (rec int32, err error) {
	p.mux.Lock()
	defer p.mux.Unlock()
	autoscaler, ok := p.autoscalers[part]
	if !ok {
		return 0, fmt.Errorf("scale for non-existant partition: %v", part)
	}
	return autoscaler.Scale(now)
}

func (p *pluginServer) createAutoscaler(part string, a *skplug.Autoscaler) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.autoscalers[part]; ok {
		return fmt.Errorf("duplicate create autoscaler event in partition %v", part)
	}
	autoscaler, err := knativeplugin.NewAutoscaler(a.Yaml)
	if err != nil {
		return err
	}
	p.autoscalers[part] = autoscaler
	log.Println("created autoscaler", part)
	return nil
}

func (p *pluginServer) deleteAutoscaler(part string) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	if _, ok := p.autoscalers[part]; !ok {
		return fmt.Errorf("delete autoscaler event for non-existant partition %v", part)
	}
	delete(p.autoscalers, part)
	log.Println("deleted autoscaler", part)
	return nil
}

func (p *pluginServer) createPod(part string, o *skplug.Pod) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	a, ok := p.autoscalers[part]
	if !ok {
		return fmt.Errorf("create pod for non-existant partition: %v", part)
	}
	return a.CreatePod(o)
}

func (p *pluginServer) updatePod(part string, o *skplug.Pod) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	a, ok := p.autoscalers[part]
	if !ok {
		return fmt.Errorf("update pod for non-existant partition: %v", part)
	}
	return a.UpdatePod(o)
}

func (p *pluginServer) deletePod(part string, o *skplug.Pod) error {
	p.mux.Lock()
	defer p.mux.Unlock()
	a, ok := p.autoscalers[part]
	if !ok {
		return fmt.Errorf("delete pod for non-existant partition: %v", part)
	}
	return a.DeletePod(o)
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: skplug.Handshake,
		Plugins: map[string]plugin.Plugin{
			"autoscaler": &skplug.AutoscalerPlugin{Impl: newPluginServer()},
		},

		// A non-nil value here enables gRPC serving for this plugin...
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
