"use client";

import React, { createContext, useContext, useState, useEffect } from "react";

export type Language = "zh-CN" | "zh-HK" | "en";

/**
 * Languages currently offered in the UI.
 *
 * Deliberately collapsed to Simplified Chinese for now. The dictionary still
 * carries zh-HK and en for every key, and several pages (portfolio, sensitivity,
 * standards, ROI) still hold hardcoded Simplified Chinese — offering a language
 * that renders a half-translated screen costs more credibility than not
 * offering it at all.
 *
 * To offer them again: finish those pages, then list the languages here. Nothing
 * else needs to change.
 */
export const SUPPORTED_LANGUAGES: Language[] = ["zh-CN"];

export const LANGUAGE_LABELS: Record<Language, string> = {
  "zh-CN": "简体中文",
  "zh-HK": "繁體中文",
  en: "English",
};

const DEFAULT_LANGUAGE: Language = "zh-CN";
const STORAGE_KEY = "app_language";

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [language, setLanguageState] = useState<Language>(DEFAULT_LANGUAGE);

  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY) as Language | null;
    // A language stored before it was withdrawn must not strand the user in a
    // half-translated UI, so anything unsupported falls back to the default.
    if (stored && SUPPORTED_LANGUAGES.includes(stored)) {
      setLanguageState(stored);
    } else if (stored) {
      localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  const setLanguage = (lang: Language) => {
    if (!SUPPORTED_LANGUAGES.includes(lang)) return;
    localStorage.setItem(STORAGE_KEY, lang);
    setLanguageState(lang);
  };

  return (
    <LanguageContext.Provider value={{ language, setLanguage }}>
      {children}
    </LanguageContext.Provider>
  );
}

interface LanguageContextType {
  language: Language;
  setLanguage: (lang: Language) => void;
}

const LanguageContext = createContext<LanguageContextType | undefined>(undefined);

export function useLanguage() {
  const context = useContext(LanguageContext);
  if (context === undefined) {
    throw new Error("useLanguage must be used within a LanguageProvider");
  }
  return context;
}
