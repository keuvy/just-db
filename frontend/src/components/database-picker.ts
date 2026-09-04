import type { Workspace } from "../app/state";
import { escape, icon, notice } from "./html";

export function databasePicker(state: Workspace): string {
  const session = state.session;
  if (!session) return "";
  return `<div class="database-picker"><div class="database-picker-top"><label for="workspace-database">Database <span class="required-label">Required</span></label><button class="text-button" id="refresh-databases" data-action="refresh-databases" ${session.databases.loading ? "disabled" : ""}>${icon("refresh", session.databases.loading ? "spin" : "")} ${session.databases.loading ? "Loading" : "Refresh list"}</button></div><div class="input-with-icon">${icon("database")}<input id="workspace-database" list="workspace-databases" value="${escape(session.database)}" placeholder="Search or enter a database name" autocomplete="off" aria-describedby="database-help"/></div><datalist id="workspace-databases">${session.databases.data.map(name => `<option value="${escape(name)}"></option>`).join("")}</datalist><p id="database-help" class="field-help">${session.databases.loading ? "Looking up databases on this profile..." : session.databases.error ? "Database lookup failed. You can still enter a name manually." : session.databases.data.length ? `${session.databases.data.length} databases available. You can also enter a name.` : "No databases listed. Enter a database name to continue."}</p>${session.databases.error ? notice({ kind: "warning", text: session.databases.error }) : ""}</div>`;
}
