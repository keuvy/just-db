# just-db

Same-engine dump and restore for PostgreSQL and MySQL/MariaDB.

**Engine**:
PostgreSQL or MySQL/MariaDB. A dump from one engine restores only to that same engine.
_Avoid_: database type, dialect, backend

**Connection**:
Host, port, user, password, and SSL mode for one engine. Database is optional on a saved connection; dump and restore need a database chosen at that moment.
_Avoid_: DSN, connection string, credentials (as the whole object)

**Profile**:
A named Connection plus its Engine, stored encrypted under the data directory and reused from the CLI, HTTP UI, and desktop app.
_Avoid_: saved connection, preset, bookmark, account

**Dump**:
A file written by export and read by import. Custom (`.dump`) or SQL (`.sql`) for PostgreSQL; SQL only for MySQL.
_Avoid_: backup (except the on-disk `backups/` folder name), snapshot
