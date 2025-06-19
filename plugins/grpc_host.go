package plugins

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-plugin"
	"github.com/jumppad-labs/hclconfig/plugins/proto"
)

// PluginHost manages remote plugin processes and provides host services
type PluginHost struct {
	logger Logger
	state  State
	client *plugin.Client
	plugin PluginEntityProvider
}

// NewPluginHost creates a new plugin host with the specified plugin binary
func NewPluginHost(logger Logger, state State, pluginPath string) *PluginHost {
	return &PluginHost{
		logger: logger,
		state:  state,
	}
}

// Start initializes and starts the plugin process
func (h *PluginHost) Start(pluginPath string) error {

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
			// Note: ConcreteType and Adapter are not relevant for remote plugins
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

func (w *grpcPluginWrapper) Create(entityType, entitySubType string, entityData []byte) error {
	resp, err := w.client.Create(context.Background(), &proto.CreateRequest{
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

func (w *grpcPluginWrapper) Refresh(ctx context.Context) error {
	resp, err := w.client.Refresh(ctx, &proto.RefreshRequest{})
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf(resp.Error)
	}

	return nil
}

func (w *grpcPluginWrapper) Changed(entityType, entitySubType string, entityData []byte) (bool, error) {
	resp, err := w.client.Changed(context.Background(), &proto.ChangedRequest{
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

// Ensure PluginHost implements PluginEntityProvider interface
var _ PluginEntityProvider = (*PluginHost)(nil)

// GetTypes returns the types handled by the plugin
func (h *PluginHost) GetTypes() []RegisteredType {
	if h.plugin == nil {
		return nil
	}
	return h.plugin.GetTypes()
}

// Validate validates the given entity data
func (h *PluginHost) Validate(entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Validate(entityType, entitySubType, entityData)
}

// Create creates a new entity
func (h *PluginHost) Create(entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Create(entityType, entitySubType, entityData)
}

// Destroy deletes an existing entity
func (h *PluginHost) Destroy(entityType, entitySubType string, entityData []byte) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Destroy(entityType, entitySubType, entityData)
}

// Refresh refreshes the plugin state
func (h *PluginHost) Refresh(ctx context.Context) error {
	if h.plugin == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Refresh(ctx)
}

// Changed checks if the entity has changed
func (h *PluginHost) Changed(entityType, entitySubType string, entityData []byte) (bool, error) {
	if h.plugin == nil {
		return false, fmt.Errorf("plugin not initialized")
	}
	return h.plugin.Changed(entityType, entitySubType, entityData)
}
