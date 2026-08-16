"use client";

import { Button, Tooltip } from "antd";
import { MoonOutlined, SunOutlined } from "@ant-design/icons";
import { useAppTheme } from "./ThemeProvider";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

/**
 * DARK-001: header toggle for light/dark. The tooltip is trilingual; the
 * icon shows the theme you would switch TO (sun when dark, moon when light).
 */
export default function ThemeToggle() {
  const { theme, setTheme } = useAppTheme();
  const { language } = useLanguage();
  const isDark = theme === "dark";
  const label = t(isDark ? "theme.switch_light" : "theme.switch_dark", language);

  return (
    <Tooltip title={label}>
      <Button
        type="text"
        className="theme-toggle"
        aria-label={label}
        icon={isDark ? <SunOutlined /> : <MoonOutlined />}
        onClick={() => setTheme(isDark ? "light" : "dark")}
      />
    </Tooltip>
  );
}
