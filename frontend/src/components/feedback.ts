import { escape, icon } from "./html";

export function confirmDialog(title: string, description: string, action: string, destructive = false): Promise<boolean> {
  return new Promise(resolve => {
    const trigger = document.activeElement as HTMLElement | null;
    const dialog = document.createElement("dialog");
    dialog.className = "confirm-dialog";
    dialog.setAttribute("aria-labelledby", "confirmation-title");
    dialog.setAttribute("aria-describedby", "confirmation-description");
    dialog.innerHTML = `<div class="dialog-body"><span class="dialog-symbol ${destructive ? "danger-text" : ""}">${icon(destructive ? "alert" : "edit")}</span><h2 id="confirmation-title">${escape(title)}</h2><p id="confirmation-description">${escape(description)}</p></div><form method="dialog" class="dialog-footer"><button value="cancel" autofocus>Cancel</button><button class="${destructive ? "danger" : "primary"}" value="confirm">${escape(action)}</button></form>`;
    dialog.addEventListener("close", () => {
      const accepted = dialog.returnValue === "confirm";
      dialog.remove();
      if (trigger?.isConnected) trigger.focus();
      resolve(accepted);
    }, { once: true });
    document.body.append(dialog);
    dialog.showModal();
  });
}
