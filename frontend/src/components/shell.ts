import type { Workspace } from "../app/state";
import type { Operations, Operation } from "../app/operations";
import { bytes, duration, emptyState, engineName, escape, icon, notice } from "./html";

export function shell(): string {
  return `<a class="skip-link" href="#main-content">Skip to workspace</a>
    <div class="app-shell"><aside class="sidebar" id="sidebar" aria-label="Profiles and navigation">
      <div class="brand"><span class="brand-mark">${icon("database")}</span><span>just-db<span class="brand-dot">.</span></span><button class="icon-button mobile-only" data-action="close-nav" aria-label="Close navigation">${icon("close")}</button></div>
      <div class="sidebar-search"><label for="profile-search" class="sr-only">Search profiles</label>${icon("search")}<input id="profile-search" type="search" placeholder="Search profiles" autocomplete="off"/></div>
      <div class="sidebar-label"><span>Profiles <span id="profile-count"></span></span><button id="add-profile" class="icon-button" data-action="new-profile" aria-label="Add profile">${icon("plus")}</button></div>
      <div class="profile-list" id="profile-list"></div>
      <nav class="sidebar-nav" aria-label="App"><button id="nav-dumps" data-action="navigate" data-view="dumps">${icon("folder")}<span>Dumps</span>${icon("chevron", "nav-arrow")}</button><button id="nav-settings" data-action="navigate" data-view="settings">${icon("settings")}<span>Tools & settings</span></button></nav>
      <div class="sidebar-footer"><span class="quiet-dot"></span> PostgreSQL & MySQL</div>
    </aside><button class="sidebar-scrim" data-action="close-nav" aria-label="Close navigation" hidden></button>
    <section class="workspace-shell"><header class="workspace-header"><button id="open-nav" class="icon-button mobile-only" data-action="open-nav" aria-label="Open profiles and navigation" aria-expanded="false" aria-controls="sidebar">${icon("menu")}</button><div id="workspace-heading" class="workspace-heading"></div><div id="workspace-actions" class="header-actions"></div></header>
    <main id="main-content" tabindex="-1"><div id="page-notice"></div><div id="page-content"></div></main>
    <section class="activity" aria-label="Session activity"><div id="activity-summary"></div><div id="activity-list" class="activity-list" hidden></div></section></section></div>`;
}

export function profilesView(state: Workspace, search: string): string {
  if (state.profiles.loading && !state.profiles.data.length) return '<div class="list-message"><span class="spinner"></span> Loading profiles...</div>';
  const error = state.profiles.error ? `${notice({ kind: "error", text: state.profiles.error })}<button class="small" data-action="refresh-profiles">Retry profiles</button>` : "";
  if (!state.profiles.data.length) return `${error}<div class="list-message">Your connections live here.<button class="text-button" data-action="new-profile">${icon("plus")} Add your first profile</button></div>`;
  const term = search.trim().toLocaleLowerCase();
  const profiles = state.profiles.data.filter(item => `${item.name} ${item.engine} ${engineName(item.engine)} ${item.host} ${item.database}`.toLocaleLowerCase().includes(term));
  return `${error}${profiles.length ? `<ul>${profiles.map((profile, index) => `<li><button id="profile-row-${index}" class="profile-row ${profile.name === state.selectedName ? "selected" : ""}" data-action="select-profile" data-name="${escape(profile.name)}" ${profile.name === state.selectedName ? 'aria-current="true"' : ""}><span class="engine-symbol ${profile.engine === "mysql" ? "mysql" : ""}" aria-hidden="true">${profile.engine === "postgres" ? "PG" : "MY"}</span><span class="profile-label"><strong>${escape(profile.name)}</strong><span title="${escape(`${engineName(profile.engine)} / ${profile.user}@${profile.host}:${profile.port}`)}">${escape(profile.host)}<span class="muted">:${profile.port}</span></span></span>${profile.name === state.selectedName ? '<span class="selected-dot"></span>' : ""}</button></li>`).join("")}</ul>` : '<div class="list-message">No profiles match your search.</div>'}`;
}

