import type { Workspace } from "../app/state";
import { icon, notice } from "./html";
import { customSelect } from "./select";

export function databasePicker(state: Workspace): string {
  const session = state.session;
  if (!session) return "";
  return `<div class="database-picker"><div class="database-picker-top"><label for="workspace-database">Database <span class="required-label">Required</span></label><button class="text-button" id="refresh-databases" data-action="refresh-databases" ${session.databases.loading ? "disabled" : ""}>${icon("refresh", session.databases.loading ? "spin" : "")} ${session.databases.loading ? "Loading" : "Refresh list"}</button></div>${customSelect({ id: "workspace-database", label: "Database", value: session.database, editable: true, icon: "database", placeholder: "Search or enter a database name", describedBy: "database-help", options: session.databases.data.map(name => ({ value: name, label: name })), emptyMessage: session.databases.loading ? "Loading databases... You can still enter a name." : undefined })}<p id="database-help" class="field-help">${session.databases.loading ? "Looking up databases on this profile..." : session.databases.error ? "Database lookup failed. You can still enter a name manually." : session.databases.data.length ? `${session.databases.data.length} databases available. You can also enter a name.` : "No databases listed. Enter a database name to continue."}</p>${session.databases.error ? notice({ kind: "warning", text: session.databases.error }) : ""}</div>`;
}
