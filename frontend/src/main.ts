import "./style.css";
import {
  deleteDump,
  deleteProfile,
  downloadDump,
  exportDump,
  getDefaults,
  getEngines,
  getHealth,
  getProfile,
  importDump,
  listDatabases,
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

type Tab = "saved" | "new" | "dumps" | "about";

let engines: EngineInfo[] = [];
let loaded: Profile | null = null;
let knownProfiles: ProfileSummary[] = [];
let importFilePath = "";
let importFile: File | null = null;

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

function databaseField(id: string, listId: string, value: string): string {
  return `<label>Database
    <input id="${id}" list="${listId}" type="text" value="${escapeHtml(value)}" placeholder="optional" autocomplete="off" />
    <datalist id="${listId}"></datalist>
  </label>`;
}

function fillDatalist(listId: string, names: string[]): void {
  const list = document.querySelector(`#${listId}`);
  if (!list) {
    return;
  }
  list.innerHTML = names.map((name) => `<option value="${escapeHtml(name)}"></option>`).join("");
}

async function loadDatabaseNames(engine: string, connection: Connection, listId: string, statusId: string): Promise<string[]> {
  const names = await listDatabases(engine, { ...connection, database: "" });
  fillDatalist(listId, names);
  const count = names.length;
  const msg = count === 1 ? "1 database." : `${count} databases.`;
  setStatus(msg, "ok", statusId);
  return names;
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

function dumpListHtml(dumps: DumpFile[]): string {
  if (dumps.length === 0) {
    return `<p class="empty">No dumps yet. Export a database from Saved connections to create one.</p>`;
  }
  return `<ul class="dump-list">${dumps.map((dump) => {
    const name = escapeHtml(dump.name);
    const date = new Date(dump.modTime);
    const modified = Number.isNaN(date.getTime()) ? dump.modTime : date.toLocaleString();
    return `<li class="dump-item">
      <div class="dump-detail">
        <span class="dump-name">${name}</span>
        <span class="meta">${dump.size.toLocaleString()} bytes · ${escapeHtml(modified)}</span>
      </div>
      <div class="actions">
        <button type="button" data-dump-action="download" data-name="${name}" aria-label="Download ${name}">Download</button>
        <button type="button" class="ghost danger" data-dump-action="delete" data-name="${name}" aria-label="Delete ${name}">Delete</button>
      </div>
    </li>`;
  }).join("")}</ul>`;
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
      const target = item.database
        ? `${item.user}@${item.host}/${item.database}`
        : `${item.user}@${item.host}`;
      const detail = `${item.engine} · ${target}`;
      return `<li>
        <button type="button" class="profile-item${isSelected}" data-name="${escapeHtml(item.name)}">
          <span class="profile-item-name">${escapeHtml(item.name)}</span>
          <span class="profile-item-meta">${escapeHtml(detail)}</span>
        </button>
      </li>`;
    })
    .join("")}</ul>`;
}

function takenNames(): string[] {
  return knownProfiles.map((item) => item.name);
}

function uniqueProfileName(base: string, taken: string[]): string {
  const trimmed = base.trim() || "connection";
  const set = new Set(taken);
  if (!set.has(trimmed)) {
    return trimmed;
  }
  for (let n = 2; n < 1000; n++) {
    const candidate = `${trimmed}-${n}`;
    if (!set.has(candidate)) {
      return candidate;
    }
  }
  return `${trimmed}-${Date.now()}`;
}

function suggestedNewName(): string {
  const engine = inputValue("newEngine") || "postgres";
  const host = (inputValue("newHost") || "127.0.0.1").replace(/[^A-Za-z0-9._-]+/g, "-");
  return uniqueProfileName(`${engine}-${host}`, takenNames());
}

function fillSuggestedName(force: boolean): void {
  const node = document.querySelector<HTMLInputElement>("#newName");
  if (!node) {
    return;
  }
  if (!force && node.value.trim() !== "") {
    return;
  }
  node.value = suggestedNewName();
}

function savedCountLabel(n: number): string {
  if (n === 0) {
    return "";
  }
  if (n === 1) {
    return "1 saved";
  }
  return `${n} saved`;
}

function updateSavedCount(): void {
  const node = document.querySelector("#savedCount");
  if (node) {
    node.textContent = savedCountLabel(knownProfiles.length);
  }
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
  if (raw === "new" || raw === "about" || raw === "saved" || raw === "dumps") {
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
  if (tab === "new") {
    fillSuggestedName(false);
  }
  if (tab === "dumps") {
    void refreshDumps();
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

async function refreshDumps(): Promise<boolean> {
  try {
    const dumps = (await listDumps()).sort((a, b) => b.modTime.localeCompare(a.modTime) || b.name.localeCompare(a.name));
    const mount = document.querySelector("#dumpList");
    if (mount) {
      mount.innerHTML = dumpListHtml(dumps);
    }
    setStatus("", "muted", "status-dumps");
    return true;
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad", "status-dumps");
    return false;
  }
}

async function refreshProfiles(selected: string): Promise<void> {
  const mount = document.querySelector("#profileList");
  if (!mount) {
    return;
  }
  const profiles = await listProfiles();
  knownProfiles = profiles;
  mount.innerHTML = profileListHtml(profiles, selected);
  updateSavedCount();
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
  const db = document.querySelector<HTMLInputElement>("#savedDatabase");
  if (db) {
    db.value = loaded.connection.database || "";
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
    try {
      await loadDatabaseNames(loaded.engine, loaded.connection, "savedDatabaseList", "status-saved");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : String(error), "bad");
    }
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
  if (takenNames().includes(name)) {
    if (!window.confirm(`A connection named ${name} already exists. Replace it? Cancel and pick a new name to keep both.`)) {
      setStatus("Save cancelled. Use a different name to keep both connections.", "muted", "status-new");
      return;
    }
  }
  setStatus("Saving…", "muted", "status-new");
  try {
    const engine = inputValue("newEngine") || "postgres";
    await putProfile(name, engine, readNewConnection());
    setStatus(`Saved ${name}.`, "ok", "status-new");
    await selectSavedProfile(name);
    fillSuggestedName(true);
    showTab("saved");
    setStatus(`Saved ${name}. ${knownProfiles.length} stored.`, "ok");
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

async function onListSavedDatabases(): Promise<void> {
  if (!loaded) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  setStatus("Listing databases…");
  try {
    await loadDatabaseNames(loaded.engine, loaded.connection, "savedDatabaseList", "status-saved");
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
    try {
      await loadDatabaseNames(loaded.engine, loaded.connection, "savedDatabaseList", "status-saved");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : String(error), "bad");
    }
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

async function onTestNew(): Promise<void> {
  setStatus("Testing…", "muted", "status-new");
  try {
    await testConnection(inputValue("newEngine") || "postgres", readNewConnection());
    setStatus("Connection succeeded.", "ok", "status-new");
    try {
      await loadDatabaseNames(inputValue("newEngine") || "postgres", readNewConnection(), "newDatabaseList", "status-new");
    } catch (error) {
      setStatus(error instanceof Error ? error.message : String(error), "bad", "status-new");
    }
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad", "status-new");
  }
}

async function onExport(): Promise<void> {
  const button = document.querySelector<HTMLButtonElement>("#btnExport");
  if (button?.disabled) {
    return;
  }
  if (!loaded) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  loaded.connection.database = inputValue("savedDatabase").trim();
  if (!loaded.connection.database) {
    setStatus("Pick a database first.", "bad");
    return;
  }
  const format = inputValue("format") || (loaded.engine === "mysql" ? "sql" : "custom");
  if (button) {
    button.disabled = true;
  }
  setStatus("Exporting…");
  try {
    const result = await exportDump(loaded.engine, loaded.connection, format, "");
    const name = result.path.split(/[\\/]/).pop()!;
    try {
      const downloaded = await downloadDump(name);
      const message = downloaded
        ? runtimeMode() === "web" ? "Download started." : "Copy saved."
        : "Save cancelled. The dump is available in Dumps.";
      setStatus(`Exported ${name}. ${message}`, "ok");
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Exported ${name}, but download failed: ${message}. Try again from Dumps.`, "bad");
    }
    await refreshDumps();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(message, /cancelled/i.test(message) ? "muted" : "bad");
  } finally {
    if (button) {
      button.disabled = false;
    }
  }
}

