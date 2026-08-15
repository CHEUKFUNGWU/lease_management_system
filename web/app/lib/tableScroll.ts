// FIX-004: AntD applies scroll.x as a fixed inner width even when the table
// has no rows, so an empty table still renders a horizontal scrollbar across
// blank space. A scroll area should only exist when there is data to scroll;
// empty tables fall back to their empty state instead.
export function tableScrollX(rowCount: number, width: number): { x: number } | undefined {
  return rowCount > 0 ? { x: width } : undefined;
}
