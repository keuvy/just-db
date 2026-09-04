export type Tool = {
  name: string;
  path?: string;
  version?: string;
  found: boolean;
};

export type EngineInfo = {
  name: string;
  displayName: string;
  defaultPort: number;
  tools: {
    dump: Tool;
    restore: Tool;
    client: Tool;
  };
  ready: boolean;
};

export type Health = {
  status: string;
  name: string;
  version: string;
  mode: string;
  dataDir?: string;
  backupsDir?: string;
};

export type Defaults = {
  engine: string;
  connection: Connection;
  dataDir: string;
  backupsDir: string;
  auth: boolean;
};

export type Connection = {
  host: string;
  port: number;
  user: string;
  password: string;
  database: string;
  sslMode: string;
};

export type DumpFile = {
  name: string;
  path: string;
  size: number;
  modTime: string;
};

export type ProfileSummary = {
  name: string;
  engine: string;
  host: string;
  port: number;
  user: string;
  database: string;
  sslMode: string;
};

export type Profile = {
  name: string;
  engine: string;
  connection: Connection;
};

export type ExportResult = {
  path: string;
  bytes: number;
  format: string;
};

type WailsApp = {
  Health: () => Promise<Health>;
  Engines: () => Promise<EngineInfo[]>;
  TestConnection: (name: string, cfg: Connection) => Promise<void>;
  Export: (
    name: string,
    cfg: Connection,
    opts: { format: string },
    destPath: string,
  ) => Promise<ExportResult>;
  ImportDump: (
    name: string,
    cfg: Connection,
    opts: { format: string; dropExisting: boolean; confirm: boolean },
    srcPath: string,
  ) => Promise<void>;
  ListDumps: (dir: string) => Promise<DumpFile[]>;
  DeleteDump: (name: string) => Promise<void>;
  SaveDump: (name: string) => Promise<string>;
  PickSavePath: (defaultName: string) => Promise<string>;
  PickOpenPath: () => Promise<string>;
  DefaultDumpName: (name: string, database: string, format: string) => Promise<string>;
  OpenBackupsDir: () => Promise<void>;
  ListProfiles: () => Promise<ProfileSummary[]>;
  GetProfile: (name: string) => Promise<Profile>;
  PutProfile: (name: string, engineName: string, cfg: Connection) => Promise<void>;
  DeleteProfile: (name: string) => Promise<void>;
  ListDatabases: (name: string, cfg: Connection) => Promise<string[]>;
};

function wailsApp(): WailsApp | undefined {
  const win = window as unknown as {
    go?: { main?: { App?: WailsApp } };
  };
  return win.go?.main?.App;
}

export function runtimeMode(): "desktop" | "web" {
  return wailsApp() ? "desktop" : "web";
}

async function readError(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) {
      return body.error;
    }
  } catch {
    // fall through
  }
  return `${response.status} ${response.statusText}`;
}

export async function getHealth(): Promise<Health> {
  const app = wailsApp();
  if (app?.Health) {
    return app.Health();
  }
  const response = await fetch("/api/health");
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return response.json();
}

export async function getEngines(): Promise<EngineInfo[]> {
  const app = wailsApp();
  if (app?.Engines) {
    return app.Engines();
  }
  const response = await fetch("/api/engines");
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  const body = (await response.json()) as { engines: EngineInfo[] };
  return body.engines;
}

export async function testConnection(engine: string, connection: Connection): Promise<void> {
  const app = wailsApp();
  if (app?.TestConnection) {
    await app.TestConnection(engine, connection);
    return;
  }
  const response = await fetch("/api/test-connection", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ engine, connection }),
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function exportDump(
  engine: string,
  connection: Connection,
  format: string,
  fileName: string,
): Promise<ExportResult> {
  const app = wailsApp();
  if (app?.Export) {
    return app.Export(engine, connection, { format }, fileName);
  }
  const response = await fetch("/api/export", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ engine, connection, format, fileName }),
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return response.json();
}

