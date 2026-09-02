# just-db

Import and export **PostgreSQL** and **MySQL** databases. Same engine only: a Postgres dump restores to Postgres.

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
```

Client tools should match the server major version (`pg_dump` 18 into PostgreSQL 18). A newer dump can fail restore on an older server.

```bash
make test
make frontend
make serve
```

Desktop (from `desktop/`):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
cd desktop && wails dev
```

## EasyPanel

Build and run the Docker image. Mount a volume at `/data`, route the domain to port **8080**, and put this service in the same project as your databases so it can reach `*_postgres:5432` / `*_mysql:3306`.

```bash
docker build -t just-db .
docker run --rm -p 8080:8080 -v justdb-data:/data just-db
```

Environment: `JUSTDB_LISTEN`, `JUSTDB_DATA`, `JUSTDB_UI_DIR`.
