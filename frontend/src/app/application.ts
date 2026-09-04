import type { Client } from "./client";
import { Workspace, fileFormat, message, requiredTool, type Notice, type View } from "./state";
import { Operations } from "./operations";
import { destination } from "./navigation";
import { Appearance, type Theme } from "./theme";
import { ProfileEditor } from "../components/profile-editor";
import { confirmDialog } from "../components/feedback";
import { activityList, activitySummary, headerActions, heading, profilesView, shell } from "../components/shell";
import { duration, notice, replaceRegion } from "../components/html";
import { workspaceView } from "../views/workspace";
import { exportView } from "../views/export";
import { dumpsView, dumpRows, type DumpSort } from "../views/dumps";
import { settingsView } from "../views/settings";

export class Application {
  readonly state: Workspace;
  readonly operations: Operations;
  readonly appearance: Appearance;
  readonly editor: ProfileEditor;
  private profileSearch = "";
  private dumpSearch = "";
  private dumpSort: DumpSort = "newest";
  private activityExpanded = false;
  private fileBusy = new Set<string>();
  private notices = new Map<View, Notice>();
  private selectingFile = false;
  private timer: ReturnType<typeof setInterval>;
  private media = window.matchMedia("(min-width: 900px)");
  private mobileTrigger: HTMLElement | null = null;
  private hashChanged = () => { const route = destination(location.hash); this.navigate(route.view, false); if (route.editor) this.openEditor(); };
  private resized = () => { if (this.media.matches) this.closeNavigation(); };
  private beforeUnload = (event: BeforeUnloadEvent) => {
    if (this.operations.active || this.editor.dirty) { event.preventDefault(); event.returnValue = ""; }
  };
  private copyError = async (event: MouseEvent): Promise<void> => {
    const button = (event.target as Element).closest<HTMLButtonElement>("[data-copy-error]");
    if (!button) return;
    try {
      await navigator.clipboard.writeText(button.closest("details")?.querySelector("pre")?.textContent || "");
      button.textContent = "Copied";
    } catch { button.textContent = "Select the details to copy"; }
  };

  constructor(readonly root: HTMLElement, readonly api: Client) {
    this.state = new Workspace(api);
    this.operations = new Operations(api);
    this.appearance = new Appearance();
    this.editor = new ProfileEditor(api, async name => {
      await this.state.loadProfiles();
      this.navigate("workspace");
      await this.state.selectProfile(name);
      this.setNotice("workspace", { kind: "success", text: `Saved ${name}.` });
    }, () => this.state.profiles.data.map(profile => profile.name));
    root.innerHTML = shell();
    root.classList.toggle("desktop", api.runtimeMode() === "desktop");
    root.classList.toggle("mac-desktop", api.runtimeMode() === "desktop" && /Mac/.test(navigator.platform));
    this.state.onChange = () => this.render();
    this.operations.onChange = () => this.render();
    root.addEventListener("click", this.clicked);
    root.addEventListener("input", this.input);
    root.addEventListener("change", this.changed);
    root.addEventListener("keydown", this.keydown);
    root.addEventListener("dragover", this.dragover);
    root.addEventListener("dragleave", this.dragleave);
    root.addEventListener("drop", this.drop);
    window.addEventListener("hashchange", this.hashChanged);
    window.addEventListener("beforeunload", this.beforeUnload);
    document.addEventListener("click", this.copyError);
    this.media.addEventListener("change", this.resized);
    this.timer = setInterval(() => {
      if (this.operations.active) root.querySelectorAll<HTMLElement>("[data-elapsed]:not([data-ended])").forEach(element => { element.textContent = duration(Number(element.dataset.elapsed)); });
    }, 1000);
  }

  async start(): Promise<void> {
    const route = destination(location.hash);
    this.state.view = route.view;
    this.render();
    if (route.view === "dumps") void this.state.loadDumps();
    await this.state.start();
    if (route.editor) this.openEditor();
  }

  private mount(id: string): HTMLElement { return this.root.querySelector<HTMLElement>(`#${id}`)!; }

