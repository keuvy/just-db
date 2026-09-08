# Desktop app

The native app is **`just-db-desktop`**. It wraps the same Go engines as `just-db serve`. Do not name the GUI binary `just-db` — that name is the CLI.

Window title stays **just-db**. Process / package name is **just-db-desktop** (`StartupWMClass=just-db-desktop`).

## Build

Needs Go 1.25+, Node.js 22+, and [Wails v2](https://wails.io/).

Linux also needs GTK 3 and WebKitGTK **headers** (runtime packages are not enough):

| Distro | Build packages |
|---|---|
| Debian / Ubuntu | `libgtk-3-dev` and `libwebkit2gtk-4.1-dev` (or `libwebkit2gtk-4.0-dev` on older releases) |
| Fedora 40+ | `gtk3-devel webkit2gtk4.1-devel gcc` |
| Arch | `gtk3 webkit2gtk-4.1 base-devel` |

Fedora 40+ and current Debian/Ubuntu ship WebKitGTK **4.1**. `make desktop` passes `-tags webkit2_41` when `pkg-config webkit2gtk-4.1` is present. On older 4.0-only hosts, leave the tag off.

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails doctor
make desktop
```

Dev loop (Vite + live reload):

```bash
make desktop-dev
```

The binary lands at `desktop/build/bin/just-db-desktop`.

## Data directory

Dumps default to a per-user backups folder (created on first launch):

| OS | Path |
|---|---|
| Linux | `$XDG_DATA_HOME/just-db/backups` or `~/.local/share/just-db/backups` |
| macOS | `~/Library/Application Support/just-db/backups` |

Export creates a dump in the backups folder, then opens a native save dialog for a copy. Cancelling the dialog keeps the backup. The Dumps tab lists these files with Download and Delete actions. Import asks for a local file through a native open dialog.

Saved profiles live in `{data dir}/profiles/`, encrypted. The desktop app does not use `./data`; CLI commands do unless you pass `-data` pointing at the same directory.

## Client tools

just-db shells out to `pg_dump` / `pg_restore` / `psql` and `mysqldump` / `mysql` (MariaDB names work too). Install the clients that match the **server major version**. Packages are **Recommends**, not Depends — you can install only Postgres tools, only MySQL tools, or both.

| Distro | PostgreSQL | MySQL / MariaDB |
|---|---|---|
| Debian / Ubuntu | `postgresql-client` (or `postgresql-client-16`, …) | `mariadb-client` or `default-mysql-client` |
| Fedora | `postgresql` | `mariadb` or `community-mysql` |
| Arch | `postgresql` | `mariadb` |
| macOS (Homebrew) | `brew install libpq` (then `brew link --force libpq` if needed) | `brew install mysql-client` or `mariadb` |

`wails doctor` / the in-app engine cards show whether each binary is on `PATH`.

## Linux packages

Build the desktop app and package only an RPM (requires the build dependencies
above and `nfpm`):

```bash
make rpm
```

The RPM is written to `dist/`. The current packaging configuration targets amd64.

After `make desktop`, if [nfpm](https://nfpm.goreleaser.com/) is installed:

```bash
make package-linux
```

That writes `.deb` and `.rpm` into `dist/`. Install examples:

```bash
sudo apt install ./dist/just-db-desktop_*.deb
sudo dnf install ./dist/just-db-desktop-*.rpm
```

Arch: build the binary with `make desktop` and install `desktop/build/bin/just-db-desktop` plus `desktop/packaging/just-db-desktop.desktop` yourself (or wrap it in a PKGBUILD). The `.desktop` file execs `just-db-desktop`.

## macOS

From the repository root, build a universal app for Intel and Apple Silicon and create a compressed DMG:

```bash
make dmg
```

The DMG is written to `dist/just-db-<version>.dmg`, using `info.productVersion` from `desktop/wails.json`. For version `0.1.0`, the output is `dist/just-db-0.1.0.dmg`. Rebuilding replaces the DMG for that version.

The app is at `desktop/build/bin/just-db.app`. To build only the app for the current machine's architecture, run `make desktop`. For only a universal app, run `make desktop WAILS_PLATFORM=darwin/universal`.

Client tools from Homebrew are discovered on common `libpq` / `mysql-client` prefixes.
