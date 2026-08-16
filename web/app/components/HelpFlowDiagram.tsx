"use client";

/**
 * HELP-001: a zero-dependency linear flow diagram rendered as inline SVG.
 *
 * Selection rationale (against mermaid): the sample tutorials are short
 * fixed linear flows; mermaid would add a ~500KB runtime dependency and a
 * separate styling surface for no maintenance gain here. Inline SVG keeps
 * node fill, stroke and text on the design tokens, and step labels come
 * from i18n so all three languages work without extra artifacts. If a page
 * tutorial ever needs branching or loops, revisit mermaid then.
 */

export interface FlowStep {
  key: string;
  label: string;
}

const NODE_W = 168;
const NODE_H = 44;
const GAP = 36;
const ARROW_W = 16;

export default function FlowDiagram({ steps }: { steps: FlowStep[] }) {
  const width = steps.length * NODE_W + (steps.length - 1) * (GAP + ARROW_W);
  const height = NODE_H + 16;
  return (
    <svg
      className="help-flow-diagram"
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label={steps.map((s) => s.label).join(" → ")}
    >
      {steps.map((step, index) => {
        const x = index * (NODE_W + GAP + ARROW_W);
        const arrowX = x + NODE_W + GAP / 2;
        return (
          <g key={step.key}>
            <rect
              x={x}
              y={8}
              width={NODE_W}
              height={NODE_H}
              rx={8}
              fill="var(--bg-surface)"
              stroke="var(--border-default)"
            />
            <text
              x={x + NODE_W / 2}
              y={8 + NODE_H / 2 + 4}
              textAnchor="middle"
              fontSize={12}
              fill="var(--fg-primary)"
            >
              {step.label}
            </text>
            {index < steps.length - 1 && (
              <>
                <line x1={x + NODE_W} y1={8 + NODE_H / 2} x2={arrowX} y2={8 + NODE_H / 2} stroke="var(--border-strong)" strokeWidth={1.5} />
                <path
                  d={`M ${arrowX} ${8 + NODE_H / 2 - 5} L ${arrowX + ARROW_W / 2} ${8 + NODE_H / 2} L ${arrowX} ${8 + NODE_H / 2 + 5}`}
                  fill="none"
                  stroke="var(--border-strong)"
                  strokeWidth={1.5}
                />
              </>
            )}
          </g>
        );
      })}
    </svg>
  );
}
