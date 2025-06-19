package plugins

import (
	"context"
	"errors"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/hclconfig/plugins/proto"
	"google.golang.org/grpc"
)

// GRPCPlugin implements the go-plugin Plugin interface for gRPC
type GRPCPlugin struct {
	broker *plugin.GRPCBroker
	// Impl is the concrete implementation (e.g., PersonPlugin)
	Impl           Plugin
	callbackServer *GRPCHostCallbackServer
	logger         Logger
}

// Ensure GRPCPlugin implements plugin.Plugin interface
var _ plugin.Plugin = (*GRPCPlugin)(nil)
var _ plugin.GRPCPlugin = (*GRPCPlugin)(nil)

// GRPCServer is called by go-plugin to create the gRPC server
func (p *GRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Initialize the plugin implementation with broker-connected logger/state
	if err := p.Impl.Init(nil, nil); err != nil {
		return err
	}

	// Create the gRPC server with already-connected logger/state
	server, err := NewGRPCServer(p.Impl, broker)
	if err != nil {
		return err
	}

	// Register the server
	proto.RegisterPluginServiceServer(s, server)
	return nil
}

// Client is required by plugin.Plugin interface but not used for gRPC
func (p *GRPCPlugin) Client(*plugin.MuxBroker, *rpc.Client) (interface{}, error) {
	return nil, errors.New("this plugin only supports gRPC")
}

// SetupHostCallbackService configures the host callback service for bidirectional communication
func (p *GRPCPlugin) SetupHostCallbackService() {
	p.callbackServer = NewGRPCHostCallbackServer(p.logger, nil)

	var s *grpc.Server
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		s = grpc.NewServer(opts...)
		proto.RegisterHostCallbackServiceServer(s, p.callbackServer)

		return s
	}

	go p.broker.AcceptAndServe(HostCallbackServiceID, serverFunc)
}

// Server is required by plugin.Plugin interface but not used for gRPC
func (p *GRPCPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return nil, errors.New("this plugin only supports gRPC")
}

// GRPCClient is called by go-plugin to create the gRPC client
func (p *GRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	p.broker = broker
	p.SetupHostCallbackService()

	return proto.NewPluginServiceClient(c), nil
}

// Handshake configuration for go-plugin
var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "HCLCONFIG_PLUGIN",
	MagicCookieValue: "hclconfig",
}

// PluginMap for go-plugin

// Service IDs for broker communication
const (
	HostCallbackServiceID uint32 = 1
)
