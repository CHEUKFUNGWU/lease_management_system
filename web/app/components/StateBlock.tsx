import { Button, Empty, Typography } from "antd";
import { CloseCircleFilled, InfoCircleFilled, LockFilled } from "@ant-design/icons";
import type { ReactNode } from "react";
import { t, type Language } from "../lib/i18n";
import type { DataState, DataStateKind } from "../lib/dataState";

/**
 * STATE-003: the presentation half of the data-state contract.
 *
 * classifyDataState (lib/dataState) decides *what* a page is looking at —
 * empty / actionable / failed / scope_denied / ready. StateBlock renders it,
 * one component for the whole app instead of forty hand-written <Empty>s
 * and eight hand-written error <Alert>s.
 *
 *  - empty       : a quiet line + why it is empty
 *  - actionable  : the same, plus the next-step action button (STATE-001
 *                  landing points set actionLabel; onAction runs it)
 *  - failed      : error + reason + retry (onRetry is optional; the seam's
 *                  `retry` is the natural source)
 *  - scope_denied: the permission refusal, rendered distinctly from failed
 *                  and NEVER softened (AGENTS.md red line: scope_denied
 *                  keeps its reason)
 *  - ready       : renders nothing — the page owns the data view
 *
 * The component is deliberately thin: it is the shared presentation for an
 * already-unified decision layer, so it has no business logic of its own.
 */

export interface StateBlockProps<T> {
  state: DataState<T>;
  /** Action handler for actionable (STATE-001 landing points). */
  onAction?: () => void;
  /** Retry handler for failed. Pages without retry just show the reason. */
  onRetry?: () => void;
  /** Extra content under the message (e.g. a list of affected contracts). */
  extra?: ReactNode;
  language: Language;
}

const KIND_LABEL: Record<string, string> = {
  empty: "state.empty_label",
  actionable: "state.actionable_label",
  failed: "state.failed_label",
  scope_denied: "state.scope_denied_label",
};

export function StateBlock<T>({ state, onAction, onRetry, extra, language }: StateBlockProps<T>) {
  const kind: DataStateKind = state.kind;
  if (kind === "ready") return null;

  const label = t(KIND_LABEL[kind] ?? "state.empty_label", language);

  if (kind === "scope_denied" || kind === "failed" || kind === "actionable") {
    const denied = kind === "scope_denied";
    const failed = kind === "failed";
    const description = extra ?? state.reason;
    const action = failed && onRetry ? (
      <Button size="small" onClick={onRetry}>
        {t("common.retry", language)}
      </Button>
    ) : !failed && !denied && state.actionLabel && onAction ? (
      <Button size="small" onClick={onAction}>
        {state.actionLabel}
      </Button>
    ) : null;
    const icon = denied ? <LockFilled /> : failed ? <CloseCircleFilled /> : <InfoCircleFilled />;
    return (
      <div
        className={`state-block state-block-${kind}`}
        role={failed || denied ? "alert" : "status"}
        aria-live={failed || denied ? "assertive" : "polite"}
      >
        <div className="state-block-main">
          <span className="state-block-icon" aria-hidden="true">{icon}</span>
          <div className="state-block-copy">
            <div className="state-block-title">{state.message || label}</div>
            {description ? <div className="state-block-description">{description}</div> : null}
          </div>
          {action}
        </div>
      </div>
    );
  }

  // empty — quiet, like the FIX-033 right-column density
  return (
    <div className="state-block state-block-empty">
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={state.reason || label} />
      {extra ? <Typography.Text type="secondary">{extra}</Typography.Text> : null}
    </div>
  );
}
