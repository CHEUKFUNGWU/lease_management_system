/* ============================================================================
   app.js — 骨架装配、命令面板、主题与风格切换、乐观 UI
   ========================================================================== */

const App = {};

/* ── 图标：16px 单色线性，全部继承 currentColor ───────────────────── */

const ICONS = {
  grid: '<path d="M2.5 2.5h4.5v4.5H2.5zM9 2.5h4.5v4.5H9zM2.5 9h4.5v4.5H2.5zM9 9h4.5v4.5H9z"/>',
  pulse: '<path d="M1.5 8h3l2-4.5L9.5 12l2-4h3" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>',
  store: '<path d="M2.5 6.5V13h11V6.5M1.5 6.5 3 3h10l1.5 3.5zM6 13V9h4v4" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>',
  sliders: '<path d="M2.5 4.5h11M2.5 11.5h11" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/><circle cx="6" cy="4.5" r="1.8" fill="currentColor"/><circle cx="10.5" cy="11.5" r="1.8" fill="currentColor"/>',
  pie: '<path d="M8 2a6 6 0 1 0 6 6H8z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/><path d="M9.5 1.6A6 6 0 0 1 14.4 6.5H9.5z" fill="currentColor"/>',
  inbox: '<path d="M2 9.5 3.8 3h8.4L14 9.5V13H2zM2 9.5h3.2l.9 1.8h3.8l.9-1.8H14" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>',
  file: '<path d="M4 2h5l3 3v9H4zM9 2v3h3" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>',
  spark: '<path d="M8 1.5 9.4 6l4.6 1.4L9.4 8.9 8 13.5 6.6 8.9 2 7.4 6.6 6z" fill="currentColor"/>',
  calc: '<rect x="3" y="2" width="10" height="12" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.3"/><path d="M5.5 5.5h5M5.5 8.5h1.5M7.5 8.5h1M10 8.5h.5M5.5 11h1.5M8 11h2.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>',
  chart: '<path d="M2.5 13.5V7M6.5 13.5V3M10.5 13.5v-4M14 13.5V5.5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  shield: '<path d="M8 1.8 13 3.6v4.2c0 3.2-2.1 5.6-5 6.4-2.9-.8-5-3.2-5-6.4V3.6z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>',
  swatch: '<circle cx="6" cy="6" r="3.6" fill="currentColor" opacity=".85"/><circle cx="10" cy="10" r="3.6" fill="none" stroke="currentColor" stroke-width="1.3"/>',
  columns: '<rect x="2" y="2.5" width="5" height="11" rx="1" fill="none" stroke="currentColor" stroke-width="1.3"/><rect x="9" y="2.5" width="5" height="11" rx="1" fill="currentColor" opacity=".85"/>',
  search: '<circle cx="7" cy="7" r="4.3" fill="none" stroke="currentColor" stroke-width="1.4"/><path d="m10.4 10.4 3.1 3.1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>',
  moon: '<path d="M13 9.4A5.6 5.6 0 0 1 6.6 3 5.8 5.8 0 1 0 13 9.4z" fill="currentColor"/>',
  sun: '<circle cx="8" cy="8" r="3.2" fill="currentColor"/><path d="M8 1v1.8M8 13.2V15M15 8h-1.8M2.8 8H1M12.9 3.1l-1.3 1.3M4.4 11.6l-1.3 1.3M12.9 12.9l-1.3-1.3M4.4 4.4 3.1 3.1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>',
  filter: '<path d="M2 3.5h12L9.5 8.6v4.2l-3 1.7V8.6z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>',
  arrow: '<path d="M3 8h9M8.5 4.5 12 8l-3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>',
  check: '<path d="M3.5 8.5 6.5 11.5 12.5 4.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>',
  panel: '<rect x="2" y="3" width="12" height="10" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.3"/><path d="M6 3v10" stroke="currentColor" stroke-width="1.3"/>',
};

App.icon = function icon(name, size = 16) {
  const path = ICONS[name] || ICONS.grid;
  return `<svg viewBox="0 0 16 16" width="${size}" height="${size}" fill="currentColor" aria-hidden="true" focusable="false">${path}</svg>`;
};

/* ── 主题与风格：两个正交维度，各自持久化 ────────────────────────── */

const STORE_KEY = { theme: "demo.theme", skin: "demo.skin", nav: "demo.nav" };

