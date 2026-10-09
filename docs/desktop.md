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

Version 0.4.1 produces `dist/just-db-0.4.1.dmg`. The default version is read from `info.productVersion` in `desktop/wails.json`. The packager replaces an existing file for that version only after the new image succeeds; a failed packaging run retains the previous DMG.

Useful variations:

```bash
# Use a packager outside PATH.
make dmg CREATE_DMG=/path/to/create-dmg

# Repackage an existing app without compiling it.
bash desktop/packaging/macos/create-dmg.sh 0.4.1

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

### Build on macOS with Docker

With Docker running, build Linux x86-64 artifacts from the repository root:

```bash
make package-linux-docker
```

This installs the Linux build tools inside a Debian 12 container, runs frontend
tests and Go tests excluding database roundtrips, then builds the Wails app with
WebKitGTK 4.1. No host Go, Node.js, Wails, or nFPM installation is needed. The
first build downloads the toolchains and Linux libraries; later builds reuse
Docker's cache. The source, local frontend dependencies, and macOS app stay on
the host without modification.

Outputs are in `dist/linux-amd64/`:

- `just-db-desktop`, the Linux executable
- `.deb` and `.rpm` installers, using the version from `desktop/wails.json`
- `SHA256SUMS`, executable information, and a shared-library report

The build checks the executable architecture, linked libraries, and package
contents. The installers declare GTK 3, WebKitGTK 4.1, and glibc 2.36 or newer
as dependencies. Use Debian 12 or a compatible newer Linux distribution;
older distributions and WebKitGTK 4.0-only systems are not targets of this build.
Database clients remain recommended packages, so install the clients you use.

Docker exports only the generated files and does not push an image or load
`.env`. This checks compilation and packaging. Test installation, the native
window, dialogs, and database operations on a Linux desktop before release.

### Build directly on Linux

Install [nfpm](https://nfpm.goreleaser.com/), then run from the repository root:

```bash
make package-linux
```

This builds the desktop app and creates both `.deb` and `.rpm` files in `dist/`. There is no `make rpm` target. The current [`nfpm.yaml`](../desktop/packaging/nfpm.yaml) declares version 0.4.1 and `amd64`; changing the Wails target alone does not update that package metadata.

```bash
sudo apt install ./dist/just-db-desktop_*.deb
sudo dnf install ./dist/just-db-desktop-*.rpm
```

On Arch, install the built binary, [`just-db-desktop.desktop`](../desktop/packaging/just-db-desktop.desktop), and [SVG icon](../desktop/packaging/just-db.svg), or package them in a PKGBUILD. Database clients are recommendations in the Debian/RPM configuration; install the clients for the engines you use.

## Windows packages

With Docker running, build the Windows x86-64 executable and installer from
macOS or Linux:

```bash
make package-windows-docker
```

Like the Linux Docker target, this installs the build tools in a container,
runs frontend tests and Go tests excluding database roundtrips, and exports
only the artifacts. No host Go, Node.js, Wails, or NSIS installation is needed.
The Go tests run on Linux; Wails then cross-compiles the app for Windows.

Outputs are in `dist/windows-amd64/`:

- `just-db-desktop.exe`, the standalone application
- `just-db-0.4.1-windows-amd64-setup.exe`, the NSIS installer
- `SHA256SUMS` and `binary-info.txt`

The installer filename uses `info.productVersion` from `desktop/wails.json`.
The build verifies the app's Windows x86-64 GUI format and extracts the
installer to check that its embedded app matches the standalone executable.
The container does not modify the host's frontend dependencies or macOS app.

Windows needs the Microsoft WebView2 Runtime. The app and installer include
Microsoft's bootstrapper, which can install the runtime if missing; that
installation requires internet access. See the [Wails Windows guide](https://wails.io/docs/guides/windows/)
and [NSIS packaging guide](https://wails.io/docs/guides/windows-installer/).
PostgreSQL and MySQL client tools are not bundled. Install the clients you use
and add their `bin` directories to Windows `PATH` before opening the app.

The artifacts are unsigned. Compilation and payload checks do not verify
Windows installation, launch, window sizing, native dialogs, or database
operations; those need testing on Windows before release.

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
