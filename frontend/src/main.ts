import "./style.css";
import * as api from "./api";
import { Application } from "./app/application";

const root = document.querySelector<HTMLElement>("#app");
if (!root) throw new Error("#app missing");

async function start(): Promise<void> {
  const preview = import.meta.env.DEV && new URLSearchParams(location.search).has("preview");
  const client = preview ? (await import("./preview")).createPreviewClient() : api;
  if (preview) {
    const banner = document.createElement("div");
    banner.className = "preview-banner";
    banner.textContent = "Design preview · Sample data · No database connections";
    document.body.prepend(banner);
    document.body.classList.add("is-preview");
  }
  const app = new Application(root!, client);
  await app.start();
  if (preview && app.state.profiles.data.length && !location.hash) await app.state.selectProfile(app.state.profiles.data[0].name);
}

void start();
