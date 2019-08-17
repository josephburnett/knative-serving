package main

import (
	"sync"

	"knative.dev/serving/pkg/plugin"
	"github.com/josephburnett/sk-plugin/pkg/skplug"
	"github.com/josephburnett/sk-plugin/pkg/skplug/proto"
)

const (
	pluginType = "podautoscaler.v1alpha1.knative.dev"
)

var _ skplug.Plugin = &pluginServer{}

type pluginServer struct {
	mux sync.RWMutex
	autoscalers map[string]*plugin.Autoscaler
}

func newPluginServer() *pluginServer {
	return &pluginServer{}
}

func (p *pluginServer) Event(part string, time int64, typ proto.EventType, object skplug.Object) error {
	return nil
}

func (p *pluginServer) Stat(part string, stat []*proto.Stat) error {
	return nil
}

func (p *pluginServer) Scale(part string, time int64) (rec int32, err error) {
	return 0, nil
}

func main() {

}
