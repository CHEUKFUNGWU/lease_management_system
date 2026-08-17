export const fmtNum = (v: number | undefined | null) =>
  v != null ? v.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "—";

export const fmtPct = (v: number | undefined | null) =>
  v != null ? `${(v * 100).toFixed(1)}%` : "—";

// Lease dates are calendar days, not instants: the API sends them as
// "2024-01-01T00:00:00Z" and rendering that verbatim leaks a timestamp the
// business never asked about. Slicing beats parsing here — `new Date(...)`
// would shift the day backwards for anyone west of UTC, turning a lease that
// commences on the 1st into one that commences on the 31st.
export const fmtDate = (v: string | undefined | null) => {
  if (!v) return "—";
  const match = /^(\d{4}-\d{2}-\d{2})/.exec(v.trim());
  return match ? match[1] : v;
};

// Money is always shown in the currency it was measured in. A lease measured in
// USD that renders as "¥10,000.00" is not a formatting blemish — it states a
// different amount than the ledger holds, which is why the currency is a
// required argument rather than an option with a yuan default.
export const fmtMoney = (v: number | undefined | null, currency: string | undefined | null) => {
  if (v == null) return "—";
  const negative = v < 0;
  const amount = Math.abs(v);
  const code = (currency || "").trim().toUpperCase();
  if (!code) {
    // Better a bare number than a symbol that claims a currency we were not told.
    return negative ? `(${fmtNum(amount)})` : fmtNum(amount);
  }
  try {
    const formatted = amount.toLocaleString("zh-CN", {
      style: "currency",
      currency: code,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
    return negative ? `(${formatted})` : formatted;
  } catch {
    // An unrecognised code still names itself, e.g. "XYZ 10,000.00".
    const formatted = `${code} ${fmtNum(amount)}`;
    return negative ? `(${formatted})` : formatted;
  }
};
