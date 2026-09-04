# just-db UI redesign plan

Proposed direction, 2026-09-04. This is a plan for replacing the shared web and Wails interface. Implementation has not started.

Rebuild just-db around a persistent profile sidebar and a focused database workspace. Give profile management, export, restore, dumps, and tool diagnostics their own clear places. Use the window efficiently and keep the selected target visible whenever an operation can affect it.

The proposed default is a compact desktop workspace. Keep the current English interface and the graphite/lime identity, while replacing the page layout, typography, controls, and interaction model. These are design recommendations, not a recorded user selection.

## 1. What the current app needs

The audit used the current working files, including existing staged and unstaged changes. The existing graph helped locate the shared export/import paths, but its frontend references predate current changes; the source files below are the authority. No running browser or native app was available for visual inspection. Layout findings come from markup and CSS, not a rendered usability test.

| Current implementation | Consequence | Proposed change |
| --- | --- | --- |
| `#app` has an 880px maximum width; profiles sit above the operation form. | Wider windows add margins while profiles and operations compete for vertical space. | Full-window shell with independent sidebar and content scrolling. |
| Four tabs are Saved connections, Save new connection, Dumps, and About. | Creating a profile occupies a permanent destination; changing views hides useful context. | Persistent profile list, an Add profile action, a Dumps destination, and Tools & settings at the bottom. |
| All text is monospace; field labels use uppercase styling. | Navigation, labels, filenames, and diagnostic details have similar visual weight. | System sans-serif for interface text; monospace for technical values. |
| Test, Export, and Import share the same filled button style. | The workspace lacks a clear primary action. | One primary action per operation mode; test and edit stay secondary. |
| Existing profiles can be selected or deleted; replacement happens through the new-profile form. | Editing an existing profile is indirect. | A shared New/Edit profile panel with explicit save and replacement behavior. |
| A database input says optional even in the operation workspace. | Saving a profile and running an operation have different requirements that the UI does not explain. | Optional default database in the profile editor; required target database in the workspace. |
| Import uses a file-selection dialog followed by a generic browser confirmation. | The final decision does not name the source and target together. | One restore review containing source, engine, profile, host, database, and drop behavior. |
| One status line receives profile, database-list, test, and operation results. | A subsequent refresh can overwrite an earlier result. | Separate field feedback, connection-test status, and operation results. |
| Dumps contain name, path, size, and modification time. | Engine, source profile, and original database cannot be reliably displayed or filtered. | A searchable file library using supported fields; richer metadata is a separate backend extension. |
| Tool diagnostics live only in About. | Missing executables become visible away from the operation that needs them. | A contextual missing-tool message with a link to detailed diagnostics. |

Evidence: [current screens and handlers](../frontend/src/main.ts), [current styles](../frontend/src/style.css), [shared API and types](../frontend/src/api.ts), [HTTP import handlers](../server/api.go), [desktop window](../desktop/main.go), and [domain vocabulary](../CONTEXT.md). The final source check includes the newly added browser multipart import flow.

## 2. App layout and navigation

Use a 240px sidebar, a 56px workspace header, a flexible main area, and a collapsible activity drawer along the bottom. Treat these dimensions as starting values to validate at the existing 960 x 720 desktop size.

```text
+----------------------+----------------------------------------------------+
| just-db              | local-postgres             Test    Edit    More    |
|                      | PostgreSQL / user@localhost:5432                    |
| Search profiles      +----------------------------------------------------+
| Profiles          +  | Database  [ app_dev                         v ]    |
| > local-postgres     |                         Refresh database list      |
|   staging-mysql      +----------------------------------------------------+
|   reporting          | Export   Restore                                   |
|                      |                                                    |
|                      | Export dump                                        |
|                      | Format  [ Custom .dump                      v ]    |
|                      | Saved to this app's dump library                   |
|                      |                                      [Export dump] |
|                      |                                                    |
| Dumps                | Result, when available                             |
|                      | filename.dump   24.6 MB     [Save a copy]           |
| Tools & settings     +----------------------------------------------------+
|                      | Activity this session                    Expand    |
+----------------------+----------------------------------------------------+
```

This wireframe uses fictional profile and database names. It describes hierarchy, not final pixel styling.

Selecting a profile opens its workspace. Preserve its chosen database and export format for the current session. Going to Dumps or Tools & settings must not reset an in-progress form or hide an active operation's status. The profile selection stays visible in the sidebar while global destinations have their own active navigation indicator.

The main destinations are:

