"use client";

import { useEffect, useRef, useState } from "react";

import { apiRequest } from "./api";

/**
 * usePageFill is the shared consumption seam for agent-produced page prefills
 * (agent-universal-pagefill-v1 P0-C). One hook, every page:
 *
 *   const fill = usePageFill({ artifactId, apply });
 *   // fill.status: idle | loading | ready | mismatch | failed
 *   // fill.payload → confirmed values (apply straight into form state)
 *   // fill.suggestions → unconfirmed values (render visibly; never defaults)
 *
 * Safety invariants:
 *  - target_page must equal the consuming page (declared page only) — a
 *    cross-page paste is refused and surfaces as `mismatch`;
 *  - the human ALWAYS drives the commit: this hook only fills state, it never
 *    POSTs anything;
 *  - suggestions stay suggestions — callers render them for confirmation and
 *    are free to drop them.
 */
export type PageFillStatus = "idle" | "loading" | "ready" | "mismatch" | "failed";

export interface PageFillValue {
	value: unknown;
	provenance?: Record<string, unknown>;
}

export interface PageFillPayload {
	target_page?: string;
	target_api?: string;
	deep_link?: string;
	payload?: Record<string, PageFillValue>;
	suggestions?: Record<string, PageFillValue>;
}

export interface PageFillState {
	status: PageFillStatus;
	payload: Record<string, unknown>;
	suggestions: Record<string, unknown>;
	error?: string;
}

export function usePageFill(options: { artifactId?: string | null; page: string; token?: string; apply?: (payload: Record<string, unknown>) => void }): PageFillState {
	const { artifactId, page, token, apply } = options;
	const appliedRef = useRef<string | null>(null);
	const [status, setStatus] = useState<PageFillStatus>("idle");
	const [fill, setFill] = useState<PageFillPayload | null>(null);
	const [error, setError] = useState<string | undefined>(undefined);

	useEffect(() => {
		if (!artifactId) return;
		let active = true;
		setStatus("loading");
		setError(undefined);
		apiRequest<{ artifact?: { data?: PageFillPayload } }>(`/api/v1/ai/chat/artifacts/${encodeURIComponent(artifactId)}`, { token })
			.then((body) => {
				if (!active) return;
				const data = body?.artifact?.data;
				if (!data) {
					setStatus("failed");
					setError("empty prefill artifact");
					return;
				}
				if ((data.target_page || "").split("?")[0] !== page) {
					setStatus("mismatch");
					return;
				}
				setFill(data);
				setStatus("ready");
			})
			.catch((err: Error) => {
				if (!active) return;
				setStatus("failed");
				setError(err.message);
			});
		return () => {
			active = false;
		};
	}, [artifactId, token, page]);

	// Confirmed payload applies once per successful load; the effect re-runs
	// safely because appliedRef pins the artifact id that was already applied.
	useEffect(() => {
		if (!apply || status !== "ready" || !fill) return;
		if (appliedRef.current === artifactId) return;
		appliedRef.current = artifactId ?? null;
		apply(fill.payload ?? {});
	}, [status, fill, apply, artifactId]);

	if (!artifactId) return { status: "idle", payload: {}, suggestions: {} };
	if (status === "ready" && fill) {
		return { status, payload: fill.payload ?? {}, suggestions: fill.suggestions ?? {} };
	}
	return { status, payload: {}, suggestions: {}, error };
}
