# syntax=docker/dockerfile:1
FROM node:24-alpine AS ui
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/just-db ./cmd/just-db

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl postgresql-client mariadb-client \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/just-db /usr/local/bin/just-db
COPY --from=ui /src/dist /usr/share/just-db/ui
ENV JUSTDB_LISTEN=0.0.0.0:8080 \
    JUSTDB_DATA=/data \
    JUSTDB_UI_DIR=/usr/share/just-db/ui
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s CMD curl -fsS http://127.0.0.1:8080/health || exit 1
WORKDIR /data
ENTRYPOINT ["/usr/local/bin/just-db"]
CMD ["serve"]
