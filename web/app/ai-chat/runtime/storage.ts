import type { ChatSession, RuntimeSnapshot } from "./types";

const SESSIONS_KEY = "lease_chat_sessions";
const ACTIVE_SESSION_KEY = "lease_chat_active_session";

export interface RuntimeStorageAdapter {
  load(): RuntimeSnapshot;
  save(snapshot: RuntimeSnapshot): void;
}

export function createBrowserRuntimeStorage(
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">,
): RuntimeStorageAdapter {
  return {
    load() {
      try {
        const raw = storage.getItem(SESSIONS_KEY);
        const sessions = raw ? JSON.parse(raw) : [];
        return {
          sessions: Array.isArray(sessions) ? (sessions as ChatSession[]) : [],
          activeSessionId: storage.getItem(ACTIVE_SESSION_KEY),
        };
      } catch {
        return { sessions: [], activeSessionId: null };
      }
    },
    save(snapshot) {
      if (snapshot.sessions.length > 0) {
        storage.setItem(SESSIONS_KEY, JSON.stringify(snapshot.sessions));
      }
      if (snapshot.activeSessionId) {
        storage.setItem(ACTIVE_SESSION_KEY, snapshot.activeSessionId);
      } else {
        storage.removeItem(ACTIVE_SESSION_KEY);
      }
    },
  };
}
