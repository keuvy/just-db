import "./style.css";
import {
  exportDump,
  getEngines,
  getHealth,
  importDump,
  listDumps,
  runtimeMode,
  testConnection,
  type Connection,
  type DumpFile,
  type EngineInfo,
  type Tool,
} from "./api";

const el = document.querySelector<HTMLDivElement>("#app");
if (!el) {
  throw new Error("#app missing");
}
const root: HTMLDivElement = el;
root.innerHTML = `<p class="lede">Loading…</p>`;

const defaultConn: Connection = {
  host: "127.0.0.1",
  port: 5432,
  user: "",
  password: "",
  database: "",
  sslMode: "prefer",
};

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

function dumpOptions(dumps: DumpFile[]): string {
  if (dumps.length === 0) {
    return `<option value="">No dumps in data/backups yet</option>`;
  }
  return dumps
    .map((dump) => `<option value="${escapeHtml(dump.name)}">${escapeHtml(dump.name)} (${dump.size} bytes)</option>`)
    .join("");
}

function readConnection(): Connection {
  return {
    host: inputValue("host"),
    port: Number(inputValue("port") || "5432"),
    user: inputValue("user"),
    password: inputValue("password"),
    database: inputValue("database"),
    sslMode: inputValue("sslMode") || "prefer",
  };
}

function inputValue(id: string): string {
  const node = document.querySelector<HTMLInputElement | HTMLSelectElement>(`#${id}`);
  return node?.value ?? "";
}

function setStatus(message: string, kind: "ok" | "bad" | "muted" = "muted"): void {
  const node = document.querySelector<HTMLParagraphElement>("#status");
  if (!node) {
    return;
  }
  node.className = `status ${kind}`;
  node.textContent = message;
}

async function refreshDumps(): Promise<void> {
  const select = document.querySelector<HTMLSelectElement>("#dumpFile");
  if (!select || runtimeMode() === "desktop") {
    return;
  }
  const dumps = await listDumps();
  select.innerHTML = dumpOptions(dumps);
}

async function render(): Promise<void> {
  try {
    const [health, engines] = await Promise.all([getHealth(), getEngines()]);
    const dumps = runtimeMode() === "web" ? await listDumps() : [];
    const postgres = engines.find((item) => item.name === "postgres");
    const port = postgres?.defaultPort ?? defaultConn.port;
    root.innerHTML = `
      <header class="top">
        <h1>${escapeHtml(health.name ?? "just-db")}</h1>
        <div class="meta">${escapeHtml(health.version)} · ${escapeHtml(health.mode)} · ${runtimeMode()}</div>
      </header>
      <p class="lede">
        Same-engine import and export. PostgreSQL dump/restore is live; MySQL follows in the next phase.
      </p>
      <section class="grid">
        ${engines.map(engineCard).join("")}
      </section>
      <section class="card workbench">
        <h2>PostgreSQL</h2>
        <p class="port">Passwords are passed to client tools through the environment, never on the command line.</p>
        <div class="form-grid">
          ${field("host", "Host", "text", defaultConn.host)}
          ${field("port", "Port", "number", port)}
          ${field("user", "User", "text", "")}
          ${field("password", "Password", "password", "")}
          ${field("database", "Database", "text", "")}
          <label>SSL mode
            <select id="sslMode">
              <option value="prefer">prefer</option>
              <option value="disable">disable</option>
              <option value="require">require</option>
            </select>
          </label>
          <label>Format
            <select id="format">
              <option value="custom">custom (.dump)</option>
              <option value="sql">sql (.sql)</option>
            </select>
          </label>
          ${
            runtimeMode() === "web"
              ? `<label>Existing dump<select id="dumpFile">${dumpOptions(dumps)}</select></label>`
              : `<p class="port">Desktop uses native file dialogs for dump files.</p>`
          }
        </div>
        <label class="check"><input id="dropExisting" type="checkbox" /> Drop existing objects on import</label>
        <div class="actions">
          <button type="button" id="btnTest">Test connection</button>
          <button type="button" id="btnExport">Export</button>
          <button type="button" id="btnImport">Import</button>
        </div>
        <p id="status" class="status muted">Ready.</p>
      </section>
    `;
    document.querySelector("#btnTest")?.addEventListener("click", onTest);
    document.querySelector("#btnExport")?.addEventListener("click", onExport);
    document.querySelector("#btnImport")?.addEventListener("click", onImport);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    root.innerHTML = `<p class="error">${escapeHtml(message)}</p>`;
  }
}

async function onTest(): Promise<void> {
  setStatus("Testing…");
  try {
    await testConnection("postgres", readConnection());
    setStatus("Connection succeeded.", "ok");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

async function onExport(): Promise<void> {
  const connection = readConnection();
  const format = inputValue("format") || "custom";
  setStatus("Exporting…");
  try {
    const result = await exportDump("postgres", connection, format, "");
    setStatus(`Wrote ${result.path} (${result.bytes} bytes, ${result.format}).`, "ok");
    await refreshDumps();
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

async function onImport(): Promise<void> {
  const connection = readConnection();
  const format = inputValue("format") || "";
  const fileName = inputValue("dumpFile");
  const dropExisting = Boolean(document.querySelector<HTMLInputElement>("#dropExisting")?.checked);
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
    await importDump("postgres", connection, format, fileName, dropExisting);
    setStatus("Import finished.", "ok");
  } catch (error) {
    setStatus(error instanceof Error ? error.message : String(error), "bad");
  }
}

void render();
