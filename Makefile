.PHONY: serve tools test frontend desktop desktop-dev package-linux rpm docker

export PATH := $(HOME)/.local/go/bin:$(HOME)/go/bin:$(PATH)
GO ?= go
LISTEN ?= 127.0.0.1:8080
WAILS ?= wails
NFPM ?= nfpm
VERSION ?= 0.1.0

# Fedora 40+ / Debian with WebKitGTK 4.1 need this Wails build tag.
# Skip the probe when pkg-config is missing so `make serve` stays quiet on macOS.
ifeq ($(origin WAILS_TAGS), undefined)
  WAILS_TAGS := $(shell command -v pkg-config >/dev/null 2>&1 && pkg-config --exists webkit2gtk-4.1 && echo webkit2_41)
endif

serve:
	$(GO) run ./cmd/just-db serve -listen $(LISTEN) -data ./data -ui frontend/dist

tools:
	$(GO) run ./cmd/just-db tools

test:
	$(GO) test ./... -count=1 -timeout 4m

frontend:
	npm --prefix frontend install
	npm --prefix frontend run build

desktop/build/appicon.png: desktop/packaging/appicon.png
	mkdir -p desktop/build
	cp -f desktop/packaging/appicon.png desktop/build/appicon.png

desktop: frontend desktop/build/appicon.png
	cd desktop && $(WAILS) build $(if $(WAILS_TAGS),-tags $(WAILS_TAGS),)

desktop-dev: desktop/build/appicon.png
	cd desktop && $(WAILS) dev $(if $(WAILS_TAGS),-tags $(WAILS_TAGS),)

package-linux: desktop
	mkdir -p dist
	$(NFPM) pkg --packager deb --config desktop/packaging/nfpm.yaml --target dist/
	$(NFPM) pkg --packager rpm --config desktop/packaging/nfpm.yaml --target dist/

docker:
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
		docker build --tag "$${DOCKER_REGISTRY:?Set DOCKER_REGISTRY in .env}/just-db:latest" . && \
		docker push "$${DOCKER_REGISTRY}/just-db:latest"

version:
	$(GO) run ./cmd/just-db version
