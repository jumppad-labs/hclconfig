package plugins

import (
	"context"
	"errors"

	"github.com/jumppad-labs/hclconfig/schema"
	"github.com/jumppad-labs/hclconfig/types"
)

type RegisteredType struct {
	// The top level type name, i.e. resource
	Type string
	// the sub type, i.e. k8s_config
	SubType string
	// The json schema for the type
	Schema []byte
	// The concrete type that is used to create the entity
	ConcreteType interface{}
	// The provider that handles this type
	Provider Provider
}

// Plugin is a private interface that defines the contract between HCLConfig
// and the providers
type Plugin interface {
	Init() error
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

type PluginBase struct {
	logger          Logger
	registeredTypes []RegisteredType
}

// RegisterType registers a type with the plugin. This is used to
// register the types that the plugin can handle. The type name is
// the top level type name, i.e. resource, and the sub type is the
// sub type, i.e. k8s_config. The t parameter is a reference to a struct that
// defines the type.
func (p *PluginBase) RegisterType(typeName, subTypeName string, t interface{}, prov Provider) error {
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
		Provider:     prov,
		ConcreteType: t,
	})

	return nil
}

func (p *PluginBase) SetLogger(l Logger) {
	p.logger = l
}

func (p *PluginBase) Log() Logger {
	return p.logger
}

func (p *PluginBase) getRegisteredType(entityType, entitySubType string) *RegisteredType {
	if len(p.registeredTypes) == 0 {
		return nil
	}

	return &p.registeredTypes[0]
}

// lifecycle methods

// Validate validates the given entity data.
func (p *PluginBase) Validate(entityType, entitySubType string, entityData []byte) error {
	// Example implementation
	if entityType == "" || entitySubType == "" {
		return errors.New("entityType and entitySubType cannot be empty")
	}
	return nil
}

// Create creates a new entity.
func (p *PluginBase) Create(entityType, entitySubType string, entityData []byte) error {
	rt := p.getRegisteredType(entityType, entitySubType)

	// convert to a concrete type
	c, err := schema.CreateStructFromSchema(rt.Schema)
	if err != nil {
		return err
	}

	ct := schema.UnmarshalUntyped(c, rt.ConcreteType)

	// Get the provider for this type
	provider := rt.Provider
	provider.Init(ct.(types.Resource), p.logger)
	return provider.Create(context.Background())
}

// Destroy deletes an existing entity.
func (p *PluginBase) Destroy(entityType, entitySubType string, entityData []byte) error {
	// Example implementation
	return nil
}

// Refresh refreshes the plugin state.
func (p *PluginBase) Refresh(ctx context.Context) error {
	// Example implementation
	return nil
}

// Changed checks if the entity has changed.
func (p *PluginBase) Changed(entityType, entitySubType string, entityData []byte) (bool, error) {
	// Example implementation
	return false, nil
}
