package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jumppad-labs/hclconfig/plugins/proto"
)

// GRPCLogger implements Logger interface using consolidated HostCallbackService client
type GRPCLogger struct {
	client proto.HostCallbackServiceClient
}

// Ensure GRPCLogger implements Logger interface
var _ Logger = (*GRPCLogger)(nil)

func (l *GRPCLogger) Info(msg string, args ...interface{}) {
	l.client.Info(context.Background(), &proto.LogRequest{
		Message: msg,
		Args:    interfaceArgsToStrings(args),
	})
}

func (l *GRPCLogger) Debug(msg string, args ...interface{}) {
	l.client.Debug(context.Background(), &proto.LogRequest{
		Message: msg,
		Args:    interfaceArgsToStrings(args),
	})
}

func (l *GRPCLogger) Warn(msg string, args ...interface{}) {
	l.client.Warn(context.Background(), &proto.LogRequest{
		Message: msg,
		Args:    interfaceArgsToStrings(args),
	})
}

func (l *GRPCLogger) Error(msg string, args ...interface{}) {
	l.client.Error(context.Background(), &proto.LogRequest{
		Message: msg,
		Args:    interfaceArgsToStrings(args),
	})
}

// GRPCState implements State interface using consolidated HostCallbackService client
type GRPCState struct {
	client proto.HostCallbackServiceClient
}

// Ensure GRPCState implements State interface
var _ State = (*GRPCState)(nil)

func (s *GRPCState) Get(key string) (any, error) {
	resp, err := s.client.Get(context.Background(), &proto.StateGetRequest{Key: key})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}

	// Unmarshal the resource data back to any
	// This is a simplified implementation - in practice you'd need type information
	// to properly unmarshal to the correct concrete type
	var resource map[string]interface{}
	if err := json.Unmarshal(resp.ResourceData, &resource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	// TODO: This needs to be enhanced to return proper typed resources
	// For now, returning nil as this is a complex cross-process serialization issue
	return nil, fmt.Errorf("resource deserialization not implemented")
}

func (s *GRPCState) Find(pattern string) ([]any, error) {
	resp, err := s.client.Find(context.Background(), &proto.StateFindRequest{Pattern: pattern})
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf(resp.Error)
	}

	// Similar issue as Get - need to properly deserialize resources
	// TODO: Implement proper resource deserialization
	return nil, fmt.Errorf("resource deserialization not implemented")
}

// Helper functions
func interfaceArgsToStrings(args []interface{}) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			result[i] = v
		case int:
			result[i] = strconv.Itoa(v)
		case float64:
			result[i] = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			result[i] = strconv.FormatBool(v)
		default:
			result[i] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
