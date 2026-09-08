import { afterEach, describe, expect, it, vi } from "vitest";
import { customSelect, type CustomSelect } from "../src/components/select";
import { replaceRegion } from "../src/components/html";

afterEach(() => { document.body.innerHTML = ""; });

function mount(editable = false, disabled = false) {
  document.body.innerHTML = `<form>${customSelect({
    id: "choice", name: "database", label: "Database", value: "alpha", editable, disabled,
    options: [{ value: "alpha", label: "Alpha" }, { value: "blocked", label: "Blocked", disabled: true }, { value: "beta", label: "Beta" }, { value: "bravo", label: "Bravo" }],
  })}<button id="next">Next</button></form>`;
  const select = document.querySelector<CustomSelect>("app-select")!;
  const control = document.querySelector<HTMLInputElement | HTMLButtonElement>("#choice")!;
  const popup = select.querySelector<HTMLElement>(".select-popup")!;
  return { select, control, popup };
}

function key(control: HTMLElement, key: string) {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
  control.dispatchEvent(event);
  return event;
}

function type(control: HTMLInputElement | HTMLButtonElement, value: string) {
  control.value = value;
  control.dispatchEvent(new Event("input", { bubbles: true }));
}

describe("custom select interactions", () => {
  it("navigates past disabled options and commits one form change with focus retained", () => {
    const { select, control, popup } = mount();
    const change = vi.fn(), input = vi.fn();
    document.querySelector("form")!.addEventListener("change", change);
    document.querySelector("form")!.addEventListener("input", input);
    control.focus();
    key(control, "ArrowDown");
    expect(control.getAttribute("aria-expanded")).toBe("true");
    expect(control.getAttribute("aria-activedescendant")).toBe("choice-option-0");
    key(control, "ArrowDown");
    expect(control.getAttribute("aria-activedescendant")).toBe("choice-option-2");
    key(control, "Enter");
    expect(control.value).toBe("beta");
    expect(new FormData(document.querySelector("form")!).get("database")).toBe("beta");
    expect(change).toHaveBeenCalledTimes(1);
    expect(input).toHaveBeenCalledTimes(1);
    expect(select.querySelector('[aria-selected="true"]')?.getAttribute("data-value")).toBe("beta");
    expect(popup.hidden).toBe(true);
    expect(document.activeElement).toBe(control);
    expect(control.hasAttribute("aria-activedescendant")).toBe(false);
  });

  it("supports typeahead, Home and End without committing on Escape or Tab", () => {
    const { control, popup } = mount();
    key(control, "b");
    expect(control.getAttribute("aria-activedescendant")).toBe("choice-option-2");
    key(control, "b");
    expect(control.getAttribute("aria-activedescendant")).toBe("choice-option-3");
    key(control, "Home");
    expect(control.getAttribute("aria-activedescendant")).toBe("choice-option-0");
    key(control, "End");
    expect(control.getAttribute("aria-activedescendant")).toBe("choice-option-3");
    expect(key(control, "Escape").defaultPrevented).toBe(true);
    expect(control.value).toBe("alpha");
    expect(popup.hidden).toBe(true);
    control.click();
    expect(key(control, "Tab").defaultPrevented).toBe(false);
    expect(popup.hidden).toBe(true);
    expect(control.value).toBe("alpha");
  });

  it("keeps Escape inside the open control so the enclosing editor stays open", () => {
    const { select, control } = mount();
    const dialog = document.createElement("dialog");
    document.body.append(dialog);
    dialog.append(select);
    dialog.showModal();
    const escaped = vi.fn();
    dialog.addEventListener("keydown", escaped);
    control.click();
    key(control, "Escape");
    expect(escaped).not.toHaveBeenCalled();
    expect(dialog.open).toBe(true);
    key(control, "Escape");
    expect(escaped).toHaveBeenCalledTimes(1);
  });

  it("filters database suggestions while allowing an unlisted name without accepting a suggestion", () => {
    const { select, control, popup } = mount(true);
    type(control, "BR");
    expect(Array.from(select.querySelectorAll<HTMLElement>('[role="option"]')).filter(option => !option.hidden).map(option => option.dataset.value)).toEqual(["bravo"]);
    expect(control.hasAttribute("aria-activedescendant")).toBe(false);
    key(control, "Enter");
    expect(control.value).toBe("BR");
    expect(popup.hidden).toBe(true);
    type(control, "new_database");
    expect(select.querySelector<HTMLElement>(".select-empty")?.hidden).toBe(false);
    key(control, "Enter");
    expect(control.value).toBe("new_database");
    control.click();
    expect(select.querySelectorAll('[role="option"][hidden]').length).toBe(0);
  });

  it("chooses a filtered database with the keyboard or pointer and does not reopen after its input event", () => {
    const { select, control, popup } = mount(true);
    type(control, "bet");
    key(control, "ArrowDown");
    key(control, "Enter");
    expect(control.value).toBe("beta");
    expect(popup.hidden).toBe(true);
    control.click();
    select.querySelector<HTMLElement>('[data-value="bravo"]')!.click();
    expect(control.value).toBe("bravo");
    expect(popup.hidden).toBe(true);
  });

  it("closes on an outside pointer or focus move, but keeps list scrolling open", () => {
    const { select, control, popup } = mount();
    control.click();
    select.querySelector(".select-options")!.dispatchEvent(new Event("scroll"));
    expect(popup.hidden).toBe(false);
    document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
    expect(popup.hidden).toBe(true);
    control.click();
    document.querySelector<HTMLElement>("#next")!.focus();
    expect(popup.hidden).toBe(true);
  });

  it("keeps disabled controls inert and escapes labels and discovered database names", () => {
    const { select, control, popup } = mount(true, true);
    control.click();
    key(control, "ArrowDown");
    expect(popup.hidden).toBe(true);
    select.setOptions([{ value: '<img src=x onerror="bad()">', label: "<script>bad()</script>" }]);
    expect(select.querySelector("img,script")).toBeNull();
    expect(select.querySelector('[role="option"]')?.textContent).toBe("<script>bad()</script>");
  });

  it("refreshes discovered options and releases the old popup when a view rerenders", () => {
    const { select, control, popup } = mount(true);
    control.click();
    select.setOptions([{ value: "alpha", label: "Alpha" }, { value: "analytics", label: "Analytics" }]);
    expect(select.querySelector('[aria-selected="true"]')?.getAttribute("data-value")).toBe("alpha");
    type(control, "ana");
    key(control, "ArrowDown");
    key(control, "Enter");
    expect(control.value).toBe("analytics");
    control.click();
    replaceRegion(document.querySelector("form")!, customSelect({ id: "choice", label: "Database", value: "analytics", editable: true, options: [] }));
    expect(popup.hidden).toBe(true);
    expect(document.activeElement?.id).toBe("choice");
    document.querySelector<HTMLElement>("#choice")!.click();
    expect(document.querySelector("#choice")?.getAttribute("aria-expanded")).toBe("true");
  });

  it("opens above controls near the viewport edge and limits the menu width", () => {
    const { control, popup } = mount();
    vi.spyOn(control, "getBoundingClientRect").mockReturnValue({ left: window.innerWidth - 180, top: window.innerHeight - 60, bottom: window.innerHeight - 20, width: 240, height: 40 } as DOMRect);
    vi.spyOn(popup, "getBoundingClientRect").mockReturnValue({ height: 160 } as DOMRect);
    control.click();
    expect(parseFloat(popup.style.top)).toBe(window.innerHeight - 226);
    expect(parseFloat(popup.style.left) + parseFloat(popup.style.width)).toBeLessThanOrEqual(window.innerWidth - 8);
  });
});
