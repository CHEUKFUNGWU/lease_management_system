"use client";

import type { ReactNode } from "react";

interface PageHeaderProps {
  title: ReactNode;
  primaryAction?: ReactNode;
  secondaryAction?: ReactNode;
  meta?: ReactNode;
  /** HELP-001: a dedicated help slot — a quiet question-mark entry that
   *  opens the usage tutorial. Never mixed into primaryAction /
   *  secondaryAction: it is not an operation button. */
  help?: ReactNode;
}

/**
 * Shared business-page header: one hierarchy across every route.
 *
 * FIX-017: there is no subtitle slot. "What this page is for" copy is gone;
 * counts and identifiers ride beside the title as a tag; the scope and
 * compliance sentences that must survive go through `meta`, which is the one
 * place a page states the basis of its numbers.
 */
export default function PageHeader({
  title,
  primaryAction,
  secondaryAction,
  meta,
  help,
}: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="page-header-copy">
        <h1 className="page-header-title">{title}</h1>
        {meta && <p className="page-header-meta">{meta}</p>}
      </div>
      {(primaryAction || secondaryAction || help) && (
        <div className="page-header-actions">
          {help && <span className="page-header-help">{help}</span>}
          {primaryAction}
          {secondaryAction}
        </div>
      )}
    </div>
  );
}
