package plugins

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/hclconfig/plugins/proto"
)

// GRPCServer wraps PluginBase and implements the gRPC PluginService
type GRPCServer struct {
	proto.UnimplementedPluginServiceServer
	plugin       Plugin
	broker       *plugin.GRPCBroker
	logger       Logger
	state        State
	cachedLogger Logger // cached logger instance
}

// NewGRPCServer creates a new gRPC server with provided logger and state
func NewGRPCServer(plugin Plugin, broker *plugin.GRPCBroker) (*GRPCServer, error) {
	server := &GRPCServer{
		plugin: plugin,
		broker: broker,
	}

	return server, nil
}

func (s *GRPCServer) GetTypes(ctx context.Context, req *proto.GetTypesRequest) (*proto.GetTypesResponse, error) {
	l, err := s.getLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	// set the logger for the plugin
	s.plugin.SetLogger(l)

	l.Info("Getting types")

	types := s.plugin.GetTypes()
	protoTypes := make([]*proto.RegisteredType, len(types))

	for i, t := range types {
		protoTypes[i] = &proto.RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
		}
	}

	return &proto.GetTypesResponse{Types: protoTypes}, nil
}

func (s *GRPCServer) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	l, err := s.getLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	// set the logger for the plugin
	s.plugin.SetLogger(l)

	l.Info("Validating entity")

	err = s.plugin.Validate(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.ValidateResponse{Error: errorToString(err)}, nil
}

func (s *GRPCServer) Create(ctx context.Context, req *proto.CreateRequest) (*proto.CreateResponse, error) {
	err := s.plugin.Create(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.CreateResponse{Error: errorToString(err)}, nil
}

func (s *GRPCServer) Destroy(ctx context.Context, req *proto.DestroyRequest) (*proto.DestroyResponse, error) {
	err := s.plugin.Destroy(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.DestroyResponse{Error: errorToString(err)}, nil
}

func (s *GRPCServer) Refresh(ctx context.Context, req *proto.RefreshRequest) (*proto.RefreshResponse, error) {

	err := s.plugin.Refresh(ctx)
	return &proto.RefreshResponse{Error: errorToString(err)}, nil
}

func (s *GRPCServer) Changed(ctx context.Context, req *proto.ChangedRequest) (*proto.ChangedResponse, error) {
	changed, err := s.plugin.Changed(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.ChangedResponse{
		Changed: changed,
		Error:   errorToString(err),
	}, nil
}

func (s *GRPCServer) getLogger() (Logger, error) {
	// Return cached logger if already created
	if s.cachedLogger != nil {
		return s.cachedLogger, nil
	}

	// Try to connect to logger service
	hostConn, err := s.broker.Dial(HostCallbackServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to host callback service: %w", err)
	}

	// Cache the logger for future use
	s.cachedLogger = &GRPCLogger{client: proto.NewHostCallbackServiceClient(hostConn)}
	return s.cachedLogger, nil
}

// Helper function to convert error to string
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