App.prefs = {
  get(key, fallback) {
    try { return localStorage.getItem(STORE_KEY[key]) || fallback; }
    catch { return fallback; }
  },
  set(key, value) {
    try { localStorage.setItem(STORE_KEY[key], value); } catch { /* 隐私模式 */ }
  },
};

/** URL 参数 > 本地偏好 > 系统偏好。与各页 <head> 里的引导脚本保持同一优先级。 */
App.applyPrefs = function applyPrefs() {
  const root = document.documentElement;
  const params = new URLSearchParams(location.search);
  const systemDark = window.matchMedia?.("(prefers-color-scheme: dark)").matches;
  root.dataset.theme =
    params.get("theme") || App.prefs.get("theme", systemDark ? "dark" : "light");
  root.dataset.skin = params.get("skin") || App.prefs.get("skin", "restrained");
  // 被并排对比页嵌入时，风格是外层锁定的，页内不再提供切换器
  App.embedded = params.has("embed");
  if (App.embedded) root.dataset.embed = "1";
};

// 首屏之前就应用，避免明暗闪烁
App.applyPrefs();

App.setTheme = function setTheme(theme) {
  document.documentElement.dataset.theme = theme;
  App.prefs.set("theme", theme);
  App.syncToggles();
  App.emit("prefs");
};

App.setSkin = function setSkin(skin) {
  document.documentElement.dataset.skin = skin;
  App.prefs.set("skin", skin);
  App.syncToggles();
  App.emit("prefs");
};

App.toggleTheme = () =>
  App.setTheme(document.documentElement.dataset.theme === "dark" ? "light" : "dark");
App.toggleSkin = () =>
  App.setSkin(document.documentElement.dataset.skin === "expressive" ? "restrained" : "expressive");

App.toggleNav = function toggleNav() {
  const shell = document.querySelector(".shell");
  if (!shell) return;
  const next = shell.dataset.nav === "collapsed" ? "expanded" : "collapsed";
  shell.dataset.nav = next;
  App.prefs.set("nav", next);
};

App.syncToggles = function syncToggles() {
  const { theme, skin } = document.documentElement.dataset;
  document.querySelectorAll("[data-theme-btn]").forEach((btn) => {
    btn.setAttribute("aria-pressed", String(btn.dataset.themeBtn === theme));
  });
  document.querySelectorAll("[data-skin-btn]").forEach((btn) => {
    btn.setAttribute("aria-pressed", String(btn.dataset.skinBtn === skin));
  });
};

/* 简易事件总线，供并排对比页监听偏好变化 */
const listeners = new Set();
App.emit = (name) => listeners.forEach((fn) => fn(name));
App.on = (fn) => listeners.add(fn);

/* ── 骨架装配 ─────────────────────────────────────────────────────── */