| Destination | Contents |
| --- | --- |
| Profile workspace | Profile identity, database selector, Export and Restore modes, current result. |
| Dumps | All dumps in the current runtime's storage, search, sorting, file actions. |
| Tools & settings | Engine tools, executable paths and versions, storage location, appearance, app information. |
| New/Edit profile panel | Contextual form opened from Add or Edit, with an explicit Cancel action. |
| Activity drawer | Running and completed operations from this app session. |

Retain the old `#saved`, `#new`, `#dumps`, and `#about` entry points as aliases during migration. Store no passwords or connection payloads in hashes. Do not introduce a separate home dashboard; first launch and no-selection states live inside this shell.

## 3. Visual system

Use flat surfaces, subtle separators, and compact rows. Remove the radial page background, stacked outer cards, uppercase field labels, and large repeated section padding. Keep whitespace around decisions and related field groups.

| Element | Proposed specification |
| --- | --- |
| Typography | Local system sans-serif for UI, 14px base with readable line height. Local monospace stack for hosts, paths, database names, and error details. No network font dependency. |
| Hierarchy | Page titles around 20px, section titles 15-16px, labels 13px. Use weight and spacing before adding color. |
| Dark palette | Start with canvas `#101114`, sidebar `#15171B`, surface `#1B1E24`, border `#30343C`, text `#F0F1F3`, secondary text `#A7ADB7`, accent `#C8F464`. |
| Light palette | Warm neutral canvas and white surfaces. Use a darker green accent for text and focus indicators instead of reusing pale lime on white. |
| Appearance | System, Dark, and Light. Follow the system by default and persist only the user's appearance preference. |
| Spacing | 4px base scale; 8px inside compact groups, 16px between fields, 24px between sections. |
| Controls | About 36px high on desktop; increase to 44px in touch layouts. Consistent 6-8px corners. |
| Lists | Profile rows around 48-56px to accommodate name and endpoint. Dump rows around 44px, with a compact column header. |
| Buttons | Filled accent for the primary action, bordered or plain for secondary actions, destructive treatment for the final destructive action. |
| Selection | Persistent filled row plus a leading marker, separate hover treatment, and a visible keyboard focus ring. |
| Icons | One consistent small SVG set; retain labels for primary navigation and actions. Engine names remain readable text. |
| Motion | Short state transitions; respect reduced-motion preferences. No animation that delays an action. |

The colors are starting tokens, not an accessibility certification. Validate actual foreground/background combinations in both themes. Normal text must meet 4.5:1 contrast and qualifying large text 3:1, following [W3C contrast guidance](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html).

## 4. Screen and workflow specifications

### First launch and loading

Show the shell immediately with a loading state inside each region. An unavailable dump list should not replace the whole app with an error. Identify failed regions and give them a Retry action.

With no profiles, the main pane explains the first step and offers Add profile. Preserve the existing environment defaults as the initial form values. Existing dumps and tool diagnostics remain accessible even with no profiles.

With profiles but no selection, offer a short instruction and the visible list. Restore an explicitly remembered profile if it still exists; do not silently choose another target after deletion or a load failure.

### Profiles and connection editing

Search profiles by name, host, engine, and optional default database. Show the profile name first, engine and endpoint second. Do not label a profile Connected merely because it loaded or its client tools were found. A successful test produces a timestamped Last tested result.

Use the same side panel for New profile and Edit profile. On narrow screens it becomes a full-width sheet. Group name and engine first, endpoint and account fields next, then SSL mode and optional default database. Keep engine-specific port defaults and current supported SSL values. Label the password field clearly and provide a reveal control.

The footer has Test connection, Cancel, and Save profile. Testing is optional before saving. Field errors appear beside their controls and preserve entered values. Show a separate warning when the connection succeeds but database discovery fails; manual database entry remains available.

Profile name is the storage identity. Lock it when editing an existing profile. Save changes overwrites that same profile through the existing API. A new profile whose name already exists must offer an explicit Replace profile decision. Duplicate, if included, opens a new draft with a unique suggested name; it does not rename or delete the source. Atomic rename remains a separate capability.

Deleting a profile belongs in its More menu. The confirmation names the profile and explains that it removes the stored profile, while database contents and dumps are unaffected. After deletion, show the no-selection state. A dirty editor needs an explicit discard decision when the user leaves it.

Keep passwords in the existing encrypted profile flow and only in memory while needed by the UI. Appearance preferences, recent selections, and activity entries must never serialize connection passwords.

### Selected database workspace

Keep profile name, engine, `user@host:port`, and database visible above the operation content. A profile's default database and the workspace's current database are separate state values. Selecting a different database for export or restore must not silently save it back to the profile.

