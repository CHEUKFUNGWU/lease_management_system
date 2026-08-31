"use client";

import React, { type CSSProperties, type ReactNode } from "react";

export type BentoTileVariant = "hero" | "feature" | "metric" | "accent";

export interface BentoGridProps {
  children: ReactNode;
  columns?: number;
  gap?: number;
  className?: string;
  style?: CSSProperties;
}

export interface BentoTileProps {
  children?: ReactNode;
  span?: number; // grid-column span (1..12)
  rows?: number; // grid-row span (1..3)
  variant?: BentoTileVariant;
  title?: ReactNode;
  subtitle?: ReactNode;
  action?: ReactNode;
  className?: string;
  style?: CSSProperties;
  bodyStyle?: CSSProperties;
  noPadding?: boolean;
}

export function BentoGrid({
  children,
  columns = 12,
  gap = 16,
  className = "",
  style,
}: BentoGridProps) {
  return (
    <div
      className={`bento-grid ${className}`}
      style={{ "--bento-columns": columns, "--bento-gap": `${gap}px`, ...style } as CSSProperties}
    >
      {children}
    </div>
  );
}

export function BentoTile({
  children,
  span = 12,
  rows = 1,
  variant = "feature",
  title,
  subtitle,
  action,
  className = "",
  style,
  bodyStyle,
  noPadding = false,
}: BentoTileProps) {
  const isHero = variant === "hero";
  const isAccent = variant === "accent";

  return (
    <div
      className={`bento-tile bento-tile--${variant}${isHero ? " is-hero" : ""}${isAccent ? " is-accent" : ""} ${className}`}
      style={{ "--bento-span": span, "--bento-rows": rows, ...style } as CSSProperties}
    >
      {/* Header if title or action is provided */}
      {(title || action) && (
        <div className="bento-tile-header">
          <div>
            {title && <div className={`bento-tile-title${isHero ? " is-hero" : ""}`}>{title}</div>}
            {subtitle && <div className="bento-tile-subtitle">{subtitle}</div>}
          </div>
          {action && <div className="bento-tile-action">{action}</div>}
        </div>
      )}

      {/* Body Content */}
      <div
        className={`bento-tile-body${noPadding ? " is-no-padding" : ""}`}
        style={bodyStyle}
      >
        {children}
      </div>
    </div>
  );
}