  render(): void {
    replaceRegion(this.mount("profile-list"), profilesView(this.state, this.profileSearch));
    this.mount("profile-count").textContent = String(this.state.profiles.data.length || "");
    replaceRegion(this.mount("workspace-heading"), heading(this.state));
    replaceRegion(this.mount("workspace-actions"), headerActions(this.state));
    for (const view of ["dumps", "settings"]) {
      const item = this.mount(`nav-${view}`);
      item.classList.toggle("active", this.state.view === view);
      if (this.state.view === view) item.setAttribute("aria-current", "page"); else item.removeAttribute("aria-current");
    }
    this.renderPage();
    replaceRegion(this.mount("activity-summary"), activitySummary(this.operations, this.activityExpanded));
    this.mount("activity-list").hidden = !this.activityExpanded;
    if (this.activityExpanded) replaceRegion(this.mount("activity-list"), activityList(this.operations, this.api.runtimeMode() === "desktop"));
  }

  private renderPage(): void {
    replaceRegion(this.mount("page-notice"), notice(this.notices.get(this.state.view)));
    replaceRegion(this.mount("page-content"), this.state.view === "dumps" ? dumpsView(this.state, this.operations, this.dumpSearch, this.dumpSort, this.fileBusy) : this.state.view === "settings" ? settingsView(this.state, this.appearance.value) : workspaceView(this.state, this.operations));
  }

  private setNotice(view: View, value: Notice): void {
    this.notices.set(view, value);
    if (this.state.view === view) replaceRegion(this.mount("page-notice"), notice(value));
  }

  private navigate(view: View, updateHash = true): void {
    this.state.view = view;
    if (updateHash && location.hash !== `#${view}`) history.pushState(null, "", `#${view}`);
    this.closeNavigation(false);
    this.render();
    this.mount("main-content").scrollTop = 0;
    this.mount("main-content").focus({ preventScroll: true });
    if (view === "dumps") void this.state.loadDumps();
  }

  private openEditor(edit = false): void {
    this.closeNavigation(false);
    if (edit && !this.state.profile) return;
    this.editor.open(edit ? this.state.profile : null, this.state.defaults, this.state.engines.data);
  }

  private openNavigation(): void {
    this.mobileTrigger = document.activeElement as HTMLElement;
    this.root.classList.add("nav-open");
    this.mount("open-nav").setAttribute("aria-expanded", "true");
    this.mount("sidebar").setAttribute("role", "dialog");
    this.mount("sidebar").setAttribute("aria-modal", "true");
    this.root.querySelector<HTMLElement>(".sidebar-scrim")!.hidden = false;
    this.root.querySelector<HTMLElement>(".workspace-shell")!.inert = true;
    this.mount("profile-search").focus();
  }

  private closeNavigation(restoreFocus = true): void {
    const wasOpen = this.root.classList.contains("nav-open");
    this.root.classList.remove("nav-open");
    this.mount("open-nav").setAttribute("aria-expanded", "false");
    this.mount("sidebar").removeAttribute("role");
    this.mount("sidebar").removeAttribute("aria-modal");
    this.root.querySelector<HTMLElement>(".sidebar-scrim")!.hidden = true;
    this.root.querySelector<HTMLElement>(".workspace-shell")!.inert = false;
    if (wasOpen && restoreFocus) this.mobileTrigger?.focus();
  }

