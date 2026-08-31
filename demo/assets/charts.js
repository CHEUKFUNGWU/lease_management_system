/* ============================================================================
   charts.js — 极简 SVG 图表原语
   ----------------------------------------------------------------------------
   为什么不用报告推荐的 Tremor / Recharts：
     1. Tremor 的默认配色是一条多彩序列（蓝紫绿橙…），直接违反本产品
        「多序列优先用灰度深浅 + 形状区分，确需多色不超过 3 色」的规则；
        改配色要覆盖它的内部 class，等于再造一次现有 globals.css 的困境。
     2. 数据缺口必须可见。多数图表库默认 connectNulls 或把 null 当 0，
        两者在本产品里都是错的——0 和「没有数据」是两件事。
     3. 这里需要的图形只有四种，自建成本低于驯服一个库的成本。

   所有图形只消费 CSS 变量（--chart-*），因此主题与风格切换自动生效。
   ========================================================================== */

const Chart = {};

const NS = "http://www.w3.org/2000/svg";

function el(name, attrs = {}, text) {
  const node = document.createElementNS(NS, name);
  for (const [key, value] of Object.entries(attrs)) {
    if (value == null) continue;
    node.setAttribute(key, String(value));
  }
  if (text != null) node.textContent = text;
  return node;
}

function frame(width, height) {
  const svg = el("svg", {
    viewBox: `0 0 ${width} ${height}`,
    width: "100%",
    height: "100%",
    preserveAspectRatio: "none",
    role: "img",
    focusable: "false",
  });
  svg.style.display = "block";
  svg.style.overflow = "visible";
  return svg;
}

function niceBounds(values) {
  const clean = values.filter((v) => v != null && Number.isFinite(v));
  if (!clean.length) return { min: 0, max: 1 };
  let min = Math.min(...clean);
  let max = Math.max(...clean);
  if (min === max) {
    min -= Math.abs(min) * 0.1 || 1;
    max += Math.abs(max) * 0.1 || 1;
  }
  // 折线不从 0 起：起点为 0 会把一条 -6% 的曲线压成一条直线，读不出变化。
  const pad = (max - min) * 0.18;
  return { min: min - pad, max: max + pad };
}

/* ── 折线图：数据缺口断开，并用竖向底纹标出缺口区间 ───────────────── */

Chart.line = function line(host, rows, opts = {}) {
  const key = opts.key || "value";
  const W = 640;
  const H = opts.height || 200;
  const padL = opts.padL ?? 8;
  const padR = 8;
  const padT = 10;
  const padB = 22;
  const svg = frame(W, H);

  const values = rows.map((r) => r[key]);
  const { min, max } = niceBounds(values);
  const innerW = W - padL - padR;
  const innerH = H - padT - padB;
  const x = (i) => padL + (rows.length === 1 ? innerW / 2 : (i / (rows.length - 1)) * innerW);
  const y = (v) => padT + innerH - ((v - min) / (max - min)) * innerH;

  // 水平网格
  for (let i = 0; i <= 3; i++) {
    const gy = padT + (i / 3) * innerH;
    svg.appendChild(
      el("line", {
        x1: padL, x2: W - padR, y1: gy, y2: gy,
        stroke: "var(--chart-grid)", "stroke-width": 1, "vector-effect": "non-scaling-stroke",
      })
    );
  }

  // 缺口区间的竖向底纹：缺失必须看得见，而不是被平滑掉
  rows.forEach((row, i) => {
    if (row[key] != null) return;
    const half = innerW / Math.max(rows.length - 1, 1) / 2;
    svg.appendChild(
      el("rect", {
        x: x(i) - half, y: padT, width: half * 2, height: innerH,
        fill: "var(--chart-grid)", opacity: 0.9,
      })
    );
  });

  // 分段折线：遇到 null 就断开，不做 connectNulls
  let segment = [];
  const flush = () => {
    if (segment.length > 1) {
      svg.appendChild(
        el("polyline", {
          points: segment.map((p) => `${p[0]},${p[1]}`).join(" "),
          fill: "none",
          stroke: "var(--chart-primary)",
          "stroke-width": 2,
          "stroke-linecap": "round",
          "stroke-linejoin": "round",
          "vector-effect": "non-scaling-stroke",
        })
      );
    } else if (segment.length === 1) {
      svg.appendChild(
        el("circle", { cx: segment[0][0], cy: segment[0][1], r: 2.5, fill: "var(--chart-primary)" })
      );
    }
    segment = [];
  };
  rows.forEach((row, i) => {
    const v = row[key];
    if (v == null) { flush(); return; }
    segment.push([x(i), y(v)]);
  });
  flush();

  // X 轴标签：稀疏取样，避免挤在一起
  const step = Math.ceil(rows.length / 7);
  rows.forEach((row, i) => {
    if (i % step !== 0 && i !== rows.length - 1) return;
    const label = el("text", {
      x: x(i), y: H - 6,
      "text-anchor": i === 0 ? "start" : i === rows.length - 1 ? "end" : "middle",
      fill: "var(--chart-axis)",
      "font-size": 10,
    }, row.date || row.label || "");
    label.style.fontVariantNumeric = "tabular-nums";
    svg.appendChild(label);
  });

  const gapCount = values.filter((v) => v == null).length;
  svg.setAttribute(
    "aria-label",
    `${opts.title || "趋势"}折线图，${rows.length} 个数据点${gapCount ? `，其中 ${gapCount} 天无数据` : ""}`
  );

  host.replaceChildren(svg);
  return svg;
};