function resetImportFile(): void {
  importFilePath = "";
  importFile = null;
  const input = document.querySelector<HTMLInputElement>("#importFile");
  if (input) {
    input.value = "";
  }
  const chosen = document.querySelector("#importFileChosen");
  if (chosen) {
    chosen.textContent = "No file selected";
  }
}

function setImportFileChosen(name: string): void {
  const chosen = document.querySelector("#importFileChosen");
  if (chosen) {
    chosen.textContent = name || "No file selected";
  }
}

async function onPickImportFile(): Promise<void> {
  if (runtimeMode() === "desktop") {
    const path = await pickOpenPath();
    if (!path) {
      return;
    }
    importFilePath = path;
    importFile = null;
    setImportFileChosen(path.split(/[\\/]/).pop() || path);
    return;
  }
  document.querySelector<HTMLInputElement>("#importFile")?.click();
}

function onImportFileChange(): void {
  const input = document.querySelector<HTMLInputElement>("#importFile");
  const file = input?.files?.[0];
  if (!file) {
    resetImportFile();
    return;
  }
  importFile = file;
  importFilePath = "";
  setImportFileChosen(file.name);
}

async function onShowImport(): Promise<void> {
  if (!loaded || !inputValue("savedDatabase").trim()) {
    setStatus(loaded ? "Pick a database first." : "Pick a connection first.", "bad");
    return;
  }
  resetImportFile();
  setStatus("", "muted", "status-import");
  document.querySelector<HTMLDialogElement>("#importDialog")?.showModal();
}

