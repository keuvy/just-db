# What is already built

Version **0.1.0**. Same-engine only: a PostgreSQL dump restores to PostgreSQL, a MySQL dump restores to MySQL. SQLite is not in this version.

Two binaries share one Go engine (`internal/`):

| Binary | Role |
|---|---|
| `just-db` | CLI plus `just-db serve` (HTTP UI for EasyPanel / Docker) |
| `just-db-desktop` | Wails v2 native window (Debian, Fedora, Arch, macOS) |

Passwords never go on `pg_dump` / `mysqldump` argv. Postgres uses `PGPASSWORD`. MySQL/MariaDB uses a mode-0600 `--defaults-file` (must be the first argument).

How to run and deploy: [README](../README.md), [EasyPanel](easypanel.md), [desktop](desktop.md).

## Phases

| Phase | What shipped |
|---|---|
| 0 Skeleton | Go module, CLI stubs, engine interface, Vite UI, Dockerfile, Makefile |
| 1 PostgreSQL | `pg_dump` / `pg_restore` / `psql`, custom + SQL formats, roundtrip tests |
| 2 MySQL / MariaDB | `mysqldump` / `mysql` (MariaDB names too), SQL only, roundtrip tests |
| 3 EasyPanel | HTTP API, basic auth, `/data` volume, env prefill, docs |
| 4 Native desktop | Wails app, XDG/mac data dir, native dialogs, `.desktop` + nfpm packaging |

Not built (later): scheduled backups, encrypted connection profiles, SQLite.

## Layout

```
cmd/just-db/           CLI: serve, tools, test, export, import, version
internal/
  engine/              Engine interface, Connection, formats, CoerceFormat
  postgres/            pg_dump, pg_restore, psql
  mysql/               mysqldump / mysql via --defaults-file
  job/                 ExportToFile, ImportFromFile, ListDumps, SafeFileName
  proc/                subprocess runner
  tools/               LookPath (including Homebrew / pgsql dirs)
  registry/            postgres + mysql
  appdir/              desktop data dir + backups/
  appmeta/             name + version
server/                HTTP API + basic auth + env defaults
desktop/               Wails v2 (outputfilename just-db-desktop)
  packaging/           icon, .desktop, nfpm.yaml
frontend/              Vite + vanilla TypeScript (HTTP or Wails)
Dockerfile             CGO_ENABLED=0 server + postgresql-client + mariadb-client
docker-compose.yml     app + optional postgres:18 / mysql:8.4 test profiles
```

## Engines

**PostgreSQL** (default port 5432)

- Formats: `custom` (`.dump`, default) and `sql` (`.sql`)
- Restore of custom format uses a seekable file (`pg_restore` cannot take stdin)
- Import requires `confirm`
- Drop-existing drops objects via restore flags; it does not `DROP DATABASE`
- Client major version must match the server (`pg_dump` 18 into PG 16 fails on options such as `SET transaction_timeout`)

**MySQL / MariaDB** (default port 3306)

- Format: SQL only (`custom` is coerced to `sql`)
- `--get-server-public-key` for MySQL 8 `caching_sha2_password` without TLS
- `--ssl-mode=DISABLED` vs MariaDB `--skip-ssl`
- Drop-existing drops **tables**, not the database
- `mysqldump` is not passed `--connect-timeout` (unknown variable)

`just-db tools` and the UI engine cards report whether dump/restore/client binaries are on `PATH`.

## CLI (`just-db`)

```
just-db version
just-db tools
just-db serve [-listen] [-data] [-ui] [-auth-user] [-auth-password]
just-db test  -engine postgres|mysql …
just-db export -engine … -out <path> [-format sql|custom]
just-db import -engine … -in <path> -confirm [-drop] [-format]
```

Password: `-password`, `JUSTDB_PASSWORD`, `PGPASSWORD`, or `MYSQL_PWD`.

## HTTP (`just-db serve`)

Listen default in the image: `0.0.0.0:8080`. Dumps live under `{dataDir}/backups/`.

| Method | Path | Auth |
|---|---|---|
| GET | `/health`, `/api/health` | public even when basic auth is on |
| GET | `/api/engines`, `/api/defaults`, `/api/dumps` | protected if auth is set |
| GET | `/api/dumps/{name}` | download |
| POST | `/api/test-connection`, `/api/export`, `/api/import` | import needs `confirm: true` |

Basic auth: `JUSTDB_AUTH_USER` / `JUSTDB_AUTH_PASSWORD`.

UI prefill: `JUSTDB_ENGINE`, `JUSTDB_HOST`, `JUSTDB_PORT`, `JUSTDB_USER`, `JUSTDB_PASSWORD`, `JUSTDB_DATABASE`, `JUSTDB_SSLMODE`.

Docker volume: `/data`. Health check: `GET /health`.

## Desktop (`just-db-desktop`)

Same UI as the web app. Detects Wails (`window.go.main.App`) vs HTTP.

- Data dir: `$XDG_DATA_HOME/just-db` or `~/.local/share/just-db` (Linux); `~/Library/Application Support/just-db` (macOS)
- Export: native save dialog, default filename like `postgres-app-20260901-201500.dump`
- Import: dump list from the backups folder, or a file picker
- Window title **just-db**; process / `.desktop` / package name **just-db-desktop**
- Linux: `desktop/packaging/just-db-desktop.desktop`, SVG icon, nfpm `.deb`/`.rpm` with client tools as Recommends

`make desktop` needs GTK 3 + WebKitGTK **headers** (see [desktop.md](desktop.md)). On Fedora 40+ that is `gtk3-devel` and `webkit2gtk4.1-devel`, plus `-tags webkit2_41`.

## Frontend

Vanilla TypeScript: engine switch, connection form, test / export / import, dump list. Web can download dumps. Desktop uses native dialogs and can open the backups folder.

## Tests

`make test` (`go test ./...`). Postgres roundtrip uses `postgres:18-alpine` (Podman/Docker). MySQL roundtrip uses `mysql:8.4`.
