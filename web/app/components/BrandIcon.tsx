import React from "react";

interface BrandIconProps {
  size?: number | string;
  className?: string;
  variant?: "default" | "inverse" | "monochrome";
  ariaHidden?: boolean;
}

export const BrandIcon: React.FC<BrandIconProps> = ({
  size = 28,
  className = "",
  variant = "default",
  ariaHidden = true,
}) => {
  const isInverse = variant === "inverse";

  // Color tokens
  const frameColor = isInverse ? "#FFFFFF" : "#111827";
  const barColor = isInverse ? "#E5E7EB" : "#1F2937";
  const arrowColor = isInverse ? "#F3F4F6" : "#4B5563";
  const arrowHighlight = isInverse ? "#FFFFFF" : "#9CA3AF";
  const ringColor = isInverse ? "#9CA3AF" : "#6B7280";
  const hubColor = isInverse ? "#D1D5DB" : "#374151";
  const nodeColor = isInverse ? "#FFFFFF" : "#1F2937";

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 100 100"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-hidden={ariaHidden}
      style={{ display: "inline-block", verticalAlign: "middle" }}
    >
      {/* 1. Outer Hexagon Frame */}
      {/* Top-Left & Top Edge */}
      <path
        d="M 23 35 L 50 19 L 68 29.5"
        stroke={frameColor}
        strokeWidth="5.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* Left Edge & Bottom Edge */}
      <path
        d="M 23 35 L 23 66 L 50 82 L 59 77"
        stroke={frameColor}
        strokeWidth="5.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />

      {/* 2. Vertical Data Bar Charts */}
      {/* Bar 1 */}
      <path d="M 30 45 V 65 L 35 68 V 45 Z" fill={barColor} />
      {/* Bar 2 */}
      <path d="M 38 38 V 70 L 43 73 V 38 Z" fill={barColor} />
      {/* Bar 3 (Tallest) */}
      <path d="M 46 29 V 75 L 51 78 V 29 Z" fill={barColor} />
      {/* Bar 4 */}
      <path d="M 54 36 V 71 L 59 68 V 36 Z" fill={barColor} />

      {/* 3. Upward Trend / Growth Arrow */}
      <g>
        {/* Trend Polyline */}
        <path
          d="M 22 62 L 41 39 L 51 51 L 75 20"
          stroke={arrowColor}
          strokeWidth="5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {/* Inner Highlight Ridge for Geometric Depth */}
        <path
          d="M 24 61.5 L 41 40 L 51 52 L 72 24"
          stroke={arrowHighlight}
          strokeWidth="1.8"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {/* Arrowhead */}
        <path
          d="M 68 18 L 83 16 L 80 31 L 74.5 25.5 L 71 30 L 65.5 25.5 L 69 21 Z"
          fill={arrowColor}
        />
      </g>

      {/* 4. Network Hub & Nodes (Bottom Right) */}
      <g transform="translate(65, 68)">
        {/* Segmented Ring Arc */}
        <path
          d="M -3 -12 A 12 12 0 1 0 8 10"
          stroke={ringColor}
          strokeWidth="3.2"
          strokeLinecap="round"
          fill="none"
        />
        {/* Center Hub */}
        <circle cx="0" cy="0" r="5.5" fill={hubColor} />
        {/* Spokes */}
        <line x1="0" y1="0" x2="0" y2="-15" stroke={barColor} strokeWidth="2" strokeLinecap="round" />
        <line x1="0" y1="0" x2="11" y2="-9" stroke={barColor} strokeWidth="2" strokeLinecap="round" />
        <line x1="0" y1="0" x2="11" y2="9" stroke={barColor} strokeWidth="2" strokeLinecap="round" />
        <line x1="0" y1="0" x2="0" y2="15" stroke={barColor} strokeWidth="2" strokeLinecap="round" />
        {/* Satellite Nodes */}
        <circle cx="0" cy="-15" r="2.8" fill={nodeColor} />
        <circle cx="11" cy="-9" r="2.8" fill={nodeColor} />
        <circle cx="11" cy="9" r="2.8" fill={nodeColor} />
        <circle cx="0" cy="15" r="2.8" fill={nodeColor} />
      </g>
    </svg>
  );
};

export default BrandIcon;
