import { describe, expect, it, vi } from "vitest";
import { Workspace, exportOptions, exportProblem, fileFormat, requiredTool } from "../src/app/state";
import { Operations } from "../src/app/operations";
import { createPreviewClient } from "../src/preview";
import type { Profile, Table } from "../src/api";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

const full = (format: string) => ({ format, content: "all" as const, tables: [] });

async function ready() {
  const api = createPreviewClient({ delay: 0 });
  const state = new Workspace(api);
  await state.start();
  await state.selectProfile("local-postgres");
  return { api, state };
}

describe("profile and database state", () => {
  it("ignores an old profile response and its database discovery", async () => {
    const api = createPreviewClient({ delay: 0 });
    const a = deferred<Profile>();
    const b = await api.getProfile("staging-mysql");
    api.getProfile = vi.fn(name => name === "old" ? a.promise : Promise.resolve(b));
    api.listDatabases = vi.fn().mockResolvedValue(["storefront"]);
    const state = new Workspace(api);
    const old = state.selectProfile("old");
    await state.selectProfile(b.name);
    a.resolve({ name: "old", engine: "postgres", connection: { ...b.connection, host: "old-host" } });
    await old;
    expect(state.profile?.name).toBe(b.name);
    expect(state.session?.databases.data).toEqual(["storefront"]);
    expect(api.listDatabases).toHaveBeenCalledTimes(1);
  });

  it("ignores database lists arriving after a profile switch", async () => {
    const { api, state } = await ready();
    const list = deferred<string[]>();
    api.listDatabases = vi.fn().mockImplementationOnce(() => list.promise).mockResolvedValue(["mysql_db"]);
    const pending = state.loadDatabases();
    await state.selectProfile("staging-mysql");
    list.resolve(["wrong_database"]);
    await pending;
    expect(state.session?.databases.data).toEqual(["mysql_db"]);
  });

  it("keeps per-profile choices without saving them to a profile", async () => {
    const { api, state } = await ready();
    api.putProfile = vi.fn();
    state.setDatabase("chosen_for_operation");
    state.session!.format = "sql";
    await state.selectProfile("staging-mysql");
    state.setDatabase("mysql_choice");
    await state.selectProfile("local-postgres");
    expect(state.session?.database).toBe("chosen_for_operation");
    expect(state.session?.format).toBe("sql");
    expect(state.profile?.connection.database).toBe("app_development");
    expect(api.putProfile).not.toHaveBeenCalled();
  });

  it("keeps manual database entry available after failed lookup", async () => {
    const { api, state } = await ready();
    api.listDatabases = vi.fn().mockRejectedValue(new Error("permission denied"));
    await state.loadDatabases();
    state.setDatabase("known_database");
    expect(state.session?.databases.error).toBe("permission denied");
    expect(state.session?.database).toBe("known_database");
    expect(state.profile).not.toBeNull();
  });

  it("isolates initial region failures", async () => {
    const api = createPreviewClient({ delay: 0 });
    api.getHealth = vi.fn().mockRejectedValue(new Error("health failed"));
    const state = new Workspace(api);
    await state.start();
    expect(state.health.error).toBe("health failed");
    expect(state.profiles.data).toHaveLength(3);
    expect(state.engines.data).toHaveLength(2);
  });

  it("does not attach a late connection test to a reloaded profile", async () => {
    const { api, state } = await ready();
    const test = deferred<void>();
    api.testConnection = () => test.promise;
    const pending = state.testProfile();
    await state.selectProfile("local-postgres");
    test.resolve();
    await pending;
    expect(state.session?.test).toBeUndefined();
    expect(state.session?.testing).toBe(false);
  });
});

