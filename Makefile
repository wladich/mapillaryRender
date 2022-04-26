all: vet fmt lint build

check: vet fmt lint

vet:
	go vet ./...

fmt:
	go list -f '{{.Dir}}' ./... | xargs -L1 gofmt -l
	test -z $$(go list -f '{{.Dir}}' ./... | xargs -L1 gofmt -l)

lint:
	golint -set_exit_status ./...

build:
	go build -o bin/mapillary-image-tile ./cmd/cli
	go build -o bin/mapillary-image-tile-server ./cmd/server