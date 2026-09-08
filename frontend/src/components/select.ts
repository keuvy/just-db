import { escape, icon } from "./html";

export interface SelectOption {
  value: string;
  label: string;
  description?: string;
  disabled?: boolean;
}

interface SelectConfig {
  id: string;
  label: string;
  value: string;
  options: SelectOption[];
  name?: string;
  editable?: boolean;
  placeholder?: string;
  icon?: string;
  describedBy?: string;
  disabled?: boolean;
  emptyMessage?: string;
}

function optionMarkup(id: string, options: SelectOption[], value: string): string {
  return options.map((option, index) => `<div id="${escape(id)}-option-${index}" class="select-option" role="option" aria-selected="${option.value === value}" ${option.disabled ? 'aria-disabled="true"' : ""} data-value="${escape(option.value)}" data-label="${escape(option.label)}"><span class="select-option-copy"><span class="select-option-label">${escape(option.label)}</span>${option.description ? `<span class="select-option-description">${escape(option.description)}</span>` : ""}</span>${icon("check", "select-check")}</div>`).join("");
}

// Keep ordinary inputs/buttons and bubbling form events for the app's delegated handlers.
export function customSelect(config: SelectConfig): string {
  const { id, value, options, label, editable, name, placeholder = "Choose an option" } = config;
  const selected = options.find(option => option.value === value);
  const attributes = `id="${escape(id)}" class="select-control" role="combobox" aria-label="${escape(label)}" aria-haspopup="listbox" aria-expanded="false" aria-controls="${escape(id)}-listbox" ${config.describedBy ? `aria-describedby="${escape(config.describedBy)}"` : ""} ${config.disabled ? "disabled" : ""}`;
  return `<app-select class="custom-select ${editable ? "select-editable" : ""} ${config.icon ? "select-with-icon" : ""}" data-empty-message="${escape(config.emptyMessage || (editable ? "No matching databases. Enter a name to continue." : "No options available."))}">
    <div class="select-field">${config.icon ? icon(config.icon, "select-leading-icon") : ""}${editable
      ? `<input ${attributes} ${name ? `name="${escape(name)}"` : ""} value="${escape(value)}" placeholder="${escape(placeholder)}" autocomplete="off" autocapitalize="off" spellcheck="false" aria-autocomplete="list"/>`
      : `<button type="button" ${attributes} value="${escape(value)}"><span class="select-value ${!value ? "select-placeholder" : ""}">${escape(selected?.label || placeholder)}</span>${icon("chevron", "select-chevron")}</button>${name ? `<input type="hidden" name="${escape(name)}" value="${escape(value)}" ${config.disabled ? "disabled" : ""}/>` : ""}`
    }${editable ? `<button type="button" class="select-toggle" tabindex="-1" aria-label="Show ${escape(label.toLowerCase())} options" ${config.disabled ? "disabled" : ""}>${icon("chevron", "select-chevron")}</button>` : ""}</div>
    <div class="select-popup" hidden><div id="${escape(id)}-listbox" class="select-options" role="listbox" aria-label="${escape(label)}">${optionMarkup(id, options, value)}</div><p class="select-empty" role="status" hidden></p>${editable ? '<div class="select-hint">Choose a database or enter a name</div>' : ""}</div>
  </app-select>`;
}

export class CustomSelect extends HTMLElement {
  private static opened: CustomSelect | null = null;
  private controller?: AbortController;
  private active: HTMLElement | null = null;
  private search = "";
  private lastTyped = 0;
  private expanded = false;
  private selecting = false;

  private get control(): HTMLInputElement | HTMLButtonElement { return this.querySelector(".select-control")!; }
  private get popup(): HTMLElement { return this.querySelector(".select-popup")!; }
  private get editable(): boolean { return this.control instanceof HTMLInputElement; }
  private get options(): HTMLElement[] { return Array.from(this.querySelectorAll<HTMLElement>('[role="option"]')); }
  private get available(): HTMLElement[] { return this.options.filter(option => !option.hidden && option.getAttribute("aria-disabled") !== "true"); }

