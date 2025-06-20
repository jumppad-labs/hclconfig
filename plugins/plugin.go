package plugins

import (
	"context"
	"errors"

	"github.com/jumppad-labs/hclconfig/schema"
	"github.com/jumppad-labs/hclconfig/types"
)

// RegisteredType represents a registered resource type with its metadata
type RegisteredType struct {
	// The top level type name, i.e. resource
	Type string
	// the sub type, i.e. k8s_config
	SubType string
	// The json schema for the type
	Schema []byte
	// The concrete type that is used to create the entity
	ConcreteType interface{}
	// The provider adapter that handles this type
	Adapter ProviderAdapter
}

type PluginEntityProvider interface {
	// GetTypes returns the types handled by the plugin.
	GetTypes() []RegisteredType
	Validate(entityType, entitySubType string, entityData []byte) error
	Create(entityType, entitySubType string, entityData []byte) error
	Destroy(entityType, entitySubType string, entityData []byte) error
	Refresh(ctx context.Context) error
	Changed(entityType, entitySubType string, entityData []byte) (bool, error)
}

// RegisterResourceProvider registers a typed resource provider with the plugin.
// This creates a typed adapter and registers it with the plugin.
func RegisterResourceProvider[T types.Resource](p *PluginBase, logger Logger, state State, typeName, subTypeName string, resourceInstance T, provider ResourceProvider[T]) error {
	// Create a typed adapter for the provider
	adapter := NewTypedProviderAdapter(provider, resourceInstance)
	
	// Initialize the adapter with state, functions (can be nil), and logger
	err := adapter.Init(state, nil, logger)
	if err != nil {
		return err
	}

	return p.RegisterType(typeName, subTypeName, resourceInstance, adapter)
}

// Plugin is a private interface that defines the contract between HCLConfig
// and the providers
type Plugin interface {
	Init(Logger, State) error
	SetLogger(logger Logger)
	SetState(state State)
	PluginEntityProvider
}

type PluginBase struct {
	logger          Logger // external logger passed to the plugin via Init
	state           State  // state functions passed to the plugin via Init
	registeredTypes []RegisteredType
}

// SetLogger sets the logger for the plugin base
func (p *PluginBase) SetLogger(logger Logger) {
	p.logger = logger
}

// SetState sets the state for the plugin base
func (p *PluginBase) SetState(state State) {
	p.state = state
}

// RegisterType registers a type with the plugin using type-safe parameters.
// This method ensures that t implements types.Resource.
func (p *PluginBase) RegisterType(typeName, subTypeName string, t types.Resource, prov ProviderAdapter) error {
	entitySchema, err := schema.GenerateFromInstance(t, 10)
	if err != nil {
		return err
	}

	if p.registeredTypes == nil {
		p.registeredTypes = []RegisteredType{}
	}

	p.registeredTypes = append(p.registeredTypes, RegisteredType{
		Type:         typeName,
		SubType:      subTypeName,
		Schema:       entitySchema,
		ConcreteType: t,
		Adapter:      prov,
	})

	return nil
}

func (p *PluginBase) getRegisteredType(entityType, entitySubType string) *RegisteredType {
	for i := range p.registeredTypes {
		if p.registeredTypes[i].Type == entityType && p.registeredTypes[i].SubType == entitySubType {
			return &p.registeredTypes[i]
		}
	}
	return nil
}

// GetTypes returns all registered types
func (p *PluginBase) GetTypes() []RegisteredType {
	return p.registeredTypes
}

// lifecycle methods

// Validate validates the given entity data.
func (p *PluginBase) Validate(entityType, entitySubType string, entityData []byte) error {
	if entityType == "" || entitySubType == "" {
		return errors.New("entityType and entitySubType cannot be empty")
	}

	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Validate(context.Background(), entityData)
}

// Create creates a new entity.
func (p *PluginBase) Create(entityType, entitySubType string, entityData []byte) error {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Create(context.Background(), entityData)
}

// Destroy deletes an existing entity.
func (p *PluginBase) Destroy(entityType, entitySubType string, entityData []byte) error {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Destroy(context.Background(), entityData, false)
}

// Refresh refreshes the plugin state.
func (p *PluginBase) Refresh(ctx context.Context) error {
	// Refresh all registered providers
	for _, rt := range p.registeredTypes {
		if err := rt.Adapter.Refresh(ctx, nil); err != nil {
			return err
		}
	}
	return nil
}

// Changed checks if the entity has changed.
func (p *PluginBase) Changed(entityType, entitySubType string, entityData []byte) (bool, error) {
	rt := p.getRegisteredType(entityType, entitySubType)
	if rt == nil {
		return false, errors.New("no registered type found for " + entityType + "." + entitySubType)
	}

	return rt.Adapter.Changed(context.Background(), entityData)
}
