# syntax=docker/dockerfile:1
FROM node:24-bookworm-slim AS node
FROM golang:1.27-bookworm AS build

ARG WAILS_VERSION=v2.15.0
COPY --from=node /usr/local/ /usr/local/
# GTK/WebKit are used by the Linux test build; NSIS packages the Windows app.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config nsis file binutils p7zip-full \
    && rm -rf /var/lib/apt/lists/*
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION}

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN --mount=type=cache,target=/root/.npm npm --prefix frontend ci
COPY . .
RUN npm --prefix frontend test && npm --prefix frontend run build
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test -tags webkit2_41 ./... -skip '^TestRoundTrip' -count=1 -timeout 4m \
    && mkdir -p desktop/build \
    && cp desktop/packaging/appicon.png desktop/build/appicon.png \
    && cd desktop \
    && CGO_ENABLED=0 wails build -platform windows/amd64 -s -m -trimpath -webview2 embed -nsis

# Read the same version embedded in the app and installer, then verify the payload.
RUN version=$(node -p "require('./desktop/wails.json').info.productVersion") \
    && mkdir -p /out \
    && cp desktop/build/bin/just-db-desktop.exe /out/ \
    && cp desktop/build/bin/just-db-amd64-installer.exe "/out/just-db-${version}-windows-amd64-setup.exe" \
    && file /out/*.exe | tee /out/binary-info.txt \
    && objdump -f /out/just-db-desktop.exe | grep 'architecture: i386:x86-64' \
    && objdump -p /out/just-db-desktop.exe | grep 'Windows GUI' \
    && 7z x "/out/just-db-${version}-windows-amd64-setup.exe" -o/tmp/installer \
    && cmp /out/just-db-desktop.exe /tmp/installer/just-db.exe \
    && cd /out \
    && sha256sum *.exe > SHA256SUMS

FROM scratch AS artifacts
COPY --from=build /out/ /
