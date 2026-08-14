"use client";

import React, { useState } from "react";
import { Button } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

// DESIGN.md §9: the reasoning trace, collapsed by default so the answer
// reads first and the trace is one click away.
export default function ThinkingTrace({ thinking }: { thinking: string }) {
  const { language } = useLanguage();
  const [expanded, setExpanded] = useState(false);
  if (!thinking) return null;
  return (
    <div className="ai-thinking-trace">
      <Button type="text" size="small" className="ai-thinking-trace-toggle" onClick={() => setExpanded(!expanded)} aria-expanded={expanded}>
        <span className="ai-thinking-trace-arrow">{expanded ? "▼" : "▶"}</span>
        {t("ai.thinking_process", language)}
      </Button>
      {expanded && <pre className="ai-thinking-trace-body">{thinking}</pre>}
    </div>
  );
}
