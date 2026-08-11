"use client";

import { message } from "antd";

/**
 * Error toasts, collapsed by content.
 *
 * A page that loads four endpoints in parallel and fails on all of them was
 * stacking four identical toasts; the portfolio page fires two independent
 * loaders and did the same. The user learns nothing from the second copy, and
 * under React StrictMode every one of them doubled again.
 */
const lastShownAt = new Map<string, number>();
const DEDUPE_WINDOW_MS = 3000;

export function notifyError(text: string) {
  const content = String(text || "").trim();
  if (!content) return;

  const now = Date.now();
  const previous = lastShownAt.get(content);
  if (previous !== undefined && now - previous < DEDUPE_WINDOW_MS) return;

  // Keep the map from growing without bound on a long-lived session.
  lastShownAt.forEach((at, key) => {
    if (now - at >= DEDUPE_WINDOW_MS) lastShownAt.delete(key);
  });

  lastShownAt.set(content, now);
  message.error(content);
}
