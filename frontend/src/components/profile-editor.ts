import type { Defaults, EngineInfo, Profile } from "../api";
import type { Client } from "../app/client";
import { message } from "../app/state";
import { confirmDialog } from "./feedback";
import { escape, icon, notice } from "./html";

export class ProfileEditor {
  private dialog: HTMLDialogElement | null = null;
  private initial = "";
  private saving = false;
  private testing = false;
  private version = 0;
  private editing = false;
  private trigger: HTMLElement | null = null;

  constructor(private api: Client, private onSaved: (name: string) => Promise<void>, private names: () => string[]) {}

  get dirty(): boolean { return Boolean(this.dialog && JSON.stringify(this.read()) !== this.initial); }
  get isOpen(): boolean { return Boolean(this.dialog?.open); }

  open(profile: Profile | null, defaults: Defaults | null, engines: EngineInfo[]): void {
    if (this.dialog) return;
    this.trigger = document.activeElement as HTMLElement;
    this.editing = Boolean(profile);
    this.saving = false;
    this.testing = false;
    this.version = 0;
    const engine = profile?.engine || defaults?.engine || "postgres";
    const connection = profile?.connection || defaults?.connection || { host: "127.0.0.1", port: engine === "mysql" ? 3306 : 5432, user: "", password: "", database: "", sslMode: "prefer" };
    const base = `${engine}-${connection.host || "localhost"}`.replace(/[^A-Za-z0-9._-]/g, "-").slice(0, 58);
    let name = base;
    for (let n = 2; this.names().includes(name); n++) name = `${base}-${n}`;
    const options = engines.length ? engines : [{ name: "postgres", displayName: "PostgreSQL" }, { name: "mysql", displayName: "MySQL / MariaDB" }];
    const field = (id: string, label: string, value: string | number, type = "text", attributes = "") => `<label for="editor-${id}">${label}</label><input id="editor-${id}" name="${id}" type="${type}" value="${escape(value)}" ${attributes}/>`;
    const dialog = document.createElement("dialog");
    dialog.className = "profile-editor";
    dialog.setAttribute("aria-labelledby", "editor-title");
    dialog.innerHTML = `<form id="profile-form" autocomplete="off">
      <header class="dialog-header"><div><span class="eyebrow">Profiles</span><h2 id="editor-title">${profile ? "Edit profile" : "New profile"}</h2></div><button type="button" class="icon-button" data-editor="close" aria-label="Close profile editor">${icon("close")}</button></header>
      <div class="editor-content"><p class="muted">A named connection you can reuse for exports and restores.</p>
      <div class="field">${field("name", "Profile name", profile?.name || name, "text", `required maxlength="64" pattern="[A-Za-z0-9 ._\\-]+" ${profile ? "readonly" : "autofocus"}`)}<span class="field-help">${profile ? "The name identifies this profile and cannot be changed here." : "Up to 64 letters, numbers, spaces, dots, underscores, or hyphens."}</span></div>
      <div class="field"><label for="editor-engine">Engine</label><select id="editor-engine" name="engine">${options.map(option => `<option value="${escape(option.name)}" ${engine === option.name ? "selected" : ""}>${escape(option.displayName)}</option>`).join("")}</select></div>
      <div class="section-label">Connection</div><div class="form-grid endpoint-fields"><div class="field">${field("host", "Host", connection.host, "text", 'placeholder="127.0.0.1"')}</div><div class="field">${field("port", "Port", connection.port, "number", 'required min="1" max="65535"')}</div></div>
      <div class="field">${field("user", "User", connection.user, "text", 'required autocomplete="off"')}</div>
      <div class="field"><label for="editor-password">Password</label><div class="password-field"><input id="editor-password" name="password" type="password" value="${escape(connection.password)}" autocomplete="new-password"/><button type="button" data-editor="password" class="icon-button" aria-label="Show password" aria-pressed="false">${icon("eye")}</button></div><span class="field-help">Saved profiles are encrypted. An empty password is allowed.</span></div>
      <div class="section-label">Options</div><div class="field"><label for="editor-sslMode">SSL mode</label><select id="editor-sslMode" name="sslMode">${["prefer", "require", "disable"].map(value => `<option ${value === (connection.sslMode || "prefer") ? "selected" : ""} value="${value}">${value}</option>`).join("")}</select></div>
      <div class="field">${field("database", "Default database", connection.database, "text", 'list="editor-databases" placeholder="Optional"')}<datalist id="editor-databases"></datalist><span class="field-help">You can choose a database later in the workspace.</span></div>
      <div id="editor-test-status" aria-live="polite"></div><div id="editor-error" aria-live="polite"></div>
      </div><footer class="dialog-footer"><button type="button" data-editor="test">Test connection</button><span class="spacer"></span><button type="button" data-editor="close">Cancel</button><button type="submit" class="primary" id="editor-save">Save profile</button></footer></form>`;
    this.dialog = dialog;
    document.body.append(dialog);
    this.initial = JSON.stringify(this.read());
    dialog.addEventListener("input", () => {
      ++this.version;
      const test = dialog.querySelector("#editor-test-status");
      if (test) test.innerHTML = "";
    });
    dialog.querySelector("#editor-engine")?.addEventListener("change", () => {
      ++this.version;
      const value = this.read().engine;
      const port = dialog.querySelector<HTMLInputElement>("#editor-port")!;
      port.value = String(engines.find(item => item.name === value)?.defaultPort || (value === "mysql" ? 3306 : 5432));
      dialog.querySelector("#editor-databases")!.innerHTML = "";
      dialog.querySelector("#editor-test-status")!.innerHTML = "";
    });
    dialog.addEventListener("click", event => {
      const button = (event.target as Element).closest<HTMLButtonElement>("[data-editor]");
      if (!button) return;
      if (button.dataset.editor === "close") void this.close();
      if (button.dataset.editor === "test") void this.test();
      if (button.dataset.editor === "password") {
        const input = dialog.querySelector<HTMLInputElement>("#editor-password")!;
        input.type = input.type === "password" ? "text" : "password";
        button.setAttribute("aria-label", input.type === "password" ? "Show password" : "Hide password");
        button.setAttribute("aria-pressed", String(input.type === "text"));
      }
    });
    dialog.addEventListener("cancel", event => { event.preventDefault(); void this.close(); });
    dialog.querySelector("form")!.addEventListener("submit", event => { event.preventDefault(); void this.save(); });
    dialog.showModal();
    if (profile) dialog.querySelector<HTMLElement>("#editor-host")?.focus();
  }

