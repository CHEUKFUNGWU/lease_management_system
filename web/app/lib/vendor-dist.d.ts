/**
 * Browser bundles for the export libraries (M3). The npm main entries pull
 * in node:fs / node:https which webpack cannot bundle for the client; the
 * dist bundles are browser-safe UMD builds of the same libraries.
 */
declare module "exceljs/dist/exceljs.min.js" {
  import ExcelJS from "exceljs";
  export default ExcelJS;
}