App.mount = function mount(opts) {
  const shell = document.querySelector(".shell");
  if (!shell) return;
  shell.dataset.nav = App.prefs.get("nav", "expanded");

  const navHTML = DEMO.nav
    .map(
      (group) => `
      <nav class="nav-group" aria-label="${group.label}">
        <div class="nav-group__label">${group.label}</div>
        ${group.items
          .map(
            (item) => `
          <a class="nav-item" href="${item.href}"${item.id === opts.page ? ' aria-current="page"' : ""}>
            <span class="nav-item__icon">${App.icon(item.icon)}</span>
            <span class="nav-item__text">${item.text}</span>
            ${item.badge ? `<span class="nav-item__badge">${item.badge}</span>` : ""}
          </a>`
          )
          .join("")}
      </nav>`
    )
    .join("");

  const crumbs = (opts.crumbs || [])
    .map((c, i, all) =>
      i === all.length - 1
        ? `<span class="crumbs__current">${c}</span>`
        : `<span>${c}</span><span class="crumbs__sep">/</span>`
    )
    .join("");

  const header = document.createElement("div");
  header.style.display = "contents";
  header.innerHTML = `
    <div class="brand">
      <span class="brand__mark" aria-hidden="true">M</span>
      <span class="brand__name">零售经营工作站</span>
    </div>

    <header class="topbar">
      <button class="btn btn--ghost btn--icon btn--sm" data-action="toggle-nav"
              aria-label="收起或展开侧栏" title="收起 / 展开侧栏  [">
        ${App.icon("panel", 15)}
      </button>
      <div class="crumbs">${crumbs}</div>

      <div class="grow"></div>

      <button class="cmd-trigger" data-action="open-cmdk" aria-label="打开命令面板">
        ${App.icon("search", 13)}
        <span class="cmd-trigger__text">搜索门店、合同、操作…</span>
        <span class="kbd">⌘K</span>
      </button>

      <div class="segmented" role="group" aria-label="视觉风格">
        <button data-skin-btn="restrained" aria-pressed="false" title="克制版：延续现有单色体系">克制</button>
        <button data-skin-btn="expressive" aria-pressed="false" title="高质感版：报告推荐的 Linear/Vercel 式质感">高质感</button>
      </div>

      <div class="segmented" role="group" aria-label="明暗主题">
        <button data-theme-btn="light" aria-pressed="false" aria-label="浅色" title="浅色">${App.icon("sun", 13)}</button>
        <button data-theme-btn="dark" aria-pressed="false" aria-label="深色" title="深色">${App.icon("moon", 13)}</button>
      </div>
    </header>

    <aside class="side" aria-label="主导航">${navHTML}</aside>
  `;
  shell.prepend(header);

  App.mountCmdK();
  App.syncToggles();
  App.bindGlobalKeys();

  document.addEventListener("click", (event) => {
    const target = event.target.closest("[data-action]");
    if (!target) return;
    const action = target.dataset.action;
    if (action === "toggle-nav") App.toggleNav();
    if (action === "open-cmdk") App.openCmdK();
    if (action === "toggle-theme") App.toggleTheme();
    if (action === "toggle-skin") App.toggleSkin();
  });

  document.querySelectorAll("[data-theme-btn]").forEach((btn) =>
    btn.addEventListener("click", () => App.setTheme(btn.dataset.themeBtn))
  );
  document.querySelectorAll("[data-skin-btn]").forEach((btn) =>
    btn.addEventListener("click", () => App.setSkin(btn.dataset.skinBtn))
  );
};

/* ============================================================================
   命令面板
   ----------------------------------------------------------------------------
   报告 §2.2 的主张：命令面板不只是跳转，还要能直接执行操作（改筛选、切主题）。
   这里三类条目并存：跳转 / 操作 / 实体（门店、合同）。
   ========================================================================== */

let cmdkState = { open: false, query: "", active: 0, results: [], restoreFocus: null };

/** 子序列模糊匹配：连续命中加权，词首命中加权 */
function fuzzy(text, query) {
  if (!query) return { score: 0, ranges: [] };
  const lower = text.toLowerCase();
  const q = query.toLowerCase();
  let ti = 0;
  let score = 0;
  let streak = 0;
  const ranges = [];
  for (const ch of q) {
    const found = lower.indexOf(ch, ti);
    if (found === -1) return null;
    score += found === ti ? 3 + streak : 1;
    if (found === 0 || /[\s·\-/]/.test(lower[found - 1])) score += 2;
    streak = found === ti ? streak + 1 : 0;
    ranges.push(found);
    ti = found + 1;
  }
  score -= (lower.length - q.length) * 0.02;
  return { score, ranges };
}

function highlight(text, ranges) {
  if (!ranges || !ranges.length) return escapeHTML(text);
  const set = new Set(ranges);
  return [...text]
    .map((ch, i) => (set.has(i) ? `<mark>${escapeHTML(ch)}</mark>` : escapeHTML(ch)))
    .join("");
}

