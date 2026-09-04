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
  it("keeps database input and operation results when navigating", async () => {
    const { app, client } = await start();
    type("#workspace-database", "selected_db");
    const exporting = vi.spyOn(client, "exportDump");
    click("#export-dump");
    click("#nav-dumps");
    await vi.waitFor(() => expect(app.operations.active).toBeNull());
    click('[data-action="select-profile"][data-name="local-postgres"]');
    await vi.waitFor(() => expect(document.querySelector<HTMLInputElement>("#workspace-database")?.value).toBe("selected_db"));
    expect(exporting).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "selected_db" }), "custom", "");
    expect(document.querySelector(".latest-result")?.textContent).toContain("Dump created");
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
    click("#restore-next-target");
    const submit = document.querySelector<HTMLButtonElement>("#restore-submit")!;
    expect(submit.disabled).toBe(true);
    expect(document.querySelector(".review-summary")?.textContent).toContain("localhost:5432");
    change("#restore-ack", true);
    expect(submit.disabled).toBe(true);
    type("#restore-confirm-name", "target_db");
    expect(submit.disabled).toBe(false);
    submit.click();
    submit.click();
    expect(client.importDump).toHaveBeenCalledTimes(1);
    await vi.waitFor(() => expect(app.operations.active).toBeNull());
    expect(client.importDump).toHaveBeenCalledWith("postgres", expect.objectContaining({ database: "target_db", host: "localhost" }), "sql", "source.sql", true);
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
  it("preserves the stored-file JSON contract", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response('{}', { status: 200 }));
    const connection = { host: "sample", port: 5432, user: "u", password: "", database: "db", sslMode: "prefer" };
    await api.importDump("postgres", connection, "custom", "source.dump", false);
    expect(fetcher).toHaveBeenCalledWith("/api/import", expect.objectContaining({ method: "POST", body: JSON.stringify({ engine: "postgres", connection, format: "custom", fileName: "source.dump", dropExisting: false, confirm: true }) }));
  });

  it("uploads the selected File with the exact reviewed target and flags", async () => {
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response('{}', { status: 200 }));
    const file = new File(["select 1;"], "sample.sql");
    const connection = { host: "sample", port: 3306, user: "u", password: "", database: "db", sslMode: "require" };
    await api.importDump("mysql", connection, "sql", file, true);
    const body = fetcher.mock.calls[0][1]?.body as FormData;
    expect(body.get("connection")).toBe(JSON.stringify(connection));
    expect(body.get("engine")).toBe("mysql");
    expect(body.get("confirm")).toBe("true");
    expect(body.get("dropExisting")).toBe("true");
    expect((body.get("file") as File).name).toBe("sample.sql");
  });
});
