BINARY := lancache_spy
export CGO_ENABLED := 0

.PHONY: build linux test vet clean

build:
	go build -o $(BINARY) .

linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)_linux_amd64 .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY)_* test_no_ui