function escapeHTML(str) {
  return String(str).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

App.mountCmdK = function mountCmdK() {
  if (document.getElementById("cmdk")) return;

  const scrim = document.createElement("div");
  scrim.className = "scrim";
  scrim.id = "cmdk-scrim";
  scrim.dataset.open = "false";
  scrim.addEventListener("click", App.closeCmdK);

  const panel = document.createElement("div");
  panel.className = "cmdk";
  panel.id = "cmdk";
  panel.dataset.open = "false";
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-modal", "true");
  panel.setAttribute("aria-label", "命令面板");
  panel.innerHTML = `
    <div class="cmdk__input-row">
      <span class="fg-muted" aria-hidden="true">${App.icon("search", 16)}</span>
      <input class="cmdk__input" id="cmdk-input" type="text" autocomplete="off" spellcheck="false"
             placeholder="搜索门店、合同，或直接执行操作…"
             role="combobox" aria-expanded="true" aria-controls="cmdk-list" aria-autocomplete="list">
      <span class="kbd">ESC</span>
    </div>
    <div class="cmdk__list" id="cmdk-list" role="listbox" aria-label="命令结果"></div>
    <div class="cmdk__foot">
      <span class="cmdk__hint"><span class="kbd">↑</span><span class="kbd">↓</span> 选择</span>
      <span class="cmdk__hint"><span class="kbd">↵</span> 执行</span>
      <span class="cmdk__hint"><span class="kbd">G</span> 然后 <span class="kbd">P</span> 直达经营脉搏</span>
      <span class="cmdk__hint" style="margin-left:auto"><span class="kbd">⇧</span><span class="kbd">S</span> 切换风格</span>
    </div>
  `;

  document.body.append(scrim, panel);

  const input = panel.querySelector("#cmdk-input");
  input.addEventListener("input", () => {
    cmdkState.query = input.value;
    cmdkState.active = 0;
    App.renderCmdK();
  });
  input.addEventListener("keydown", (event) => {
    const { results } = cmdkState;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      cmdkState.active = (cmdkState.active + 1) % Math.max(results.length, 1);
      App.renderCmdK();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      cmdkState.active = (cmdkState.active - 1 + results.length) % Math.max(results.length, 1);
      App.renderCmdK();
    } else if (event.key === "Enter") {
      event.preventDefault();
      const item = results[cmdkState.active];
      if (item) App.runCommand(item);
    } else if (event.key === "Escape") {
      event.preventDefault();
      App.closeCmdK();
    } else if (event.key === "Tab") {
      // 焦点不允许离开面板
      event.preventDefault();
    }
  });

  document.getElementById("cmdk-list").addEventListener("click", (event) => {
    const node = event.target.closest("[data-index]");
    if (!node) return;
    const item = cmdkState.results[Number(node.dataset.index)];
    if (item) App.runCommand(item);
  });
};

App.renderCmdK = function renderCmdK() {
  const list = document.getElementById("cmdk-list");
  const query = cmdkState.query.trim();

  const scored = DEMO.commands
    .map((cmd) => {
      if (!query) return { cmd, score: 0, ranges: [] };
      const onLabel = fuzzy(cmd.label, query);
      const onSub = cmd.sub ? fuzzy(cmd.sub, query) : null;
      const onGroup = fuzzy(cmd.group, query);
      const best = [onLabel, onSub && { ...onSub, ranges: [] }, onGroup && { ...onGroup, ranges: [] }]
        .filter(Boolean)
        .sort((a, b) => b.score - a.score)[0];
      if (!best) return null;
      return { cmd, score: best.score, ranges: best === onLabel ? onLabel.ranges : [] };
    })
    .filter(Boolean);

  if (query) scored.sort((a, b) => b.score - a.score);
  cmdkState.results = scored.slice(0, 40).map((s) => ({ ...s.cmd, _ranges: s.ranges }));

  if (!cmdkState.results.length) {
    list.innerHTML = `<div class="state"><div class="state__title">没有匹配的命令</div>
      <div class="state__hint">试试门店代码（SH-0412）、合同号（LC-2026）或"切换"。</div></div>`;
    return;
  }

  let html = "";
  let lastGroup = null;
  cmdkState.results.forEach((item, index) => {
    if (item.group !== lastGroup) {
      html += `<div class="cmdk__group-label">${item.group}</div>`;
      lastGroup = item.group;
    }
    const keys = (item.keys || []).map((k) => `<span class="kbd">${k}</span>`).join("");
    html += `
      <button class="cmdk__item" role="option" data-index="${index}"
              aria-selected="${index === cmdkState.active}"
              data-active="${index === cmdkState.active}">
        <span class="cmdk__item-icon">${App.icon(item.icon, 15)}</span>
        <span class="grow truncate">
          ${highlight(item.label, item._ranges)}
          ${item.sub ? `<span class="cmdk__item-sub"> · ${escapeHTML(item.sub)}</span>` : ""}
        </span>
        <span class="cmdk__item-tail">${keys}</span>
      </button>`;
  });
  list.innerHTML = html;

  const activeNode = list.querySelector('[data-active="true"]');
  if (activeNode) activeNode.scrollIntoView({ block: "nearest" });
};

