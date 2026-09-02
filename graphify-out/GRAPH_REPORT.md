# Graph Report - .  (2026-09-02)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 372 nodes · 770 edges · 22 communities (20 shown, 2 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 114 edges (avg confidence: 0.78)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d49e80f1`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Name
- main.ts
- Server
- mysql.go
- Run
- App
- .Import
- just-db/main.go
- compilerOptions
- wails.json
- New
- package.json
- TestRoundTripSQL
- Detect
- Registry
- genicon.go
- .withAuth
- TestDistContainsIndex
- just-db

## God Nodes (most connected - your core abstractions)
1. `Name` - 22 edges
2. `App` - 18 edges
3. `render()` - 18 edges
4. `Run()` - 16 edges
5. `Server` - 16 edges
6. `New()` - 15 edges
7. `ExportToFile()` - 14 edges
8. `ImportFromFile()` - 13 edges
9. `compilerOptions` - 12 edges
10. `Engine` - 11 edges

## Surprising Connections (you probably didn't know these)
- `cmdServe()` --calls--> `DefaultsFromEnv()`  [INFERRED]
  cmd/just-db/main.go → server/defaults.go
- `cmdExport()` --calls--> `ExportToFile()`  [INFERRED]
  cmd/just-db/main.go → internal/job/job.go
- `cmdImport()` --calls--> `ImportFromFile()`  [INFERRED]
  cmd/just-db/main.go → internal/job/job.go
- `App` --references--> `Registry`  [EXTRACTED]
  desktop/app.go → internal/registry/registry.go
- `Options` --references--> `Name`  [EXTRACTED]
  server/server.go → internal/engine/engine.go

## Import Cycles
- None detected.

## Communities (22 total, 2 thin omitted)

### Community 0 - "Name"
Cohesion: 0.07
Nodes (35): Connection, DumpFile, Engine, ExportOptions, ExportResult, ImportOptions, Info, Name (+27 more)

### Community 1 - "main.ts"
Cohesion: 0.13
Nodes (42): Connection, Defaults, dumpDownloadURL(), DumpFile, EngineInfo, exportDump(), ExportResult, getDefaults() (+34 more)

### Community 2 - "Server"
Cohesion: 0.14
Nodes (16): BackupsDir(), ServeMux, Connection, Request, ResponseWriter, Server, statusFor(), Request (+8 more)

### Community 3 - "mysql.go"
Cohesion: 0.17
Nodes (19): clientArgs(), cnfQuote(), ContainsPassword(), dumpArgs(), Connection, Context, Reader, Tool (+11 more)

### Community 4 - "Run"
Cohesion: 0.13
Nodes (21): New(), adminCfg(), containerRuntime(), envOr(), execSQL(), freePort(), Connection, Context (+13 more)

### Community 5 - "App"
Cohesion: 0.14
Nodes (12): App, Connection, Context, DumpFile, ExportResult, NewApp(), Health, main() (+4 more)

### Community 6 - ".Import"
Cohesion: 0.20
Nodes (16): clientArgs(), clientEnv(), ContainsPassword(), dumpArgs(), fileForRestore(), Connection, Context, Reader (+8 more)

### Community 7 - "just-db/main.go"
Cohesion: 0.24
Nodes (15): atoiDefault(), cmdExport(), cmdImport(), cmdServe(), cmdTest(), cmdTools(), envOr(), Connection (+7 more)

### Community 8 - "compilerOptions"
Cohesion: 0.11
Nodes (17): compilerOptions, isolatedModules, lib, module, moduleResolution, noEmit, noUnusedLocals, noUnusedParameters (+9 more)

### Community 9 - "wails.json"
Cohesion: 0.12
Nodes (16): author, name, frontend:build, frontend:dev:serverUrl, frontend:dev:watcher, frontend:dir, frontend:install, info (+8 more)

### Community 10 - "New"
Cohesion: 0.24
Nodes (15): T, TestAPIRequiresAuth(), TestDefaults(), TestDumpDownload(), TestHealthUnauthenticatedWhenAuthEnabled(), TestWrongPassword(), Options, Connection (+7 more)

### Community 11 - "package.json"
Cohesion: 0.14
Nodes (13): devDependencies, typescript, vite, name, private, scripts, build, dev (+5 more)

### Community 12 - "TestRoundTripSQL"
Cohesion: 0.38
Nodes (12): New(), adminCfg(), containerRuntime(), envOr(), execSQL(), freePort(), Connection, Context (+4 more)

### Community 13 - "Detect"
Cohesion: 0.29
Nodes (10): Detect(), extraDirs(), Context, Tool, LookPath(), T, TestDetectKnownBinary(), TestDetectMissing() (+2 more)

### Community 14 - "Registry"
Cohesion: 0.31
Nodes (6): Engine, New(), T, TestGetUnknown(), TestInfosListsPostgresAndMySQL(), Registry

### Community 15 - "genicon.go"
Cohesion: 0.42
Nodes (7): fillRoundRect(), insideRoundRect(), main(), set(), strokeEllipse(), strokeLine(), RGBA

### Community 16 - ".withAuth"
Cohesion: 0.33
Nodes (4): Handler, Server, isPublicPath(), secureEqual()

## Knowledge Gaps
- **47 isolated node(s):** `$schema`, `name`, `outputfilename`, `frontend:dir`, `frontend:install` (+42 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Name` connect `Name` to `Server`, `mysql.go`, `.Import`, `just-db/main.go`, `New`, `Registry`?**
  _High betweenness centrality (0.159) - this node is a cross-community bridge._
- **Why does `ExportToFile()` connect `Name` to `Server`, `Run`, `App`, `just-db/main.go`, `TestRoundTripSQL`?**
  _High betweenness centrality (0.076) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `mysql.go`, `TestRoundTripSQL`, `.Import`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Are the 7 inferred relationships involving `render()` (e.g. with `engineCard()` and `onDownload()`) actually correct?**
  _`render()` has 7 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `Run()` (e.g. with `execSQL()` and `queryCount()`) actually correct?**
  _`Run()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `name`, `outputfilename` to the rest of the system?**
  _47 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Name` be split into smaller, more focused modules?**
  _Cohesion score 0.07342995169082125 - nodes in this community are weakly interconnected._