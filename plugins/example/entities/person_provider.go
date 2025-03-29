package entities

import (
	"context"

	"github.com/jumppad-labs/hclconfig/plugins"
	"github.com/jumppad-labs/hclconfig/types"
)

type Provider struct {
}

func (p *Provider) Init(cfg types.Resource, log plugins.Logger) error {
	// ...initialize the provider with the given configuration and logger...
	return nil
}

func (p *Provider) Create(ctx context.Context) error {
	// ...logic to create a person resource...
	return nil
}

func (p *Provider) Destroy(ctx context.Context, force bool) error {
	// ...logic to destroy a person resource...
	return nil
}

func (p *Provider) Refresh(ctx context.Context) error {
	// ...logic to refresh the state of a person resource...
	return nil
}

func (p *Provider) Changed() (bool, error) {
	// ...logic to check if the person resource has changed...
	return false, nil
}

func (p *Provider) Lookup() ([]string, error) {
	// ...logic to look up a person resource...
	return []string{"example_person"}, nil
}