describe("restore review", () => {
  it("reviews a new database without changing the export target or dropping objects", async () => {
    const { api, state } = await ready();
    state.setSource({ name: "source.dump", value: "source.dump", library: true });
    state.dropExisting = true;
    state.setCreateDatabase(true);
    state.setNewDatabase(" new_restore ");
    const review = state.prepareReview();
    expect(review).toMatchObject({ createDatabase: true, dropExisting: false, connection: { database: "new_restore" } });
    expect(state.session?.database).toBe("app_development");
    api.importDump = vi.fn().mockResolvedValue(undefined);
    state.setNewDatabase("later_name");
    await new Operations(api).restore(review);
    expect(api.importDump).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "new_restore" }), "custom", "source.dump", false, false, true);
    expect(state.review).toBeNull();
    state.setCreateDatabase(false);
    expect(state.prepareReview().connection.database).toBe("app_development");
  });

  it("validates a new name and requires the client even for PostgreSQL custom archives", async () => {
    const { state } = await ready();
    state.setSource({ name: "source.dump", value: "source.dump", library: true });
    state.setCreateDatabase(true);
    expect(() => state.prepareReview()).toThrow("Choose a target");
    state.setNewDatabase("app_development");
    expect(() => state.prepareReview()).toThrow("already exists");
    state.setNewDatabase("é".repeat(32));
    expect(() => state.prepareReview()).toThrow("63 bytes");
    state.setNewDatabase("new_restore");
    state.engines.data.find(info => info.name === "postgres")!.tools.client.found = false;
    expect(() => state.prepareReview()).toThrow("psql");
  });

  it.each(["backup.sql.gz", "backup.GZ", "backup.ZIP"])("reviews %s with automatic format detection and acknowledgment", async name => {
    const { state } = await ready();
    expect(fileFormat(name)).toBe("auto");
    state.setSource({ name, value: name, library: true });
    expect(state.prepareReview().format).toBe("auto");
    expect(state.canRestore()).toBe(false);
    state.acknowledged = true;
    expect(state.canRestore()).toBe(true);
    await state.selectProfile("staging-mysql");
    expect(state.prepareReview().format).toBe("auto");
    expect(state.canRestore()).toBe(false);
    state.acknowledged = true;
    expect(state.canRestore()).toBe(true);
  });

  it("defers compressed PostgreSQL tool selection until the format is known", async () => {
    const { state } = await ready();
    const postgres = state.engines.data.find(info => info.name === "postgres")!;
    postgres.tools.restore.found = false;
    expect(requiredTool(state.engines.data, "postgres", "restore", "auto")).toBe("");
    postgres.tools.client.found = false;
    expect(requiredTool(state.engines.data, "postgres", "restore", "auto")).toContain("missing");
  });

  it("freezes the reviewed target and submits exactly that snapshot", async () => {
    const { api, state } = await ready();
    const file = new File(["-- sample"], "source.sql");
    state.setSource({ name: file.name, value: file, library: false });
    state.setDatabase("reviewed_db");
    state.dropExisting = true;
    const review = state.prepareReview();
    expect(state.canRestore()).toBe(false);
    state.acknowledged = true;
    expect(state.canRestore()).toBe(false);
    state.confirmation = "reviewed_db";
    expect(state.canRestore()).toBe(true);
    api.importDump = vi.fn().mockResolvedValue(undefined);
    const operations = new Operations(api);
    const job = operations.restore(review);
    state.setDatabase("different_db");
    state.profile!.connection.host = "different_host";
    await job;
    expect(api.importDump).toHaveBeenCalledWith("postgres", expect.objectContaining({ host: "localhost", database: "reviewed_db" }), "sql", file, true, false, false);
    expect(state.review).toBeNull();
    expect(operations.records[0]).toMatchObject({ profileName: "local-postgres", database: "reviewed_db", host: "localhost:5432" });
  });

  it("invalidates acknowledgment when changing the source or profile", async () => {
    const { state } = await ready();
    state.setSource({ name: "first.sql", value: "first.sql", library: true });
    state.prepareReview();
    state.acknowledged = true;
    state.setSource({ name: "second.sql", value: "second.sql", library: true });
    expect(state.canRestore()).toBe(false);
    state.prepareReview();
    state.acknowledged = true;
    await state.selectProfile("staging-mysql");
    expect(state.review).toBeNull();
    expect(state.acknowledged).toBe(false);
  });

  it("rejects a PostgreSQL archive for MySQL and checks only the needed executable", async () => {
    const { state } = await ready();
    await state.selectProfile("staging-mysql");
    state.setSource({ name: "source.dump", value: "source.dump", library: true });
    expect(() => state.prepareReview()).toThrow("requires a SQL dump");
    const postgres = state.engines.data[0];
    postgres.tools.restore.found = false;
    expect(requiredTool(state.engines.data, "postgres", "restore", "sql")).toBe("");
    expect(requiredTool(state.engines.data, "postgres", "restore", "custom")).toContain("pg_restore");
  });
});

describe("operation results", () => {
  it("serializes work and captures an export target before awaits", async () => {
    const { api, state } = await ready();
    const pending = deferred<{ path: string; bytes: number; format: string }>();
    api.exportDump = vi.fn(() => pending.promise);
    const operations = new Operations(api);
    const first = operations.export(state.profile!, "original_db", full("custom"));
    state.profile!.connection.database = "changed";
    const second = await operations.export(state.profile!, "other_db", full("custom"));
    expect(second).toBe(false);
    expect(api.exportDump).toHaveBeenCalledTimes(1);
    pending.resolve({ path: "/backups/original.dump", bytes: 100, format: "custom" });
    await first;
    expect(api.exportDump).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "original_db" }), full("custom"), "");
    expect(operations.records[0].database).toBe("original_db");
  });

  it("preserves export success when the desktop save dialog is cancelled", async () => {
    const { api, state } = await ready();
    api.runtimeMode = () => "desktop";
    api.downloadDump = vi.fn().mockResolvedValue(false);
    const operations = new Operations(api);
    await operations.export(state.profile!, "example", full("custom"));
    expect(operations.records[0].status).toBe("success");
    expect(operations.records[0].text).toContain("Save cancelled");
    expect(operations.records[0].fileName).toBeTruthy();
  });

  it("retries the existing file after copy failure without running another export", async () => {
    const { api, state } = await ready();
    const original = api.exportDump;
    api.exportDump = vi.fn(original);
    api.downloadDump = vi.fn().mockRejectedValueOnce(new Error("copy failed")).mockResolvedValue(true);
    const operations = new Operations(api);
    await operations.export(state.profile!, "example", full("sql"));
    const result = operations.records[0];
    expect(result.status).toBe("warning");
    await operations.copy(result);
    expect(api.exportDump).toHaveBeenCalledTimes(1);
    expect(result.status).toBe("success");
    expect(result.text).toContain("Download started");
  });

  it("redacts the connection password in failed operation records", async () => {
    const { api, state } = await ready();
    state.profile!.connection.password = "test-secret-value";
    api.exportDump = vi.fn().mockRejectedValue(new Error("failed test-secret-value"));
    const operations = new Operations(api);
    await operations.export(state.profile!, "example", full("custom"));
    expect(JSON.stringify(operations.records)).not.toContain("test-secret-value");
    expect(operations.records[0].status).toBe("error");
  });
});