App.openCmdK = function openCmdK() {
  cmdkState.open = true;
  cmdkState.query = "";
  cmdkState.active = 0;
  cmdkState.restoreFocus = document.activeElement;
  document.getElementById("cmdk").dataset.open = "true";
  document.getElementById("cmdk-scrim").dataset.open = "true";
  const input = document.getElementById("cmdk-input");
  input.value = "";
  App.renderCmdK();
  input.focus();
};

App.closeCmdK = function closeCmdK() {
  cmdkState.open = false;
  document.getElementById("cmdk").dataset.open = "false";
  document.getElementById("cmdk-scrim").dataset.open = "false";
  cmdkState.restoreFocus?.focus?.();
};

App.runCommand = function runCommand(item) {
  if (item.href && item.href !== "#") {
    window.location.href = item.href;
    return;
  }
  App.closeCmdK();
  const map = {
    "toggle-theme": () => { App.toggleTheme(); App.toast("已切换明暗主题"); },
    "toggle-skin": () => {
      App.toggleSkin();
      const skin = document.documentElement.dataset.skin;
      App.toast(skin === "expressive" ? "已切到高质感版" : "已切到克制版");
    },
    "toggle-nav": () => App.toggleNav(),
    "filter-critical": () => App.applyFilter("severity", "critical"),
    "filter-28d": () => App.applyFilter("window", "28"),
    "filter-production": () => App.applyFilter("classification", "production"),
  };
  (map[item.action] || (() => App.toast("演示原型：该操作未接后端")))();
};

/** 命令面板直接改当前页筛选，不跳转、不丢上下文 —— 报告 §2.2 的动态操作执行器 */
App.applyFilter = function applyFilter(kind, value) {
  const handler = App.filterHandlers?.[kind];
  if (handler) { handler(value); return; }
  App.toast(`本页没有「${kind}」筛选项`);
};

/* ── 全局快捷键 ───────────────────────────────────────────────────── */

let gPending = false;
let gTimer = null;

App.bindGlobalKeys = function bindGlobalKeys() {
  document.addEventListener("keydown", (event) => {
    const inField =
      event.target.matches?.("input, textarea, select, [contenteditable='true']") &&
      event.target.id !== "cmdk-input";

    // ⌘K / Ctrl+K 全局可用，即便焦点在输入框里
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      cmdkState.open ? App.closeCmdK() : App.openCmdK();
      return;
    }
    if (cmdkState.open || inField || event.metaKey || event.ctrlKey || event.altKey) return;

    if (event.key === "Escape") { App.closeDrawer(); return; }
    if (event.key === "[") { event.preventDefault(); App.toggleNav(); return; }
    if (event.shiftKey && event.key.toLowerCase() === "d") { event.preventDefault(); App.toggleTheme(); return; }
    if (event.shiftKey && event.key.toLowerCase() === "s") { event.preventDefault(); App.toggleSkin(); return; }

    // g 前缀序列：g h / g p / g s / g c / g t
    if (!gPending && event.key.toLowerCase() === "g") {
      gPending = true;
      clearTimeout(gTimer);
      gTimer = setTimeout(() => { gPending = false; }, 1200);
      return;
    }
    if (gPending) {
      gPending = false;
      clearTimeout(gTimer);
      const routes = {
        h: "home.html", p: "pulse.html", s: "store360.html",
        c: "contracts.html", t: "closing.html", d: "tokens.html",
      };
      const href = routes[event.key.toLowerCase()];
      if (href) { event.preventDefault(); window.location.href = href; }
    }
  });
};

/* ── 抽屉 ─────────────────────────────────────────────────────────── */

App.openDrawer = function openDrawer(id) {
  const drawer = document.getElementById(id || "drawer");
  if (!drawer) return;
  drawer.dataset.open = "true";
  drawer.querySelector("[data-drawer-close]")?.focus();
};

App.closeDrawer = function closeDrawer() {
  document.querySelectorAll(".drawer[data-open='true']").forEach((d) => {
    d.dataset.open = "false";
  });
};

/* ── 吐司 ─────────────────────────────────────────────────────────── */

