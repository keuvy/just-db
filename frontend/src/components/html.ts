import type { Notice } from "../app/state";

export function escape(value: unknown): string {
  return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
}

const paths: Record<string, string> = {
  database: '<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 4 16 4 16 0V5M4 12c0 4 16 4 16 0"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  search: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/>',
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z M14 2v6h6M8 13h8M8 17h5"/>',
  export: '<path d="M12 16V3m-5 5 5-5 5 5M4 15v5h16v-5"/>',
  restore: '<path d="M12 3v13m-5-5 5 5 5-5M4 16v5h16v-5"/>',
  settings: '<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>',
  refresh: '<path d="M20 7v5h-5M4 17v-5h5M6 7a7 7 0 0 1 12-1l2 6M4 12l2 6a7 7 0 0 0 12-1"/>',
  chevron: '<path d="m9 5 7 7-7 7"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  alert: '<path d="m12 3 10 18H2ZM12 9v4M12 17h.01"/>',
  close: '<path d="M6 6l12 12M6 18l12-12"/>',
  more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  edit: '<path d="m15 4 5 5M4 20l5-1L21 7l-5-5L4 14Z"/>',
  activity: '<path d="M2 12h5l3-8 4 16 3-8h5"/>',
  folder: '<path d="M3 7V4h6l3 3h9v13H3Z"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  eye: '<path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1 1M18 18l1 1M5 19l1-1M18 6l1-1"/>',
};

export function icon(name: string, className = ""): string {
  return `<svg class="icon ${className}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.database}</svg>`;
}

export function notice(value?: Notice): string {
  if (!value) return "";
  const detailed = (value.kind === "error" || value.kind === "warning") && (value.text.length > 220 || value.text.includes("\n"));
  const summary = detailed ? value.text.split("\n")[0].slice(0, 180) : value.text;
  return `<div class="notice ${value.kind}" role="${value.kind === "error" ? "alert" : "status"}">${icon(value.kind === "success" ? "check" : "alert")}<div class="notice-body"><span>${escape(summary)}${detailed && summary.length < value.text.split("\n")[0].length ? "..." : ""}</span>${detailed ? `<details class="error-details"><summary>Error details</summary><pre>${escape(value.text)}</pre><button type="button" class="small" data-copy-error>Copy details</button></details>` : ""}</div></div>`;
}

export function emptyState(title: string, description: string, action = "", symbol = "database"): string {
  return `<div class="empty-state"><span class="empty-icon">${icon(symbol)}</span><h2>${escape(title)}</h2><p>${escape(description)}</p>${action}</div>`;
}

export function bytes(value: number): string {
  if (value < 1024) return `${value} B`;
  const unit = Math.min(Math.floor(Math.log(value) / Math.log(1024)), 4);
  return `${(value / 1024 ** unit).toLocaleString(undefined, { maximumFractionDigits: 1 })} ${["B", "KB", "MB", "GB", "TB"][unit]}`;
}

export function duration(start: number, end = Date.now()): string {
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

export function engineName(name: string): string { return name === "postgres" ? "PostgreSQL" : name === "mysql" ? "MySQL / MariaDB" : name; }

// Update one region while keeping typing, keyboard focus, and scroll positions intact.
export function replaceRegion(element: HTMLElement, html: string): void {
  const active = document.activeElement as HTMLInputElement | null;
  const id = active && element.contains(active) ? active.id : "";
  const action = active && element.contains(active) ? active.dataset.action : "";
  const name = active?.dataset.name;
  const theme = active?.dataset.theme;
  const selection = id && active && /^(text|search|password)$/.test(active.type) ? [active.selectionStart, active.selectionEnd] : null;
  const top = element.scrollTop;
  element.innerHTML = html;
  element.scrollTop = top;
  if (id || action) {
    const selector = id ? `#${CSS.escape(id)}` : `[data-action="${CSS.escape(action!)}"]${name ? `[data-name="${CSS.escape(name)}"]` : ""}${theme ? `[data-theme="${CSS.escape(theme)}"]` : ""}`;
    const replacement = element.querySelector<HTMLElement>(selector);
    replacement?.focus({ preventScroll: true });
    if (replacement instanceof HTMLInputElement && selection) replacement.setSelectionRange(selection[0], selection[1]);
  }
}