/* ── 横向条形图：单色，不用图例 ──────────────────────────────────── */

Chart.barsH = function barsH(host, rows, opts = {}) {
  const labelW = opts.labelW ?? 92;
  const rowH = opts.rowH ?? 26;
  const gap = 6;
  const W = 400;
  const H = rows.length * rowH + (rows.length - 1) * gap;
  const svg = frame(W, H);
  svg.setAttribute("preserveAspectRatio", "xMinYMin meet");

  const max = Math.max(...rows.map((r) => r.weight ?? r.value ?? 0), 0.0001);
  const trackX = labelW + 8;
  const trackW = W - trackX - 44;

  rows.forEach((row, i) => {
    const value = row.weight ?? row.value ?? 0;
    const y = i * (rowH + gap);

    const label = el("text", {
      x: labelW, y: y + rowH / 2 + 4, "text-anchor": "end",
      fill: "var(--chart-axis)", "font-size": 11,
    }, row.label);
    svg.appendChild(label);

    svg.appendChild(
      el("rect", {
        x: trackX, y: y + 5, width: trackW, height: rowH - 10,
        rx: 2, fill: "var(--chart-grid)",
      })
    );
    svg.appendChild(
      el("rect", {
        x: trackX, y: y + 5, width: Math.max((value / max) * trackW, 2), height: rowH - 10,
        rx: 2, fill: "var(--chart-primary)",
      })
    );

    const num = el("text", {
      x: W, y: y + rowH / 2 + 4, "text-anchor": "end",
      fill: "var(--chart-axis)", "font-size": 11,
    }, value.toFixed(2));
    num.style.fontVariantNumeric = "tabular-nums";
    svg.appendChild(num);
  });

  svg.setAttribute("aria-label", `${opts.title || "构成"}条形图，${rows.length} 个类别`);
  host.replaceChildren(svg);
  return svg;
};

/* ── 贡献桥 (Contribution Bridge) ─────────────────────────────────
   各项相加必须等于总变动；对不上的残差如实显示，不用配平项抹平。 */