export async function importDump(
  engine: string,
  connection: Connection,
  format: string,
  source: string | File,
  dropExisting: boolean,
): Promise<void> {
  const app = wailsApp();
  if (app?.ImportDump) {
    let src = typeof source === "string" ? source : "";
    if (!src && app.PickOpenPath) {
      src = await app.PickOpenPath();
    }
    if (!src) {
      throw new Error("open cancelled");
    }
    await app.ImportDump(engine, connection, { format, dropExisting, confirm: true }, src);
    return;
  }
  if (source instanceof File) {
    const form = new FormData();
    form.append("engine", engine);
    form.append("connection", JSON.stringify(connection));
    form.append("format", format);
    form.append("dropExisting", dropExisting ? "true" : "false");
    form.append("confirm", "true");
    form.append("file", source, source.name);
    const response = await fetch("/api/import", { method: "POST", body: form });
    if (!response.ok) {
      throw new Error(await readError(response));
    }
    return;
  }
  const response = await fetch("/api/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      engine,
      connection,
      format,
      fileName: source,
      dropExisting,
      confirm: true,
    }),
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function getDefaults(): Promise<Defaults | null> {
  if (runtimeMode() === "desktop") {
    return null;
  }
  const response = await fetch("/api/defaults");
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return response.json();
}

export function dumpDownloadURL(name: string): string {
  return `/api/dumps/${encodeURIComponent(name)}`;
}

export async function downloadDump(name: string): Promise<boolean> {
  const app = wailsApp();
  if (app) {
    return Boolean(await app.SaveDump(name));
  }
  const url = dumpDownloadURL(name);
  const response = await fetch(url, { method: "HEAD" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  // Let the browser stream the file without buffering a whole database in memory.
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  return true;
}

export async function deleteDump(name: string): Promise<void> {
  const app = wailsApp();
  if (app) {
    await app.DeleteDump(name);
    return;
  }
  const response = await fetch(dumpDownloadURL(name), { method: "DELETE" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function pickOpenPath(): Promise<string> {
  const app = wailsApp();
  if (!app?.PickOpenPath) {
    return "";
  }
  return app.PickOpenPath();
}

export async function openBackupsDir(): Promise<void> {
  const app = wailsApp();
  if (!app?.OpenBackupsDir) {
    return;
  }
  await app.OpenBackupsDir();
}

export async function listDumps(): Promise<DumpFile[]> {
  const app = wailsApp();
  if (app?.ListDumps) {
    return app.ListDumps("");
  }
  const response = await fetch("/api/dumps");
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  const body = (await response.json()) as { dumps: DumpFile[] };
  return body.dumps ?? [];
}

export async function listProfiles(): Promise<ProfileSummary[]> {
  const app = wailsApp();
  if (app?.ListProfiles) {
    return app.ListProfiles();
  }
  const response = await fetch("/api/profiles");
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  const body = (await response.json()) as { profiles: ProfileSummary[] };
  return body.profiles ?? [];
}

export async function getProfile(name: string): Promise<Profile> {
  const app = wailsApp();
  if (app?.GetProfile) {
    return app.GetProfile(name);
  }
  const response = await fetch(`/api/profiles/${encodeURIComponent(name)}`);
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  return response.json();
}

export async function putProfile(name: string, engine: string, connection: Connection): Promise<void> {
  const app = wailsApp();
  if (app?.PutProfile) {
    await app.PutProfile(name, engine, connection);
    return;
  }
  const response = await fetch(`/api/profiles/${encodeURIComponent(name)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ engine, connection }),
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function deleteProfile(name: string): Promise<void> {
  const app = wailsApp();
  if (app?.DeleteProfile) {
    await app.DeleteProfile(name);
    return;
  }
  const response = await fetch(`/api/profiles/${encodeURIComponent(name)}`, { method: "DELETE" });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function listDatabases(engine: string, connection: Connection): Promise<string[]> {
  const app = wailsApp();
  if (app?.ListDatabases) {
    return app.ListDatabases(engine, connection);
  }
  const response = await fetch("/api/databases", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ engine, connection }),
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  const body = (await response.json()) as { databases: string[] };
  return body.databases ?? [];
}
