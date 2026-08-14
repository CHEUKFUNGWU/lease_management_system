"use client";

import React from "react";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

// DESIGN.md §9: one evidence source, rendered as a citation badge that links
// back to the evidence when a URL exists. A source without a title still
// shows its type/id so the citation is never anonymous.
export interface SourceCitationLike {
  type?: string;
  id?: string;
  title?: string;
  snippet?: string;
  url?: string;
  classification?: string;
}

export default function SourceCitation({ source }: { source: SourceCitationLike | string }) {
  const { language } = useLanguage();
  const value = typeof source === "string" ? { title: source } : source;
  const label = value.title || value.id || value.type || t("ai.citation.anonymous", language);
  const inner = (
    <span className="ai-source-citation" data-type={value.type || "source"}>
      <span className="ai-source-citation-icon" aria-hidden="true">⤴</span>
      {label}
    </span>
  );
  if (value.url) {
    return (
      <a className="ai-source-citation-link" href={value.url} target="_blank" rel="noreferrer">
        {inner}
      </a>
    );
  }
  return inner;
}
