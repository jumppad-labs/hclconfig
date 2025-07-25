package plugins

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/hclconfig/plugins/proto"
)

// GRPCPluginHost manages external plugin processes via gRPC and provides host services
type GRPCPluginHost struct {
	logger         Logger
	state          State
	client         *plugin.Client
	plugin         PluginEntityProvider
	cachedTypes    []RegisteredType // cached types with adapters
	typesCached    bool             // flag to track if types have been cached
}

// NewGRPCPluginHost creates a new gRPC plugin host for external plugin binaries
func NewGRPCPluginHost(logger Logger, state State) *GRPCPluginHost {
	return &GRPCPluginHost{
		logger: logger,
		state:  state,
	}
}

// Start initializes and starts the external plugin process
func (h *GRPCPluginHost) Start(pluginPath string) error {
	var PluginMap = map[string]plugin.Plugin{
		"plugin": &GRPCPlugin{logger: h.logger},
	}

	// Create the plugin client
	h.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  HandshakeConfig,
		Plugins:          PluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
	})

	// Connect to the plugin
	rpcClient, err := h.client.Client()
	if err != nil {
		return fmt.Errorf("failed to get plugin client: %w", err)
	}

	// Get the plugin interface
	raw, err := rpcClient.Dispense("plugin")
	if err != nil {
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Cast to gRPC client and wrap it
	grpcClient := raw.(proto.PluginServiceClient)
	h.plugin = &grpcPluginWrapper{client: grpcClient}

	return nil
}

// Stop shuts down the plugin host and cleans up resources
func (h *GRPCPluginHost) Stop() {
	if h.client != nil {
		h.client.Kill()
	}
}

// grpcPluginWrapper wraps a gRPC client to implement PluginEntityProvider
type grpcPluginWrapper struct {
	client proto.PluginServiceClient
}

func (w *grpcPluginWrapper) GetTypes() []RegisteredType {
	resp, err := w.client.GetTypes(context.Background(), &proto.GetTypesRequest{})
	if err != nil {
		return nil
	}

	types := make([]RegisteredType, len(resp.Types))
	for i, t := range resp.Types {
		types[i] = RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
			// Note: Adapter is set to nil for remote plugins as it's handled by the gRPC wrapper
		}
	}

	return types
}

func (w *grpcPluginWrapper) Validate(entityType, entitySubType string, entityData []byte) error {
	resp, err := w.client.Validate(context.Background(), &proto.ValidateRequest{
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

func (w *grpcPluginWrapper) Create(entityType, entitySubType string, entityData []byte) ([]byte, error) {
	resp, err := w.client.Create(context.Background(), &proto.CreateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}

	return resp.MutatedEntityData, nil
}

func (w *grpcPluginWrapper) Destroy(entityType, entitySubType string, entityData []byte) error {
	resp, err := w.client.Destroy(context.Background(), &proto.DestroyRequest{
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

func (w *grpcPluginWrapper) Refresh(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	resp, err := w.client.Refresh(ctx, &proto.RefreshRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}

	return resp.RefreshedEntityData, nil
}

func (w *grpcPluginWrapper) Update(entityType, entitySubType string, entityData []byte) ([]byte, error) {
	resp, err := w.client.Update(context.Background(), &proto.UpdateRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		EntityData:    entityData,
	})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}

	return resp.UpdatedEntityData, nil
}

func (w *grpcPluginWrapper) Changed(entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error) {
	resp, err := w.client.Changed(context.Background(), &proto.ChangedRequest{
		EntityType:    entityType,
		EntitySubType: entitySubType,
		OldEntityData: oldEntityData,
		NewEntityData: newEntityData,
	})
	if err != nil {
		return false, err
	}

	if resp.Error != "" {
		return false, fmt.Errorf(resp.Error)
	}

	return resp.Changed, nil
}

// Ensure GRPCPluginHost implements PluginHost interface
var _ PluginHost = (*GRPCPluginHost)(nil)

// GetTypes returns the types handled by the plugin, creating and caching adapters on first call
func (h *GRPCPluginHost) GetTypes() []RegisteredType {
	if h.plugin == nil {
		return nil
	}

	// Return cached types if already created
	if h.typesCached {
		return h.cachedTypes
	}

	// Get types from remote plugin (this calls grpcPluginWrapper.GetTypes())
	remoteTypes := h.plugin.GetTypes()
	if remoteTypes == nil {
		return nil
	}

	// Create resource-specific adapters for each type
	h.cachedTypes = make([]RegisteredType, len(remoteTypes))
	wrapper := h.plugin.(*grpcPluginWrapper) // cast to access wrapper

	for i, t := range remoteTypes {
		// Create a resource-specific adapter
		adapter := NewGRPCResourceProviderAdapter(wrapper, t.Type, t.SubType)
		
		h.cachedTypes[i] = RegisteredType{
			Type:    t.Type,
			SubType: t.SubType,
			Schema:  t.Schema,
			Adapter: adapter,
		}
	}

	h.typesCached = true
	return h.cachedTypes
}

// Validate validates the given entity data
func (h *GRPCPluginHost) Validate(entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Validate(entityType, entitySubType, entityData)
}

// Create creates a new entity
func (h *GRPCPluginHost) Create(entityType, entitySubType string, entityData []byte) ([]byte, error) {
	if h.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.(*grpcPluginWrapper).Create(entityType, entitySubType, entityData)
}

// Destroy deletes an existing entity
func (h *GRPCPluginHost) Destroy(entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Destroy(entityType, entitySubType, entityData)
}

// Refresh refreshes the plugin state
func (h *GRPCPluginHost) Refresh(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error) {
	if h.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.(*grpcPluginWrapper).Refresh(ctx, entityType, entitySubType, entityData)
}

// Update updates an existing entity
func (h *GRPCPluginHost) Update(entityType, entitySubType string, entityData []byte) ([]byte, error) {
	if h.plugin == nil {
		return nil, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.(*grpcPluginWrapper).Update(entityType, entitySubType, entityData)
}

// Changed checks if the entity has changed by comparing old and new
func (h *GRPCPluginHost) Changed(entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) (bool, error) {
	if h.plugin == nil {
		return false, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Changed(entityType, entitySubType, oldEntityData, newEntityData)
}
