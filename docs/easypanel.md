# Deploy just-db on EasyPanel

just-db is an HTTP app. Put it in the **same EasyPanel project** as the PostgreSQL or MySQL service you want to dump, so it can reach the database by service name on the internal Docker network.

## 1. Create the App

1. Open the project that already has your database.
2. **New Service → App**.
3. Source: this Git repository, or an image built from the root `Dockerfile`.
4. Set the **domain** target port to **8080**.
5. Add a **Volume** mount: name `justdb-data`, path `/data`.

EasyPanel health checks can use `GET /health`. That path stays public even when basic auth is on.

## 2. Environment

| Variable | Example | Purpose |
|---|---|---|
| `JUSTDB_LISTEN` | `0.0.0.0:8080` | Bind address (default in the image) |
| `JUSTDB_DATA` | `/data` | Volume root |
| `JUSTDB_UI_DIR` | `/usr/share/just-db/ui` | Built UI (set in the image) |
| `JUSTDB_AUTH_USER` | `admin` | HTTP basic auth user |
| `JUSTDB_AUTH_PASSWORD` | a long secret | HTTP basic auth password |
| `JUSTDB_ENGINE` | `postgres` or `mysql` | Prefills the UI |
| `JUSTDB_HOST` | `myproject_postgres` | Database hostname on the project network |
| `JUSTDB_PORT` | `5432` or `3306` | Database port inside Docker |
| `JUSTDB_USER` | `app` | Database user |
| `JUSTDB_PASSWORD` | | Database password (UI default) |
| `JUSTDB_DATABASE` | `app` | Database name |
| `JUSTDB_SSLMODE` | `disable` | Use `disable` for in-project Docker traffic |
| `JUSTDB_PROFILES_KEY` | a long random secret | Encrypts saved profiles; if unset, a key file is written under `/data/profiles/key` |

Host names are the EasyPanel **service names**, often `{project}_{service}` (for example `shop_postgres`). Check the database service hostname in the panel if a test connection fails.

On the internal network, TLS to Postgres/MySQL is usually off: set `JUSTDB_SSLMODE=disable`.

### Restrict web access by IP

Set `JUSTDB_ALLOWED_IPS` in the app service environment to your public client IP
or a comma-separated list of IPs/CIDRs, for example `203.0.113.10`.
An empty value disables the restriction. IPv4 and IPv6 are supported.
Other clients receive HTTP 403; basic auth still applies to allowed clients.
Only `/health` bypasses the IP restriction for container health checks.

Behind EasyPanel, also set `JUSTDB_TRUSTED_PROXIES` to the actual proxy peer
IP or its narrowly scoped network CIDR. The app accepts `X-Forwarded-For` only
from those peers and checks the chain from right to left. Do not use
`0.0.0.0/0` or `::/0`; trust only proxies that correctly append or overwrite
the forwarded header. Without a trusted proxy, the socket peer IP is used.
Denied requests log `client_ip` in the container logs.

Invalid entries prevent server startup. Restart/redeploy after changing these
variables. With Docker Compose, put them in `.env`; Compose forwards them to
the container. Restricting inbound web access does not restrict outbound
database connections.

## 3. Volume layout

Dumps are written under the volume:

```
/data/backups/*.dump    PostgreSQL custom format
/data/backups/*.sql     PostgreSQL or MySQL SQL
/data/profiles/*.jdb    Encrypted connection profiles
/data/profiles/key      Generated key (absent when JUSTDB_PROFILES_KEY is set)
```

The UI lists those files, can restore them, and can download them. Saved profiles appear in the same UI. EasyPanel **volume backups** can copy `/data` to object storage as a second layer; that is a file copy, not a logical database dump. If you did not set `JUSTDB_PROFILES_KEY`, a volume backup also copies the profile key.

## 4. Client tools

The image includes PostgreSQL 17 client tools and `mariadb-client`. The runtime
uses `postgres:17-bookworm`, with its entrypoint replaced by just-db; it does not
start a PostgreSQL server. Client major version should match the server. For a
different PostgreSQL major version, change the runtime image tag and rebuild.
If restore fails with unknown `SET` options, the dump was made with a newer
client than the target server.

## 5. Same engine only

A PostgreSQL dump restores only to PostgreSQL. A MySQL dump restores only to MySQL. There is no conversion.

## 6. Local check

```bash
docker compose up --build just-db
# http://127.0.0.1:8080
```

With auth:

```bash
JUSTDB_AUTH_USER=admin JUSTDB_AUTH_PASSWORD=secret \
  go run ./cmd/just-db serve -listen 127.0.0.1:8080
```
