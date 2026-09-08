# EasyPanel and Docker deployment

The web app executes database tools on its server or inside its container. A browser user's installed PostgreSQL/MySQL clients do not participate. Source review: 2026-09-08, version 0.2.0.

Use the same EasyPanel project/network as the database when connecting by an internal service hostname. Configure the database's actual service hostname and internal port. Inside a container, `127.0.0.1` refers to that container, not another service or the browser's computer.

## Build and publish

EasyPanel can build from this repository's root [Dockerfile](../Dockerfile), or run a previously published image.

For the registry workflow, copy [`.env.example`](../.env.example) to `.env` and set `DOCKER_REGISTRY` to a registry host and optional namespace, such as `registry.example.com/team`. Omit `https://` and a trailing slash. Log in to that registry before running:

```bash
make docker
```

This command both builds and pushes `DOCKER_REGISTRY/just-db:latest`. It loads `.env` as shell assignments. `.env` is excluded from the Docker build context; its auth and access settings must also be configured in the deployed service. Direct Go commands do not automatically load `.env`.

For a local image without a registry push:

```bash
docker build -t just-db .
```

Docker builds the frontend with Node 24 and the CLI with Go 1.27. The final image contains the CLI, built frontend, PostgreSQL 17 clients, and MariaDB clients. It starts `just-db serve`; it does not start the PostgreSQL server from the base image.

## Configure the service

1. Create an App service from the repository or published image.
2. Route its domain to container port `8080`.
3. Mount persistent storage at `/data`, for example a volume named `justdb-data`.
4. Set the web-access and database-default environment values below.
5. Deploy and check `/health`, then verify the runtime and executable paths in Tools & settings.

| Variable | Example/default | Purpose |
| --- | --- | --- |
| `JUSTDB_LISTEN` | `0.0.0.0:8080` | HTTP bind address, set in the image |
| `JUSTDB_DATA` | `/data` | Dumps and profile storage, set in the image |
| `JUSTDB_UI_DIR` | `/usr/share/just-db/ui` | Built frontend directory, set in the image |
| `JUSTDB_AUTH_USER` | `admin` | Enables basic auth when nonempty |
| `JUSTDB_AUTH_PASSWORD` | A secret configured in the service | Web login password |
| `JUSTDB_ALLOWED_IPS` | `203.0.113.10` | Optional comma-separated client IPs/CIDRs |
| `JUSTDB_TRUSTED_PROXIES` | Actual proxy peer IPs/CIDRs | Peers allowed to supply `X-Forwarded-For` |
| `JUSTDB_ENGINE` | `postgres` or `mysql` | Initial web profile editor engine |
| `JUSTDB_HOST` | Database service hostname | Initial database endpoint |
| `JUSTDB_PORT` | `5432` or `3306` | Database service port |
| `JUSTDB_USER` | `app` | Initial database user |
| `JUSTDB_PASSWORD` | Database password | Initial web connection password |
| `JUSTDB_DATABASE` | `app` | Optional initial default database |
| `JUSTDB_SSLMODE` | Value matching the database TLS setup | Initial SSL mode; Compose defaults to `disable` |
| `JUSTDB_PROFILES_KEY` | Stable random secret | Encryption key material for saved profiles |

Web credentials and database credentials are separate. Set both basic-auth fields for a public deployment. Database environment values are initial editor defaults; existing saved profiles retain their stored settings. `/api/defaults` can return the configured database password and uses the same access controls as the rest of the API.

`PGPASSWORD` and `MYSQL_PWD` are password fallbacks if `JUSTDB_PASSWORD` is empty. Keep the profile encryption key consistent across redeployments; changing it does not re-encrypt existing files.

## Restrict web access by IP

`JUSTDB_ALLOWED_IPS` accepts comma-separated IPv4/IPv6 addresses and CIDRs. An empty value disables the restriction. Nonmatching clients receive HTTP 403. Basic auth still applies to allowed clients.

Behind a reverse proxy, set `JUSTDB_TRUSTED_PROXIES` to its actual socket peer address or narrowly scoped network. The server reads `X-Forwarded-For` only from trusted peers and walks the chain from right to left until reaching an untrusted address. Without trusted-proxy configuration, the socket peer is treated as the client. Do not use an all-address range to trust arbitrary forwarded headers.

Invalid IP/CIDR entries prevent server startup. Denied requests log `client_ip`. Restart/redeploy after changing these environment values. These controls affect incoming HTTP traffic, not outbound database connections.

| Route | Basic auth | IP allowlist |
| --- | --- | --- |
| `/health` | Bypassed | Bypassed |
| `/api/health` | Bypassed | Enforced when configured |
| UI, other API routes, downloads | Enforced when configured | Enforced when configured |

Use `/health` for container liveness checks. Its success does not establish database connectivity. Source: [auth middleware](../server/auth.go), [IP middleware](../server/ip_access.go).

## Persistent data

```text
/data/backups/       Dump library: .sql, .dump, .backup, .pgdump
/data/profiles/      Encrypted .jdb connection profiles
/data/profiles/key   Generated key when no environment key is supplied
```

A previously generated key file may remain after an environment key is configured; the environment key takes precedence. Preserve the key that encrypted the current profiles. A volume backup includes the generated key if it is stored on that volume. See the [encryption decision](adr/0001-encrypted-profiles.md).

Exports remain in the library after browser download. Browser restore uploads use temporary files and are removed after the request; they are not added to `/data/backups`. Volume backups copy those files; they do not replace a logical database export.

## Client tools and PostgreSQL versions

The runtime base is currently `postgres:17-bookworm`, plus `mariadb-client`. The image's PostgreSQL 17 `pg_dump` cannot export a PostgreSQL 18 server. For a different server major, choose a compatible runtime tag/client installation and rebuild the image.

For a backup that should restore to the same server major, use matching-major tools. Newer `pg_dump` can read supported older servers, but output from a newer client is not guaranteed to restore to an older major version. A connection test does not validate this. See [PostgreSQL compatibility](https://www.postgresql.org/docs/18/app-pgdump.html#APP-PGDUMP-NOTES) and the [version mismatch guide](troubleshooting.md#postgresql-server-version-mismatch).

The app currently selects the first executable found; it does not automatically choose a tool version for each saved connection. Supporting multiple PostgreSQL client sets in one deployment requires deliberate process/path configuration or future version-selection support.

## Local Compose workflow

```bash
docker compose up --build just-db
```

The web app is exposed on host port `8080` and stores data in `justdb-data`. Compose forwards the variables defined in [`docker-compose.yml`](../docker-compose.yml), including values loaded by Compose from `.env`.

The optional test profile starts PostgreSQL 18 and MySQL 8.4. The web image's PostgreSQL 17 clients are not compatible with exporting that PostgreSQL 18 test service. Use compatible host clients for the [roundtrip tests](testing.md#database-roundtrips), or adjust the web image before testing through the web UI.

A PostgreSQL dump restores to PostgreSQL; a MySQL/MariaDB dump restores to that engine family. The app does not convert between engines.
