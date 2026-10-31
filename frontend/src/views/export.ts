import type { Session, Workspace } from "../app/state";
import { chosenTables, exportProblem, filteredTables, requiredTool, type Parts } from "../app/state";
import type { Operations } from "../app/operations";
import { escape, icon, notice } from "../components/html";
import { operationResult } from "../components/shell";

export function exportView(state: Workspace, operations: Operations): string {
  const profile = state.profile!;
  const session = state.session!;
  const missing = requiredTool(state.engines.data, profile.engine, "export");
  const result = operations.records.find(record => record.profileName === profile.name && record.kind === "export");
  const storage = state.health.data?.backupsDir || state.defaults?.backupsDir;
  const mysql = profile.engine === "mysql";
  return `<div class="section-heading"><span class="section-icon">${icon("export")}</span><div><h2>Export dump</h2><p>Create a dump of the selected database.</p></div></div>
    <fieldset class="format-fieldset"><legend>File format</legend>${mysql ? '<div class="format-static"><span class="file-extension">SQL</span><div><strong>SQL file</strong><p>The supported format for MySQL / MariaDB.</p></div></div>' : `<div class="format-options"><label class="format-option ${session.format === "custom" ? "selected" : ""}"><input type="radio" id="export-format-custom" name="export-format" value="custom" ${session.format === "custom" ? "checked" : ""}/><span><strong>Custom <span class="recommended">Recommended</span></strong><span>PostgreSQL archive <code>.dump</code></span></span></label><label class="format-option ${session.format === "sql" ? "selected" : ""}"><input type="radio" id="export-format-sql" name="export-format" value="sql" ${session.format === "sql" ? "checked" : ""}/><span><strong>SQL</strong><span>Plain SQL statements <code>.sql</code></span></span></label></div>`}</fieldset>
    <fieldset class="format-fieldset"><legend>Tables</legend><div class="format-options">${choice("radio", "export-tables-all", "all", session.tableMode === "all", "All tables", "Every table and view in the database.")}${choice("radio", "export-tables-selected", "selected", session.tableMode === "selected", "Choose tables", "Pick structure and data for each table.")}</div>${session.tableMode === "selected" ? tablePicker(session, mysql) : ""}</fieldset>
    ${session.tableMode === "all" ? `<fieldset class="format-fieldset"><legend>Include</legend><div class="format-options">${choice("checkbox", "export-schema", "", session.schema, "Table structure", "Statements that create tables, views, and indexes.")}${choice("checkbox", "export-data", "", session.data, "Table data", mysql ? "Rows as INSERT statements." : "Rows as COPY statements.")}</div></fieldset>` : ""}
    <div class="storage-note">${icon("folder")}<div><strong>Saved in your dump library</strong><p>${state.api.runtimeMode() === "desktop" ? "Then choose where to save a copy on your computer." : "Then downloaded to your computer. The original stays on the server."}</p>${storage ? `<code title="${escape(storage)}">${escape(storage)}</code>` : ""}</div></div>
    ${missing ? `${notice({ kind: "warning", text: missing })}<button class="text-button" data-action="navigate" data-view="settings">View tools ${icon("chevron")}</button>` : ""}
    <div class="operation-footer" id="export-footer">${exportFooter(state, operations)}</div>
    ${result ? `<div class="latest-result" aria-live="polite">${operationResult(result, state.api.runtimeMode() === "desktop")}</div>` : ""}`;
}

function choice(type: "checkbox" | "radio", id: string, value: string, checked: boolean, title: string, description: string): string {
  const name = type === "radio" ? ' name="export-table-mode"' : "";
  return `<label class="format-option ${checked ? "selected" : ""}"><input type="${type}" id="${id}"${name} value="${value}" ${checked ? "checked" : ""}/><span><strong>${title}</strong><span>${description}</span></span></label>`;
}

export function exportFooter(state: Workspace, operations: Operations): string {
  const session = state.session!;
  const missing = requiredTool(state.engines.data, state.profile!.engine, "export");
  const problem = exportProblem(session);
  return `<span class="field-help" id="export-help">${operations.active ? "Another operation is running. Its result stays in Activity." : problem || "The source database is read during export."}</span><button id="export-dump" class="primary" data-action="export" ${problem || operations.active || missing ? "disabled" : ""}>${icon("export")} Export dump</button>`;
}

