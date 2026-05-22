export function parseTagString(tags?: string): string[] {
  if (!tags) return [];
  return tags
    .split(/[\n,,，;；|\s]+/)
    .map((tag) => tag.trim())
    .filter(Boolean)
    .map((tag) => (tag.startsWith("#") ? tag : `#${tag}`));
}

export function normalizeTagValues(values?: string[]): string {
  if (!values?.length) return "";
  return Array.from(
    new Set(
      values
        .map((tag) => tag.trim())
        .filter(Boolean)
        .map((tag) => (tag.startsWith("#") ? tag : `#${tag}`)),
    ),
  ).join(", ");
}

export const DEFAULT_TAG_SUGGESTIONS = parseTagString("#华东, #华南, #直营, #加盟, #旗舰店, #购物中心");