export function heading(state: Workspace): string {
  const title = state.view === "dumps" ? "Dumps" : state.view === "settings" ? "Tools & settings" : state.profile?.name || "Workspace";
  const subtitle = state.view === "dumps" ? "Your saved database files" : state.view === "settings" ? "Tools, storage, and appearance" : state.profile ? `${engineName(state.profile.engine)} · ${state.profile.connection.user}@${state.profile.connection.host}:${state.profile.connection.port}` : "Database export & restore";
  return `<div class="heading-title"><h1>${escape(title)}</h1>${state.view === "workspace" && state.session?.database ? `<span class="target-chip" title="Selected database">${icon("database")}<span>${escape(state.session.database)}</span></span>` : ""}</div><p>${escape(subtitle)}</p>`;
}

export function headerActions(state: Workspace): string {
  if (state.view === "workspace" && state.profile) return `<button class="quiet" data-action="test-profile" id="test-profile" ${state.session?.testing ? "disabled" : ""}>${icon(state.session?.testing ? "refresh" : "activity", state.session?.testing ? "spin" : "")}<span>${state.session?.testing ? "Testing" : "Test"}</span></button><button class="quiet" data-action="edit-profile" id="edit-profile">${icon("edit")}<span>Edit</span></button><details class="action-menu"><summary class="icon-button" aria-label="Profile actions">${icon("more")}</summary><div class="menu-items"><button class="danger-text" data-action="delete-profile">Delete profile</button></div></details>`;
  if (state.view === "dumps") return `${state.api.runtimeMode() === "desktop" ? `<button class="quiet" data-action="open-folder">${icon("folder")}<span>Open folder</span></button>` : ""}<button class="quiet" data-action="refresh-dumps" ${state.dumps.loading ? "disabled" : ""}>${icon("refresh", state.dumps.loading ? "spin" : "")}<span>Refresh</span></button>`;
  if (state.view === "settings") return `<button class="quiet" data-action="refresh-tools" ${state.engines.loading ? "disabled" : ""}>${icon("refresh", state.engines.loading ? "spin" : "")}<span>Refresh tools</span></button>`;
  return `<button class="primary" data-action="new-profile">${icon("plus")} Add profile</button>`;
}

export function operationResult(record: Operation, desktop: boolean, compact = false): string {
  const running = record.status === "running";
  return `<article class="operation-result ${record.status} ${compact ? "compact" : ""}"><div class="result-icon">${running ? '<span class="spinner"></span>' : icon(record.status === "success" ? "check" : "alert")}</div><div class="result-body"><div class="result-heading"><strong>${record.kind === "export" ? record.fileName ? "Dump created" : running ? "Export in progress" : "Export failed" : running ? "Restore in progress" : record.status === "success" ? "Restore complete" : "Restore failed"}</strong><span class="meta" data-elapsed="${record.started}" ${record.ended ? `data-ended="${record.ended}"` : ""}>${duration(record.started, record.ended)}</span></div><p class="mono result-target">${escape(record.profileName)} / ${escape(record.database)} <span class="muted">· ${escape(record.host)}</span></p><p>${escape(record.text)}</p>${record.fileName ? `<div class="result-file">${icon("file")}<span class="mono">${escape(record.fileName)}</span><span class="meta">${bytes(record.bytes || 0)}</span></div>` : ""}${record.fileName ? `<button class="small" data-action="copy-result" data-id="${record.id}" ${record.copying ? "disabled" : ""}>${icon("restore")}${record.copying ? "Saving..." : desktop ? "Save a copy" : "Download"}</button>` : ""}</div></article>`;
}

export function activitySummary(operations: Operations, expanded: boolean): string {
  const active = operations.active;
  const latest = operations.records[0];
  return `<button class="activity-toggle" id="toggle-activity" data-action="toggle-activity" aria-expanded="${expanded}" aria-controls="activity-list">${active ? '<span class="spinner"></span>' : icon("activity")}<span>Activity <span class="muted">this session</span></span>${latest ? `<span class="activity-latest">${active ? `${active.kind === "export" ? "Exporting" : "Restoring"} ${escape(active.database)}` : escape(latest.text)}</span>` : '<span class="activity-latest muted">No operations yet</span>'}<span class="count-badge">${operations.records.length}</span>${icon("chevron", expanded ? "rotate-up" : "rotate-down")}</button>`;
}

export function activityList(operations: Operations, desktop: boolean): string {
  return operations.records.length ? `<p class="activity-note">Activity lasts for this session. Closing the app does not cancel a running operation.</p>${operations.records.map(record => operationResult(record, desktop, true)).join("")}` : emptyState("No operations yet", "Exports and restores will appear here during this session.", "", "activity");
}
