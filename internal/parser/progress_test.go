package parser

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// newTestNetwork builds a network resource with the given name, subnet and
// status, the way the parser would hold it in memory.
func newTestNetwork(name, subnet, status string) *structs.Network {
	return &structs.Network{
		ResourceBase: types.ResourceBase{
			Meta: types.Meta{
				Name:   name,
				Type:   structs.TypeNetwork,
				Status: status,
			},
		},
		Subnet: subnet,
	}
}

// newStateWith builds a state holding the given resources.
func newStateWith(t *testing.T, resources ...any) *state.State {
	t.Helper()

	st := state.NewState()
	for _, r := range resources {
		err := st.AppendResource(r)
		require.NoError(t, err)
	}

	return st
}

func TestBuildStateKeepsReachedResources(t *testing.T) {
	reached := newTestNetwork("reached", "10.0.0.0/16", types.StatusCreated)
	current := newStateWith(t, reached)
	previous := state.NewState()

	progress := newApplyProgress()
	progress.record("resource.network.reached", outcome{saved: reached})

	built, err := progress.buildState(current, previous)
	require.NoError(t, err)

	require.Equal(t, 1, built.ResourceCount())

	saved, err := built.FindResource("resource.network.reached")
	require.NoError(t, err)

	network := saved.(*structs.Network)
	require.Equal(t, "10.0.0.0/16", network.Subnet)
	require.Equal(t, types.StatusCreated, network.Meta.Status)
	require.Equal(t, "resource.network.reached", network.Meta.ID)
}

func TestBuildStateKeepsFailedResourceAsFailed(t *testing.T) {
	failed := newTestNetwork("broken", "10.0.0.0/16", types.StatusFailed)
	current := newStateWith(t, failed)

	// the previous apply had created it, the failed outcome must win
	previous := newStateWith(t, newTestNetwork("broken", "10.9.0.0/16", types.StatusCreated))

	progress := newApplyProgress()
	progress.record("resource.network.broken", outcome{failed: true, saved: failed})

	built, err := progress.buildState(current, previous)
	require.NoError(t, err)

	require.Equal(t, 1, built.ResourceCount())

	saved, err := built.FindResource("resource.network.broken")
	require.NoError(t, err)

	network := saved.(*structs.Network)
	require.Equal(t, types.StatusFailed, network.Meta.Status)
	require.Equal(t, "10.0.0.0/16", network.Subnet)
}

func TestBuildStateKeepsPreviousEntryForUnreachedExistingResource(t *testing.T) {
	// the config now holds an edited subnet, but the walk never reached it
	configured := newTestNetwork("unreached", "10.1.0.0/16", "")
	current := newStateWith(t, configured)

	previousNetwork := newTestNetwork("unreached", "10.0.0.0/16", types.StatusCreated)
	previousNetwork.ProviderID = "id-unreached"
	previous := newStateWith(t, previousNetwork)

	progress := newApplyProgress()

	built, err := progress.buildState(current, previous)
	require.NoError(t, err)

	require.Equal(t, 1, built.ResourceCount())

	saved, err := built.FindResource("resource.network.unreached")
	require.NoError(t, err)

	network := saved.(*structs.Network)
	require.Equal(t, "10.0.0.0/16", network.Subnet)
	require.Equal(t, "id-unreached", network.ProviderID)
	require.Equal(t, types.StatusCreated, network.Meta.Status)
}

func TestBuildStateOmitsUnreachedNewResource(t *testing.T) {
	reached := newTestNetwork("reached", "10.0.0.0/16", types.StatusCreated)
	unreached := newTestNetwork("new", "10.1.0.0/16", "")
	current := newStateWith(t, reached, unreached)
	previous := state.NewState()

	progress := newApplyProgress()
	progress.record("resource.network.reached", outcome{saved: reached})

	built, err := progress.buildState(current, previous)
	require.NoError(t, err)

	require.Equal(t, 1, built.ResourceCount())

	_, err = built.FindResource("resource.network.new")
	require.Error(t, err)

	_, err = built.FindResource("resource.network.reached")
	require.NoError(t, err)
}
