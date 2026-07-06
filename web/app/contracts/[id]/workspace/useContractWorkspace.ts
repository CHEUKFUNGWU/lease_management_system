"use client";

import { message } from "antd";
import { useEffect, useMemo, useRef, useSyncExternalStore } from "react";

import type { Language } from "../../../lib/i18n";
import { t } from "../../../lib/i18n";
import { createHTTPContractWorkspaceTransport } from "./transport";
import {
  createContractWorkspace,
  createInitialContractWorkspaceState,
} from "./workspace";

const emptyState = createInitialContractWorkspaceState();
const noopSubscribe = () => () => undefined;
const getEmptyState = () => emptyState;

interface UseContractWorkspaceOptions {
  contractId: string;
  token: string | null;
  language: Language;
}

export function useContractWorkspace({
  contractId,
  token,
  language,
}: UseContractWorkspaceOptions) {
  const languageRef = useRef(language);
  languageRef.current = language;

  const workspace = useMemo(() => {
    if (!contractId || !token) return null;

    return createContractWorkspace({
      contractId,
      transport: createHTTPContractWorkspaceTransport(contractId, token),
      notify: (notice) => {
        const translated = t(notice.key, languageRef.current, notice.params);
        const content = notice.fallback || translated;
        message[notice.kind](content);
      },
    });
  }, [contractId, token]);

  const state = useSyncExternalStore(
    workspace?.subscribe ?? noopSubscribe,
    workspace?.getSnapshot ?? getEmptyState,
    workspace?.getSnapshot ?? getEmptyState,
  );

  useEffect(() => {
    if (workspace) void workspace.load();
  }, [workspace]);

  const commands = useMemo(() => ({
    dispatch: workspace?.dispatch.bind(workspace) ?? (() => undefined),
    execute: workspace?.execute.bind(workspace) ?? (async () => false),
    reload: workspace?.load.bind(workspace) ?? (async () => undefined),
  }), [workspace]);

  return {
    state,
    ...commands,
  };
}
