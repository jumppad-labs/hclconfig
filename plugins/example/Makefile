build_plugin:
	go build -o build/example main.go

test: build_plugin
	go test -v ./...

clean:
	rm -rf build/