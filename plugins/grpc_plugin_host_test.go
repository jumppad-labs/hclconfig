package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/jumppad-labs/xcl/plugins/proto"
)

// fakeReadServiceClient is a fake gRPC client that returns a fixed response
// from Read. The embedded interface satisfies the remaining methods, which
// must not be called by these tests.
type fakeReadServiceClient struct {
	proto.PluginServiceClient

	readResponse *proto.ReadResponse
}

func (c *fakeReadServiceClient) Read(ctx context.Context, in *proto.ReadRequest, opts ...grpc.CallOption) (*proto.ReadResponse, error) {
	return c.readResponse, nil
}

func TestGRPCPluginWrapperReadPreservesErrorMessageContainingPercent(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{Error: "disk 100% full"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	result, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.EqualError(t, err, "disk 100% full")
	require.Nil(t, result)
}

func TestGRPCPluginWrapperReadErrorWithoutNotFoundIsNotErrNotFound(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{Error: "disk 100% full"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	_, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNotFound)
}

func TestGRPCPluginWrapperReadNotFoundIsErrNotFound(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{NotFound: true, Error: "resource not found"},
	}
	wrapper := &grpcPluginWrapper{client: client}

	result, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorContains(t, err, "resource not found")
	require.Nil(t, result)
}

func TestGRPCPluginWrapperReadReturnsEntityData(t *testing.T) {
	client := &fakeReadServiceClient{
		readResponse: &proto.ReadResponse{EntityData: []byte(`{"name":"test"}`)},
	}
	wrapper := &grpcPluginWrapper{client: client}

	result, err := wrapper.Read(context.Background(), "resource", "test", []byte(`{}`), []byte(`{}`))

	require.NoError(t, err)
	require.Equal(t, []byte(`{"name":"test"}`), result)
}
