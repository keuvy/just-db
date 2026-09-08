# Desktop builds and packaging

The native app uses the same engines and frontend as the web app. Its window and macOS bundle are named `just-db`; the executable and Linux package are named `just-db-desktop`.

Source: [Makefile](../Makefile), [Wails configuration](../desktop/wails.json), [desktop bindings](../desktop/app.go). Source review: 2026-09-08, version 0.2.0.

## Build prerequisites

Use Go 1.27 and Node.js 24 to match the current Docker build toolchains. The Go module declares `go 1.25.0`; the frontend also depends on the engine requirements of the locked Vite/Vitest versions. Install the Wails CLI version used by the module:

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails doctor
```

On macOS, Xcode or its command-line developer tools provide the native compiler and SDK. `wails doctor` checks the Wails build environment. Database client availability is checked separately by `make tools` or Tools & settings in the app.

Linux builds require GTK 3 and WebKitGTK development headers. Typical packages are:

| Distro | Build packages |
| --- | --- |
| Debian / Ubuntu | `libgtk-3-dev` and `libwebkit2gtk-4.1-dev`, or `libwebkit2gtk-4.0-dev` on older systems |
| Fedora | `gtk3-devel`, `webkit2gtk4.1-devel`, `gcc` |
| Arch | `gtk3`, `webkit2gtk`, `base-devel` |

The Makefile detects `webkit2gtk-4.1` through `pkg-config` and supplies `-tags webkit2_41`. `WAILS_TAGS` can override the detected value. See [Wails installation](https://wails.io/docs/gettingstarted/installation/) for platform prerequisites.

## Build or develop

Run from the repository root:

```bash
make desktop
```

This installs/builds the frontend, copies the app icon when needed, and runs Wails. The default is the host architecture. Outputs are:

| Platform | Output |
| --- | --- |
| macOS | `desktop/build/bin/just-db.app` |
| macOS executable inside the bundle | `desktop/build/bin/just-db.app/Contents/MacOS/just-db-desktop` |
| Linux | `desktop/build/bin/just-db-desktop` |

Launch the macOS build with:

```bash
open desktop/build/bin/just-db.app
```

For Vite and Wails live reload:

```bash
make desktop-dev
```

## Universal macOS app and DMG

A universal app contains both Intel `amd64` and Apple Silicon `arm64` code. To build only that app:

```bash
make desktop WAILS_PLATFORM=darwin/universal
```

For the branded installer, install [create-dmg](https://github.com/create-dmg/create-dmg), then run:

```bash
brew install create-dmg
make dmg
```

`make dmg` checks macOS and packager availability before compiling. It then requests `darwin/universal` and stages only `just-db.app`, excluding unrelated build outputs. The DMG contains an Applications shortcut, a background, and drag-to-install instructions. Packaging uses Finder to arrange the window and needs a logged-in macOS desktop session; approve the terminal's Finder automation request if prompted by macOS.

Version 0.2.0 produces `dist/just-db-0.2.0.dmg`. The default version is read from `info.productVersion` in `desktop/wails.json`. The packager replaces an existing file for that version only after the new image succeeds; a failed packaging run retains the previous DMG.

Useful variations:

```bash
# Use a packager outside PATH.
make dmg CREATE_DMG=/path/to/create-dmg

# Repackage an existing app without compiling it.
bash desktop/packaging/macos/create-dmg.sh 0.2.0

# Inspect the architectures actually present in the built app.
lipo -archs desktop/build/bin/just-db.app/Contents/MacOS/just-db-desktop
```

Repackaging does not change an app's architecture or embedded version. A `VERSION` override changes the output filename only. See [release metadata](README.md#release-metadata) for the files to update together.

The layout and artwork are in [`desktop/packaging/macos/`](../desktop/packaging/macos/). Regenerate the background after editing its Swift source with:

```bash
swift desktop/packaging/macos/background.swift desktop/packaging/macos/background.png
```

To install an updated app, quit the running copy, open the DMG, and drag `just-db.app` into Applications, replacing the old bundle. Profiles and dumps live separately in the data directory. Check the runtime version in Tools & settings after reopening.

The repository does not configure Developer ID signing or Apple notarization. Earlier local Wails builds were ad-hoc signed. Building a universal binary alone does not establish compatibility with every macOS release; the earlier build record and its deployment-target warning are documented in the [UI verification notes](ui.md#verification-recorded-on-2026-09-04).

## Linux packages

Install [nfpm](https://nfpm.goreleaser.com/), then run from the repository root:

```bash
make package-linux
```

This builds the desktop app and creates both `.deb` and `.rpm` files in `dist/`. There is no `make rpm` target. The current [`nfpm.yaml`](../desktop/packaging/nfpm.yaml) declares version 0.2.0 and `amd64`; changing the Wails target alone does not update that package metadata.

```bash
sudo apt install ./dist/just-db-desktop_*.deb
sudo dnf install ./dist/just-db-desktop-*.rpm
```

On Arch, install the built binary, [`just-db-desktop.desktop`](../desktop/packaging/just-db-desktop.desktop), and [SVG icon](../desktop/packaging/just-db.svg), or package them in a PKGBUILD. Database clients are recommendations in the Debian/RPM configuration; install the clients for the engines you use.

## Data and client tools

| OS | Data directory |
| --- | --- |
| macOS | `~/Library/Application Support/just-db` |
| Linux | `$XDG_DATA_HOME/just-db`, or `~/.local/share/just-db` |

Dumps are in `backups/`; encrypted profiles are in `profiles/`. The CLI defaults to `./data` or `JUSTDB_DATA`. To inspect the desktop profiles from the CLI on macOS:

```bash
go run ./cmd/just-db profile list -data "$HOME/Library/Application Support/just-db"
```

The app bundle and DMG do not include PostgreSQL or MySQL client executables. Install them on every computer that runs the desktop app. Typical macOS packages are `libpq`, `mysql-client`, or `mariadb`; choose versioned PostgreSQL clients when needed for a particular server.

The app searches `PATH`, then common directories including Homebrew `libpq`/`mysql-client` prefixes and `/usr/local/mysql/bin` for the MySQL macOS installer. It does not automatically select a PostgreSQL version per connection. See [troubleshooting](troubleshooting.md) for exact discovery behavior, Finder launch differences, and version mismatch examples.
