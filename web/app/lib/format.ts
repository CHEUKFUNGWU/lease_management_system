export const fmtNum = (v: number | undefined | null) =>
  v != null ? v.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : "—";
