# Troubleshooting

These checks describe the 0.2.0 source. Run them on the machine or container executing just-db. The [desktop app](desktop.md) uses local client tools; the [web app](easypanel.md) uses tools on its server, regardless of which computer opens the browser.

## Identify the running copy

Open Tools & settings and check the runtime, version, data directory, and paths of the detected executables. A CLI check from the repository is:

```bash
make version
make tools
```

These commands inspect the repository's CLI environment. They do not prove that a separately installed app or a remote web server is running the same version or has the same `PATH`.

## Tools missing

`Not found on the app host` means executable discovery failed. The required programs are:

| Engine | Export | SQL restore and connection checks | Custom restore |
| --- | --- | --- | --- |
| PostgreSQL | `pg_dump` | `psql` | `pg_restore` |
| MySQL / MariaDB | `mysqldump` or `mariadb-dump` | `mysql` or `mariadb` | Not supported |

The resolver searches `PATH` first, then the directories listed in [`internal/tools/tools.go`](../internal/tools/tools.go). This includes `/opt/homebrew/bin`, `/usr/local/bin`, common Homebrew `libpq` and `mysql-client` prefixes, `/usr/local/mysql/bin`, and selected `/usr/pgsql-*/bin` directories. It is a fixed list, not a scan of every installed database version.

For MySQL installed with the macOS package installer, check:

```bash
/usr/local/mysql/bin/mysql --version
/usr/local/mysql/bin/mysqldump --version
```

Version 0.2.0 source includes `/usr/local/mysql/bin` as a fallback. Rebuild and replace an older installed app to receive that fix. Installing clients on the build computer does not install them on a recipient's Mac.

A terminal and a Finder-launched app can have different search paths. Changing `.zshrc` alone does not configure an already running desktop process. Quit the old app before testing a launch with an explicit tool directory.

`Tools available` reports discovery, not server compatibility or database connectivity. The resolver also attempts `--version`; an empty version does not make a discovered executable count as missing.

## PostgreSQL server version mismatch

Example:

```text
server version: 15.3; pg_dump version: 14.21
aborting because of server version mismatch
```

The selected `pg_dump` is older than the server's major version. Minor releases need not be identical: a current 15.x client is the appropriate choice for a backup that will be restored to PostgreSQL 15.

Newer `pg_dump` can export supported older servers, but its output is not guaranteed to restore into an older server major version. This also applies when the original server was that older version. Always consider the destination as well as the source. See the [PostgreSQL compatibility notes](https://www.postgresql.org/docs/18/app-pgdump.html#APP-PGDUMP-NOTES).

On a Mac using Homebrew, an example for PostgreSQL 15 is:

```bash
brew install postgresql@15
```

Quit just-db and launch the installed app with that directory first:

```bash
env PATH="$(brew --prefix postgresql@15)/bin:$PATH" \
  /Applications/just-db.app/Contents/MacOS/just-db-desktop
```

Adjust the app path if it is installed elsewhere. Verify all three PostgreSQL tool paths in Tools & settings before retrying. This affects this launch; it does not persist a selection in a profile. The [Homebrew package](https://formulae.brew.sh/formula/postgresql%4015) is versioned and is not linked into the generic Homebrew binary directory by default.

The current app chooses the first executable found for each tool independently. Automatic server detection, a client-version selector per connection, and installation of missing versions are not implemented. Replacing the app alone does not replace an older `pg_dump` on the host.

For Docker, install the appropriate clients in the image or change the runtime PostgreSQL image tag and rebuild. The current image contains PostgreSQL 17 clients. See [container client versions](easypanel.md#client-tools-and-postgresql-versions).

## Restore fails with version or SQL errors

For a custom archive error such as `unsupported version in file header`, check which `pg_dump` created it and choose a `pg_restore` capable of reading that archive, normally from the same client release. Also verify compatibility with the destination server.

For SQL errors such as an unknown `SET` option, a newer client may have produced commands that an older target does not understand. Use a dump/client combination suitable for the destination, or a deliberate database upgrade workflow. Changing a file extension does not convert its format or version.

A successful connection test only proves the test query succeeded. Export permissions, supported formats, and restore compatibility are separate. A failed restore may already have applied database changes. Review the exact target and error before retrying; enabling drop-existing is not a version-compatibility fix.

## Dump created, but download or save failed

Export first creates the dump in the library. A cancelled desktop save dialog leaves the original there. A copy/download error is reported separately from an export error; retry saving from the result or Dumps instead of exporting again.

In web mode, the original remains on the server. In desktop mode, it is under the local data directory. Uploaded browser restore inputs are temporary and are not added to the dump library.

## Profiles missing or cannot decrypt profile

Check the data directory in Tools & settings. The desktop app uses its per-user data directory, while the CLI defaults to `./data` or `JUSTDB_DATA`, and Docker uses `/data`.

Profiles require the same encryption key that wrote them. `JUSTDB_PROFILES_KEY` takes precedence over the key file; setting or changing it does not migrate existing profiles. When transferring profiles, retain the original environment key or the original `profiles/key` file as appropriate. See the [encryption decision](adr/0001-encrypted-profiles.md).

## Web access denied

HTTP 401 indicates the basic-auth credentials were not accepted. HTTP 403 with `IP not allowed` comes from the IP allowlist. Check `JUSTDB_ALLOWED_IPS`, the actual proxy peer addresses in `JUSTDB_TRUSTED_PROXIES`, and the logged `client_ip`.

`/health` bypasses both controls for container health checks. `/api/health` bypasses basic auth but remains subject to the IP allowlist. For proxy configuration and examples, use the [deployment guide](easypanel.md#restrict-web-access-by-ip).