function tablePicker(session: Session, mysql: boolean): string {
  const database = session.database.trim();
  const loaded = session.tablesFor === database;
  let body: string;
  if (!database) body = '<p class="field-help">Choose a database to list its tables.</p>';
  else if (session.tables.loading && !loaded) body = '<p class="loading-line"><span class="spinner"></span> Loading tables...</p>';
  else if (session.tables.error && !loaded) body = `${notice({ kind: "error", text: session.tables.error })}<button class="small" data-action="load-tables">${icon("refresh")} Retry</button>`;
  else if (!loaded) body = `<button class="small" data-action="load-tables">${icon("refresh")} Load tables</button>`;
  else if (!session.tables.data.length) body = '<p class="field-help">This database has no tables.</p>';
  else body = `<div class="table-picker-bar"><div class="search-field">${icon("search")}<label for="table-filter" class="sr-only">Filter tables</label><input id="table-filter" type="search" value="${escape(session.tableFilter)}" placeholder="Filter tables" autocomplete="off" spellcheck="false" aria-controls="table-rows"/></div><button class="icon-button" data-action="load-tables" aria-label="Refresh table list" ${session.tables.loading ? "disabled" : ""}>${icon("refresh", session.tables.loading ? "spin" : "")}</button></div>
    <div class="table-summary" id="table-summary">${tableSummary(session)}</div>
    <div class="table-rows" id="table-rows" role="group" aria-label="Tables to export">${tableRows(session)}</div>
    ${session.tables.error ? notice({ kind: "warning", text: `Refresh failed: ${session.tables.error}` }) : ""}`;
  const note = `<p class="field-help">Tables with data only are restored into tables that already exist. ${mysql ? "Each group of choices is dumped separately, so mixed choices are not one snapshot." : "Types, functions, and other objects the chosen tables depend on are not included."}</p>`;
  return `<div class="table-picker">${body}</div>${loaded && session.tables.data.length ? note : ""}`;
}

// The header row: a count, and boxes that set a column for every shown table.
export function tableSummary(session: Session): string {
  const shown = filteredTables(session);
  const column = (part: keyof Parts, title: string) => {
    const eligible = shown.filter(table => part === "schema" || !table.view);
    const on = eligible.filter(table => session.tableParts.get(table.name)?.[part]).length;
    const scope = session.tableFilter.trim() ? "shown tables" : "all tables";
    return `<label class="table-part"><span>${title}</span><input type="checkbox" id="table-column-${part}" data-column="${part}" aria-label="${title} for ${scope}" ${eligible.length && on === eligible.length ? "checked" : ""} ${on && on < eligible.length ? "data-mixed" : ""} ${eligible.length ? "" : "disabled"}/></label>`;
  };
  return `<span class="table-count" aria-live="polite">${chosenTables(session).length} of ${session.tables.data.length} tables</span>${column("schema", "Structure")}${column("data", "Data")}`;
}

export function tableRows(session: Session): string {
  const shown = filteredTables(session);
  if (!shown.length) return `<p class="field-help">No tables match “${escape(session.tableFilter.trim())}”.</p>`;
  const index = new Map(session.tables.data.map((table, position) => [table.name, position]));
  const box = (name: string, part: keyof Parts, title: string) => `<label class="table-part"><input type="checkbox" id="export-table-${index.get(name)}-${part}" data-table="${escape(name)}" data-part="${part}" aria-label="${title} of ${escape(name)}" ${session.tableParts.get(name)?.[part] ? "checked" : ""}/></label>`;
  return shown.map(table => `<div class="table-row"><span class="table-name"><span class="mono">${escape(table.name)}</span>${table.view ? '<span class="table-kind">view</span>' : ""}</span>${box(table.name, "schema", "Structure")}${table.view ? '<span class="table-part" title="Views have no rows"></span>' : box(table.name, "data", "Data")}</div>`).join("");
}
