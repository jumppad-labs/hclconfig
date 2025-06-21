package person

import (
	"context"

	"github.com/jumppad-labs/hclconfig/plugins"
)

// ExampleProvider is a basic implementation of Provider[*Person]
// that demonstrates the structure and lifecycle methods for Person resources.
type ExampleProvider struct {
	state     plugins.State
	functions plugins.ProviderFunctions
	logger    plugins.Logger
}

// Compile-time check to ensure ExampleProvider implements ResourceProvider[*Person]
var _ plugins.ResourceProvider[*Person] = (*ExampleProvider)(nil)

func (p *ExampleProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger plugins.Logger) error {
	p.state = state
	p.functions = functions
	p.logger = logger
	return nil
}

func (p *ExampleProvider) Create(ctx context.Context, person *Person) (*Person, error) {
	if p.logger != nil {
		p.logger.Info("Creating person", "id", person.Metadata().ID, "name", person.FirstName+" "+person.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Simulate person creation (e.g., save to database, create user account, etc.)
	// In a real implementation, this would interact with APIs, databases, etc.

	return person, nil
}

func (p *ExampleProvider) Destroy(ctx context.Context, person *Person, force bool) error {
	if p.logger != nil {
		p.logger.Info("Destroying person", "id", person.Metadata().ID, "name", person.FirstName+" "+person.LastName, "force", force)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate person destruction (e.g., delete from database, remove user account, etc.)
	// In a real implementation, this would clean up resources

	return nil
}

func (p *ExampleProvider) Refresh(ctx context.Context, person *Person) error {
	if p.logger != nil {
		p.logger.Info("Refreshing person", "id", person.Metadata().ID, "name", person.FirstName+" "+person.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate person refresh (e.g., sync from database, update fields, etc.)
	// In a real implementation, this would sync resource state

	return nil
}

func (p *ExampleProvider) Changed(ctx context.Context, person *Person) (bool, error) {
	if p.logger != nil {
		p.logger.Info("Checking if person changed", "id", person.Metadata().ID, "name", person.FirstName+" "+person.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// Simulate drift detection for person (e.g., compare with database state)
	// In a real implementation, this would compare desired vs actual state

	return false, nil
}

func (p *ExampleProvider) Functions() plugins.ProviderFunctions {
	return p.functions
}
