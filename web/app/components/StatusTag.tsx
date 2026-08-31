"use client";

import React from "react";
import type { CSSProperties, ReactNode } from "react";
export type StatusKind = "success" | "processing" | "warning" | "error" | "neutral";

export function statusKindFromAntColor(color?: string | null): StatusKind {
  switch (color) {
    case "success":
    case "green":
      return "success";
    case "processing":
    case "blue":
      return "processing";
    case "warning":
    case "orange":
    case "gold":
      return "warning";
    case "error":
    case "red":
      return "error";
    default:
      return "neutral";
  }
}

interface StatusTagProps {
  kind?: StatusKind;
  children: ReactNode;
  style?: CSSProperties;
  className?: string;
  icon?: ReactNode;
}

export function StatusTag({ kind = "neutral", children, style, className, icon }: StatusTagProps) {
  return (
    <span className={`status-tag status-tag-${kind}${className ? ` ${className}` : ""}`} style={style}>
      <span aria-hidden="true" className="status-tag-icon">
        {icon || <span className="status-tag-dot" />}
      </span>
      {children}
    </span>
  );
}
