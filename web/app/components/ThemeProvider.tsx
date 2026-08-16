"use client";

import React from "react";
import { ConfigProvider, theme as antdTheme } from "antd";
import { MotionConfig } from "framer-motion";
import { antdTheme as lightTheme, antdDarkTheme } from "../design-system/theme";
import { THEME_COOKIE, THEME_COOKIE_MAX_AGE, type AppTheme } from "../lib/theme-cookie";

export type { AppTheme };

/**
 * DARK-003: the theme is decided by the server and never changes while a page
 * is alive.
 *
 * DARK-001 switched the ConfigProvider theme on the client. That produced a
 * defect no palette-level check could see: antd generated its styles under the
 * new theme's cache hash while the mounted elements kept the class names from
 * the first render, so in dark mode every antd component matched no rule at
 * all — the login form rendered unstyled and collapsed.
 *
 * The fix is to remove the transition rather than to chase it. The choice
 * lives in a cookie, the server reads it and renders <html data-theme>, and
 * this provider takes that value as a prop. Within one page lifetime the theme
 * is constant, so the class names on the elements and the injected styles are
 * always generated from the same config.
 *
 * The cost is a reload when the user flips the toggle, which is the honest
 * trade: a themed document is a server-rendered artifact here, not client
 * state. It also removes the flash of the wrong theme on first paint, since
 * the markup arrives already correct.
 */
export const ThemeContext = React.createContext<{ theme: AppTheme; setTheme: (t: AppTheme) => void }>({
  theme: "light",
  setTheme: () => undefined,
});

export function useAppTheme() {
  return React.useContext(ThemeContext);
}

export function writeThemeCookie(next: AppTheme) {
  document.cookie = `${THEME_COOKIE}=${next}; path=/; max-age=${THEME_COOKIE_MAX_AGE}; samesite=lax`;
}

export default function ThemeProvider({
  initialTheme,
  children,
}: {
  initialTheme: AppTheme;
  children: React.ReactNode;
}) {
  // Deliberately not state: the theme cannot change without a reload, and
  // pretending otherwise is what broke DARK-001.
  const theme = initialTheme;

  const setTheme = (next: AppTheme) => {
    if (next === theme) return;
    writeThemeCookie(next);
    // The server owns the rendered theme, so ask it for the other one.
    window.location.reload();
  };

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      <MotionConfig reducedMotion="user">
        <ConfigProvider
          theme={{
            ...(theme === "dark" ? antdDarkTheme : lightTheme),
            algorithm: theme === "dark" ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
          }}
        >
          {children}
        </ConfigProvider>
      </MotionConfig>
    </ThemeContext.Provider>
  );
}
