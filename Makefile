protos:
	mkdir -p plugins/proto
	protoc --go_out=. --go_opt=module=github.com/jumppad-labs/hclconfig --go-grpc_out=. --go-grpc_opt=module=github.com/jumppad-labs/hclconfig --proto_path=. plugins/plugin.proto

# Install mockery for generating mocks
install-mockery:
	go install github.com/vektra/mockery/v2@latest

# Generate mocks using mockery configuration
mocks:
	mockery

# Generate mocks with clean slate (removes existing mocks first)
mocks-clean:
	rm -rf plugins/mocks
	mockery

# Install tools and generate mocks
setup-mocks: install-mockery mocks

.PHONY: protos install-mockery mocks mocks-clean setup-mocks