The database control is searchable and allows a typed name. Show loading, no results, lookup failure, and a Refresh action explicitly. Reset results when the profile changes, and ignore responses belonging to a previously selected profile. For a failed or restricted listing, preserve manual entry.

Test connection reports connectivity for the profile's configured connection. Do not present that result as proof that the selected database supports a particular export or restore. Tool availability is also a separate status. Disable only actions that need a missing executable and explain the reason next to them.

### Export

Export is the default operation mode. It shows the source database, format, and where the dump will be kept. PostgreSQL offers Custom and SQL. MySQL/MariaDB shows SQL as the supported format without an unnecessary one-option selector.

Use Export dump as the primary action. Capture the engine, connection, selected database, and format when it starts. Show Exporting with elapsed time while waiting. The current API provides no percentage, throughput, or ETA, so use an indeterminate indicator.

Keep the existing create-and-copy behavior. After creation, attempt the browser download or desktop save dialog, then show a durable result containing the actual filename and size. These are separate outcomes:

- Dump created and browser download started.
- Dump created and desktop copy saved.
- Dump created; desktop save dialog cancelled.
- Dump created; copy/download failed, with a retry action using the existing dump.
- Dump creation failed, with the operation error.

Use Save a copy on desktop and Download on web. A cancelled save dialog must not label the dump creation as failed. A browser download starting must not claim the file has finished saving.

### Restore

Restore uses Source, Target, and Review steps inside the main workspace. The file, target, and options persist when moving back a step. Opening Restore from a dump row prefills the source only; it never starts the operation.

Source offers Choose file and From dump library in both runtimes. Desktop opens its native file picker. Web uses a labeled file input with an optional drag-and-drop target. Preserve the current `.sql`, `.dump`, `.backup`, and `.pgdump` file choices. File selection only prepares the source; upload and restore begin after the final review.

The current web adapter accepts a browser `File` and submits multipart data to `/api/import`; stored dump names still use its JSON path. Reuse these contracts. Uploaded files are temporary import inputs and are removed by the handler, so do not claim they were saved in the dump library. While the combined upload/import request is pending, show Uploading and restoring without a fabricated stage boundary or percentage.

Target requires an explicit profile and database. The review shows the exact target even when Restore started from the global Dumps destination. Changing profile, database, source, or drop options invalidates the previous review and requires reviewing again.

```text
Restore dump

Source       postgres-app-20260904.dump
File type    PostgreSQL custom dump
Target       staging-postgres
Engine       PostgreSQL
Host         db.internal:5432
Database     app_staging

[ ] Drop existing objects before restore
    Engine- and format-specific explanation appears here.

This operation writes to app_staging on db.internal:5432.

[Back]                              [Restore into app_staging]
```

The file type must be described as inferred when known only from its extension. A `.sql` extension does not prove its source engine. For SQL and other unverified files, show Source engine not verified and require the user to acknowledge that the dump belongs to the target engine. Known incompatible formats should be rejected before submission. Strong engine validation requires backend metadata or content inspection; frontend copy cannot provide that guarantee.

Keep drop-existing off by default. Its label and review explanation must match the implementation:

| Target and format | Current drop-existing effect |
| --- | --- |
| PostgreSQL custom | Uses `pg_restore --clean --if-exists` for objects restored from the archive. |
| PostgreSQL SQL | Drops the `public` schema with `CASCADE`, recreates it, and applies the current grants before running the SQL. |
| MySQL/MariaDB SQL | Drops the target database's base tables before running the SQL. |

Explain that leaving this option off does not prevent destructive statements already contained in SQL. Avoid a generic claim that the existing database is protected.

Use one final in-app confirmation with the reviewed target, replacing the current extra browser confirmation. Require typing the database name when drop-existing is enabled. Send `confirm: true` only after this review action. After submission, replace the form action with running status and retain the target summary.

### Dumps

Use a full-width table with Filename, Size, Modified, and Actions. Format sizes for humans, retain exact bytes in file details, and allow the full filename/path to be inspected or copied. Search filename; sort by name, size, or modified time. Default to newest modified first.

Actions are Restore, Download or Save a copy, and Delete in a menu. Desktop also offers Open dumps folder. Web labels the storage location as being on the server. Do not imply that desktop and web automatically share files.

Do not invent source profile, source database, engine, verification status, or creation time from the current file records. Modification time is the timestamp the API actually returns. Engine/profile filters need a future metadata change.

