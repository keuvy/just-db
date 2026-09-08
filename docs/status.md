# Project status and interfaces

Version **0.2.0**. Source review: **2026-09-08**. This page describes implemented behavior, not a claim that every platform and database version has been tested. Start with the [documentation index](README.md) for operational guides.

## Implemented behavior

Two entry points share the Go engines in `internal/`:

| Entry point | Role |
| --- | --- |
| `just-db` | CLI operations and the `serve` HTTP application |
| `just-db-desktop` | Wails v2 native app with the same frontend and engine operations |

The application saves encrypted connection profiles, lists databases, exports dumps, restores local or stored dumps, and lists/downloads/deletes files in the dump library. The UI has a profile sidebar, an Export/Restore workspace, session activity, Tools & settings, responsive navigation, and light/dark/system appearance.

macOS packaging supports a universal Intel/Apple Silicon app in a versioned DMG with an Applications shortcut. The web server supports optional basic authentication and an IP allowlist with explicit trusted-proxy handling.

SQLite, cross-engine conversion, scheduled backups, persistent background jobs, progress percentages, cancellation, automatic PostgreSQL client-version selection, and a client-version selector per profile are not implemented. The UI permits one export or restore at a time in that UI session; it is not a server-wide job queue.

## Source layout

| Path | Responsibility |
| --- | --- |
| [`cmd/just-db/`](../cmd/just-db/) | CLI commands and profile/connection flag resolution |
| [`internal/engine/`](../internal/engine/) | Engine interface, connection fields, formats, and import/export options |
| [`internal/postgres/`](../internal/postgres/), [`internal/mysql/`](../internal/mysql/) | Database client commands and engine-specific behavior |
| [`internal/tools/`](../internal/tools/) | Executable discovery and client version display |
| [`internal/proc/`](../internal/proc/) | Subprocess execution, environment, and error output |
| [`internal/job/`](../internal/job/) | Dump filenames, file operations, export/import orchestration |
| [`internal/profile/`](../internal/profile/) | Named encrypted profiles |
| [`internal/appdir/`](../internal/appdir/), [`internal/appmeta/`](../internal/appmeta/) | Desktop storage locations and app identity/version |
| [`server/`](../server/) | HTTP routes, authentication, IP access, environment defaults, export logs |
| [`desktop/`](../desktop/) | Wails bindings, native dialogs, window setup, and packaging |
| [`frontend/src/`](../frontend/src/) | Shared TypeScript interface and HTTP/Wails adapter |
| [`Dockerfile`](../Dockerfile) | Node 24 frontend build, Go 1.27 server build, PostgreSQL 17 runtime tools |
| [`docker-compose.yml`](../docker-compose.yml) | Web app plus optional PostgreSQL 18/MySQL 8.4 test services |

## Engines and client selection

| Engine | Export formats | Restore executable |
| --- | --- | --- |
| PostgreSQL | Custom `.dump` by default, or plain `.sql` | `pg_restore` for custom archives; `psql` for SQL |
| MySQL / MariaDB | SQL only; custom requests are coerced to SQL | `mysql` or `mariadb` |

The app searches `PATH` first, then a fixed list of common directories. Each PostgreSQL executable is resolved independently. It displays the detected version but does not compare it with the server version. Tool presence and a successful connection test do not prove that export or restore will work. See [tool discovery and compatibility](troubleshooting.md).

For backups intended to return to the same PostgreSQL major version, use that major's client tools. Newer `pg_dump` can read supported older servers; older `pg_dump` refuses newer-major servers. Loading a dump into an older major version is not guaranteed, even if the source server was that older version. See the [PostgreSQL compatibility notes](https://www.postgresql.org/docs/18/app-pgdump.html#APP-PGDUMP-NOTES).

PostgreSQL custom restore uses a seekable file. The Go import contract requires `Confirm`; the UI adds a review step, SQL engine acknowledgment, and typed database-name confirmation when drop-existing is enabled.

| Drop-existing mode | Behavior |
| --- | --- |
| PostgreSQL custom archive | `pg_restore --clean --if-exists` drops the objects being restored |
| PostgreSQL SQL | Drops `public` with `CASCADE`, recreates it, and reapplies grants before running the SQL |
| MySQL / MariaDB SQL | Drops base tables in the selected database before running the SQL |

These operations do not issue `DROP DATABASE`. SQL files can contain destructive statements independently of the drop-existing option; PostgreSQL SQL exports currently include `--clean --if-exists`. A failed restore can leave partial changes.

PostgreSQL passwords are passed through `PGPASSWORD`. MySQL uses a temporary mode-0600 defaults file with `--defaults-file` first in the command arguments. MySQL/MariaDB-specific SSL and public-key flags are selected in the engine. See the [engine source](../internal/mysql/mysql.go).