async function onImport(): Promise<void> {
  if (!loaded) {
    setStatus("Pick a connection first.", "bad");
    return;
  }
  loaded.connection.database = inputValue("savedDatabase").trim();
  if (!loaded.connection.database) {
    setStatus("Pick a database first.", "bad");
    return;
  }
  const source = importFile ?? importFilePath;
  if (!source) {
    setStatus("Choose a file first.", "bad", "status-import");
    return;
  }
  const button = document.querySelector<HTMLButtonElement>("#btnConfirmImport");
  if (button?.disabled) return;
  if (button) button.disabled = true;
  const dialog = document.querySelector<HTMLDialogElement>("#importDialog");
  try {
    const dropExisting = Boolean(document.querySelector<HTMLInputElement>("#dropExisting")?.checked);
    if (!window.confirm("Import will write into the selected database. Continue?")) {
      setStatus("Import cancelled.", "muted", "status-import");
      return;
    }
    dialog?.close();
    setStatus("Importing…");
    await importDump(loaded.engine, loaded.connection, "", source, dropExisting);
    resetImportFile();
    setStatus("Import finished.", "ok");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(message, /cancelled/i.test(message) ? "muted" : "bad", dialog?.open ? "status-import" : "status-saved");
  } finally {
    if (button) button.disabled = false;
  }
}

async function onOpenBackups(): Promise<void> {
  try {
    await openBackupsDir();
    setStatus("Opened backups folder.", "muted", "status-dumps");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad", "status-dumps");
  }
}

