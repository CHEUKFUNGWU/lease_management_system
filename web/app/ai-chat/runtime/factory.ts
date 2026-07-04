import { t, type Language } from "../../lib/i18n";
import type { ChatSession, Message } from "./types";

export function generateRuntimeId() {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 11);
}

export function createWelcomeMessage(language: Language): Message {
  return {
    id: "welcome",
    role: "assistant",
    content: t("ai.welcome", language),
    timestamp: Date.now(),
  };
}

export function getSessionTitle(messages: Message[], language: Language): string {
  if (messages.length === 0) return t("ai.new_session", language);
  const firstUserMessage = messages.find((message) => message.role === "user");
  if (!firstUserMessage) return t("ai.new_session", language);
  return firstUserMessage.content.length > 20
    ? `${firstUserMessage.content.slice(0, 20)}...`
    : firstUserMessage.content;
}

export function createLocalSession(language: Language, model: string): ChatSession {
  const now = Date.now();
  return {
    id: generateRuntimeId(),
    title: t("ai.new_session", language),
    messages: [createWelcomeMessage(language)],
    createdAt: now,
    updatedAt: now,
    model,
  };
}
