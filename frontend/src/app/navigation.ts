import type { View } from "./state";

export function destination(hash: string): { view: View; editor: boolean } {
  const route = hash.replace(/^#/, "");
  return { view: route === "dumps" ? "dumps" : route === "settings" || route === "about" ? "settings" : "workspace", editor: route === "new" };
}
