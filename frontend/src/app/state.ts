import type { Connection, Defaults, DumpFile, EngineInfo, Health, Profile, ProfileSummary } from "../api";
import type { Client } from "./client";

export type View = "workspace" | "dumps" | "settings";
export type Notice = { text: string; kind: "info" | "success" | "error" | "warning" };
export type Resource<T> = { data: T; loading: boolean; error: string };
export type Session = {
  database: string;
  format: string;
  databases: Resource<string[]>;
  testing: boolean;
  test?: Notice;
};
export type Source = { name: string; value: string | File; library: boolean; size?: number };
export type RestoreReview = Readonly<{
  profileName: string;
  engine: string;
  connection: Readonly<Connection>;
  format: string;
  source: Source;
  dropExisting: boolean;
}>;

export function message(error: unknown, password = ""): string {
  let text = error instanceof Error ? error.message : String(error);
  if (password) text = text.replaceAll(password, "[redacted]");
  return text.replace(/((?:password|PGPASSWORD|MYSQL_PWD)\s*[=:]\s*)[^\s,;]+/gi, "$1[redacted]");
}

export function fileFormat(name: string): string {
  if (/\.sql$/i.test(name)) return "sql";
  return /\.(dump|backup|pgdump)$/i.test(name) ? "custom" : "";
}

export function requiredTool(engines: EngineInfo[], engine: string, operation: "export" | "restore" | "client", format = ""): string {
  const info = engines.find(item => item.name === engine);
  if (!info) return "Tool availability has not loaded. Refresh tools in settings.";
  const kind = operation === "export" ? "dump" : operation === "client" || engine === "mysql" || format === "sql" ? "client" : "restore";
  const tool = info.tools[kind];
  return tool.found ? "" : `${tool.name} is missing on the app host. Open Tools & settings for details.`;
}

export function dropExplanation(engine: string, format: string): string {
  if (engine === "mysql") return "Drops all base tables in the target database before running the SQL file.";
  if (format === "sql") return "Drops the public schema with CASCADE, recreates it, and reapplies grants before running the SQL file. Objects depending on this schema can also be removed.";
  return "Drops existing objects restored from this archive using pg_restore --clean --if-exists.";
}

export class Workspace {
  onChange: () => void = () => {};
  view: View = "workspace";
  mode: "export" | "restore" = "export";
  health: Resource<Health | null> = { data: null, loading: true, error: "" };
  engines: Resource<EngineInfo[]> = { data: [], loading: true, error: "" };
  profiles: Resource<ProfileSummary[]> = { data: [], loading: true, error: "" };
  dumps: Resource<DumpFile[]> = { data: [], loading: false, error: "" };
  defaults: Defaults | null = null;
  defaultsError = "";
  selectedName = "";
  profile: Profile | null = null;
  profileLoading = false;
  profileError = "";
  sessions = new Map<string, Session>();
  source: Source | null = null;
  sourceMode: "file" | "library" = "file";
  restoreStep = 1;
  dropExisting = false;
  review: RestoreReview | null = null;
  acknowledged = false;
  confirmation = "";
  restoreNotice?: Notice;
  private selectionGeneration = 0;
  private databaseGeneration = 0;
  private profileListGeneration = 0;
  private dumpGeneration = 0;
  private enginesGeneration = 0;

  constructor(readonly api: Client) {}

  async start(): Promise<void> {
    await Promise.allSettled([this.loadHealth(), this.loadEngines(), this.loadProfiles(), this.loadDefaults()]);
  }

  async loadHealth(): Promise<void> {
    this.health.loading = true;
    this.onChange();
    try { this.health.data = await this.api.getHealth(); this.health.error = ""; }
    catch (error) { this.health.error = message(error); }
    finally { this.health.loading = false; this.onChange(); }
  }

  async loadDefaults(): Promise<void> {
    try { this.defaults = await this.api.getDefaults(); this.defaultsError = ""; }
    catch (error) { this.defaultsError = message(error); }
    this.onChange();
  }

  async loadEngines(): Promise<void> {
    const generation = ++this.enginesGeneration;
    this.engines.loading = true;
    this.onChange();
    try {
      const data = await this.api.getEngines();
      if (generation === this.enginesGeneration) { this.engines.data = data; this.engines.error = ""; }
    } catch (error) { if (generation === this.enginesGeneration) this.engines.error = message(error); }
    finally { if (generation === this.enginesGeneration) { this.engines.loading = false; this.onChange(); } }
  }

  async loadProfiles(): Promise<void> {
    const generation = ++this.profileListGeneration;
    this.profiles.loading = true;
    this.onChange();
    try {
      const data = await this.api.listProfiles();
      if (generation !== this.profileListGeneration) return;
      this.profiles.data = data;
      this.profiles.error = "";
      if (this.selectedName && !data.some(item => item.name === this.selectedName)) this.clearProfile();
    } catch (error) { if (generation === this.profileListGeneration) this.profiles.error = message(error); }
    finally { if (generation === this.profileListGeneration) { this.profiles.loading = false; this.onChange(); } }
  }

