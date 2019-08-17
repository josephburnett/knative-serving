package main

import (
	"sync"

	knativeplugin "knative.dev/serving/pkg/plugin"
	"github.com/josephburnett/sk-plugin/pkg/skplug"
	"github.com/josephburnett/sk-plugin/pkg/skplug/proto"
	"github.com/hashicorp/go-plugin"
)

const (
	pluginType = "podautoscaler.v1alpha1.knative.dev"
)

var _ skplug.Plugin = &pluginServer{}

type pluginServer struct {
	mux sync.RWMutex
	autoscalers map[string]*knativeplugin.Autoscaler
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
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: skplug.Handshake,
		Plugins: map[string]plugin.Plugin{
			"autoscaler": &skplug.AutoscalerPlugin{Impl: newPluginServer()},
		},

		// A non-nil value here enables gRPC serving for this plugin...
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