Delete confirmation identifies the exact filename and states that deletion is permanent. Keep the row until the API succeeds. Report a missing or already deleted file with Refresh; do not offer Undo without actual recovery support.

### Activity and feedback

Keep a collapsible session activity drawer available from every destination. Each record contains operation type, profile/database display identity, start time, elapsed duration, and result. Preserve export results while refreshing the dump list. Connection and field messages remain near their controls.

For the first release, serialize export/restore operations initiated by this UI. Users may inspect other pages during an operation, but another export/restore stays unavailable until it finishes. Each result remains attached to its captured target even if the selected profile changes. Prevent deletion of a source dump while this UI is restoring it.

Activity is limited to this frontend session. Keep a bounded in-memory list, for example the last 50 completed operations. Refreshing or closing the app does not provide reconnection, durable history, or an operation cancellation guarantee. Do not show Cancel job or a fake percentage. Those require backend job support. A network failure during restore must not claim that database changes were rolled back.

### Tools & settings

Show PostgreSQL and MySQL/MariaDB in a compact diagnostic table with dump, restore, and client executable availability, path, and version. A Refresh tools action reruns discovery. Put technical error details behind a disclosure, with a copy control that excludes passwords.

Group storage location and runtime information separately from appearance. Keep app version and mode here instead of in the main task header. Show contextual links into this page when an operation lacks its required tool. Any installation instructions should reflect the host where the tools actually run, including a Docker server rather than the browser's computer.

## 5. Keyboard, window sizes, and accessibility