Chart.bridge = function bridge(host, rows, opts = {}) {
  const W = 640;
  const H = opts.height || 230;
  const padT = 16;
  const padB = 46;
  const innerH = H - padT - padB;
  const svg = frame(W, H);

  const steps = [];
  let cumulative = 0;
  rows.forEach((row) => {
    steps.push({ label: row.label, from: cumulative, to: cumulative + row.value, value: row.value });
    cumulative += row.value;
  });
  // 合计柱画的是实测总变动，不是分项之和。两者不等时差额就是残差——
  // 让合计柱去迁就分项之和，等于在图上把残差抹掉。
  const total = opts.total ?? cumulative;
  steps.push({ label: "合计变动", from: 0, to: total, value: total, isTotal: true });

  const allY = steps.flatMap((s) => [s.from, s.to]).concat(0);
  const min = Math.min(...allY);
  const max = Math.max(...allY);
  const span = max - min || 1;
  const y = (v) => padT + innerH - ((v - min) / span) * innerH;

  const colW = W / steps.length;
  const barW = Math.min(colW * 0.56, 52);

  // 零基线
  svg.appendChild(
    el("line", {
      x1: 0, x2: W, y1: y(0), y2: y(0),
      stroke: "var(--chart-axis)", "stroke-width": 1, opacity: 0.5,
      "vector-effect": "non-scaling-stroke",
    })
  );

  steps.forEach((step, i) => {
    const cx = i * colW + colW / 2;
    const top = Math.min(y(step.from), y(step.to));
    const height = Math.max(Math.abs(y(step.to) - y(step.from)), 2);
    const fill = step.isTotal
      ? "var(--chart-neutral)"
      : step.value >= 0
        ? "var(--chart-positive)"
        : "var(--chart-negative)";

    svg.appendChild(
      el("rect", { x: cx - barW / 2, y: top, width: barW, height, rx: 2, fill })
    );

    // 连接线，让累积关系可读
    if (i < steps.length - 2) {
      svg.appendChild(
        el("line", {
          x1: cx + barW / 2, x2: (i + 1) * colW + colW / 2 - barW / 2,
          y1: y(step.to), y2: y(step.to),
          stroke: "var(--chart-axis)", "stroke-width": 1,
          "stroke-dasharray": "2 2", opacity: 0.6,
          "vector-effect": "non-scaling-stroke",
        })
      );
    }

    const valueText = el("text", {
      x: cx, y: top - 5, "text-anchor": "middle",
      fill: "var(--chart-axis)", "font-size": 10,
    }, `${step.value >= 0 ? "+" : "−"}${Math.abs(Math.round(step.value / 1000))}k`);
    valueText.style.fontVariantNumeric = "tabular-nums";
    svg.appendChild(valueText);

    // 标签两行排布，避免中文标签互相压
    const words = step.label.length > 4 ? [step.label.slice(0, 4), step.label.slice(4)] : [step.label];
    words.forEach((word, wi) => {
      svg.appendChild(
        el("text", {
          x: cx, y: H - padB + 18 + wi * 13, "text-anchor": "middle",
          fill: "var(--chart-axis)", "font-size": 10,
        }, word)
      );
    });
  });

  svg.setAttribute("aria-label", `${opts.title || "贡献分解"}瀑布图，${rows.length} 项贡献`);
  host.replaceChildren(svg);
  return svg;
};

/* ── 迷你走势线：嵌在 KPI 单元格里 ───────────────────────────────── */

Chart.spark = function spark(host, values, opts = {}) {
  const W = 120;
  const H = opts.height || 28;
  const svg = frame(W, H);
  const { min, max } = niceBounds(values);
  const x = (i) => (i / Math.max(values.length - 1, 1)) * W;
  const y = (v) => H - 2 - ((v - min) / (max - min)) * (H - 4);

  let segment = [];
  const flush = () => {
    if (segment.length > 1) {
      svg.appendChild(
        el("polyline", {
          points: segment.map((p) => `${p[0]},${p[1]}`).join(" "),
          fill: "none", stroke: "var(--chart-primary)", "stroke-width": 1.5,
          "stroke-linecap": "round", "stroke-linejoin": "round",
          "vector-effect": "non-scaling-stroke",
        })
      );
    }
    segment = [];
  };
  values.forEach((v, i) => {
    if (v == null) { flush(); return; }
    segment.push([x(i), y(v)]);
  });
  flush();

  svg.setAttribute("aria-hidden", "true");
  host.replaceChildren(svg);
  return svg;
};

/* ── 同类分位条：门店位置 vs 四分位区间 ─────────────────────────── */

Chart.percentile = function percentile(host, row) {
  const W = 260;
  const H = 30;
  const svg = frame(W, H);
  svg.setAttribute("preserveAspectRatio", "xMinYMin meet");

  const lo = Math.min(row.p25, row.value) * 0.94;
  const hi = Math.max(row.p75, row.value) * 1.06;
  const x = (v) => ((v - lo) / (hi - lo)) * W;
  const mid = 13;

  // 四分位区间
  svg.appendChild(
    el("rect", {
      x: x(row.p25), y: mid - 5, width: Math.max(x(row.p75) - x(row.p25), 2), height: 10,
      rx: 2, fill: "var(--chart-grid)",
    })
  );
  // 中位数
  svg.appendChild(
    el("line", {
      x1: x(row.median), x2: x(row.median), y1: mid - 8, y2: mid + 8,
      stroke: "var(--chart-neutral)", "stroke-width": 2, "vector-effect": "non-scaling-stroke",
    })
  );
  // 本店位置：菱形，与中位数的竖线形状可分（灰度打印也读得出）
  const px = Math.max(4, Math.min(W - 4, x(row.value)));
  svg.appendChild(
    el("path", {
      d: `M ${px} ${mid - 7} L ${px + 6} ${mid} L ${px} ${mid + 7} L ${px - 6} ${mid} Z`,
      fill: row.percentile < 25 || row.percentile > 75 ? "var(--chart-negative)" : "var(--chart-primary)",
    })
  );

  svg.setAttribute(
    "aria-label",
    `${row.label}：本店 ${row.value}，同类中位数 ${row.median}，位于第 ${row.percentile} 百分位`
  );
  host.replaceChildren(svg);
  return svg;
};
