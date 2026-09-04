import type { Workspace } from "../app/state";
import { requiredTool } from "../app/state";
import type { Operations } from "../app/operations";
import { escape, icon, notice } from "../components/html";
import { operationResult } from "../components/shell";

export function exportView(state: Workspace, operations: Operations): string {
  const profile = state.profile!;
  const session = state.session!;
  const missing = requiredTool(state.engines.data, profile.engine, "export");
  const result = operations.records.find(record => record.profileName === profile.name && record.kind === "export");
  const storage = state.health.data?.backupsDir || state.defaults?.backupsDir;
  return `<div class="section-heading"><span class="section-icon">${icon("export")}</span><div><h2>Export dump</h2><p>Create a dump of the selected database.</p></div></div>
    <fieldset class="format-fieldset"><legend>File format</legend>${profile.engine === "mysql" ? '<div class="format-static"><span class="file-extension">SQL</span><div><strong>SQL file</strong><p>The supported format for MySQL / MariaDB.</p></div></div>' : `<div class="format-options"><label class="format-option ${session.format === "custom" ? "selected" : ""}"><input type="radio" id="export-format-custom" name="export-format" value="custom" ${session.format === "custom" ? "checked" : ""}/><span><strong>Custom <span class="recommended">Recommended</span></strong><span>PostgreSQL archive <code>.dump</code></span></span></label><label class="format-option ${session.format === "sql" ? "selected" : ""}"><input type="radio" id="export-format-sql" name="export-format" value="sql" ${session.format === "sql" ? "checked" : ""}/><span><strong>SQL</strong><span>Plain SQL statements <code>.sql</code></span></span></label></div>`}</fieldset>
    <div class="storage-note">${icon("folder")}<div><strong>Saved in your dump library</strong><p>${state.api.runtimeMode() === "desktop" ? "Then choose where to save a copy on your computer." : "Then downloaded to your computer. The original stays on the server."}</p>${storage ? `<code title="${escape(storage)}">${escape(storage)}</code>` : ""}</div></div>
    ${missing ? `${notice({ kind: "warning", text: missing })}<button class="text-button" data-action="navigate" data-view="settings">View tools ${icon("chevron")}</button>` : ""}
    <div class="operation-footer"><span class="field-help" id="export-help">${operations.active ? "Another operation is running. Its result stays in Activity." : !session.database.trim() ? "Choose a database above to continue." : "The source database is read during export."}</span><button id="export-dump" class="primary" data-action="export" ${!session.database.trim() || operations.active || missing ? "disabled" : ""}>${icon("export")} Export dump</button></div>
    ${result ? `<div class="latest-result" aria-live="polite">${operationResult(result, state.api.runtimeMode() === "desktop")}</div>` : ""}`;
}
