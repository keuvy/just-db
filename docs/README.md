# Documentation

Reference for just-db 0.2.0, checked against the source on 2026-09-08. Commands below the repository root assume that working directory unless a guide says otherwise.

## Choose a guide

| Document | Read it when |
| --- | --- |
| [Project status and interfaces](status.md) | Checking implemented features, source layout, CLI commands, or HTTP routes |
| [Desktop builds and packaging](desktop.md) | Building the native app, creating a universal macOS DMG, or installing Linux packages |
| [EasyPanel and Docker](easypanel.md) | Deploying the web app, configuring access, or choosing container client tools |
| [UI guide](ui.md) | Using profiles, export, restore, the dump library, and development previews |
| [Troubleshooting](troubleshooting.md) | Investigating missing tools, PostgreSQL version mismatches, failed restores, or missing profiles |
| [Testing](testing.md) | Choosing automated checks and understanding which ones need disposable databases |
| [Encrypted profiles decision](adr/0001-encrypted-profiles.md) | Understanding the storage and encryption-key design |

[UI redesign plan](ui-redesign-plan.md) is a historical design record from 2026-09-04. The [screenshots](ui.md#screenshots) and dated verification notes record that earlier implementation; they are not fresh screenshots or a release certification for 0.2.0. Use the current guides and source when they differ from the plan.

## Release metadata

These files currently identify version 0.2.0:

| File | Used by |
| --- | --- |
| [`internal/appmeta/appmeta.go`](../internal/appmeta/appmeta.go) | CLI output and web/desktop health information |
| [`desktop/wails.json`](../desktop/wails.json) | macOS app metadata and the default DMG filename |
| [`frontend/package.json`](../frontend/package.json), [`frontend/package-lock.json`](../frontend/package-lock.json) | Frontend package metadata |
| [`frontend/src/preview.ts`](../frontend/src/preview.ts) | Development preview health information |
| [`desktop/packaging/nfpm.yaml`](../desktop/packaging/nfpm.yaml) | Linux package metadata |

Keep these values aligned when changing the version. `make dmg VERSION=...` changes the DMG filename; it does not update the embedded app version or Linux package metadata.

## Maintaining these guides

Check commands against the [Makefile](../Makefile), deployment claims against the [Dockerfile](../Dockerfile) and [Compose configuration](../docker-compose.yml), and behavior against the linked source modules. Record the date and scope of actual test runs. A source check, successful build, skipped database test, and native UI smoke test establish different things.

The 2026-09-08 documentation refresh checked source contracts, local links, and build recipes without building an app, packaging a DMG, deploying, or running database operations. This folder is reference material; the desktop packager stages the app bundle, and the installed app does not read these Markdown files.
