BINARY := lancache_spy

.PHONY: build linux test race vet clean

# Static binaries, no libc dependency
build:
	CGO_ENABLED=0 go build -o $(BINARY) .

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY)_linux_amd64 .

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY)_* test_no_ui
