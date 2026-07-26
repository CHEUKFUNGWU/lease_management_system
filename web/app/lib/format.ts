export const fmtNum = (v: number | undefined | null) =>
  v != null ? v.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "—";

// Money is always shown in the currency it was measured in. A lease measured in
// USD that renders as "¥10,000.00" is not a formatting blemish — it states a
// different amount than the ledger holds, which is why the currency is a
// required argument rather than an option with a yuan default.
export const fmtMoney = (v: number | undefined | null, currency: string | undefined | null) => {
  if (v == null) return "—";
  const code = (currency || "").trim().toUpperCase();
  if (!code) {
    // Better a bare number than a symbol that claims a currency we were not told.
    return fmtNum(v);
  }
  try {
    return v.toLocaleString("zh-CN", {
      style: "currency",
      currency: code,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  } catch {
    // An unrecognised code still names itself, e.g. "XYZ 10,000.00".
    return `${code} ${fmtNum(v)}`;
  }
};