  private read(): Profile {
    const value = (name: string) => this.dialog?.querySelector<HTMLInputElement | HTMLSelectElement>(`#editor-${name}`)?.value || "";
    return { name: value("name").trim(), engine: value("engine"), connection: { host: value("host").trim(), port: Number(value("port")), user: value("user").trim(), password: value("password"), database: value("database").trim(), sslMode: value("sslMode") } };
  }

  private busy(): void {
    if (!this.dialog) return;
    this.dialog.querySelectorAll<HTMLButtonElement>("button").forEach(button => { button.disabled = this.saving || (this.testing && button.dataset.editor === "test"); });
    this.dialog.querySelectorAll<HTMLInputElement | HTMLSelectElement>("input,select").forEach(input => { input.disabled = this.saving; });
    this.dialog.querySelector("#editor-save")!.textContent = this.saving ? "Saving..." : "Save profile";
    this.dialog.querySelector('[data-editor="test"]')!.textContent = this.testing ? "Testing..." : "Test connection";
  }

  private async test(): Promise<void> {
    if (!this.dialog || this.testing || this.saving) return;
    const profile = this.read();
    const user = this.dialog.querySelector<HTMLInputElement>("#editor-user")!;
    if (!user.reportValidity()) return;
    const dialog = this.dialog;
    const version = this.version;
    this.testing = true;
    this.busy();
    const status = dialog.querySelector("#editor-test-status")!;
    status.innerHTML = notice({ kind: "info", text: "Testing connection..." });
    try {
      await this.api.testConnection(profile.engine, profile.connection);
      if (this.dialog !== dialog || version !== this.version) return;
      status.innerHTML = notice({ kind: "success", text: "Connection succeeded." });
      try {
        const databases = await this.api.listDatabases(profile.engine, { ...profile.connection, database: "" });
        if (this.dialog === dialog && version === this.version) dialog.querySelector("#editor-databases")!.innerHTML = databases.map(name => `<option value="${escape(name)}"></option>`).join("");
      } catch (error) {
        if (this.dialog === dialog && version === this.version) status.innerHTML += notice({ kind: "warning", text: `Database discovery failed: ${message(error, profile.connection.password)}. You can enter a database manually.` });
      }
    } catch (error) {
      if (this.dialog === dialog && version === this.version) status.innerHTML = notice({ kind: "error", text: message(error, profile.connection.password) });
    } finally { if (this.dialog === dialog) { this.testing = false; this.busy(); } }
  }

  private async save(): Promise<void> {
    if (!this.dialog || this.saving) return;
    const form = this.dialog.querySelector<HTMLFormElement>("form")!;
    if (!form.reportValidity()) return;
    const profile = this.read();
    const errorBox = this.dialog.querySelector("#editor-error")!;
    if (!profile.name || !profile.connection.user || profile.name === "key") {
      errorBox.innerHTML = notice({ kind: "error", text: "Enter a profile name and user. The name key is reserved." });
      return;
    }
    this.saving = true;
    this.busy();
    try {
      // Check the current list, not only the list loaded when the editor opened.
      if (!this.editing && (await this.api.listProfiles()).some(item => item.name === profile.name)) {
        if (!await confirmDialog("Replace profile?", `A profile named ${profile.name} already exists. Replace its connection settings?`, "Replace profile", true)) return;
      }
      await this.api.putProfile(profile.name, profile.engine, profile.connection);
      this.dismiss();
      await this.onSaved(profile.name);
    } catch (error) { if (errorBox.isConnected) errorBox.innerHTML = notice({ kind: "error", text: message(error, profile.connection.password) }); }
    finally { this.saving = false; this.busy(); }
  }

  async close(): Promise<void> {
    if (this.saving) return;
    if (this.dirty && !await confirmDialog("Discard changes?", "The changes to this profile have not been saved.", "Discard changes", true)) return;
    this.dismiss();
  }

  private dismiss(): void {
    const dialog = this.dialog;
    this.dialog = null;
    this.initial = "";
    dialog?.close();
    dialog?.remove();
    if (this.trigger?.isConnected) this.trigger.focus();
  }
}