## CLI

Run from the repository with `go run ./cmd/just-db <command>`, or use a built `just-db` binary.

| Command | Purpose and main flags |
| --- | --- |
| `version` | Print the runtime version |
| `tools` | Report executable paths, versions, and missing tools |
| `serve` | HTTP app; `-listen`, `-data`, `-ui`, `-auth-user`, `-auth-password` |
| `test` | Test a connection; connection flags or `-profile NAME` |
| `databases` | List databases; connection flags or `-profile NAME` |
| `export` | Export with required `-out`; optional `-format sql` or `-format custom` |
| `import` | Restore with required `-in` and `-confirm`; optional `-drop` and `-format` |
| `profile list` | List summaries; optional `-data` |
| `profile save` | Create or overwrite `-name NAME` using connection flags |
| `profile show` | Read `-name NAME`; password hidden unless `-include-password` |
| `profile delete` | Delete `-name NAME` |

Connection flags are `-engine`, `-host`, `-port`, `-user`, `-password`, `-database`, and `-sslmode`. With `-profile`, explicitly supplied flags override stored fields. Password fallback is `JUSTDB_PASSWORD`, then `PGPASSWORD`, then `MYSQL_PWD`. Prefer the environment to putting secrets in shell command history.

CLI storage defaults to `JUSTDB_DATA` or `./data`. `make serve` explicitly uses `./data`, `frontend/dist`, and `LISTEN`, whose default is `127.0.0.1:8080`. Direct `serve` defaults to `0.0.0.0:8080` and honors the corresponding `JUSTDB_*` settings. See [the CLI source](../cmd/just-db/main.go).

## HTTP

Routes are defined in [`server/server.go`](../server/server.go); payloads and file handling are in [`server/api.go`](../server/api.go).

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/health`, `/api/health` | Runtime version, mode, and storage paths |
| GET | `/api/engines`, `/api/tools` | Engine descriptions and detected client tools |
| GET | `/api/defaults` | Web connection defaults, storage paths, and auth status |
| GET | `/api/dumps` | Stored dump files |
| GET | `/api/dumps/{name}` | Download a stored dump; HEAD is also supported |
| DELETE | `/api/dumps/{name}` | Delete a stored dump |
| GET | `/api/profiles` | Profile summaries without passwords |
| GET | `/api/profiles/{name}` | Full profile including its password |
| PUT | `/api/profiles/{name}` | Create or replace the named profile |
| DELETE | `/api/profiles/{name}` | Delete the named profile |
| POST | `/api/test-connection` | Test `{engine, connection}` |
| POST | `/api/databases` | List databases for `{engine, connection}` |
| POST | `/api/export` | Export `{engine, connection, format, fileName, tables}` and return path/size/format |
| POST | `/api/import` | Restore a stored dump via JSON or a browser upload via multipart |

Import JSON uses `engine`, `connection`, `format`, `fileName`, `dropExisting`, and `confirm`. Multipart uses the same connection/options fields plus `file`; `connection` is JSON text. Uploads accept `.sql`, `.dump`, `.backup`, and `.pgdump`, are copied to temporary storage, and are removed after the request. The 32 MiB multipart setting is a memory threshold, not a maximum upload size.

Setting `JUSTDB_AUTH_USER` enables basic authentication. Both health paths bypass basic auth, but only `/health` bypasses the IP allowlist. `/api/health` remains subject to `JUSTDB_ALLOWED_IPS`. The UI, downloads, defaults, and profiles use the same access controls. Full profiles and web defaults can contain database passwords.

Exports log `export started`, `export completed`, or `export failed` with a request ID and timing; success includes bytes. The export failure log redacts the supplied password and configured PostgreSQL password values. This is request logging, not persistent job history. See [deployment and access configuration](easypanel.md).

## Storage and profiles

Profiles are `{dataDir}/profiles/<name>.jdb`. The name is the identity; saving the same name overwrites it. The store uses AES-256-GCM with an environment-supplied key or a generated key file. See the [encryption decision](adr/0001-encrypted-profiles.md) and [recovery guidance](troubleshooting.md#profiles-missing-or-cannot-decrypt-profile).

Dumps are `{dataDir}/backups/`. The library accepts regular `.sql`, `.dump`, `.backup`, and `.pgdump` files and does not record server version, source engine, or profile provenance. Export creates a library file before the UI offers a download or native save-copy dialog. Cancelling that dialog keeps the original. Failed exports remove the output file.

Desktop storage is `~/Library/Application Support/just-db` on macOS, and `$XDG_DATA_HOME/just-db` or `~/.local/share/just-db` on Linux. Desktop and CLI profiles are separate unless the CLI is pointed at the desktop data directory. See [desktop setup](desktop.md) and [testing](testing.md).
