package person

import (
	"context"
	"strings"

	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
)

// ExampleProvider is a basic implementation of Provider[*Person]
// that demonstrates the structure and lifecycle methods for Person resources.
//
// It embeds DefaultChanged to get change detection without writing any.
type ExampleProvider struct {
	plugins.DefaultChanged[*Person]

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
		p.logger.Info("Creating person", "event", "create", "resource", person.Meta.ID, "name", person.FirstName+" "+person.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Simulate person creation (e.g., insert into a database). Provider-owned
	// computed fields are set here, configured fields are never changed.
	person.PersonID = "person-" + strings.ToLower(person.FirstName+"-"+person.LastName)

	return person, nil
}

func (p *ExampleProvider) Destroy(ctx context.Context, person *Person, force bool) error {
	if p.logger != nil {
		p.logger.Info("Destroying person", "event", "destroy", "resource", person.Meta.ID, "name", person.FirstName+" "+person.LastName, "force", force)
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

// MissingPersonEmail is a sentinel used to demonstrate ErrNotFound. When the
// saved person has this email, Read reports the person as no longer existing.
const MissingPersonEmail = "missing@example.com"

func (p *ExampleProvider) Read(ctx context.Context, old *Person, new *Person) (*Person, error) {
	if p.logger != nil {
		p.logger.Info("Reading person", "event", "read", "resource", new.Meta.ID, "name", new.FirstName+" "+new.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// A real provider would use old to locate the person, for example by
	// old.PersonID, and report ErrNotFound when it no longer exists
	if old.Email == MissingPersonEmail {
		return nil, plugins.ErrNotFound
	}

	// xcl has already carried the computed PersonID over onto new. A real
	// provider would fill in observed fields here. Configured fields are never
	// changed.
	return new, nil
}

func (p *ExampleProvider) Update(ctx context.Context, person *Person) (*Person, error) {
	// Handle nil person (when no entity data is provided)
	if person == nil {
		if p.logger != nil {
			p.logger.Info("Updating person with no entity data", "event", "update")
		}
		return nil, nil
	}

	if p.logger != nil {
		p.logger.Info("Updating person", "event", "update", "resource", person.Meta.ID, "name", person.FirstName+" "+person.LastName)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Simulate person update (e.g., update in database, modify user account, etc.)
	// In a real implementation, this would update the resource, configured
	// fields are never changed.

	return person, nil
}

func (p *ExampleProvider) Functions() plugins.ProviderFunctions {
	return p.functions
}
