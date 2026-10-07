# syntax=docker/dockerfile:1
FROM node:24-bookworm-slim AS node
FROM golang:1.27-bookworm AS build

ARG WAILS_VERSION=v2.15.0
ARG NFPM_VERSION=v2.47.0
ENV CGO_ENABLED=1
COPY --from=node /usr/local/ /usr/local/
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config python3-yaml rpm file \
    && rm -rf /var/lib/apt/lists/*
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/wailsapp/wails/v2/cmd/wails@${WAILS_VERSION} \
    && go install github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VERSION}

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
    && wails build -platform linux/amd64 -tags webkit2_41 -s -m -trimpath

# Use the embedded app version, and declare the libraries this build links to.
# Keep the native Linux packaging configuration unchanged for other toolchains.
RUN python3 - <<'PY'
import json
import yaml

with open('desktop/packaging/nfpm.yaml') as source:
    config = yaml.safe_load(source)
with open('desktop/wails.json') as source:
    config['version'] = json.load(source)['info']['productVersion']
config['overrides']['deb']['depends'] = [
    'libc6 (>= 2.36)', 'libgtk-3-0', 'libwebkit2gtk-4.1-0',
]
config['overrides']['rpm']['depends'] = [
    'glibc >= 2.36', 'gtk3', 'webkit2gtk4.1',
]
with open('/tmp/nfpm-linux.yaml', 'w') as target:
    yaml.safe_dump(config, target)
PY
RUN mkdir -p /out \
    && nfpm pkg --packager deb --config /tmp/nfpm-linux.yaml --target /out/ \
    && nfpm pkg --packager rpm --config /tmp/nfpm-linux.yaml --target /out/ \
    && cp desktop/build/bin/just-db-desktop /out/ \
    && file /out/just-db-desktop | tee /out/binary-info.txt \
    && readelf -h /out/just-db-desktop | grep 'Advanced Micro Devices X86-64' \
    && ldd /out/just-db-desktop > /out/shared-libraries.txt \
    && ! grep -q 'not found' /out/shared-libraries.txt \
    && dpkg-deb --info /out/*.deb \
    && dpkg-deb --contents /out/*.deb \
    && rpm -qip /out/*.rpm \
    && rpm -qlp /out/*.rpm \
    && cd /out \
    && sha256sum just-db-desktop *.deb *.rpm > SHA256SUMS

FROM scratch AS artifacts
COPY --from=build /out/ /