  connectedCallback(): void {
    this.controller?.abort();
    this.controller = new AbortController();
    const { signal } = this.controller;
    this.addEventListener("click", this.clicked, { signal });
    this.addEventListener("keydown", this.keydown, { signal });
    this.addEventListener("input", this.typed, { signal });
    this.addEventListener("pointerdown", event => {
      if ((event.target as Element).closest(".select-popup, .select-toggle")) event.preventDefault();
    }, { signal });
    this.addEventListener("pointermove", event => {
      const option = (event.target as Element).closest<HTMLElement>('[role="option"]');
      if (option && this.available.includes(option)) this.highlight(option, false);
    }, { signal });
    this.addEventListener("focusout", event => { if (!this.contains(event.relatedTarget as Node | null)) this.close(); }, { signal });
  }

  disconnectedCallback(): void {
    this.close();
    this.controller?.abort();
  }

  setOptions(options: SelectOption[]): void {
    this.querySelector('[role="listbox"]')!.innerHTML = optionMarkup(this.control.id, options, this.control.value);
    if (this.expanded) { this.filter(this.editable ? this.control.value : ""); this.position(); }
  }

  private open(): void {
    if (this.control.disabled || this.expanded) return;
    CustomSelect.opened?.close();
    CustomSelect.opened = this;
    this.expanded = true;
    this.control.setAttribute("aria-expanded", "true");
    this.classList.add("select-open");
    this.popup.hidden = false;
    // The top layer escapes scrolling containers and the modal editor's clipping.
    // Fixed positioning remains available in older desktop webviews.
    if (typeof this.popup.showPopover === "function") {
      this.popup.setAttribute("popover", "manual");
      this.popup.showPopover();
    }
    this.filter("");
    this.position();
    this.highlight(this.available.find(option => option.dataset.value === this.control.value) || null);
    document.addEventListener("pointerdown", this.outside, true);
    document.addEventListener("scroll", this.scrolled, true);
    window.addEventListener("resize", this.resized);
    window.visualViewport?.addEventListener("resize", this.resized);
  }

  private close(): void {
    if (!this.expanded) return;
    this.expanded = false;
    if (CustomSelect.opened === this) CustomSelect.opened = null;
    if (this.popup.isConnected && this.popup.hasAttribute("popover")) this.popup.hidePopover();
    this.popup.hidden = true;
    this.control.setAttribute("aria-expanded", "false");
    this.control.removeAttribute("aria-activedescendant");
    this.classList.remove("select-open");
    this.search = "";
    this.highlight(null);
    document.removeEventListener("pointerdown", this.outside, true);
    document.removeEventListener("scroll", this.scrolled, true);
    window.removeEventListener("resize", this.resized);
    window.visualViewport?.removeEventListener("resize", this.resized);
  }

  private outside = (event: Event): void => { if (!this.contains(event.target as Node)) this.close(); };
  private scrolled = (event: Event): void => { if (!this.popup.contains(event.target as Node)) this.close(); };
  private resized = (): void => { if (this.expanded) this.position(); };

  private position(): void {
    const rect = this.control.getBoundingClientRect();
    const viewport = window.visualViewport;
    const top = viewport?.offsetTop || 0, left = viewport?.offsetLeft || 0;
    const width = viewport?.width || window.innerWidth, height = viewport?.height || window.innerHeight;
    const below = top + height - rect.bottom - 14, above = rect.top - top - 14;
    const upwards = below < Math.min(this.popup.scrollHeight || 280, 280) && above > below;
    this.popup.style.width = `${Math.min(rect.width, width - 16)}px`;
    this.popup.style.maxHeight = `${Math.max(0, Math.min(320, upwards ? above : below))}px`;
    this.popup.style.left = `${Math.max(left + 8, Math.min(rect.left, left + width - rect.width - 8))}px`;
    this.popup.style.top = `${upwards ? rect.top - this.popup.getBoundingClientRect().height - 6 : rect.bottom + 6}px`;
  }

  private filter(query: string): void {
    const needle = query.toLocaleLowerCase();
    this.options.forEach(option => {
      option.hidden = !option.dataset.label!.toLocaleLowerCase().includes(needle);
      option.setAttribute("aria-selected", String(option.dataset.value === this.control.value));
    });
    const empty = this.querySelector<HTMLElement>(".select-empty")!;
    empty.hidden = this.options.some(option => !option.hidden);
    empty.textContent = this.dataset.emptyMessage || "No options available.";
    this.highlight(null);
  }

