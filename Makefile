arch := $(shell go env GOARCH)
os := $(shell go env GOOS)
version := $(shell ech ${DRONE_TAG:-0.1})

build:
	 CGO_CFLAGS_ALLOW=".*" CGO_LDFLAGS_ALLOW=".*" go mod download
	 CGO_CFLAGS_ALLOW=".*" CGO_LDFLAGS_ALLOW=".*" go build -ldflags="-s -w -X main.VERSION=$(version)" -o lpis

package:
	tar -czf lpis-$(version)-$(os)-$(arch).tar.gz lpis