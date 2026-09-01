.PHONY: serve tools test frontend desktop docker

export PATH := $(HOME)/.local/go/bin:$(PATH)
GO ?= go
LISTEN ?= 127.0.0.1:8080

serve:
	$(GO) run ./cmd/just-db serve -listen $(LISTEN) -data ./data -ui frontend/dist

tools:
	$(GO) run ./cmd/just-db tools

test:
	$(GO) test ./...

frontend:
	npm --prefix frontend install
	npm --prefix frontend run build

desktop: frontend
	cd desktop && wails build

docker: frontend
	docker build -t just-db:dev .

version:
	$(GO) run ./cmd/just-db version
