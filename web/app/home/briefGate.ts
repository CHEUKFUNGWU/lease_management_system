import { aiChatApi } from "../lib/api";
import type { Language } from "../lib/i18n";
import type { HomeBriefResult } from "./types";

export interface HomeBriefParams {
  token: string;
  language: Language;
  message: string;
  title: string;
  filters: Record<string, string>;
}

export interface HomeBriefCacheEntry {
  key: string;
  promise: Promise<HomeBriefResult>;
}

let cached: HomeBriefCacheEntry | null = null;

export function homeBriefCacheKey(params: HomeBriefParams): string {
  return JSON.stringify({ token: params.token, language: params.language, message: params.message, filters: params.filters });
}

/**
 * B5: the auto-run brief is a real query on every home visit, so the same
 * SPA session must not repeat it. The first run's in-flight promise is
 * memoized; re-entering the home page reuses the settled result. A failure
 * is never cached — the retry button must be able to re-run.
 */
export function runHomeBrief(params: HomeBriefParams): Promise<HomeBriefResult> {
  const key = homeBriefCacheKey(params);
  if (cached && cached.key === key) return cached.promise;
  const promise = aiChatApi
    .chat(
      {
        message: params.message,
        language: params.language,
        skill_id: "retail_operations",
        skill_version: "v1",
        page_context: { page: "home", title: params.title, filters: params.filters },
      },
      params.token,
    )
    .then((response) => response as HomeBriefResult);
  promise.catch(() => {
    if (cached && cached.promise === promise) cached = null;
  });
  cached = { key, promise };
  return promise;
}

export function resetHomeBriefCache(): void {
  cached = null;
}
