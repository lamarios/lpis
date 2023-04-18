arch := $(shell go env GOARCH)
os := $(shell go env GOOS)

build:
	 CGO_CFLAGS_ALLOW=".*" CGO_LDFLAGS_ALLOW=".*" go mod download
	 CGO_CFLAGS_ALLOW=".*" CGO_LDFLAGS_ALLOW=".*" go build -ldflags="-s -w -X main.VERSION=$(version)" -o lpis

package:
	tar -czf lpis-$(os)-$(arch).tar.gz lpis