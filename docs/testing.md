# Testing

This page lists available checks and their scope. It is not a record that they passed for the current working tree. Earlier visual and build results remain dated in the [UI guide](ui.md#verification-recorded-on-2026-09-04).

## Automated checks

From the repository root:

```bash
# Go checks without the two database roundtrip tests.
go test ./... -skip '^TestRoundTrip' -count=1 -timeout 4m

# Frontend state, UI interaction, and HTTP payload tests.
npm --prefix frontend test

# TypeScript checking and production frontend compilation.
npm --prefix frontend run build
```

Install the frontend dependencies with `npm --prefix frontend ci` first when needed. Native Go packages may require the [desktop build prerequisites](desktop.md#build-prerequisites). Frontend tests use Vitest with happy-dom; they do not open a native Wails window.

`make test` runs `go test ./... -count=1 -timeout 4m`, including database roundtrip tests. Check the output for skips before claiming that database export and restore were verified.

## Database roundtrips

The tests create tables and databases and perform restores. Use fresh, disposable database instances. They do not make an existing user database safe for testing and are not written as an idempotent migration.

| Test | Default container | Coverage |
| --- | --- | --- |
| `TestRoundTripCustomAndSQL` | `postgres:18-alpine` | PostgreSQL connection, custom/SQL export and restore, confirmation requirement, restored data |
| `TestRoundTripSQL` | `mysql:8.4` | MySQL connection, SQL export and restore, drop-existing behavior, confirmation requirement, restored data |

When no test host is configured, each test looks for Podman before Docker and tries to start a temporary container. It skips if neither command is available or the container cannot start. The host running the Go tests still needs compatible client executables; starting a database container does not provide client tools to that host.

The tests also accept externally provisioned disposable services:

| Engine | Environment variables |
| --- | --- |
| PostgreSQL | `JUSTDB_TEST_PGHOST`, `JUSTDB_TEST_PGPORT`, `JUSTDB_TEST_PGUSER`, `JUSTDB_TEST_PGPASSWORD`, `JUSTDB_TEST_PGDATABASE` |
| MySQL | `JUSTDB_TEST_MYSQLHOST`, `JUSTDB_TEST_MYSQLPORT`, `JUSTDB_TEST_MYSQLUSER`, `JUSTDB_TEST_MYSQLPASSWORD`, `JUSTDB_TEST_MYSQLDATABASE`, `JUSTDB_TEST_MYSQL_ROOT` |

`JUSTDB_TEST_PGIMAGE` and `JUSTDB_TEST_MYSQLIMAGE` override images for containers started by the tests. The MySQL test also uses an administrative account for creating its restore database.

The repository's Compose file can start only the test services:

```bash
docker compose --profile test up -d postgres mysql
```

| Service | Host endpoint | Database/user/password |
| --- | --- | --- |
| PostgreSQL 18 | `127.0.0.1:55432` | `justdb` / `justdb` / `justdb` |
| MySQL 8.4 | `127.0.0.1:33060` | `justdb` / `justdb` / `justdb`; root password is also `justdb` |

Set the corresponding test host/port variables to use these services. The Compose web app uses PostgreSQL 17 clients by default, so it cannot export the PostgreSQL 18 test service without changing its client version. The Go roundtrip tests run on the host and use that host's clients.

## Packaging checks

For recipe inspection without creating an app or installer:

```bash
make -n desktop
make -n dmg
make -n package-linux
git diff --check
```

A dry run confirms recipe expansion, not successful compilation, executable architectures, signing, or Finder layout. Actual packaging commands and output paths are in the [desktop guide](desktop.md).

## Runtime checks

For a release, record which OS, app architecture, client versions, and database versions were used. Check the full path from profile selection through a disposable export/restore, plus saving or downloading the resulting dump.

Native checks include open/save dialogs, cancellation, opening the dump folder, window dragging, theme changes, and installation from the DMG. Browser fixture tests cover UI state and contracts but do not validate database connectivity or native dialogs. Linux webview behavior, Apple Silicon execution, and older macOS versions require their own runtime evidence.

## Documentation refresh on 2026-09-08

The 0.2.0 documentation refresh compared docs with source, configuration, routes, and packaging scripts. It checked local Markdown links and Makefile recipe expansion. It did not run the test suite, build or install an app, create a DMG, deploy a container, or connect to a database.
