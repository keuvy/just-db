# UI guide

Behavior checked against the 0.2.0 source on 2026-09-08. Screenshots and the dated test record below are from the earlier 2026-09-04 UI work. See the [documentation index](README.md) for builds, deployment, and troubleshooting.

The web app and Wails desktop app share the same workspace. The sidebar holds connection profiles; the main area contains the selected database and operation. Dumps and Tools & settings are always available from navigation. Narrow windows use a navigation drawer.

![Database workspace](ui-screenshots/workspace-light.png)

## Profiles and databases

Use **Add profile** to open the editor. Enter an engine, endpoint, and account; the default database is optional. Testing is optional before saving. **Edit** updates the selected profile under its existing name. Creating another profile with the same name asks explicitly before replacing it. Closing an edited draft asks before discarding changes.

Select a profile, then select or type the database for this operation. This choice stays in the current session and does not change the profile's saved default. If database discovery fails, manual entry remains available. A connection test reports its own result separately from database discovery and tool availability. It does not establish that `pg_dump` is compatible with the server or that a restore will succeed.

Database fields use searchable dropdowns and also accept a manually entered name. Engine, SSL mode, restore profile, stored dump, and dump-sort choices use the shared dropdown control. Use the arrow keys to move through options, Enter to choose, and Escape to close. Typing in a database field filters its suggestions; typing in a fixed-choice dropdown jumps to matching labels. The implementation is in [`components/select.ts`](../frontend/src/components/select.ts). The earlier screenshots below predate these controls.

![Profile editor](ui-screenshots/profile-editor.png)

## Export and restore

**Export** offers PostgreSQL custom or SQL format; MySQL/MariaDB uses SQL. The app creates the dump in its library, then starts a browser download or opens the desktop save dialog. Cancelling that dialog leaves the original dump intact. A failed copy can be retried from the result without exporting again.

**Restore** has three steps:

1. Choose a local file or a file from the dump library.
2. Choose the target profile and database, and decide whether to drop existing objects.
3. Review the exact file, engine, host, account, database, and drop behavior before submitting.

Accepted extensions are `.sql`, `.dump`, `.backup`, and `.pgdump`. File format is inferred from the extension. SQL does not identify its source engine, so the review requires an acknowledgment that the dump belongs to the target engine. Enabling drop-existing also requires typing the exact target database name. Changing the target or options invalidates the review. Browser files are uploaded only on submission and are temporary restore inputs. Desktop file selection opens a native dialog and passes the selected local path to the engine. Uploaded browser files are not added to the dump library.

With drop-existing enabled, the review explains the selected behavior: PostgreSQL SQL restore resets the `public` schema, custom restore cleans objects in the archive, and MySQL restore drops base tables. SQL statements in the file can remove data even when drop-existing is unchecked. A failed restore can leave partial changes; its result calls this out.

![Restore review](ui-screenshots/restore-review.png)

Only one export or restore can run from this UI session at a time. This is not a global lock across other windows or HTTP clients. You can navigate while it runs. The result stays attached to the target captured at submission, with elapsed time and the actual outcome. Activity keeps up to 50 completed operations in memory for the current session. It does not provide persistent jobs, percentage progress, reconnection, or cancellation.

## Files, diagnostics, and appearance

**Dumps** supports filename search, name/size/modified sorting, file details, path copying, restore, download or save-copy, and confirmed deletion. Desktop also provides **Open folder**. File records do not contain engine or source-database metadata, so the library does not claim that provenance.

**Tools & settings** shows each engine's dump, restore, and client binaries, storage paths, runtime, and version. Use **Refresh tools** after installing client programs. Missing tools also appear beside affected operations. Paths refer to this computer in desktop mode and to the app server in web mode. The app displays the first tools it finds; there is no per-connection client-version selector or automatic server-version match. See [troubleshooting](troubleshooting.md) for PostgreSQL version mismatches and macOS search paths.

Light, dark, and system appearance are available; only the appearance preference is persisted in browser storage. Profiles and dumps are persisted by the backend, while operation history and selected databases are session state.

## Screenshots

These images record the 2026-09-04 UI and have not been regenerated for 0.2.0.

| View | Screenshot |
| --- | --- |
| Dark workspace at 960 × 720 | [Open](ui-screenshots/workspace-dark.png) |
| Dump library | [Open](ui-screenshots/dumps.png) |
| Tools & settings | [Open](ui-screenshots/settings.png) |
| Phone workspace | [Open](ui-screenshots/workspace-mobile.png) |
| First launch | [Open](ui-screenshots/first-launch.png) |

## Development preview

Run from `frontend/`:

```sh
npm install
npm run dev -- --host 127.0.0.1 --port 5174
```

Open the dev server with one of these query strings:

| Query | Fixture |
| --- | --- |
| `?preview` | Populated workspace |
| `?preview=empty` | First launch |
| `?preview=failure` | Failed discovery, connection test, download, and missing MySQL tools |
| `?preview=many` | 100 profiles and 1,000 dump files |

The preview banner identifies sample data. Preview operations use an in-memory client and make no database requests. Reloading resets the samples. Vite removes the preview client from production builds.

Production uses the shared [HTTP/Wails adapter](../frontend/src/api.ts). Without `?preview`, the Vite server proxies `/api` and `/health` to `http://127.0.0.1:8080`; start the Go server separately for that workflow. `src/app/` owns selection, request generations, operations, navigation, and appearance; `src/components/` contains shared controls and dialogs; `src/views/` renders each destination; `src/styles/` separates tokens, controls, layout, and views.

## Verification recorded on 2026-09-04

The following results were recorded during the earlier UI implementation. They were not rerun for this documentation refresh; use [Testing](testing.md) for current commands and their scope.

- TypeScript and the Vite production build passed in that run.
- The 20 frontend tests present in that run passed. They cover stale responses, per-profile database choices, frozen restore payloads, SQL/drop confirmation, duplicate submissions, copy cancellation/retry, redaction, profile editing, escaping, and JSON/multipart request contracts.
- `go test ./... -count=1 -timeout 4m` passed. PostgreSQL and MySQL roundtrip tests skipped because the local Docker daemon was unavailable; this run does not prove live export/restore behavior.
- Browser inspection covered both themes, populated and empty states, profile editing and discard, restore review, failure states, and layouts at 1440, 960, 768, and 390 pixels wide. The large-list fixture was inspected and filtered. At phone width the dump table scrolled inside its own container.
- The production frontend was served by the real Go HTTP server with an isolated temporary data directory. Creating, reopening, reloading, and deleting a profile worked; actual tool discovery populated settings. The deliberately unreachable test endpoint produced the expected database-list error without blocking profile storage.
- The macOS production application compiled, packaged, and self-signed with Wails. Direct tagged Go compilation also passed with the explicit native-dialog framework link. Wails packaging needed system access for its macOS-version `sysctl` query. The linker warns that the Go object targets macOS 13 while Wails requests 10.13; compatibility with older macOS releases is not established by this build.

Native open/save dialogs, Finder folder opening, titlebar dragging, native theme changes, and Linux webview behavior still need runtime smoke tests. The browser fixture does not validate those behaviors. Large uploads, real database roundtrips, deployment authentication, and screen-reader/200% browser-zoom behavior also remain unverified in this pass.