async function onDumpAction(button: HTMLButtonElement): Promise<void> {
  const name = button.dataset.name;
  if (!name || button.disabled) {
    return;
  }
  const deleting = button.dataset.dumpAction === "delete";
  if (deleting && !window.confirm(`Delete dump ${name}? This cannot be undone.`)) {
    return;
  }
  button.disabled = true;
  setStatus(deleting ? "Deleting…" : "Downloading…", "muted", "status-dumps");
  try {
    if (deleting) {
      await deleteDump(name);
      button.closest("li")?.remove();
      if (await refreshDumps()) {
        setStatus(`Deleted ${name}.`, "ok", "status-dumps");
      }
    } else {
      const downloaded = await downloadDump(name);
      const message = downloaded
        ? runtimeMode() === "web" ? `Download started for ${name}.` : `Saved ${name}.`
        : "Save cancelled.";
      setStatus(message, downloaded ? "ok" : "muted", "status-dumps");
    }
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad", "status-dumps");
  } finally {
    button.disabled = false;
  }
}

async function render(): Promise<void> {
  try {
    const [health, loadedEngines, defaults] = await Promise.all([getHealth(), getEngines(), getDefaults()]);
    engines = loadedEngines;
    let profiles: ProfileSummary[] = [];
    let profileError = "";
    try {
      profiles = await listProfiles();
    } catch (error) {
      profileError = error instanceof Error ? error.message : String(error);
    }
    knownProfiles = profiles;
    const initial = defaults?.engine || engines[0]?.name || "postgres";
    const conn = defaults?.connection;
    const port = conn?.port || engineInfo(initial)?.defaultPort || 5432;
    const ssl = conn?.sslMode || "prefer";

    root.innerHTML = `
      <header class="top">
        <h1>${escapeHtml(health.name ?? "just-db")}</h1>
        <div class="meta">${escapeHtml(health.version)} · ${escapeHtml(health.mode)}${runtimeMode() === "web" && health.backupsDir ? ` · ${escapeHtml(health.backupsDir)}` : ""}</div>
      </header>
      <div class="tabs" role="tablist" aria-label="just-db">
        <button type="button" role="tab" id="tab-saved" data-tab="saved" aria-controls="panel-saved" aria-selected="true" class="active">Saved connections</button>
        <button type="button" role="tab" id="tab-new" data-tab="new" aria-controls="panel-new" aria-selected="false">Save new connection</button>
        <button type="button" role="tab" id="tab-dumps" data-tab="dumps" aria-controls="panel-dumps" aria-selected="false">Dumps</button>
        <button type="button" role="tab" id="tab-about" data-tab="about" aria-controls="panel-about" aria-selected="false">About</button>
      </div>
      <section class="card workbench" id="panel-saved" data-panel="saved" role="tabpanel" aria-labelledby="tab-saved">
        <div class="saved-head">
          <h2>Saved connections</h2>
          <span id="savedCount" class="count">${escapeHtml(savedCountLabel(profiles.length))}</span>
        </div>
        <div id="profileList">${profileListHtml(profiles, "")}</div>
        <p id="savedHint" class="empty"${profiles.length === 0 ? " hidden" : ""}>Pick a connection to test, export, or import.</p>
        <div id="savedWorkbench" hidden>
          <h3 id="savedTitle"></h3>
          <div class="form-grid">
            ${databaseField("savedDatabase", "savedDatabaseList", "")}
            <div class="profile-actions">
              <button type="button" class="ghost" id="btnListDatabases">List databases</button>
            </div>
            <label>Format
              <select id="format">${formatOptions(initial)}</select>
            </label>
          </div>
          <div class="actions workbench-actions">
            <button type="button" id="btnTest">Test connection</button>
            <button type="button" id="btnExport">Export</button>
            <button type="button" id="btnImport">Import</button>
            <button type="button" class="ghost" id="btnDeleteProfile">Delete connection</button>
          </div>
        </div>
        <p id="status-saved" class="status muted" role="status">Ready.</p>
      </section>
      <section class="card workbench" id="panel-new" data-panel="new" role="tabpanel" aria-labelledby="tab-new" hidden>
        <h2>Save new connection</h2>
        <p class="port">Each connection needs its own name. Saving an existing name replaces that entry.</p>
        <div class="form-grid">
          ${field("newName", "Name", "text", "")}
          ${engineSelect("newEngine", initial)}
          ${field("newHost", "Host", "text", conn?.host || "127.0.0.1")}
          ${field("newPort", "Port", "number", port)}
          ${field("newUser", "User", "text", conn?.user || "")}
          ${field("newPassword", "Password", "password", conn?.password || "")}
          ${databaseField("newDatabase", "newDatabaseList", conn?.database || "")}
          ${sslSelect("newSslMode", ssl)}
        </div>
        <div class="actions">
          <button type="button" id="btnTestNew">Test connection</button>
          <button type="button" id="btnSaveProfile">Save connection</button>
        </div>
        <p id="status-new" class="status muted">Ready.</p>
      </section>
      <section class="card workbench" id="panel-dumps" data-panel="dumps" role="tabpanel" aria-labelledby="tab-dumps" hidden>
        <div class="saved-head">
          <h2>Dumps</h2>
          <div class="actions">
            ${runtimeMode() === "desktop" ? `<button type="button" class="ghost" id="btnOpenBackups">Open backups folder</button>` : ""}
            <button type="button" class="ghost" id="btnRefreshDumps">Refresh</button>
          </div>
        </div>
        <div id="dumpList"><p class="empty">Loading dumps…</p></div>
        <p id="status-dumps" class="status muted" role="status"></p>
      </section>
      <dialog id="importDialog" class="card import-dialog" aria-labelledby="importTitle">
        <h2 id="importTitle">Import file</h2>
        <p class="port">Choose a .sql or .dump file to import into the selected database.</p>
        <div class="file-field">
          <span class="field-caption">File</span>
          <input id="importFile" type="file" accept=".sql,.dump,.backup,.pgdump" hidden />
          <span class="file-pick">
            <button type="button" class="ghost" id="btnPickImportFile">Choose file</button>
            <span id="importFileChosen" class="file-name">No file selected</span>
          </span>
        </div>
        <label class="check"><input id="dropExisting" type="checkbox" /> Drop existing objects on import</label>
        <div class="actions">
          <button type="button" id="btnConfirmImport">Import</button>
          <button type="button" class="ghost" id="btnCancelImport">Cancel</button>
        </div>
        <p id="status-import" class="status muted" role="status"></p>
      </dialog>
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
        if (tab === "saved" || tab === "new" || tab === "about" || tab === "dumps") {
          showTab(tab);
        }
      });
      btn.addEventListener("keydown", (event) => {
        const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>("[data-tab]"));
        const index = tabs.indexOf(btn);
        const next = event.key === "ArrowRight" ? (index + 1) % tabs.length
          : event.key === "ArrowLeft" ? (index + tabs.length - 1) % tabs.length
          : event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : -1;
        if (next < 0) return;
        event.preventDefault();
        tabs[next].click();
        tabs[next].focus();
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
    document.querySelector("#btnListDatabases")?.addEventListener("click", () => {
      void onListSavedDatabases();
    });
    document.querySelector("#btnExport")?.addEventListener("click", () => {
      void onExport();
    });
    document.querySelector("#btnImport")?.addEventListener("click", () => {
      void onShowImport();
    });
    document.querySelector("#btnConfirmImport")?.addEventListener("click", () => {
      void onImport();
    });
    document.querySelector("#btnPickImportFile")?.addEventListener("click", () => {
      void onPickImportFile();
    });
    document.querySelector("#importFile")?.addEventListener("change", onImportFileChange);
    document.querySelector("#btnCancelImport")?.addEventListener("click", () => {
      document.querySelector<HTMLDialogElement>("#importDialog")?.close();
    });
    document.querySelector("#btnRefreshDumps")?.addEventListener("click", () => {
      void refreshDumps();
    });
    document.querySelector("#dumpList")?.addEventListener("click", (event) => {
      const button = (event.target as HTMLElement).closest<HTMLButtonElement>("button[data-dump-action]");
      if (button) void onDumpAction(button);
    });
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
