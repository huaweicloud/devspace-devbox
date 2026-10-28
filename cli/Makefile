GO ?= go
BIN=bin
NAME=devbox

# API server URL baked into the binary at build time.
# Override per build: make build API_SERVER_URL=https://...
API_SERVER_URL ?= https://devbox-test.developer.myhuaweicloud.com

# Login page URL baked into the binary at build time.
# Override per build: make build LOGIN_URL=https://...
LOGIN_URL ?= https://devstation.connect.huaweicloud.com

# Domain of the API key verification endpoint (devbridge relay controller).
# Override per build: make build AUTH_CHECK_DOMAIN=https://...
AUTH_CHECK_DOMAIN ?= https://bridge.developer.myhuaweicloud.com

LDFLAGS := -X github.com/huaweicloud/devspace-devbox/cli/cmd.Version=$(shell git describe --tags --always 2>/dev/null || echo dev) \
	-X github.com/huaweicloud/devspace-devbox/cli/internal/config.DefaultServer=$(API_SERVER_URL) \
	-X github.com/huaweicloud/devspace-devbox/cli/internal/config.LoginURL=$(LOGIN_URL) \
	-X github.com/huaweicloud/devspace-devbox/cli/internal/config.AuthCheckDomain=$(AUTH_CHECK_DOMAIN)

.PHONY: build
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/$(NAME) ./cmd/devbox

.PHONY: install
install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/devbox

.PHONY: test
test:
	$(GO) test ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: generate
generate:
	$(GO) generate ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: check
check: fmt vet lint test

.PHONY: clean
clean:
	rm -rf $(BIN)