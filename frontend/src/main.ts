import "./style.css";
import { getEngines, getHealth, runtimeMode, type EngineInfo, type Tool } from "./api";

const el = document.querySelector<HTMLDivElement>("#app");
if (!el) {
  throw new Error("#app missing");
}
const root: HTMLDivElement = el;

root.innerHTML = `<p class="lede">Loading…</p>`;

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

async function render(): Promise<void> {
  try {
    const [health, engines] = await Promise.all([getHealth(), getEngines()]);
    root.innerHTML = `
      <header class="top">
        <h1>${escapeHtml(health.name ?? "just-db")}</h1>
        <div class="meta">${escapeHtml(health.version)} · ${escapeHtml(health.mode)} · ${runtimeMode()}</div>
      </header>
      <p class="lede">
        Same-engine import and export for PostgreSQL and MySQL.
        Dump and restore land in phase 1; this build detects client tools on the machine.
      </p>
      <section class="grid">
        ${engines.map(engineCard).join("")}
      </section>
    `;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    root.innerHTML = `<p class="error">${escapeHtml(message)}</p>`;
  }
}

void render();
