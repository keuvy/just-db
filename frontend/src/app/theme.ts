export type Theme = "system" | "dark" | "light";

export class Appearance {
  value: Theme = "system";
  private media = window.matchMedia("(prefers-color-scheme: dark)");
  private systemChanged = () => { if (this.value === "system") this.apply(); };

  constructor() {
    try {
      const stored = localStorage.getItem("just-db-theme");
      if (stored === "dark" || stored === "light") this.value = stored;
    } catch { /* Storage is optional in embedded/private windows. */ }
    this.media.addEventListener("change", this.systemChanged);
    this.apply();
  }

  set(value: Theme): void {
    this.value = value;
    try { localStorage.setItem("just-db-theme", value); } catch { /* Keep the session preference. */ }
    this.apply();
  }

  apply(): void {
    const dark = this.value === "dark" || this.value === "system" && this.media.matches;
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    const runtime = (window as unknown as { runtime?: {
      WindowSetBackgroundColour?: (r: number, g: number, b: number, a: number) => void;
      WindowSetDarkTheme?: () => void;
      WindowSetLightTheme?: () => void;
      WindowSetSystemDefaultTheme?: () => void;
    } }).runtime;
    runtime?.WindowSetBackgroundColour?.(...(dark ? [16, 17, 20, 255] : [247, 248, 245, 255]) as [number, number, number, number]);
    if (this.value === "system") runtime?.WindowSetSystemDefaultTheme?.();
    else if (dark) runtime?.WindowSetDarkTheme?.();
    else runtime?.WindowSetLightTheme?.();
  }

  dispose(): void { this.media.removeEventListener("change", this.systemChanged); }
}
