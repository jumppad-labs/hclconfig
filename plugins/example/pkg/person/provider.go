package person

import (
	"context"

	"github.com/jumppad-labs/hclconfig/logger"
	"github.com/jumppad-labs/hclconfig/plugins"
)

// ExampleProvider is a basic implementation of Provider[*Person]
// that demonstrates the structure and lifecycle methods for Person resources.
type ExampleProvider struct {
	state     plugins.State
	functions plugins.ProviderFunctions
	logger    logger.Logger
}

// Compile-time check to ensure ExampleProvider implements ResourceProvider[*Person]
var _ plugins.ResourceProvider[*Person] = (*ExampleProvider)(nil)

func (p *ExampleProvider) Init(state plugins.State, functions plugins.ProviderFunctions, logger logger.Logger) error {
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

func (p *ExampleProvider) Changed(ctx context.Context, old *Person, new *Person) (bool, error) {
	if p.logger != nil {
		p.logger.Info("Checking if person changed", "id", new.Metadata().ID, "old_name", old.FirstName+" "+old.LastName, "new_name", new.FirstName+" "+new.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// Simulate drift detection for person (e.g., compare old vs new state)
	// In a real implementation, this would compare the old state with the new desired state
	// For this example, we'll check if names or ages differ
	if old.FirstName != new.FirstName || old.LastName != new.LastName || old.Age != new.Age {
		return true, nil
	}

	return false, nil
}

func (p *ExampleProvider) Update(ctx context.Context, person *Person) error {
	if p.logger != nil {
		p.logger.Info("Updating person", "id", person.Metadata().ID, "name", person.FirstName+" "+person.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate person update (e.g., update in database, modify user account, etc.)
	// In a real implementation, this would update the resource

	return nil
}

func (p *ExampleProvider) Functions() plugins.ProviderFunctions {
	return p.functions
}
