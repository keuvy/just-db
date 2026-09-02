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
  PickSavePath: (defaultName: string) => Promise<string>;
  PickOpenPath: () => Promise<string>;
  DefaultDumpName: (name: string, database: string, format: string) => Promise<string>;
  OpenBackupsDir: () => Promise<void>;
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
    let dest = fileName;
    if (app.PickSavePath) {
      let suggested = fileName;
      if (!suggested && app.DefaultDumpName) {
        suggested = await app.DefaultDumpName(engine, connection.database, format);
      }
      const picked = await app.PickSavePath(suggested);
      if (!picked) {
        throw new Error("save cancelled");
      }
      dest = picked;
    }
    return app.Export(engine, connection, { format }, dest);
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
  fileName: string,
  dropExisting: boolean,
): Promise<void> {
  const app = wailsApp();
  if (app?.ImportDump) {
    let src = fileName;
    if (!src && app.PickOpenPath) {
      src = await app.PickOpenPath();
    }
    if (!src) {
      throw new Error("open cancelled");
    }
    await app.ImportDump(engine, connection, { format, dropExisting, confirm: true }, src);
    return;
  }
  const response = await fetch("/api/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      engine,
      connection,
      format,
      fileName,
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
