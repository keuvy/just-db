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
};

type WailsApp = {
  Health: () => Promise<Health>;
  Engines: () => Promise<EngineInfo[]>;
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

export async function getHealth(): Promise<Health> {
  const app = wailsApp();
  if (app?.Health) {
    return app.Health();
  }
  const response = await fetch("/api/health");
  if (!response.ok) {
    throw new Error(`health failed: ${response.status}`);
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
    throw new Error(`engines failed: ${response.status}`);
  }
  const body = (await response.json()) as { engines: EngineInfo[] };
  return body.engines;
}
