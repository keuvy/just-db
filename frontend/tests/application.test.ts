import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Application } from "../src/app/application";
import { createPreviewClient } from "../src/preview";
import * as api from "../src/api";

let app: Application | undefined;

beforeEach(() => {
  history.replaceState(null, "", "/");
  localStorage.clear();
  document.body.innerHTML = '<div id="app"></div>';
});
afterEach(() => { app?.dispose(); app = undefined; document.querySelectorAll("dialog").forEach(dialog => dialog.remove()); });

function click(selector: string) { document.querySelector<HTMLButtonElement>(selector)!.click(); }
function type(selector: string, value: string) {
  const input = document.querySelector<HTMLInputElement>(selector)!;
  input.value = value;
  input.dispatchEvent(new Event("input", { bubbles: true }));
}
function change(selector: string, value: string | boolean) {
  const input = document.querySelector<HTMLInputElement>(selector)!;
  if (typeof value === "boolean") input.checked = value; else input.value = value;
  input.dispatchEvent(new Event("change", { bubbles: true }));
}

async function start() {
  const client = createPreviewClient({ delay: 0 });
  app = new Application(document.querySelector("#app")!, client);
  await app.start();
  await app.state.selectProfile("local-postgres");
  return { client, app };
}

describe("rendered workflows", () => {
  it.each(["local-postgres", "staging-mysql"])("creates a new restore target for %s only after review", async profile => {
    const { app, client } = await start();
    await app.state.selectProfile(profile);
    client.importDump = vi.fn().mockResolvedValue(undefined);
    app.state.mode = "restore";
    app.state.setSource({ name: "source.sql", value: "source.sql", library: true });
    app.state.restoreStep = 2;
    app.state.dropExisting = true;
    app.render();
    click('[name="restore-target-mode"][value="new"]');
    expect(document.querySelector("#workspace-database")).toBeNull();
    expect(document.querySelector("#drop-existing")).toBeNull();
    expect(document.querySelector<HTMLButtonElement>("#restore-next-target")!.disabled).toBe(true);
    type("#restore-new-database", "restore_copy");
    click("#restore-next-target");
    expect(document.querySelector(".review-summary")?.textContent).toContain("Before restore");
    expect(client.importDump).not.toHaveBeenCalled();
    click('[data-action="restore-back"]');
    click('[name="restore-target-mode"][value="existing"]');
    expect(document.querySelector<HTMLInputElement>("#workspace-database")!.value).toBe(app.state.session!.database);
    click('[name="restore-target-mode"][value="new"]');
    expect(document.querySelector<HTMLInputElement>("#restore-new-database")!.value).toBe("restore_copy");
    click("#restore-next-target");
    change("#restore-ack", true);
    click("#restore-submit");
    await vi.waitFor(() => expect(client.importDump).toHaveBeenCalledWith(app.state.profile!.engine, expect.objectContaining({ database: "restore_copy" }), "sql", "source.sql", false, false, true));
  });

  it.each(["source.sql.gz", "source.zip"])("reviews and submits compressed file %s", async name => {
    const { app, client } = await start();
    client.importDump = vi.fn().mockResolvedValue(undefined);
    click("#tab-restore");
    const input = document.querySelector<HTMLInputElement>("#restore-file")!;
    expect(input.accept).toContain(".gz,.zip");
    const file = new File(["compressed fixture"], name);
    Object.defineProperty(input, "files", { value: [file] });
    input.dispatchEvent(new Event("change", { bubbles: true }));
    expect(app.state.source?.name).toBe(name);
    expect(document.querySelector(".selected-file")?.textContent).toContain("format detected after decompression");
    click("#restore-next-source");
    change("#drop-existing", true);
    click("#restore-next-target");
    expect(document.querySelector(".review-summary")?.textContent).toContain("Automatic");
    expect(document.body.textContent).toContain("public schema with CASCADE");
    const submit = document.querySelector<HTMLButtonElement>("#restore-submit")!;
    expect(submit.disabled).toBe(true);
    change("#restore-ack", true);
    expect(submit.disabled).toBe(true);
    type("#restore-confirm-name", app.state.review!.connection.database);
    expect(submit.disabled).toBe(false);
    click("#restore-submit");
    await vi.waitFor(() => expect(client.importDump).toHaveBeenCalledWith("postgres", expect.any(Object), "auto", file, true, false, false));
  });

  it("saves engine, SSL mode and discovered database choices through the custom controls", async () => {
    const { client } = await start();
    const put = vi.spyOn(client, "putProfile");
    click("#edit-profile");
    click("#editor-engine");
    click('#editor-engine-listbox [data-value="mysql"]');
    expect(document.querySelector<HTMLInputElement>("#editor-port")?.value).toBe("3306");
    click("#editor-sslMode");
    click('#editor-sslMode-listbox [data-value="require"]');
    click('[data-editor="test"]');
    await vi.waitFor(() => expect(document.querySelector('#editor-database-listbox [data-value="storefront"]')).not.toBeNull());
    click("#editor-database");
    click('#editor-database-listbox [data-value="storefront"]');
    document.querySelector("#profile-form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await vi.waitFor(() => expect(put).toHaveBeenCalledWith("local-postgres", "mysql", expect.objectContaining({ port: 3306, sslMode: "require", database: "storefront" })));
  });

  it("updates sorting, stored source and restore target from custom option clicks", async () => {
    const { app } = await start();
    click("#nav-dumps");
    await vi.waitFor(() => expect(document.querySelector(".dump-table")).not.toBeNull());
    click("#dump-sort");
    click('#dump-sort-listbox [data-value="size"]');
    expect(document.querySelector(".size-cell")?.textContent).toBe("24.6 MB");
    click('[data-action="select-profile"][data-name="local-postgres"]');
    await vi.waitFor(() => expect(document.querySelector("#tab-restore")).not.toBeNull());
    click("#tab-restore");
    click("#source-library");
    await vi.waitFor(() => expect(document.querySelectorAll('#restore-library-listbox [role="option"]').length).toBeGreaterThan(1));
    const dump = app.state.dumps.data[0];
    click("#restore-library");
    click("#restore-library-option-1");
    expect(app.state.source?.name).toBe(dump.name);
    click("#restore-next-source");
    click("#restore-profile");
    click('#restore-profile-listbox [data-value="reporting"]');
    await vi.waitFor(() => expect(app.state.profile?.name).toBe("reporting"));
    await vi.waitFor(() => expect(document.querySelector('#workspace-database-listbox [data-value="app_test"]')).not.toBeNull());
    click("#workspace-database");
    click('#workspace-database-listbox [data-value="app_test"]');
    expect(app.state.session?.database).toBe("app_test");
    expect(document.querySelector<HTMLInputElement>("#workspace-database")?.value).toBe("app_test");
    expect(document.querySelector("select,datalist")).toBeNull();
  });

  it("keeps database input and operation results when navigating", async () => {
    const { app, client } = await start();
    type("#workspace-database", "selected_db");
    const exporting = vi.spyOn(client, "exportDump");
    click("#export-dump");
    click("#nav-dumps");
    await vi.waitFor(() => expect(app.operations.active).toBeNull());
    click('[data-action="select-profile"][data-name="local-postgres"]');
    await vi.waitFor(() => expect(document.querySelector<HTMLInputElement>("#workspace-database")?.value).toBe("selected_db"));
    expect(exporting).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "selected_db" }), { format: "custom", content: "all", tables: [] }, "");
    expect(document.querySelector(".latest-result")?.textContent).toContain("Dump created");
  });

  it("exports structure and data chosen per table", async () => {
    const { app, client } = await start();
    const exporting = vi.spyOn(client, "exportDump");
    expect(document.querySelector("#export-schema")).not.toBeNull();
    change("#export-tables-selected", true);
    expect(document.querySelector("#export-schema")).toBeNull();
    await vi.waitFor(() => expect(document.querySelectorAll("#table-rows .table-row")).toHaveLength(7));
    expect(document.querySelector("#table-summary")?.textContent).toContain("7 of 7 tables");
    expect(document.querySelector('[data-table="public.active_customers"][data-part="data"]')).toBeNull();
    change("#export-format-sql", true);
    type("#table-filter", "order");
    expect(Array.from(document.querySelectorAll<HTMLInputElement>('#table-rows [data-part="schema"]'), input => input.dataset.table)).toEqual(["public.order_items", "public.orders"]);
    change("#table-column-schema", false);
    expect(document.querySelectorAll('#table-rows [data-part="schema"]:checked')).toHaveLength(0);
    type("#table-filter", "");
    const column = document.querySelector<HTMLInputElement>("#table-column-schema")!;
    expect(column.indeterminate).toBe(true);
    change('[data-table="public.sessions"][data-part="data"]', false);
    change('[data-table="public.sessions"][data-part="schema"]', false);
    expect(document.querySelector("#table-summary")?.textContent).toContain("6 of 7 tables");
    change('[data-table="public.audit_log"][data-part="data"]', false);
    click("#export-dump");
    await vi.waitFor(() => expect(app.operations.active).toBeNull());
    const options = exporting.mock.calls[0][2];
    expect(options).toMatchObject({ format: "sql", content: "all", tables: [] });
    expect(Object.fromEntries(options.perTable!.map(table => [table.name, [table.schema, table.data]]))).toEqual({
      "public.active_customers": [true, false],
      "public.audit_log": [true, false],
      "public.customers": [true, true],
      "public.order_items": [false, true],
      "public.orders": [false, true],
      "public.products": [true, true],
    });
    expect(document.querySelector(".latest-result")?.textContent).toContain("6 tables · structure for 4, data for 4");
  });

  it("blocks a custom archive that mixes data-only tables with structure", async () => {
    await start();
    change("#export-tables-selected", true);
    await vi.waitFor(() => expect(document.querySelectorAll("#table-rows .table-row")).toHaveLength(7));
    change('[data-table="public.orders"][data-part="schema"]', false);
    expect(document.querySelector<HTMLButtonElement>("#export-dump")!.disabled).toBe(true);
    expect(document.querySelector("#export-help")?.textContent).toContain("Choose SQL format");
    change("#export-format-sql", true);
    expect(document.querySelector<HTMLButtonElement>("#export-dump")!.disabled).toBe(false);
  });

  it("reloads tables for a database picked from the list", async () => {
    const { app, client } = await start();
    const listing = vi.spyOn(client, "listTables");
    change("#export-tables-selected", true);
    await vi.waitFor(() => expect(app.state.session?.tablesFor).toBe("app_development"));
    click("#workspace-database");
    click('#workspace-database-listbox [data-value="app_test"]');
    await vi.waitFor(() => expect(app.state.session?.tablesFor).toBe("app_test"));
    expect(listing).toHaveBeenLastCalledWith("postgres", expect.objectContaining({ database: "app_test" }));
    type("#workspace-database", "typed_db");
    expect(listing).toHaveBeenCalledTimes(2);
    expect(document.querySelector('[data-action="load-tables"]')?.textContent).toContain("Load tables");
  });

  it("requires SQL acknowledgment and typed target before one restore submission", async () => {
    const { app, client } = await start();
    client.importDump = vi.fn().mockImplementation(() => new Promise(resolve => setTimeout(resolve, 15)));
    app.state.sourceMode = "library";
    app.state.setSource({ name: "source.sql", value: "source.sql", library: true });
    click("#tab-restore");
    click("#restore-next-source");
    type("#workspace-database", "target_db");
    change("#drop-existing", true);
    change("#disable-foreign-keys", true);
    click("#restore-next-target");
    const submit = document.querySelector<HTMLButtonElement>("#restore-submit")!;
    expect(submit.disabled).toBe(true);
    expect(document.querySelector(".review-summary")?.textContent).toContain("localhost:5432");
    expect(document.querySelector(".review-summary")?.textContent).toContain("Foreign key checksDisabled");
    change("#restore-ack", true);
    expect(submit.disabled).toBe(true);
    type("#restore-confirm-name", "target_db");
    expect(submit.disabled).toBe(false);
    submit.click();
    submit.click();
    expect(client.importDump).toHaveBeenCalledTimes(1);
    await vi.waitFor(() => expect(app.operations.active).toBeNull());
    expect(client.importDump).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "target_db", host: "localhost" }), "sql", "source.sql", true, true, false);
  });

  it("does not silently restore after changing the reviewed profile", async () => {
    const { app, client } = await start();
    client.importDump = vi.fn();
    app.state.setSource({ name: "source.dump", value: "source.dump", library: true });
    app.state.mode = "restore";
    app.state.prepareReview();
    app.render();
    expect(document.querySelector("#restore-submit")).not.toBeNull();
    click('[data-action="select-profile"][data-name="reporting"]');
    await vi.waitFor(() => expect(app.state.profile?.name).toBe("reporting"));
    expect(document.querySelector("#restore-submit")).toBeNull();
    expect(app.state.restoreStep).toBe(2);
    expect(client.importDump).not.toHaveBeenCalled();
  });

  it("saves edits to the same profile and leaves the chosen workspace database independent", async () => {
    const { app, client } = await start();
    type("#workspace-database", "temporary_database");
    const put = vi.spyOn(client, "putProfile");
    click("#edit-profile");
    expect(document.querySelector<HTMLInputElement>("#editor-name")?.readOnly).toBe(true);
    type("#editor-host", "new-host");
    const form = document.querySelector<HTMLFormElement>("#profile-form")!;
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await vi.waitFor(() => expect(put).toHaveBeenCalled());
    expect(put).toHaveBeenCalledWith("local-postgres", "postgres", expect.objectContaining({ host: "new-host", database: "app_development", password: "" }));
    await vi.waitFor(() => expect(app.state.profile?.connection.host).toBe("new-host"));
    expect(app.state.session?.database).toBe("temporary_database");
  });

  it("renders untrusted names as text and stores only appearance preferences", async () => {
    const { app } = await start();
    app.state.profiles.data[0].name = '<img src=x onerror="alert(1)">';
    app.render();
    expect(document.querySelector("#profile-list img")).toBeNull();
    click("#nav-settings");
    click('[data-theme="dark"]');
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(Object.keys(localStorage)).toEqual(["just-db-theme"]);
  });
});

