"use client";

import React, { useEffect, useState } from "react";
import { ConfigProvider, theme as antdTheme } from "antd";
import { MotionConfig } from "framer-motion";
import { antdTheme as lightTheme, antdDarkTheme } from "../design-system/theme";

export type AppTheme = "light" | "dark";

const THEME_STORAGE_KEY = "app-theme";

/**
 * DARK-001: theme switching.
 *
 * The default follows the OS preference (prefers-color-scheme); a manual
 * choice in the header overrides it and is persisted in localStorage. The
 * active theme is written onto <html data-theme="dark"> — globals.css
 * overrides the semantic CSS variables there (equal specificity to :root,
 * later in the file, no forced-importance flag) — and AntD switches to
 * darkAlgorithm.
 *
 * The provider renders a stable context so the header toggle can read and
 * flip the current theme without prop drilling.
 */
export const ThemeContext = React.createContext<{ theme: AppTheme; setTheme: (t: AppTheme) => void }>({
  theme: "light",
  setTheme: () => undefined,
});

export function useAppTheme() {
  return React.useContext(ThemeContext);
}

function systemPrefersDark(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
}

export default function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<AppTheme>(() => {
    if (typeof window === "undefined") return "light";
    const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
    return systemPrefersDark() ? "dark" : "light";
  });

  const setTheme = (next: AppTheme) => {
    setThemeState(next);
    try {
      window.localStorage.setItem(THEME_STORAGE_KEY, next);
    } catch {
      // storage can be unavailable (private mode); the in-memory theme still works
    }
  };

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  }, [theme]);

  // Follow OS changes only while the user has not made an explicit choice.
  useEffect(() => {
    const mq = window.matchMedia?.("(prefers-color-scheme: dark)");
    if (!mq) return;
    const onChange = (e: MediaQueryListEvent) => {
      const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
      if (stored === "light" || stored === "dark") return;
      setThemeState(e.matches ? "dark" : "light");
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

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
