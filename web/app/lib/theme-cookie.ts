/**
 * DARK-003: the theme cookie contract, in a module neither side owns.
 *
 * This constant is read by the root layout (a Server Component) and written by
 * the theme provider (a Client Component). It cannot live in the provider: a
 * Server Component importing a value from a `"use client"` module receives a
 * client reference, not the string — `cookies().get(THEME_COOKIE)` silently
 * returned undefined while `getAll()` plainly showed the cookie was there.
 */
export const THEME_COOKIE = "app-theme";

export type AppTheme = "light" | "dark";

/** One year; the choice is a preference, not a session. */
export const THEME_COOKIE_MAX_AGE = 60 * 60 * 24 * 365;

export function parseTheme(value: string | undefined): AppTheme {
  return value === "dark" ? "dark" : "light";
}
