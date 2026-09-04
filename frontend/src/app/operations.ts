import type { Profile } from "../api";
import type { Client } from "./client";
import { message, type RestoreReview } from "./state";

export type Operation = {
  id: number;
  kind: "export" | "restore";
  profileName: string;
  database: string;
  host: string;
  started: number;
  ended?: number;
  status: "running" | "success" | "warning" | "error";
  text: string;
  fileName?: string;
  sourceName?: string;
  librarySource?: boolean;
  bytes?: number;
  copying?: boolean;
};

export class Operations {
  records: Operation[] = [];
  active: Operation | null = null;
  onChange: () => void = () => {};
  private sequence = 0;

  constructor(readonly api: Client) {}

  private begin(kind: Operation["kind"], profile: Profile): Operation | null {
    if (this.active) return null;
    const record: Operation = { id: ++this.sequence, kind, profileName: profile.name, database: profile.connection.database, host: `${profile.connection.host}:${profile.connection.port}`, started: Date.now(), status: "running", text: kind === "export" ? "Creating dump..." : "Restoring dump..." };
    this.active = record;
    this.records = [record, ...this.records].slice(0, 51);
    this.onChange();
    return record;
  }

  private end(record: Operation): void {
    record.ended = Date.now();
    this.active = null;
    this.records = this.records.slice(0, 50);
    this.onChange();
  }

  async export(profile: Profile, database: string, format: string): Promise<boolean> {
    const snapshot = { ...profile, connection: { ...profile.connection, database: database.trim() } };
    if (!snapshot.connection.database) throw new Error("Choose a database first.");
    const record = this.begin("export", snapshot);
    if (!record) return false;
    try {
      const result = await this.api.exportDump(snapshot.engine, snapshot.connection, format, "");
      record.fileName = result.path.split(/[\\/]/).pop()!;
      record.bytes = result.bytes;
      await this.copy(record, snapshot.connection.password);
    } catch (error) {
      record.status = "error";
      record.text = message(error, snapshot.connection.password);
    } finally { this.end(record); }
    return true;
  }

  async copy(record: Operation, password = ""): Promise<void> {
    if (!record.fileName || record.copying) return;
    record.copying = true;
    this.onChange();
    try {
      const copied = await this.api.downloadDump(record.fileName);
      record.status = "success";
      record.text = copied
        ? this.api.runtimeMode() === "desktop" ? "Dump created. Copy saved." : "Dump created. Download started."
        : "Dump created. Save cancelled; the file is available in Dumps.";
    } catch (error) {
      record.status = "warning";
      record.text = `Dump created, but the copy could not be saved: ${message(error, password)}. Retry from this result or Dumps.`;
    } finally { record.copying = false; this.onChange(); }
  }

  async restore(review: RestoreReview): Promise<boolean> {
    const profile: Profile = { name: review.profileName, engine: review.engine, connection: { ...review.connection } };
    const record = this.begin("restore", profile);
    if (!record) return false;
    record.sourceName = review.source.name;
    record.librarySource = review.source.library;
    if (typeof review.source.value !== "string") record.text = "Uploading and restoring...";
    this.onChange();
    try {
      await this.api.importDump(profile.engine, profile.connection, review.format, review.source.value, review.dropExisting);
      record.status = "success";
      record.text = "Restore finished.";
    } catch (error) {
      record.status = "error";
      record.text = `${message(error, profile.connection.password)}. Some database changes may already have been applied.`;
    } finally { this.end(record); }
    return true;
  }
}