The existing desktop window is 960 x 720 and uses a hidden inset macOS title bar. Validate the new shell at that size before proposing a larger default. Reserve space for macOS window controls and an actual draggable title area; interactive controls must not become drag regions. Wails exposes window sizing and platform title-bar configuration in its [options reference](https://wails.io/docs/reference/options/).

At approximately 900px and wider, use the persistent sidebar. Below that, show a compact profile switcher and navigation drawer. Below approximately 640px, stack fields and use full-width sheets. Keep the target summary and primary action reachable without horizontal page scrolling. Long table content may have its own scroll area, with filename and action access preserved.

Verify 1440 x 900, 960 x 720, a narrow 768px viewport, and a 390px web viewport. Also inspect zoomed text, long profile names, long filenames, empty lists, and large lists. Keyboard navigation must reach every operation, menu, and dialog without requiring a pointer.

Use semantic navigation, labeled fields, real buttons, and live announcements for operation status. Distinguish focus, selection, validation errors, and disabled states. Disabled prerequisites need explanatory text rather than a waiting cursor. Profile search and database search must have clear empty states.

For modal editors and confirmations, contain keyboard focus, choose a useful initial focus target, support Escape before submission, and restore focus to the triggering control, following the [W3C modal dialog pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/). Dismissing a status view must never be presented as cancelling a running database operation.

## 6. Implementation structure

Keep the current Vite and vanilla TypeScript stack for this redesign. Split the large `main.ts` by responsibility before expanding the interaction model. The current HTTP/Wails adapter remains the shared boundary. A framework migration would be a separate decision with its own cost.

Proposed organization, with exact file count adjusted during implementation:

```text
frontend/src/
  main.ts                         bootstrap
  api.ts                          existing HTTP/Wails adapter
  app/
    state.ts                      selection, drafts, request state
    navigation.ts                 destinations and legacy hash aliases
    operations.ts                 captured requests and session activity
  components/
    shell.ts                      sidebar, header, activity drawer
    profile-editor.ts             new/edit form
    database-picker.ts            search, manual entry, lookup states
    feedback.ts                    notices, errors, confirmation dialogs
  views/
    workspace.ts                  shared target header and operation mode
    export.ts                     export settings and result
    restore.ts                    source, target, review, result
    dumps.ts                      file library
    settings.ts                   tools, storage, appearance, about
  styles/
    tokens.css                    colors, type, spacing, theme
    controls.css                  fields, buttons, focus, dialogs
    layout.css                    shell, responsive rules, titlebar
    views.css                     screen-specific presentation
```

Use explicit idle/loading/success/error states rather than deriving state from text or DOM values. Keep profile loading and database discovery request generations so old responses cannot update a newer selection. Operation functions capture their arguments once and do not read a mutable selected profile after an `await`. Keep runtime-specific file handles and paths inside the API/operation layer.

Use `textContent` or the existing escaping discipline for all profile names, filenames, paths, and errors. Partial updates should preserve input focus, scroll position, and the user's draft. Introduce only the shared controls that these screens need.

The initial redesign can use existing APIs for profile editing, searches over loaded lists, export, stored-file restore, browser file uploads, file actions, tools, and session activity. No profile-storage migration is necessary.

| Separate backend extension | Why it is separate |
| --- | --- |
| Dump metadata with original engine, database, profile, and creation time | The current list response is file metadata only; filenames are not reliable provenance. |
| Persistent jobs, progress events, cancellation, and history | Export/import currently resolve or reject a request; there is no job lifecycle API. |
| Atomic profile rename | Profile name is identity and PUT replaces a record under that name. |

SQL editing, table browsing, scheduling, cloud storage, and additional engines remain outside this UI redesign. They require new product workflows, not merely different screens.

## 7. Delivery sequence

Each phase should leave a usable app and have its own reviewable diff. Preserve the current staged and unstaged work when implementation begins. Generated frontend assets should be updated only through the repository build process after the corresponding source changes are ready.

| Phase | Work | Completion gate |
| --- | --- | --- |
| 1. Design the core screens | Build a separate visual prototype of a populated workspace, profile editor, dump library, and restore review. Include first-launch, failure, and narrow-window variants. Use fixture data. | Compare the same export and restore tasks at 960 x 720 and 1440 x 900; settle spacing, hierarchy, and terminology before production wiring. |
| 2. Shared shell and state | Extract bootstrap, navigation, state, controls, and theme tokens. Add the sidebar, header, responsive layout, and activity container. | Existing profile, export, import, dump, and diagnostic actions still work; old hashes resolve; focus and drafts survive view changes. |
| 3. Profiles and database selection | Add profile search, New/Edit panel, replacement handling, connection-test feedback, and database picker. | Create, edit, replace, delete, empty-password entry, optional default database, lookup failure, and rapid profile switching behave correctly. |
| 4. Export and activity | Wire the focused export view, captured operation arguments, running state, copy/download outcomes, and session records. | Both engines export; desktop cancellation preserves the dump; download failure can retry the existing file; navigation cannot reassign the operation's result. |
| 5. Restore | Implement file selection/upload and stored-library sources, Target, Review, exact drop explanations, final confirmation, and result handling. | Reviewed target equals submitted target; each supported engine/format/drop combination is checked; both JSON and multipart import work; double submission and stale review are prevented. |
| 6. Dumps and settings | Complete library search/sort/actions, desktop folder action, diagnostics, appearance, and all empty/error states. | Existing file contracts work in both runtimes; unsupported provenance, durable history, and job-cancellation controls are absent. |
| 7. Verify and finish | Review both themes, keyboard flows, narrow layouts, macOS titlebar, Linux webview, and web behavior. Update screenshots and user-facing docs. | Focused checks pass and the known runtime limitations are documented. |

Complete the UI using the existing contracts before considering the backend extensions. This keeps the redesign deliverable even if richer dump metadata or persistent jobs are deferred.

## 8. Acceptance and verification

The redesign is complete when a user can add a profile, choose a database, export a dump, save or download it, locate it in Dumps, and restore it to an explicitly reviewed target without losing context. Every supported workflow must work in the shared web UI and the native wrapper where applicable.

Required verification during implementation:

- Run the frontend TypeScript/Vite build after meaningful code changes. Add focused interaction tests for stale profile responses, per-profile database state, exact reviewed restore payload, double submission, and export/copy outcome separation.
- Run relevant existing Go tests if an API contract or native binding changes. Exercise real export/restore only against designated disposable PostgreSQL and MySQL/MariaDB fixtures, including the supported drop behaviors.
- Smoke-test browser download streaming, local-file uploads, stored-file restore, authentication failures, missing tools, empty lists, and server errors. Check large-file uploads and failed upload/import handling against the actual deployment limits. Preserve the current streaming download approach for large dumps.
- Smoke-test native open/save dialogs, cancelling a save, opening the dumps folder, titlebar interaction, and both light and dark native window backgrounds. Browser checks do not validate native dialogs or Linux/macOS webview behavior.
- Inspect keyboard focus, status announcements, modal dismissal, 200% zoom, long content, and both themes at the specified viewport sizes. Check 100 profiles and 1,000 file entries with fixture data before adding more complex list rendering.
- Confirm that operation status survives in-app navigation, that successful export is distinct from saving a copy, and that errors never imply rollback or cancellation the backend cannot prove.
- Run `git diff --check` and inspect the source/build diff. Do not stage, unstage, or overwrite unrelated work as part of the redesign.

This planning pass executed no application builds, tests, database operations, or UI implementation changes. The next concrete deliverable is the fixture-based visual prototype in phase 1.