describe("HTTP import adapter", () => {
  it("passes database creation to the desktop restore binding", async () => {
    const importDump = vi.fn().mockResolvedValue(undefined);
    const desktopWindow = window as unknown as { go?: unknown };
    const previous = desktopWindow.go;
    desktopWindow.go = { main: { App: { ImportDump: importDump } } };
    try {
      const connection = { host: "sample", port: 5432, user: "u", password: "", database: "new_db", sslMode: "prefer" };
      await api.importDump("postgres", connection, "custom", "/tmp/source.dump", false, false, true);
      expect(importDump).toHaveBeenCalledWith("postgres", connection, { format: "custom", dropExisting: false, disableForeignKeys: false, createDatabase: true, confirm: true }, "/tmp/source.dump");
    } finally { desktopWindow.go = previous; }
  });

  it("preserves the stored-file JSON contract", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response('{}', { status: 200 }));
    const connection = { host: "sample", port: 5432, user: "u", password: "", database: "db", sslMode: "prefer" };
    await api.importDump("postgres", connection, "custom", "source.dump", false, true);
    expect(fetcher).toHaveBeenCalledWith("/api/import", expect.objectContaining({ method: "POST", body: JSON.stringify({ engine: "postgres", connection, format: "custom", fileName: "source.dump", dropExisting: false, disableForeignKeys: true, createDatabase: false, confirm: true }) }));
  });

  it("uploads the selected File with the exact reviewed target and flags", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response('{}', { status: 200 }));
    const file = new File(["select 1;"], "sample.sql");
    const connection = { host: "sample", port: 3306, user: "u", password: "", database: "db", sslMode: "require" };
    await api.importDump("mysql", connection, "sql", file, true, false, true);
    const body = fetcher.mock.calls[0][1]?.body as FormData;
    expect(body.get("connection")).toBe(JSON.stringify(connection));
    expect(body.get("engine")).toBe("mysql");
    expect(body.get("confirm")).toBe("true");
    expect(body.get("dropExisting")).toBe("true");
    expect(body.get("disableForeignKeys")).toBe("false");
    expect(body.get("createDatabase")).toBe("true");
    expect((body.get("file") as File).name).toBe("sample.sql");
  });
});