  private highlight(option: HTMLElement | null, scroll = true): void {
    this.active?.classList.remove("is-active");
    this.active = option;
    if (option) {
      option.classList.add("is-active");
      this.control.setAttribute("aria-activedescendant", option.id);
      if (scroll) option.scrollIntoView?.({ block: "nearest" });
    } else this.control.removeAttribute("aria-activedescendant");
  }

  private choose(option: HTMLElement): void {
    if (this.control.disabled || option.getAttribute("aria-disabled") === "true") return;
    const control = this.control;
    const changed = control.value !== option.dataset.value;
    control.value = option.dataset.value!;
    const value = this.querySelector(".select-value");
    if (value) { value.textContent = option.dataset.label!; value.classList.toggle("select-placeholder", !control.value); }
    const hidden = this.querySelector<HTMLInputElement>('input[type="hidden"]');
    if (hidden) hidden.value = control.value;
    this.options.forEach(item => item.setAttribute("aria-selected", String(item === option)));
    this.close();
    control.focus({ preventScroll: true });
    if (changed) {
      this.selecting = true;
      try {
        control.dispatchEvent(new Event("input", { bubbles: true }));
        control.dispatchEvent(new Event("change", { bubbles: true }));
      } finally { this.selecting = false; }
    }
  }

  private clicked = (event: MouseEvent): void => {
    const target = event.target as Element;
    const option = target.closest<HTMLElement>('[role="option"]');
    if (option) { this.choose(option); return; }
    if (target.closest(".select-toggle") || target.closest("button.select-control")) {
      this.control.focus({ preventScroll: true });
      if (this.expanded) this.close(); else this.open();
    } else if (target === this.control) this.open();
  };

  private typed = (event: Event): void => {
    if (event.target !== this.control || !this.editable || this.control.disabled || this.selecting) return;
    this.open();
    this.filter(this.control.value);
    this.position();
  };

  private keydown = (event: KeyboardEvent): void => {
    if (event.target !== this.control || this.control.disabled || event.isComposing) return;
    const { key } = event;
    if (key === "Escape" && this.expanded) { event.preventDefault(); event.stopPropagation(); this.close(); return; }
    if (key === "Tab") { this.close(); return; }
    if (key === "ArrowDown" || key === "ArrowUp") {
      event.preventDefault();
      const wasOpen = this.expanded;
      this.open();
      const options = this.available;
      const index = this.active ? options.indexOf(this.active) : -1;
      if (!wasOpen && this.active) return;
      const next = index < 0
        ? key === "ArrowDown" ? 0 : options.length - 1
        : Math.max(0, Math.min(options.length - 1, index + (key === "ArrowDown" ? 1 : -1)));
      this.highlight(options[next] || null);
      return;
    }
    if ((key === "Home" || key === "End") && !this.editable) {
      event.preventDefault(); this.open(); this.highlight(key === "Home" ? this.available[0] || null : this.available.at(-1) || null); return;
    }
    if (key === "Enter" || key === " " && !this.editable) {
      if (this.expanded) {
        event.preventDefault();
        if (this.active) this.choose(this.active); else this.close();
      } else if (!this.editable) { event.preventDefault(); this.open(); }
      return;
    }
    if (!this.editable && key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
      event.preventDefault();
      this.open();
      const now = Date.now();
      this.search = now - this.lastTyped > 600 ? key : this.search + key;
      this.lastTyped = now;
      const needle = this.search.toLocaleLowerCase();
      const repeated = [...needle].every(letter => letter === needle[0]);
      const options = this.available;
      const start = repeated && this.active ? options.indexOf(this.active) + 1 : 0;
      const ordered = [...options.slice(start), ...options.slice(0, start)];
      this.highlight(ordered.find(option => option.dataset.label!.toLocaleLowerCase().startsWith(repeated ? needle[0] : needle)) || this.active);
    }
  };
}

if (!customElements.get("app-select")) customElements.define("app-select", CustomSelect);
