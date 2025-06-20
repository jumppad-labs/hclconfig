package plugins

import (
	"context"
	"fmt"
	"net"

	"github.com/jumppad-labs/hclconfig/plugins/proto"
	"google.golang.org/grpc"
)

// InProcessPluginHost manages in-process gRPC plugin instances
type InProcessPluginHost struct {
	logger   Logger
	state    State
	plugin   Plugin
	server   *grpc.Server
	listener net.Listener
	client   proto.PluginServiceClient
	conn     *grpc.ClientConn
}

// NewInProcessPluginHost creates a new in-process plugin host
func NewInProcessPluginHost(logger Logger, state State, plugin Plugin) *InProcessPluginHost {
	return &InProcessPluginHost{
		logger: logger,
		state:  state,
		plugin: plugin,
	}
}

// Start initializes the in-process gRPC server and client
func (h *InProcessPluginHost) Start() error {
	// Initialize the plugin
	if err := h.plugin.Init(h.logger, h.state); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Create a listener on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	h.listener = listener

	// Create gRPC server
	h.server = grpc.NewServer()

	// Create the gRPC server implementation with simplified broker
	server := &InProcessGRPCServer{
		plugin: h.plugin,
		logger: h.logger,
		state:  h.state,
	}

	// Register the plugin service
	proto.RegisterPluginServiceServer(h.server, server)

	// Start server in background
	go func() {
		if err := h.server.Serve(h.listener); err != nil {
			h.logger.Error("gRPC server failed", "error", err)
		}
	}()

	// Create client connection
	conn, err := grpc.Dial(h.listener.Addr().String(), grpc.WithInsecure()) //nolint:staticcheck
	if err != nil {
		h.server.Stop()
		return fmt.Errorf("failed to connect to in-process server: %w", err)
	}
	h.conn = conn
	h.client = proto.NewPluginServiceClient(conn)

	return nil
}

// Stop shuts down the in-process gRPC server
func (h *InProcessPluginHost) Stop() {
	if h.conn != nil {
		h.conn.Close()
	}
	if h.server != nil {
		h.server.Stop()
	}
	if h.listener != nil {
		h.listener.Close()
	}
}

// GetTypes returns the types handled by the plugin
func (h *InProcessPluginHost) GetTypes() []RegisteredType {
	if h.client == nil {
		return nil
	}

	resp, err := h.client.GetTypes(context.Background(), &proto.GetTypesRequest{})
	if err != nil {
		h.logger.Error("failed to get types", "error", err)
		return nil
	}

	types := make([]RegisteredType, len(resp.Types))
	for i, t := range resp.Types {
		types[i] = RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
		}
	}

	return types
}

// Validate validates the given entity data
func (h *InProcessPluginHost) Validate(entityType, entitySubType string, entityData []byte) error {
	if h.client == nil {
		return fmt.Errorf("plugin not initialized")
	}

	resp, err := h.client.Validate(context.Background(), &proto.ValidateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}

	return nil
}

// Create creates a new entity
func (h *InProcessPluginHost) Create(entityType, entitySubType string, entityData []byte) error {
	if h.client == nil {
		return fmt.Errorf("plugin not initialized")
	}

	resp, err := h.client.Create(context.Background(), &proto.CreateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}

	return nil
}

// Destroy deletes an existing entity
func (h *InProcessPluginHost) Destroy(entityType, entitySubType string, entityData []byte) error {
	if h.client == nil {
		return fmt.Errorf("plugin not initialized")
	}

	resp, err := h.client.Destroy(context.Background(), &proto.DestroyRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}

	return nil
}

// Refresh refreshes the plugin state
func (h *InProcessPluginHost) Refresh(ctx context.Context) error {
	if h.client == nil {
		return fmt.Errorf("plugin not initialized")
	}

	resp, err := h.client.Refresh(ctx, &proto.RefreshRequest{})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}

	return nil
}

// Changed checks if the entity has changed
func (h *InProcessPluginHost) Changed(entityType, entitySubType string, entityData []byte) (bool, error) {
	if h.client == nil {
		return false, fmt.Errorf("plugin not initialized")
	}

	resp, err := h.client.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return false, err
	}

	if resp.Error != "" {
		return false, fmt.Errorf(resp.Error)
	}

	return resp.Changed, nil
}

// InProcessGRPCServer is a simplified gRPC server for in-process plugins
type InProcessGRPCServer struct {
	proto.UnimplementedPluginServiceServer
	plugin Plugin
	logger Logger
	state  State
}

func (s *InProcessGRPCServer) GetTypes(ctx context.Context, req *proto.GetTypesRequest) (*proto.GetTypesResponse, error) {
	s.logger.Info("Getting types")

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

func (s *InProcessGRPCServer) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	s.logger.Info("Validating entity")

	err := s.plugin.Validate(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.ValidateResponse{Error: errorToString(err)}, nil
}

func (s *InProcessGRPCServer) Create(ctx context.Context, req *proto.CreateRequest) (*proto.CreateResponse, error) {
	err := s.plugin.Create(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.CreateResponse{Error: errorToString(err)}, nil
}

func (s *InProcessGRPCServer) Destroy(ctx context.Context, req *proto.DestroyRequest) (*proto.DestroyResponse, error) {
	err := s.plugin.Destroy(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.DestroyResponse{Error: errorToString(err)}, nil
}

func (s *InProcessGRPCServer) Refresh(ctx context.Context, req *proto.RefreshRequest) (*proto.RefreshResponse, error) {
	err := s.plugin.Refresh(ctx)
	return &proto.RefreshResponse{Error: errorToString(err)}, nil
}

func (s *InProcessGRPCServer) Changed(ctx context.Context, req *proto.ChangedRequest) (*proto.ChangedResponse, error) {
	changed, err := s.plugin.Changed(req.EntityType, req.EntitySubType, req.EntityData)
	return &proto.ChangedResponse{
		Changed: changed,
		Error:   errorToString(err),
	}, nil
}

// Ensure InProcessPluginHost implements PluginEntityProvider interface
var _ PluginEntityProvider = (*InProcessPluginHost)(nil)