"use client";

import type { ReactNode } from "react";

interface PageHeaderProps {
  title: ReactNode;
  subtitle?: ReactNode;
  primaryAction?: ReactNode;
  secondaryAction?: ReactNode;
  meta?: ReactNode;
}

/** Shared business-page header: one hierarchy across every route. */
export default function PageHeader({
  title,
  subtitle,
  primaryAction,
  secondaryAction,
  meta,
}: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="page-header-copy">
        <h1 className="page-header-title">{title}</h1>
        {subtitle && <p className="page-header-subtitle">{subtitle}</p>}
        {meta}
      </div>
      {(primaryAction || secondaryAction) && (
        <div className="page-header-actions">
          {primaryAction}
          {secondaryAction}
        </div>
      )}
    </div>
  );
}
