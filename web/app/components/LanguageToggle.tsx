"use client";

import { Dropdown } from "antd";
import { GlobalOutlined } from "@ant-design/icons";
import { LANGUAGE_LABELS, SUPPORTED_LANGUAGES, useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

const SHORT_CODE: Record<string, string> = { "zh-CN": "CN", "zh-HK": "HK", en: "EN" };

/**
 * I18N-003: the language switcher, shared by the app header and the login
 * screen.
 *
 * It was written inline in AppLayout, which meant the one page reachable
 * without signing in — the login screen — had no way to change language. This
 * is the header's markup moved out verbatim rather than a second
 * implementation, so the two can never drift.
 *
 * Renders nothing while only one language is offered (see SUPPORTED_LANGUAGES).
 */
export default function LanguageToggle({ placement = "bottomRight" }: { placement?: "bottomRight" | "bottomLeft" }) {
  const { language, setLanguage } = useLanguage();

  if (SUPPORTED_LANGUAGES.length <= 1) return null;

  return (
    <Dropdown
      menu={{
        items: SUPPORTED_LANGUAGES.map((code) => ({
          key: code,
          label: LANGUAGE_LABELS[code],
          onClick: () => setLanguage(code),
        })),
      }}
      placement={placement}
    >
      <button
        type="button"
        aria-label={t("nav.language", language)}
        aria-haspopup="menu"
        className="app-language-button"
      >
        <GlobalOutlined className="app-language-icon" />
        <span className="app-language-code">{SHORT_CODE[language] ?? language}</span>
      </button>
    </Dropdown>
  );
}
