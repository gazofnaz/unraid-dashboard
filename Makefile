VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build web embed test vet run dev-api dev-web docker clean

all: build

## Build the server without the embedded UI (fast, for backend work).
build:
	go build -o bin/arraydeck ./cmd/arraydeck

## Build the frontend production bundle.
web:
	cd web && npm run build

## Build the full binary with the compiled UI embedded.
embed: web
	rm -rf internal/api/webdist
	cp -r web/dist internal/api/webdist
	go build -tags embedui \
		-ldflags "-X github.com/gazofnaz/unraid-dashboard/internal/config.Version=$(VERSION)" \
		-o bin/arraydeck ./cmd/arraydeck

test:
	go test ./...

vet:
	go vet ./...

## Run the API against the local Docker socket with throwaway data.
run: build
	DATA_DIR=/tmp/arraydeck-dev TEMPLATES_DIR=$(PWD)/internal/integration/testdata/templates \
		./bin/arraydeck

dev-api: run

## Run the Vite dev server (proxies /api to localhost:8417).
dev-web:
	cd web && npm run dev

docker:
	docker build --build-arg VERSION=$(VERSION) -t arraydeck:$(VERSION) -t arraydeck:local .

clean:
	rm -rf bin web/dist internal/api/webdist
