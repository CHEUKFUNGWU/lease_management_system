"use client";

import type { ReactNode } from "react";

interface PageHeaderProps {
  title: ReactNode;
  primaryAction?: ReactNode;
  secondaryAction?: ReactNode;
  /** HELP-001: a dedicated help slot — a quiet question-mark entry that
   *  opens the usage tutorial. Never mixed into primaryAction /
   *  secondaryAction: it is not an operation button. */
  help?: ReactNode;
}

/**
 * Shared business-page header: one hierarchy across every route.
 *
 * FIX-017: the header has one title and an action row. Counts and identifiers
 * ride beside the title; scope, basis, and provenance belong in the relevant
 * filter or data block instead of a second line under the title.
 */
export default function PageHeader({
  title,
  primaryAction,
  secondaryAction,
  help,
}: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="page-header-copy">
        <h1 className="page-header-title">{title}</h1>
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
