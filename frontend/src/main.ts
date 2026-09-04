import "./style.css";
import {
  dumpDownloadURL,
  deleteProfile,
  exportDump,
  getDefaults,
  getEngines,
  getHealth,
  getProfile,
  importDump,
  listDumps,
  listProfiles,
  openBackupsDir,
  pickOpenPath,
  putProfile,
  runtimeMode,
  testConnection,
  type Connection,
  type DumpFile,
  type EngineInfo,
  type Profile,
  type ProfileSummary,
  type Tool,
} from "./api";

const el = document.querySelector<HTMLDivElement>("#app");
if (!el) {
  throw new Error("#app missing");
}
const root: HTMLDivElement = el;
root.innerHTML = `<p class="lede">Loading…</p>`;

type Tab = "saved" | "new" | "about";

let engines: EngineInfo[] = [];
let loaded: Profile | null = null;

function engineInfo(name: string): EngineInfo | undefined {
  return engines.find((item) => item.name === name);
}

function toolRow(label: string, tool: Tool): string {
  if (!tool.found) {
    return `<dt>${label}</dt><dd class="missing">missing (${escapeHtml(tool.name)})</dd>`;
  }
  const version = tool.version ? `<div class="meta">${escapeHtml(tool.version)}</div>` : "";
  return `<dt>${label}</dt><dd>${escapeHtml(tool.path ?? tool.name)}${version}</dd>`;
}

