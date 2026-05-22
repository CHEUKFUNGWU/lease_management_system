export const exportCSV = (rows: any[], columns: any[], filename: string) => {
  const headers = columns.map((c) => c.title).join(",");
  const body = rows
    .map((row) =>
      columns.map((c) => {
        const val = row[c.dataIndex] ?? "";
        const str = typeof val === "number" ? val.toFixed(2) : String(val);
        return str.includes(",") ? `"${str}"` : str;
      }).join(","),
    )
    .join("\n");
  const bom = "﻿";
  const blob = new Blob([bom + headers + "\n" + body], { type: "text/csv;charset=utf-8" });
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${filename}.csv`;
  a.click();
  window.URL.revokeObjectURL(url);
};

export const exportExcel = (rows: any[], columns: any[], filename: string, sheetName: string = "摊销报表") => {
  const XLSX = require("xlsx");
  const headers = columns.map((c) => c.title);
  const sheetRows = rows.map((row) => {
    const obj: Record<string, any> = {};
    columns.forEach((c) => {
      const val = row[c.dataIndex];
      obj[c.title] = val ?? "";
    });
    return obj;
  });
  const sheet = XLSX.utils.json_to_sheet(sheetRows, { header: headers });
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, sheet, sheetName);
  XLSX.writeFile(wb, `${filename}.xlsx`);
};
