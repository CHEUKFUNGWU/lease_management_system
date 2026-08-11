"use client";

import { useCallback, useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

// Shared across every hook instance on the page. Several setters commonly fire
// in the same tick (clearFilters resets seven at once); a per-instance snapshot
// would leave each one starting from the same stale query string, so only the
// last router.replace() would survive.
let buffer: { path: string; params: URLSearchParams } | null = null;

export function updateUrlStateBuffer(
  pathname: string,
  currentQuery: string,
  key: string,
  defaultValue: string,
  next: string,
) {
  // A route change invalidates the buffer; rebuild rather than carry another
  // page's query string across.
  if (!buffer || buffer.path !== pathname) {
    buffer = { path: pathname, params: new URLSearchParams(currentQuery) };
  }
  const params = buffer.params;
  if (!next || next === defaultValue) params.delete(key);
  else params.set(key, next);
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

/** Keep list filters and workflow tabs shareable without polluting history. */
export function useUrlState(key: string, defaultValue: string) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const value = searchParams.get(key) ?? defaultValue;

  useEffect(() => {
    buffer = { path: pathname, params: new URLSearchParams(searchParams.toString()) };
  }, [pathname, searchParams]);

  const setValue = useCallback(
    (next: string) => {
      router.replace(
        updateUrlStateBuffer(pathname, searchParams.toString(), key, defaultValue, next),
        { scroll: false },
      );
    },
    [defaultValue, key, pathname, router, searchParams],
  );
  return [value, setValue] as const;
}