App.toast = function toast(text, tone = "neutral") {
  let host = document.querySelector(".toaster");
  if (!host) {
    host = document.createElement("div");
    host.className = "toaster";
    host.setAttribute("role", "status");
    host.setAttribute("aria-live", "polite");
    document.body.append(host);
  }
  const node = document.createElement("div");
  node.className = "toast";
  node.dataset.tone = tone;
  node.textContent = text;
  host.append(node);
  setTimeout(() => node.remove(), 2600);
};

/* ── 可信度条 ─────────────────────────────────────────────────────── */

App.trustBar = function trustBar(envelope) {
  const rate = (envelope.observed_store_days / envelope.expected_store_days) * 100;
  const tone = rate >= 95 ? "ok" : rate >= 85 ? "warning" : "error";
  const ready = envelope.decision_ready;

  return `
  <section class="trust" data-ready="${ready}" data-open="false">
    <button class="trust__summary" aria-expanded="false" data-trust-toggle>
      <span class="trust__verdict">
        ${ready
          ? `<span class="tag tag--success">可用于决策</span>`
          : `<span class="tag tag--warning">不足以支撑决策</span>`}
      </span>
      <span class="tag tag--simulated">模拟数据 · ${envelope.dataset_version}</span>
      <span class="trust__field">覆盖率
        <b>${rate.toFixed(1)}%</b>
        <span class="fg-muted">${envelope.observed_store_days}/${envelope.expected_store_days} store-days</span>
      </span>
      <span class="coverage-bar" data-tone="${tone}" style="width:72px" aria-hidden="true">
        <span class="coverage-bar__fill" style="width:${rate.toFixed(1)}%"></span>
      </span>
      <span class="trust__field">截至 <b>${envelope.as_of}</b></span>
      <span class="trust__field">来源 <b>${envelope.source_systems.join(", ")}</b></span>
      <span class="trust__toggle" aria-hidden="true">展开全部字段 ▾</span>
    </button>

    ${!ready ? `<div style="padding:0 var(--space-3) var(--space-2)"><p class="t-caption" style="color:var(--warning-fg);margin:0">
      ${escapeHTML(envelope.not_ready_reason)}
    </p></div>` : ""}

    <div class="trust__detail">
      <dl class="trust__dl"><dt>数据分类</dt><dd>simulated（模拟）</dd></dl>
      <dl class="trust__dl"><dt>数据集版本</dt><dd>${envelope.dataset_version}</dd></dl>
      <dl class="trust__dl"><dt>生成器版本</dt><dd>${envelope.generator_version}</dd></dl>
      <dl class="trust__dl"><dt>当前区间</dt><dd>${envelope.current}</dd></dl>
      <dl class="trust__dl"><dt>对比区间</dt><dd>${envelope.comparison}</dd></dl>
      <dl class="trust__dl"><dt>口径版本</dt><dd>${envelope.formula_version} · ${envelope.pulse_version}</dd></dl>
      <dl class="trust__dl"><dt>事实版本区间</dt><dd>${envelope.fact_version_range}</dd></dl>
      <dl class="trust__dl"><dt>币种</dt><dd>${envelope.currency}（不跨币种加总）</dd></dl>
    </div>
  </section>`;
};

App.bindTrustBar = function bindTrustBar(root = document) {
  root.querySelectorAll("[data-trust-toggle]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const section = btn.closest(".trust");
      const open = section.dataset.open === "true";
      section.dataset.open = String(!open);
      btn.setAttribute("aria-expanded", String(!open));
      btn.querySelector(".trust__toggle").textContent = open ? "展开全部字段 ▾" : "收起 ▴";
    });
  });
};

/* ── 乐观 UI 的行内编辑 ───────────────────────────────────────────── */

App.bindInlineEdit = function bindInlineEdit(root = document) {
  root.querySelectorAll(".cell-edit").forEach((cell) => {
    const original = cell.value ?? cell.textContent;
    cell.addEventListener("change", () => {
      // 界面立刻承认改动，同步中用左侧竖条表示尚未确认
      cell.dataset.dirty = "true";
      App.toast("已本地更新，正在同步…");
      setTimeout(() => {
        delete cell.dataset.dirty;
        App.toast("已保存");
      }, 900);
    });
    cell.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        cell.value = original;
        cell.blur();
      }
      if (event.key === "Enter") {
        event.preventDefault();
        cell.blur();
      }
    });
  });
};
