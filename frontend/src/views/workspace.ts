import type { Workspace } from "../app/state";
import type { Operations } from "../app/operations";
import { databasePicker } from "../components/database-picker";
import { emptyState, icon, notice } from "../components/html";
import { exportView } from "./export";
import { restoreView } from "./restore";

export function workspaceView(state: Workspace, operations: Operations): string {
  if (state.mode === "export" && !state.profile) {
    if (state.profileLoading) return '<div class="workspace-loading"><span class="spinner"></span><p>Loading profile...</p></div>';
    if (state.profileError) return `${notice({ kind: "error", text: state.profileError })}<button data-action="retry-profile">Retry profile</button>`;
    return `<div class="welcome"><span class="eyebrow">Your database workspace</span>${emptyState(state.profiles.data.length ? "Choose a profile to get started" : "Your next dump starts here", state.profiles.data.length ? "Select a profile from the sidebar, then choose a database to export or restore." : "Add a PostgreSQL or MySQL profile to export a database or restore a dump.", `<button class="primary" data-action="new-profile">${icon("plus")} ${state.profiles.data.length ? "Add profile" : "Add your first profile"}</button>`)}</div><div class="welcome-capabilities"><div>${icon("export")}<strong>Export a database</strong><p>Create custom PostgreSQL archives or plain SQL dumps.</p></div><div>${icon("restore")}<strong>Restore with context</strong><p>Review the source, host, and target database before restoring.</p></div></div>${state.profiles.error ? notice({ kind: "error", text: state.profiles.error }) : ""}`;
  }
  return `<div class="workbench">${state.session?.test ? notice(state.session.test) : ""}<div class="operation-tabs" role="tablist" aria-label="Database operation"><button id="tab-export" role="tab" aria-selected="${state.mode === "export"}" aria-controls="operation-panel" tabindex="${state.mode === "export" ? 0 : -1}" data-action="operation-mode" data-mode="export">${icon("export")} Export</button><button id="tab-restore" role="tab" aria-selected="${state.mode === "restore"}" aria-controls="operation-panel" tabindex="${state.mode === "restore" ? 0 : -1}" data-action="operation-mode" data-mode="restore">${icon("restore")} Restore</button></div><div id="operation-panel" role="tabpanel" aria-labelledby="tab-${state.mode}">${state.mode === "export" ? `${databasePicker(state)}<div id="export-content" class="operation-content">${exportView(state, operations)}</div>` : `<div id="restore-content" class="operation-content">${restoreView(state, operations)}</div>`}</div></div>`;
}
