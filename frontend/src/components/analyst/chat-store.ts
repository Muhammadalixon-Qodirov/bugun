import type { ChatAnswer } from "@/lib/api/gen/types.gen";

/** One turn of the conversation as the page keeps it. */
export type ChatMessage =
  | { id: string; role: "user"; text: string }
  | { id: string; role: "assistant"; answer: ChatAnswer; used?: string };

export type ChatState = { sessionId: string; messages: ChatMessage[] };

// The server remembers the conversation but does not send it back, so the
// page keeps its own copy to show after a reload.
const KEY = "bugun.chat.v1";
const MAX_MESSAGES = 60;

export const emptyChat: ChatState = { sessionId: "", messages: [] };

export function loadChat(): ChatState {
  try {
    const raw = window.localStorage.getItem(KEY);
    if (!raw) return emptyChat;
    const v = JSON.parse(raw) as Partial<ChatState>;
    if (typeof v.sessionId !== "string" || !Array.isArray(v.messages)) return emptyChat;
    return { sessionId: v.sessionId, messages: v.messages };
  } catch {
    return emptyChat;
  }
}

export function saveChat(state: ChatState) {
  try {
    if (!state.sessionId && state.messages.length === 0) {
      window.localStorage.removeItem(KEY);
      return;
    }
    window.localStorage.setItem(KEY, JSON.stringify({ sessionId: state.sessionId, messages: state.messages.slice(-MAX_MESSAGES) }));
  } catch {
    // Private windows and blocked storage: the conversation just isn't kept.
  }
}

export function newId() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

// The conversation as an external store over localStorage, read with
// useSyncExternalStore: the server renders an empty chat, the browser its copy.
let cache: ChatState | null = null;
const listeners = new Set<() => void>();

export function subscribeChat(listener: () => void) {
  listeners.add(listener);
  const onStorage = (e: StorageEvent) => {
    if (e.key !== KEY) return;
    cache = null;
    listener();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

export function chatSnapshot(): ChatState {
  cache ??= loadChat();
  return cache;
}

export function serverChatSnapshot(): ChatState {
  return emptyChat;
}

/** Updates the conversation from its latest value, keeps it and re-renders its readers. */
export function updateChat(update: (s: ChatState) => ChatState) {
  cache = update(chatSnapshot());
  saveChat(cache);
  listeners.forEach((l) => l());
}