  async loadDumps(): Promise<void> {
    const generation = ++this.dumpGeneration;
    this.dumps.loading = true;
    this.onChange();
    try {
      const data = await this.api.listDumps();
      if (generation === this.dumpGeneration) { this.dumps.data = data; this.dumps.error = ""; }
    } catch (error) { if (generation === this.dumpGeneration) this.dumps.error = message(error); }
    finally { if (generation === this.dumpGeneration) { this.dumps.loading = false; this.onChange(); } }
  }

  get session(): Session | undefined { return this.profile ? this.sessions.get(this.profile.name) : undefined; }

  clearProfile(): void {
    ++this.selectionGeneration;
    ++this.databaseGeneration;
    this.selectedName = "";
    this.profile = null;
    this.profileLoading = false;
    this.invalidateReview();
  }

  async selectProfile(name: string): Promise<void> {
    const generation = ++this.selectionGeneration;
    ++this.databaseGeneration;
    this.selectedName = name;
    this.profile = null;
    this.profileLoading = true;
    this.profileError = "";
    this.invalidateReview();
    this.onChange();
    try {
      const profile = await this.api.getProfile(name);
      if (generation !== this.selectionGeneration) return;
      this.profile = { ...profile, connection: { ...profile.connection } };
      if (!this.sessions.has(name)) {
        this.sessions.set(name, { database: profile.connection.database, format: profile.engine === "mysql" ? "sql" : "custom", databases: { data: [], loading: false, error: "" }, testing: false });
      }
      const session = this.sessions.get(name)!;
      if (profile.engine === "mysql") session.format = "sql";
      session.test = undefined;
      session.testing = false;
      this.profileLoading = false;
      this.onChange();
      await this.loadDatabases();
    } catch (error) {
      if (generation === this.selectionGeneration) {
        this.profileError = message(error);
        this.profile = null;
      }
    } finally {
      if (generation === this.selectionGeneration) { this.profileLoading = false; this.onChange(); }
    }
  }

  async loadDatabases(): Promise<void> {
    const profile = this.profile;
    const session = this.session;
    if (!profile || !session) return;
    const generation = ++this.databaseGeneration;
    const selected = this.selectionGeneration;
    session.databases = { data: [], loading: true, error: "" };
    this.onChange();
    try {
      const data = await this.api.listDatabases(profile.engine, { ...profile.connection, database: "" });
      if (generation === this.databaseGeneration && selected === this.selectionGeneration) session.databases = { data, loading: false, error: "" };
    } catch (error) {
      if (generation === this.databaseGeneration && selected === this.selectionGeneration) session.databases = { data: [], loading: false, error: message(error, profile.connection.password) };
    } finally { if (generation === this.databaseGeneration && selected === this.selectionGeneration) this.onChange(); }
  }

  async testProfile(): Promise<void> {
    const profile = this.profile;
    const session = this.session;
    if (!profile || !session || session.testing) return;
    const generation = this.selectionGeneration;
    session.testing = true;
    session.test = { text: "Testing connection...", kind: "info" };
    this.onChange();
    try {
      await this.api.testConnection(profile.engine, { ...profile.connection });
      if (generation !== this.selectionGeneration) return;
      session.test = { text: `Last tested successfully at ${new Date().toLocaleTimeString()}.`, kind: "success" };
      if (this.profile === profile) void this.loadDatabases();
    } catch (error) { if (generation === this.selectionGeneration) session.test = { text: message(error, profile.connection.password), kind: "error" }; }
    finally { if (generation === this.selectionGeneration) { session.testing = false; this.onChange(); } }
  }

  setDatabase(database: string): void {
    if (this.session) this.session.database = database;
    this.invalidateReview();
  }

  setSource(source: Source | null): void {
    this.source = source;
    this.invalidateReview();
    this.restoreStep = 1;
  }

  invalidateReview(): void {
    this.review = null;
    this.acknowledged = false;
    this.confirmation = "";
    this.restoreNotice = undefined;
    if (this.restoreStep === 3) this.restoreStep = 2;
  }

  prepareReview(): RestoreReview {
    const profile = this.profile;
    const database = this.session?.database.trim();
    if (!profile || this.profileLoading || !database) throw new Error("Choose a target profile and database.");
    if (!this.source) throw new Error("Choose a source file.");
    const format = fileFormat(this.source.name);
    if (!format) throw new Error("Choose a .sql, .dump, .backup, or .pgdump file.");
    if (profile.engine === "mysql" && format !== "sql") throw new Error("MySQL / MariaDB requires a SQL dump. Custom archives are for PostgreSQL.");
    const toolError = requiredTool(this.engines.data, profile.engine, "restore", format);
    if (toolError) throw new Error(toolError);
    this.restoreNotice = undefined;
    this.review = Object.freeze({ profileName: profile.name, engine: profile.engine, connection: Object.freeze({ ...profile.connection, database }), format, source: Object.freeze({ ...this.source }), dropExisting: this.dropExisting });
    this.restoreStep = 3;
    this.acknowledged = false;
    this.confirmation = "";
    return this.review;
  }

  canRestore(): boolean {
    return Boolean(this.review && (this.review.format !== "sql" || this.acknowledged) && (!this.review.dropExisting || this.confirmation === this.review.connection.database));
  }
}
