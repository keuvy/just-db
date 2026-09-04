import type { DumpFile, EngineInfo, Profile } from "./api";
import type { Client } from "./app/client";

// Imported only by the Vite development entry point. No real API is called.
export function createPreviewClient(options: { empty?: boolean; many?: boolean; failure?: boolean; delay?: number; desktop?: boolean } = {}): Client {
  const query = new URLSearchParams(typeof location === "undefined" ? "" : location.search);
  const empty = options.empty ?? query.get("preview") === "empty";
  const many = options.many ?? query.get("preview") === "many";
  const failure = options.failure ?? query.get("preview") === "failure";
  const wait = () => new Promise<void>(resolve => setTimeout(resolve, options.delay ?? 180));
  let profiles: Profile[] = empty ? [] : [
    { name: "local-postgres", engine: "postgres", connection: { host: "localhost", port: 5432, user: "postgres", password: "", database: "app_development", sslMode: "prefer" } },
    { name: "staging-mysql", engine: "mysql", connection: { host: "staging.internal", port: 3306, user: "app_user", password: "", database: "storefront", sslMode: "require" } },
    { name: "reporting", engine: "postgres", connection: { host: "analytics.internal", port: 5432, user: "reader", password: "", database: "warehouse", sslMode: "require" } },
  ];
  if (many) profiles = Array.from({ length: 100 }, (_, index) => ({ ...profiles[index % 3], name: `team-${String(index + 1).padStart(3, "0")}-database-profile`, connection: { ...profiles[index % 3].connection } }));
  let dumps: DumpFile[] = empty ? [] : Array.from({ length: many ? 1000 : 6 }, (_, index) => ({ name: `${index % 2 ? "mysql-storefront" : "postgres-app_development"}-2026090${4 - index % 4}-0830${String(index).padStart(2, "0")}${index % 2 ? ".sql" : ".dump"}`, path: `/data/backups/sample-${index}.dump`, size: [25794969, 5242880, 8126464][index % 3], modTime: new Date(Date.UTC(2026, 8, 4, 12 - index % 12, index % 60)).toISOString() }));
  const engines: EngineInfo[] = ["postgres", "mysql"].map(name => {
    const tool = (binary: string) => ({ name: binary, found: !(failure && name === "mysql"), path: `/usr/local/bin/${binary}`, version: name === "postgres" ? "PostgreSQL 18.0" : "MySQL 8.4.0" });
    return { name, displayName: name === "postgres" ? "PostgreSQL" : "MySQL / MariaDB", defaultPort: name === "postgres" ? 5432 : 3306, tools: { dump: tool(name === "postgres" ? "pg_dump" : "mysqldump"), restore: tool(name === "postgres" ? "pg_restore" : "mysql"), client: tool(name === "postgres" ? "psql" : "mysql") }, ready: !(failure && name === "mysql") };
  });
  return {
    runtimeMode: () => options.desktop ? "desktop" : "web",
    getHealth: async () => ({ status: "ok", name: "just-db", version: "0.1.0", mode: options.desktop ? "desktop" : "web", dataDir: "/data", backupsDir: "/data/backups" }),
    getDefaults: async () => null,
    getEngines: async () => { await wait(); return structuredClone(engines); },
    listProfiles: async () => { await wait(); return profiles.map(profile => ({ name: profile.name, engine: profile.engine, host: profile.connection.host, port: profile.connection.port, user: profile.connection.user, database: profile.connection.database, sslMode: profile.connection.sslMode })); },
    getProfile: async name => { await wait(); const profile = profiles.find(item => item.name === name); if (!profile) throw new Error("Profile not found."); return structuredClone(profile); },
    putProfile: async (name, engine, connection) => { await wait(); profiles = [...profiles.filter(profile => profile.name !== name), { name, engine, connection: { ...connection } }]; },
    deleteProfile: async name => { await wait(); profiles = profiles.filter(profile => profile.name !== name); },
    testConnection: async () => { await wait(); if (failure) throw new Error("Could not reach the sample database host."); },
    listDatabases: async engine => { await wait(); if (failure) throw new Error("Database listing is unavailable. Enter a database name manually."); return engine === "mysql" ? ["storefront", "storefront_test", "analytics"] : ["app_development", "app_test", "warehouse", "postgres"]; },
    listDumps: async () => { await wait(); return structuredClone(dumps); },
    deleteDump: async name => { await wait(); dumps = dumps.filter(dump => dump.name !== name); },
    downloadDump: async () => { await wait(); if (failure) throw new Error("Sample download failed."); return true; },
    exportDump: async (engine, connection, format) => { await wait(); await wait(); const name = `${engine}-${connection.database}-${Date.now()}.${format === "sql" ? "sql" : "dump"}`; const dump = { name, path: `/data/backups/${name}`, size: 25794969, modTime: new Date().toISOString() }; dumps.unshift(dump); return { path: dump.path, bytes: dump.size, format }; },
    importDump: async () => { await wait(); await wait(); if (failure) throw new Error("Sample restore failed."); },
    pickOpenPath: async () => "/sample/example.dump",
    openBackupsDir: async () => {},
  };
}
