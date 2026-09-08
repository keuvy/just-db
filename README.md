# just-db

Same-engine only: a Postgres dump restores to Postgres, a MySQL dump restores to MySQL.

Two binaries share one Go engine:

- `just-db serve` — web UI for EasyPanel / Docker
- Wails desktop app — native window on Debian, Fedora, Arch, and macOS

SQLite is intentionally not in v1.

## Requirements

- Go 1.25+ (`export PATH="$HOME/.local/go/bin:$PATH"` if you installed the official tarball under `~/.local/go`)
- Node.js 22+ (frontend)
- For desktop: [Wails v2](https://wails.io/) plus GTK/WebKit on Linux
- Client tools on the machine or in the image: `pg_dump`, `pg_restore`, `psql`, `mysqldump`/`mariadb-dump`, `mysql`/`mariadb`

## Commands

```bash
go run ./cmd/just-db version
go run ./cmd/just-db tools
go run ./cmd/just-db serve -listen 127.0.0.1:8080
go run ./cmd/just-db test -engine postgres -host 127.0.0.1 -user justdb -database justdb
go run ./cmd/just-db export -engine postgres -host 127.0.0.1 -user justdb -database justdb -format custom -out ./data/backups/app.dump
go run ./cmd/just-db import -engine postgres -host 127.0.0.1 -user justdb -database justdb -in ./data/backups/app.dump -confirm
go run ./cmd/just-db profile save -name prod -engine postgres -host 127.0.0.1 -user justdb -database justdb
go run ./cmd/just-db test -profile prod
go run ./cmd/just-db export -profile prod -format custom -out ./data/backups/app.dump
go run ./cmd/just-db export -engine mysql -host 127.0.0.1 -port 3306 -user justdb -database justdb -format sql -out ./data/backups/app.sql
go run ./cmd/just-db import -engine mysql -host 127.0.0.1 -port 3306 -user justdb -database justdb -in ./data/backups/app.sql -confirm
```

Password: `-password`, `JUSTDB_PASSWORD`, or `PGPASSWORD` / a MySQL defaults file. Never on the `pg_dump` / `mysqldump` command line.

Client tools should match the server major version (`pg_dump` 18 into PostgreSQL 18). A newer dump can fail restore on an older server.

```bash
make test
make frontend
make serve
```

Desktop (from `desktop/`):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails doctor
make desktop
```

The GUI binary is **`just-db-desktop`** so it does not clash with the CLI. Details: [docs/desktop.md](docs/desktop.md).

## EasyPanel

To build and push an image to your registry, copy `.env.example` to `.env` and
set `DOCKER_REGISTRY` (for example, `registry.example.com/team`, without a
URL scheme or trailing slash). From the repository root, run:

```bash
make docker
```

The Makefile loads `.env` as shell-compatible assignments. This builds
`DOCKER_REGISTRY/just-db:latest` and pushes it after a successful build.
Docker builds both the frontend and Go binary inside the image. Log in with
`docker login <registry-host>` first if your registry requires authentication.

Step-by-step: [docs/easypanel.md](docs/easypanel.md).

Same project as the database, domain to port **8080**, volume `/data`. Optional basic auth: `JUSTDB_AUTH_USER` / `JUSTDB_AUTH_PASSWORD`. Prefill the UI with `JUSTDB_ENGINE`, `JUSTDB_HOST`, `JUSTDB_USER`, `JUSTDB_PASSWORD`, `JUSTDB_DATABASE`. Optional profile key: `JUSTDB_PROFILES_KEY` (keeps the encryption key off the volume).

```bash
docker build -t just-db .
docker run --rm -p 8080:8080 -v justdb-data:/data \
  -e JUSTDB_AUTH_USER=admin \
  -e JUSTDB_AUTH_PASSWORD=secret \
  just-db
```
