package plugins

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jumppad-labs/hclconfig/plugins/proto"
)

// GRPCHostCallbackServer implements HostCallbackService for the host side
// This consolidates both logger and state functionality into one service
type GRPCHostCallbackServer struct {
	proto.UnimplementedHostCallbackServiceServer
	logger Logger
	state  State
}

// NewGRPCHostCallbackServer creates a new consolidated host callback server
func NewGRPCHostCallbackServer(logger Logger, state State) *GRPCHostCallbackServer {
	return &GRPCHostCallbackServer{
		logger: logger,
		state:  state,
	}
}

// Logger methods
func (s *GRPCHostCallbackServer) Info(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if s.logger != nil {
		s.logger.Info(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

func (s *GRPCHostCallbackServer) Debug(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if s.logger != nil {
		s.logger.Debug(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

func (s *GRPCHostCallbackServer) Warn(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if s.logger != nil {
		s.logger.Warn(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

func (s *GRPCHostCallbackServer) Error(ctx context.Context, req *proto.LogRequest) (*proto.LogResponse, error) {
	if s.logger != nil {
		s.logger.Error(req.Message, stringArgsToInterfaces(req.Args)...)
	}
	return &proto.LogResponse{}, nil
}

// State methods
func (s *GRPCHostCallbackServer) Get(ctx context.Context, req *proto.StateGetRequest) (*proto.StateGetResponse, error) {
	if s.state == nil {
		return &proto.StateGetResponse{Error: "state service not available"}, nil
	}

	resource, err := s.state.Get(req.Key)
	if err != nil {
		return &proto.StateGetResponse{Error: err.Error()}, nil
	}

	// Serialize the resource to bytes
	resourceData, err := json.Marshal(resource)
	if err != nil {
		return &proto.StateGetResponse{Error: err.Error()}, nil
	}

	return &proto.StateGetResponse{ResourceData: resourceData}, nil
}

func (s *GRPCHostCallbackServer) Find(ctx context.Context, req *proto.StateFindRequest) (*proto.StateFindResponse, error) {
	if s.state == nil {
		return &proto.StateFindResponse{Error: "state service not available"}, nil
	}

	resources, err := s.state.Find(req.Pattern)
	if err != nil {
		return &proto.StateFindResponse{Error: err.Error()}, nil
	}

	// Serialize all resources to bytes
	resourcesData := make([][]byte, len(resources))
	for i, resource := range resources {
		data, err := json.Marshal(resource)
		if err != nil {
			return &proto.StateFindResponse{Error: err.Error()}, nil
		}
		resourcesData[i] = data
	}

	return &proto.StateFindResponse{ResourcesData: resourcesData}, nil
}

// Helper functions
func stringArgsToInterfaces(args []string) []interface{} {
	result := make([]interface{}, len(args))
	for i, arg := range args {
		// Try to parse as different types, fallback to string
		if val, err := strconv.Atoi(arg); err == nil {
			result[i] = val
		} else if val, err := strconv.ParseFloat(arg, 64); err == nil {
			result[i] = val
		} else if val, err := strconv.ParseBool(arg); err == nil {
			result[i] = val
		} else {
			result[i] = arg
		}
	}
	return result
}