describe("export choices", () => {
  it("sends a whole-database export until per-table choices differ", async () => {
    const { state } = await ready();
    const session = state.session!;
    expect(exportOptions(session)).toEqual(full("custom"));
    state.setTableMode("selected");
    expect(exportProblem(session)).toBe("Loading tables...");
    await vi.waitFor(() => expect(session.tablesFor).toBe("app_development"));
    expect(session.tableParts.get("public.orders")).toEqual({ schema: true, data: true });
    expect(session.tableParts.get("public.active_customers")).toEqual({ schema: true, data: false });
    expect(exportOptions(session)).toEqual(full("custom"));
    const names = session.tables.data.map(table => table.name);
    state.setTablePart(names, "data", false);
    expect(exportOptions(session)).toEqual({ format: "custom", content: "schema", tables: [] });
    state.setTablePart(["public.orders"], "data", true);
    state.setTablePart(["public.sessions"], "schema", false);
    const options = exportOptions(session);
    expect(options.perTable).toHaveLength(names.length - 1);
    expect(options.perTable).toContainEqual({ name: "public.orders", schema: true, data: true });
    expect(options.perTable).toContainEqual({ name: "public.active_customers", schema: true, data: false });
    expect(options.perTable?.some(table => table.name === "public.sessions")).toBe(false);
    state.setTablePart(["public.orders"], "schema", false);
    expect(() => exportOptions(session)).toThrow("custom archive cannot mix");
    session.format = "sql";
    expect(exportOptions(session).perTable).toContainEqual({ name: "public.orders", schema: false, data: true });
    state.setTablePart(names, "schema", false);
    state.setTablePart(names, "data", false);
    expect(() => exportOptions(session)).toThrow("at least one table");
    state.setTableMode("all");
    expect(exportOptions(session)).toEqual(full("sql"));
  });

  it("allows a custom archive with rows left out of some tables", async () => {
    const { state } = await ready();
    const session = state.session!;
    state.setTableMode("selected");
    await vi.waitFor(() => expect(session.tablesFor).toBe("app_development"));
    state.setTablePart(["public.audit_log"], "data", false);
    expect(exportProblem(session)).toBe("");
    expect(exportOptions(session).perTable).toContainEqual({ name: "public.audit_log", schema: true, data: false });
  });

  it("maps structure and data choices to export content", async () => {
    const { state } = await ready();
    const session = state.session!;
    session.data = false;
    expect(exportOptions(session).content).toBe("schema");
    session.data = true;
    session.schema = false;
    expect(exportOptions(session).content).toBe("data");
    session.data = false;
    expect(() => exportOptions(session)).toThrow("structure, table data, or both");
  });

  it("starts the table list from the Include choice", async () => {
    const { state } = await ready();
    state.session!.schema = false;
    await state.loadTables();
    expect(state.session!.tableParts.get("public.orders")).toEqual({ schema: false, data: true });
    expect(state.session!.tableParts.get("public.active_customers")).toEqual({ schema: false, data: false });
  });

  it("drops the table list when the database changes, even if it arrives late", async () => {
    const { api, state } = await ready();
    const list = deferred<Table[]>();
    api.listTables = vi.fn().mockReturnValueOnce(list.promise);
    state.session!.tableMode = "selected";
    const pending = state.loadTables();
    state.setDatabase("app_test");
    list.resolve([{ name: "public.stale" }]);
    await pending;
    expect(api.listTables).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "app_development" }));
    expect(state.session).toMatchObject({ tablesFor: "", tableMode: "selected", tables: { data: [], loading: false } });
    expect(state.session!.tableParts.size).toBe(0);
    expect(exportProblem(state.session!)).toContain("Load the table list");
  });

  it("keeps earlier choices when the table list is refreshed", async () => {
    const { api, state } = await ready();
    api.listTables = vi.fn().mockResolvedValueOnce([{ name: "a" }, { name: "b" }, { name: "c" }]).mockResolvedValueOnce([{ name: "a" }, { name: "c" }, { name: "d" }]);
    await state.loadTables();
    state.setTablePart(["c"], "data", false);
    await state.loadTables();
    expect(Object.fromEntries(state.session!.tableParts)).toEqual({ a: { schema: true, data: true }, c: { schema: true, data: false }, d: { schema: false, data: false } });
  });
});