  private clicked = (event: MouseEvent): void => {
    const target = (event.target as Element).closest<HTMLElement>("[data-action]");
    if (!target || target instanceof HTMLButtonElement && target.disabled) return;
    const { action, name = "", mode, view, id } = target.dataset;
    const menu = target.closest("details");
    if (menu) menu.open = false;
    switch (action) {
      case "navigate": this.navigate(view as View); break;
      case "open-nav": this.openNavigation(); break;
      case "close-nav": this.closeNavigation(); break;
      case "new-profile": this.openEditor(); break;
      case "edit-profile": this.openEditor(true); break;
      case "select-profile": this.navigate("workspace"); void this.state.selectProfile(name); break;
      case "retry-profile": void this.state.selectProfile(this.state.selectedName); break;
      case "refresh-profiles": void this.state.loadProfiles(); break;
      case "refresh-databases": void this.state.loadDatabases(); break;
      case "refresh-dumps": void this.state.loadDumps(); break;
      case "refresh-tools": void this.state.loadEngines(); break;
      case "refresh-health": void this.state.loadHealth(); break;
      case "refresh-defaults": void this.state.loadDefaults(); break;
      case "test-profile": void this.state.testProfile(); break;
      case "delete-profile": void this.deleteProfile(); break;
      case "export": void this.export(); break;
      case "restore": void this.restore(); break;
      case "operation-mode": this.state.mode = mode as "export" | "restore"; this.renderPage(); this.root.querySelector<HTMLElement>(`#tab-${mode}`)?.focus(); break;
      case "source-mode": this.state.sourceMode = mode as "file" | "library"; this.state.setSource(null); this.renderPage(); if (mode === "library") void this.state.loadDumps(); break;
      case "clear-source": this.state.setSource(null); this.renderPage(); break;
      case "choose-file": void this.chooseFile(); break;
      case "restore-next": this.nextRestore(); break;
      case "restore-back": { const step = this.state.restoreStep; this.state.invalidateReview(); this.state.restoreStep = Math.max(1, step - 1); this.renderPage(); break; }
      case "restore-file": this.restoreFile(name); break;
      case "download-file": void this.fileAction(name, false); break;
      case "delete-file": void this.fileAction(name, true); break;
      case "copy-path": void this.copyPath(name); break;
      case "open-folder": void this.openFolder(); break;
      case "copy-result": { const record = this.operations.records.find(record => record.id === Number(id)); if (record) void this.operations.copy(record); break; }
      case "toggle-activity": this.activityExpanded = !this.activityExpanded; this.render(); break;
      case "theme": this.appearance.set(target.dataset.theme as Theme); this.renderPage(); break;
    }
  };

  private input = (event: Event): void => {
    const input = event.target as HTMLInputElement;
    if (input.id === "profile-search") { this.profileSearch = input.value; replaceRegion(this.mount("profile-list"), profilesView(this.state, this.profileSearch)); }
    if (input.id === "dump-search") { this.dumpSearch = input.value; replaceRegion(this.mount("dump-rows"), dumpRows(this.state, this.operations, this.dumpSearch, this.dumpSort, this.fileBusy)); }
    if (input.id === "workspace-database") {
      this.state.setDatabase(input.value);
      replaceRegion(this.mount("workspace-heading"), heading(this.state));
      if (this.state.mode === "export" && this.state.profile) replaceRegion(this.mount("export-content"), exportView(this.state, this.operations));
      const next = this.root.querySelector<HTMLButtonElement>("#restore-next-target");
      if (next) next.disabled = !input.value.trim() || !this.state.profile;
    }
    if (input.id === "restore-confirm-name") { this.state.confirmation = input.value; this.updateRestoreButton(); }
  };

  private changed = (event: Event): void => {
    const input = event.target as HTMLInputElement;
    if (input.name === "export-format" && this.state.session) { this.state.session.format = input.value; this.renderPage(); }
    if (input.id === "restore-profile") { if (input.value) void this.state.selectProfile(input.value); else { this.state.clearProfile(); this.render(); } }
    if (input.id === "restore-file" && input.files?.[0]) this.acceptFile(input.files[0]);
    if (input.id === "restore-library") {
      const dump = this.state.dumps.data.find(dump => dump.name === input.value);
      this.state.setSource(dump ? { name: dump.name, value: this.api.runtimeMode() === "desktop" ? dump.path : dump.name, library: true, size: dump.size } : null);
      this.renderPage();
    }
    if (input.id === "drop-existing") { this.state.dropExisting = input.checked; this.state.invalidateReview(); }
    if (input.id === "restore-ack") { this.state.acknowledged = input.checked; this.updateRestoreButton(); }
    if (input.id === "dump-sort") { this.dumpSort = input.value as DumpSort; replaceRegion(this.mount("dump-rows"), dumpRows(this.state, this.operations, this.dumpSearch, this.dumpSort, this.fileBusy)); }
  };