function engineCard(engine: EngineInfo): string {
  const badge = engine.ready
    ? `<span class="badge ok">ready</span>`
    : `<span class="badge bad">tools missing</span>`;
  return `
    <article class="card">
      <h2>${escapeHtml(engine.displayName)}${badge}</h2>
      <p class="port">engine ${escapeHtml(engine.name)} · default port ${engine.defaultPort}</p>
      <dl>
        ${toolRow("dump", engine.tools.dump)}
        ${toolRow("restore", engine.tools.restore)}
        ${toolRow("client", engine.tools.client)}
      </dl>
    </article>
  `;
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function field(id: string, label: string, type: string, value: string | number): string {
  return `<label>${escapeHtml(label)}<input id="${id}" type="${type}" value="${escapeHtml(String(value))}" /></label>`;
}

function engineSelect(id: string, selected: string): string {
  const options = engines
    .map((item) => {
      const isSelected = item.name === selected ? " selected" : "";
      return `<option value="${escapeHtml(item.name)}"${isSelected}>${escapeHtml(item.displayName)}</option>`;
    })
    .join("");
  return `<label>Engine<select id="${id}">${options}</select></label>`;
}

function sslSelect(id: string, selected: string): string {
  const ssl = selected || "prefer";
  return `<label>SSL mode
    <select id="${id}">
      <option value="prefer"${ssl === "prefer" ? " selected" : ""}>prefer</option>
      <option value="disable"${ssl === "disable" ? " selected" : ""}>disable</option>
      <option value="require"${ssl === "require" ? " selected" : ""}>require</option>
    </select>
  </label>`;
}

function dumpOptionValue(dump: DumpFile): string {
  if (runtimeMode() === "desktop") {
    return dump.path;
  }
  return dump.name;
}

function dumpOptions(dumps: DumpFile[] | null | undefined): string {
  const list = dumps ?? [];
  if (list.length === 0) {
    const empty =
      runtimeMode() === "desktop" ? "No dumps in the backups folder yet" : "No dumps in data/backups yet";
    return `<option value="">${empty}</option>`;
  }
  return list
    .map(
      (dump) =>
        `<option value="${escapeHtml(dumpOptionValue(dump))}">${escapeHtml(dump.name)} (${dump.size} bytes)</option>`,
    )
    .join("");
}

function formatOptions(engine: string): string {
  if (engine === "mysql") {
    return `<option value="sql">sql (.sql)</option>`;
  }
  return `
    <option value="custom">custom (.dump)</option>
    <option value="sql">sql (.sql)</option>
  `;
}

function profileListHtml(profiles: ProfileSummary[], selected: string): string {
  if (profiles.length === 0) {
    return `<p class="empty">No saved connections yet. Add one under Save new connection.</p>`;
  }
  return `<ul class="profile-list">${profiles
    .map((item) => {
      const isSelected = item.name === selected ? " selected" : "";
      const detail = `${item.engine} · ${item.user}@${item.host}/${item.database}`;
      return `<li>
        <button type="button" class="profile-item${isSelected}" data-name="${escapeHtml(item.name)}">
          <span class="profile-item-name">${escapeHtml(item.name)}</span>
          <span class="profile-item-meta">${escapeHtml(detail)}</span>
        </button>
      </li>`;
    })
    .join("")}</ul>`;
}

function inputValue(id: string): string {
  const node = document.querySelector<HTMLInputElement | HTMLSelectElement>(`#${id}`);
  return node?.value ?? "";
}

function setStatus(message: string, kind: "ok" | "bad" | "muted" = "muted", id = "status-saved"): void {
  const node = document.querySelector<HTMLParagraphElement>(`#${id}`);
  if (!node) {
    return;
  }
  node.className = `status ${kind}`;
  node.textContent = message;
}

function tabFromHash(): Tab {
  const raw = location.hash.replace(/^#/, "");
  if (raw === "new" || raw === "about" || raw === "saved") {
    return raw;
  }
  return "saved";
}

function showTab(tab: Tab): void {
  document.querySelectorAll<HTMLButtonElement>("[data-tab]").forEach((btn) => {
    const on = btn.dataset.tab === tab;
    btn.setAttribute("aria-selected", on ? "true" : "false");
    btn.classList.toggle("active", on);
    btn.tabIndex = on ? 0 : -1;
  });
  document.querySelectorAll<HTMLElement>("[data-panel]").forEach((panel) => {
    panel.hidden = panel.dataset.panel !== tab;
  });
  const next = `#${tab}`;
  if (location.hash !== next) {
    history.replaceState(null, "", next);
  }
}

function readNewConnection(): Connection {
  return {
    host: inputValue("newHost"),
    port: Number(inputValue("newPort") || String(engineInfo(inputValue("newEngine"))?.defaultPort ?? 5432)),
    user: inputValue("newUser"),
    password: inputValue("newPassword"),
    database: inputValue("newDatabase"),
    sslMode: inputValue("newSslMode") || "prefer",
  };
}

async function refreshDumps(): Promise<void> {
  const select = document.querySelector<HTMLSelectElement>("#dumpFile");
  if (!select) {
    return;
  }
  const dumps = await listDumps();
  select.innerHTML = dumpOptions(dumps);
}

async function refreshProfiles(selected: string): Promise<void> {
  const mount = document.querySelector("#profileList");
  if (!mount) {
    return;
  }
  const profiles = await listProfiles();
  mount.innerHTML = profileListHtml(profiles, selected);
  const hint = document.querySelector<HTMLElement>("#savedHint");
  if (hint && !loaded) {
    hint.hidden = profiles.length === 0;
  }
}

function syncSavedWorkbench(): void {
  const workbench = document.querySelector<HTMLElement>("#savedWorkbench");
  const hint = document.querySelector<HTMLElement>("#savedHint");
  if (!workbench || !hint) {
    return;
  }
  const on = loaded !== null;
  workbench.hidden = !on;
  hint.hidden = on;
  if (!loaded) {
    return;
  }
  const title = document.querySelector("#savedTitle");
  if (title) {
    title.textContent = loaded.name;
  }
  const format = document.querySelector<HTMLSelectElement>("#format");
  if (format) {
    format.innerHTML = formatOptions(loaded.engine);
  }
}

async function selectSavedProfile(name: string): Promise<void> {
  setStatus("Loading connection…");
  try {
    loaded = await getProfile(name);
    await refreshProfiles(name);
    syncSavedWorkbench();
    setStatus(`Using ${loaded.name}.`);
  } catch (error) {
    loaded = null;
    syncSavedWorkbench();
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

function onNewEngineChange(): void {
  const name = inputValue("newEngine");
  const info = engineInfo(name);
  const port = document.querySelector<HTMLInputElement>("#newPort");
  if (port && info) {
    port.value = String(info.defaultPort);
  }
}

async function onSaveProfile(): Promise<void> {
  const name = inputValue("newName").trim();
  if (!name) {
    setStatus("Name is required.", "bad", "status-new");
    return;
  }
  setStatus("Saving…", "muted", "status-new");
  try {
    const engine = inputValue("newEngine") || "postgres";
    await putProfile(name, engine, readNewConnection());
    setStatus(`Saved ${name}.`, "ok", "status-new");
    await selectSavedProfile(name);
    showTab("saved");
    setStatus(`Saved ${name}.`, "ok");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad", "status-new");
  }
}

async function onDeleteProfile(): Promise<void> {
  const name = loaded?.name;
  if (!name) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  if (!window.confirm(`Delete connection ${name}?`)) {
    setStatus("Delete cancelled.");
    return;
  }
  setStatus("Deleting…");
  try {
    await deleteProfile(name);
    loaded = null;
    await refreshProfiles("");
    syncSavedWorkbench();
    setStatus(`Deleted ${name}.`, "ok");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

async function onTestSaved(): Promise<void> {
  if (!loaded) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  setStatus("Testing…");
  try {
    await testConnection(loaded.engine, loaded.connection);
    setStatus("Connection succeeded.", "ok");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

async function onTestNew(): Promise<void> {
  setStatus("Testing…", "muted", "status-new");
  try {
    await testConnection(inputValue("newEngine") || "postgres", readNewConnection());
    setStatus("Connection succeeded.", "ok", "status-new");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad", "status-new");
  }
}

async function onExport(): Promise<void> {
  if (!loaded) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  const format = inputValue("format") || (loaded.engine === "mysql" ? "sql" : "custom");
  setStatus("Exporting…");
  try {
    const result = await exportDump(loaded.engine, loaded.connection, format, "");
    setStatus(`Wrote ${result.path} (${result.bytes} bytes, ${result.format}).`, "ok");
    await refreshDumps();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(message, /cancelled/i.test(message) ? "muted" : "bad");
  }
}

async function onImport(): Promise<void> {
  if (!loaded) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  const format = inputValue("format") || "";
  let fileName = inputValue("dumpFile");
  const dropExisting = Boolean(document.querySelector<HTMLInputElement>("#dropExisting")?.checked);
  if (runtimeMode() === "desktop" && !fileName) {
    fileName = await pickOpenPath();
    if (!fileName) {
      setStatus("Import cancelled.");
      return;
    }
  }
  if (runtimeMode() === "web" && !fileName) {
    setStatus("Select a dump file first.", "bad");
    return;
  }
  if (!window.confirm("Import will write into the selected database. Continue?")) {
    setStatus("Import cancelled.");
    return;
  }
  setStatus("Importing…");
  try {
    await importDump(loaded.engine, loaded.connection, format, fileName, dropExisting);
    setStatus("Import finished.", "ok");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(message, /cancelled/i.test(message) ? "muted" : "bad");
  }
}

async function onOpenBackups(): Promise<void> {
  try {
    await openBackupsDir();
    setStatus("Opened backups folder.");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

function onDownload(): void {
  const fileName = inputValue("dumpFile");
  if (!fileName) {
    setStatus("Select a dump file first.", "bad");
    return;
  }
  window.location.href = dumpDownloadURL(fileName);
}

async function render(): Promise<void> {
  try {
    const [health, loadedEngines, defaults] = await Promise.all([getHealth(), getEngines(), getDefaults()]);
    engines = loadedEngines;
    const dumps = await listDumps();
    let profiles: ProfileSummary[] = [];
    let profileError = "";
    try {
      profiles = await listProfiles();
    } catch (error) {
      profileError = error instanceof Error ? error.message : String(error);
    }
    const initial = defaults?.engine || engines[0]?.name || "postgres";
    const conn = defaults?.connection;
    const port = conn?.port || engineInfo(initial)?.defaultPort || 5432;
    const ssl = conn?.sslMode || "prefer";
    const backupsNote =
      runtimeMode() === "desktop" && health.backupsDir
        ? `<p class="port">Local dumps live in ${escapeHtml(health.backupsDir)}. Export opens a save dialog; import uses the list or a file picker.</p>`
        : "";

    root.innerHTML = `
      <header class="top">
        <h1>${escapeHtml(health.name ?? "just-db")}</h1>
        <div class="meta">${escapeHtml(health.version)} · ${escapeHtml(health.mode)}${runtimeMode() === "web" && health.backupsDir ? ` · ${escapeHtml(health.backupsDir)}` : ""}</div>
      </header>
      <div class="tabs" role="tablist" aria-label="just-db">
        <button type="button" role="tab" id="tab-saved" data-tab="saved" aria-controls="panel-saved" aria-selected="true" class="active">Saved connections</button>
        <button type="button" role="tab" id="tab-new" data-tab="new" aria-controls="panel-new" aria-selected="false">Save new connection</button>
        <button type="button" role="tab" id="tab-about" data-tab="about" aria-controls="panel-about" aria-selected="false">About</button>
      </div>
      <section class="card workbench" id="panel-saved" data-panel="saved" role="tabpanel" aria-labelledby="tab-saved">
        <h2>Saved connections</h2>
        <div id="profileList">${profileListHtml(profiles, "")}</div>
        <p id="savedHint" class="empty"${profiles.length === 0 ? " hidden" : ""}>Pick a connection to test, export, or import.</p>
        <div id="savedWorkbench" hidden>
          <h3 id="savedTitle"></h3>
          <div class="form-grid">
            <label>Format
              <select id="format">${formatOptions(initial)}</select>
            </label>
            <label>Existing dump
              <select id="dumpFile">${dumpOptions(dumps)}</select>
            </label>
          </div>
          ${backupsNote}
          <label class="check"><input id="dropExisting" type="checkbox" /> Drop existing objects on import</label>
          <div class="actions">
            <button type="button" id="btnTest">Test connection</button>
            <button type="button" id="btnExport">Export</button>
            <button type="button" id="btnImport">Import</button>
            ${runtimeMode() === "web" ? `<button type="button" id="btnDownload">Download dump</button>` : `<button type="button" class="ghost" id="btnOpenBackups">Open backups folder</button>`}
            <button type="button" class="ghost" id="btnDeleteProfile">Delete connection</button>
          </div>
        </div>
        <p id="status-saved" class="status muted">Ready.</p>
      </section>
      <section class="card workbench" id="panel-new" data-panel="new" role="tabpanel" aria-labelledby="tab-new" hidden>
        <h2>Save new connection</h2>
        <div class="form-grid">
          ${field("newName", "Name", "text", "")}
          ${engineSelect("newEngine", initial)}
          ${field("newHost", "Host", "text", conn?.host || "127.0.0.1")}
          ${field("newPort", "Port", "number", port)}
          ${field("newUser", "User", "text", conn?.user || "")}
          ${field("newPassword", "Password", "password", conn?.password || "")}
          ${field("newDatabase", "Database", "text", conn?.database || "")}
          ${sslSelect("newSslMode", ssl)}
        </div>
        <div class="actions">
          <button type="button" id="btnTestNew">Test connection</button>
          <button type="button" id="btnSaveProfile">Save connection</button>
        </div>
        <p id="status-new" class="status muted">Ready.</p>
      </section>
      <section id="panel-about" data-panel="about" role="tabpanel" aria-labelledby="tab-about" hidden>
        <p class="lede">
          Same-engine import and export for PostgreSQL and MySQL / MariaDB.
          Passwords are passed to client tools through a defaults file or environment, never on the command line.
        </p>
        <section class="grid">
          ${engines.map(engineCard).join("")}
        </section>
      </section>
    `;

    document.querySelectorAll<HTMLButtonElement>("[data-tab]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const tab = btn.dataset.tab;
        if (tab === "saved" || tab === "new" || tab === "about") {
          showTab(tab);
        }
      });
    });
    document.querySelector("#profileList")?.addEventListener("click", (event) => {
      const target = (event.target as HTMLElement).closest<HTMLButtonElement>("button[data-name]");
      if (target?.dataset.name) {
        void selectSavedProfile(target.dataset.name);
      }
    });
    document.querySelector("#newEngine")?.addEventListener("change", onNewEngineChange);
    document.querySelector("#btnSaveProfile")?.addEventListener("click", () => {
      void onSaveProfile();
    });
    document.querySelector("#btnDeleteProfile")?.addEventListener("click", () => {
      void onDeleteProfile();
    });
    document.querySelector("#btnTest")?.addEventListener("click", () => {
      void onTestSaved();
    });
    document.querySelector("#btnTestNew")?.addEventListener("click", () => {
      void onTestNew();
    });
    document.querySelector("#btnExport")?.addEventListener("click", () => {
      void onExport();
    });
    document.querySelector("#btnImport")?.addEventListener("click", () => {
      void onImport();
    });
    document.querySelector("#btnDownload")?.addEventListener("click", onDownload);
    document.querySelector("#btnOpenBackups")?.addEventListener("click", () => {
      void onOpenBackups();
    });
    window.addEventListener("hashchange", () => {
      showTab(tabFromHash());
    });
    showTab(tabFromHash());
    if (profileError) {
      setStatus(profileError, "bad");
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    root.innerHTML = `<p class="error">${escapeHtml(message)}</p>`;
  }
}

void render();