  private updateRestoreButton(): void {
    const button = this.root.querySelector<HTMLButtonElement>("#restore-submit");
    if (button) button.disabled = !this.state.canRestore() || Boolean(this.operations.active);
  }

  private keydown = (event: KeyboardEvent): void => {
    if (event.key === "Escape") {
      if (this.root.classList.contains("nav-open")) { event.preventDefault(); this.closeNavigation(); }
      this.root.querySelectorAll<HTMLDetailsElement>("details[open]").forEach(details => { details.open = false; details.querySelector<HTMLElement>("summary")?.focus(); });
    }
    const tabs = (event.target as HTMLElement).closest('[role="tablist"]');
    if (tabs && ["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) {
      event.preventDefault();
      const buttons = Array.from(tabs.querySelectorAll<HTMLButtonElement>("button"));
      const current = buttons.indexOf(event.target as HTMLButtonElement);
      const index = event.key === "Home" ? 0 : event.key === "End" ? buttons.length - 1 : (current + (event.key === "ArrowRight" ? 1 : -1) + buttons.length) % buttons.length;
      buttons[index].click();
    }
    if (event.key === "Tab" && this.root.classList.contains("nav-open")) {
      const focusable = Array.from(this.mount("sidebar").querySelectorAll<HTMLElement>('button:not([disabled]),input,summary')).filter(element => element.getClientRects().length > 0);
      const first = focusable[0], last = focusable.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }
  };

  private dragover = (event: DragEvent): void => {
    if (this.api.runtimeMode() === "desktop") return;
    const zone = (event.target as Element).closest("#file-dropzone");
    if (zone) { event.preventDefault(); zone.classList.add("dragging"); }
  };
  private dragleave = (event: DragEvent): void => { (event.target as Element).closest("#file-dropzone")?.classList.remove("dragging"); };
  private drop = (event: DragEvent): void => {
    if (this.api.runtimeMode() === "desktop" || !(event.target as Element).closest("#file-dropzone")) return;
    event.preventDefault();
    if (event.dataTransfer?.files[0]) this.acceptFile(event.dataTransfer.files[0]);
  };

  private acceptFile(file: File): void {
    if (!fileFormat(file.name)) { this.state.setSource(null); this.state.restoreNotice = { kind: "error", text: "Choose a .sql, .dump, .backup, or .pgdump file." }; }
    else this.state.setSource({ name: file.name, value: file, library: false, size: file.size });
    this.renderPage();
  }

  private async chooseFile(): Promise<void> {
    if (this.api.runtimeMode() === "web") { this.root.querySelector<HTMLInputElement>("#restore-file")?.click(); return; }
    if (this.selectingFile) return;
    this.selectingFile = true;
    try {
      const path = await this.api.pickOpenPath();
      if (path) {
        const name = path.split(/[\\/]/).pop()!;
        if (!fileFormat(name)) throw new Error("Choose a .sql, .dump, .backup, or .pgdump file.");
        this.state.setSource({ name, value: path, library: false });
      }
    } catch (error) { this.state.restoreNotice = { kind: "error", text: message(error) }; }
    finally { this.selectingFile = false; this.renderPage(); }
  }

  private nextRestore(): void {
    try {
      if (this.state.restoreStep === 1) {
        if (!this.state.source) throw new Error("Choose a source file.");
        this.state.restoreStep = 2;
        this.state.restoreNotice = undefined;
      } else this.state.prepareReview();
    } catch (error) { this.state.restoreNotice = { kind: "error", text: message(error) }; }
    this.renderPage();
    const title = this.root.querySelector<HTMLElement>("#operation-panel h2");
    title?.setAttribute("tabindex", "-1");
    title?.focus();
  }

  private async export(): Promise<void> {
    const profile = this.state.profile, session = this.state.session;
    if (!profile || !session || this.operations.active) return;
    const missing = requiredTool(this.state.engines.data, profile.engine, "export");
    if (missing) { this.setNotice("workspace", { kind: "error", text: missing }); return; }
    try { await this.operations.export(profile, session.database, session.format); await this.state.loadDumps(); }
    catch (error) { this.setNotice("workspace", { kind: "error", text: message(error, profile.connection.password) }); }
  }

  private async restore(): Promise<void> {
    if (!this.state.canRestore() || !this.state.review || this.operations.active) return;
    const review = this.state.review;
    this.state.invalidateReview();
    this.state.restoreStep = 1;
    await this.operations.restore(review);
  }

  private restoreFile(name: string): void {
    const dump = this.state.dumps.data.find(dump => dump.name === name);
    if (!dump) return;
    this.state.mode = "restore";
    this.state.sourceMode = "library";
    this.state.setSource({ name, value: this.api.runtimeMode() === "desktop" ? dump.path : name, size: dump.size, library: true });
    this.state.restoreStep = 2;
    this.navigate("workspace");
  }

  private async deleteProfile(): Promise<void> {
    const profile = this.state.profile;
    if (!profile || !await confirmDialog("Delete profile?", `Remove ${profile.name} from your stored profiles? Database contents and dump files are unaffected.`, "Delete profile", true)) return;
    try {
      await this.api.deleteProfile(profile.name);
      this.state.sessions.delete(profile.name);
      if (this.state.selectedName === profile.name) this.state.clearProfile();
      await this.state.loadProfiles();
      this.setNotice("workspace", { kind: "success", text: `Deleted ${profile.name}.` });
    } catch (error) { this.setNotice("workspace", { kind: "error", text: message(error, profile.connection.password) }); }
  }

  private async fileAction(name: string, deleting: boolean): Promise<void> {
    if (this.fileBusy.has(name) || deleting && this.operations.active?.librarySource && this.operations.active.sourceName === name) return;
    this.fileBusy.add(name);
    this.render();
    try {
      if (deleting) {
        if (!await confirmDialog("Delete dump?", `Permanently delete ${name}? This file cannot be recovered through just-db.`, "Delete dump", true)) return;
        if (this.operations.active?.librarySource && this.operations.active.sourceName === name) throw new Error("This dump is being restored. Wait for the operation to finish.");
        await this.api.deleteDump(name);
        if (this.state.source?.library && this.state.source.name === name) this.state.setSource(null);
        await this.state.loadDumps();
        this.setNotice("dumps", { kind: "success", text: `Deleted ${name}.` });
      } else {
        const copied = await this.api.downloadDump(name);
        this.setNotice("dumps", { kind: copied ? "success" : "info", text: copied ? `${this.api.runtimeMode() === "desktop" ? "Copy saved" : "Download started"}: ${name}` : "Save cancelled. The dump is still in the library." });
      }
    } catch (error) { this.setNotice("dumps", { kind: "error", text: message(error) }); }
    finally { this.fileBusy.delete(name); this.render(); }
  }

  private async openFolder(): Promise<void> {
    const view = this.state.view;
    try { await this.api.openBackupsDir(); this.setNotice(view, { kind: "success", text: "Opened dumps folder." }); }
    catch (error) { this.setNotice(view, { kind: "error", text: message(error) }); }
  }

  private async copyPath(name: string): Promise<void> {
    const dump = this.state.dumps.data.find(dump => dump.name === name);
    try {
      if (!navigator.clipboard) throw new Error("Clipboard is unavailable here. Select and copy the path from File details.");
      await navigator.clipboard.writeText(dump?.path || name);
      this.setNotice("dumps", { kind: "success", text: "Path copied." });
    } catch (error) { this.setNotice("dumps", { kind: "error", text: message(error) }); }
  }

  dispose(): void {
    clearInterval(this.timer);
    this.appearance.dispose();
    this.state.onChange = () => {};
    this.operations.onChange = () => {};
    window.removeEventListener("hashchange", this.hashChanged);
    window.removeEventListener("beforeunload", this.beforeUnload);
    document.removeEventListener("click", this.copyError);
    this.media.removeEventListener("change", this.resized);
    this.root.removeEventListener("click", this.clicked);
    this.root.removeEventListener("input", this.input);
    this.root.removeEventListener("change", this.changed);
    this.root.removeEventListener("keydown", this.keydown);
    this.root.removeEventListener("dragover", this.dragover);
    this.root.removeEventListener("dragleave", this.dragleave);
    this.root.removeEventListener("drop", this.drop);
  }
}
