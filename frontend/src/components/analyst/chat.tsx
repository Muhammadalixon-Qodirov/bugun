"use client";

import { useMutation } from "@tanstack/react-query";
import { AlertTriangle, ArrowUp, Inbox, Maximize2, MessageSquarePlus, RotateCcw, SearchX, Sparkles, X } from "lucide-react";
import { m } from "motion/react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";

import { SourceLine, SourcesDisclosure, TableDisclosure } from "@/components/analyst/answer-parts";
import { ChatActionButton } from "@/components/analyst/chat-action";
import { type ChatMessage, chatSnapshot, emptyChat, newId, serverChatSnapshot, subscribeChat, updateChat } from "@/components/analyst/chat-store";
import { ease } from "@/components/motion";
import { Button } from "@/components/ui/button";
import { chatMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { ChatAnswer } from "@/lib/api/gen/types.gen";
import { problemOf } from "@/lib/errors";
import { cn } from "@/lib/utils";

type Lang = "uz" | "ru" | "en";
type Variant = "page" | "panel";
const MAX_LENGTH = 500;

const noopSubscribe = () => () => {};

/** "4 600" must not break across lines: digit groups get a no-break space (the text stays plain). */
function keepNumbersTogether(text: string) {
  return text.replace(/(\d) (?=\d{3}(?!\d))/g, "$1 ");
}

type Failure = { kind: "off" | "invalid" | "rate" | "failed"; text: string };

/**
 * The assistant as a conversation: the server keeps the context under a
 * session id, the browser keeps its own copy of the messages (shared by the
 * /tahlilchi page and the corner widget) so a reload shows them. Clarifying
 * options and proposed actions become buttons; an action does nothing until
 * its dialog is confirmed.
 */
export function Chat({
  examples,
  initialQuestion = "",
  signedIn,
  variant = "page",
  open = true,
  onClose,
  onReply,
  aside,
}: {
  examples: string[];
  initialQuestion?: string;
  signedIn: boolean;
  variant?: Variant;
  /** Panel only: whether the widget is showing, to focus and scroll when it opens. */
  open?: boolean;
  onClose?: () => void;
  /** Called when a reply arrives, e.g. to mark it while the widget is closed. */
  onReply?: () => void;
  /** Page only: a side column for wide screens, handed a way to ask a question. */
  aside?: (ask: (question: string) => void, busy: boolean) => React.ReactNode;
}) {
  const t = useTranslations("analyst");
  const locale = useLocale() as Lang;
  const router = useRouter();
  const state = useSyncExternalStore(subscribeChat, chatSnapshot, serverChatSnapshot);
  const ready = useSyncExternalStore(noopSubscribe, () => true, () => false);
  const [draft, setDraft] = useState("");
  const [failure, setFailure] = useState<Failure | null>(null);
  const end = useRef<HTMLDivElement>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLTextAreaElement>(null);
  const asked = useRef(false);
  const panel = variant === "panel";

  const chat = useMutation(chatMutation());
  const pending = chat.isPending;

  const send = useCallback(
    (raw: string, option?: { messageId: string }) => {
      const text = raw.trim();
      if (!text || text.length > MAX_LENGTH || pending) return;
      setFailure(null);
      const user: ChatMessage = { id: newId(), role: "user", text };
      updateChat((s) => ({
        ...s,
        messages: [
          ...s.messages.map((msg) => (option && msg.id === option.messageId && msg.role === "assistant" ? { ...msg, used: text } : msg)),
          user,
        ],
      }));
      setDraft("");
      chat.mutate(
        { body: { message: text, lang: locale, session_id: chatSnapshot().sessionId || undefined } },
        {
          onSuccess: (answer: ChatAnswer) => {
            updateChat((s) => ({ sessionId: answer.session_id, messages: [...s.messages, { id: newId(), role: "assistant", answer }] }));
            onReply?.();
          },
          onError: (err) => {
            const p = problemOf(err);
            if (p.status === 503 || p.code === "feature_off") setFailure({ kind: "off", text });
            else if (p.status === 422 || p.code === "question_invalid") {
              // The message never reached the conversation: give it back for editing.
              updateChat((s) => ({ ...s, messages: s.messages.filter((msg) => msg.id !== user.id) }));
              setDraft(text);
              setFailure({ kind: "invalid", text });
            } else if (p.status === 429) setFailure({ kind: "rate", text });
            else setFailure({ kind: "failed", text });
          },
        },
      );
    },
    [chat, locale, pending, onReply],
  );

  // A question handed over in the address (from the analysis page) is asked once.
  useEffect(() => {
    if (panel || !ready || asked.current || !initialQuestion.trim()) return;
    asked.current = true;
    send(initialQuestion);
    router.replace("/tahlilchi", { scroll: false });
  }, [panel, ready, initialQuestion, send, router]);

  // Keep the newest message in view: inside the panel, or on the page (also when the phone keyboard opens).
  useEffect(() => {
    if (!open) return;
    if (panel) {
      const el = scroller.current;
      el?.scrollTo({ top: el.scrollHeight, behavior: ready ? "smooth" : "auto" });
    } else {
      end.current?.scrollIntoView({ block: "end", behavior: ready ? "smooth" : "auto" });
    }
  }, [panel, open, state.messages.length, pending, failure, ready]);

  // The widget is for typing: focus the box when it opens, except on touch screens
  // where the keyboard would cover the examples.
  useEffect(() => {
    if (!panel || !open || !window.matchMedia("(pointer: fine)").matches) return;
    const id = window.setTimeout(() => input.current?.focus(), 150);
    return () => window.clearTimeout(id);
  }, [panel, open]);

  const reset = () => {
    chat.reset();
    setFailure(null);
    setDraft("");
    updateChat(() => emptyChat);
    input.current?.focus();
  };

  const retry = () => {
    if (!failure) return;
    const text = failure.text;
    // The failed question is already shown; send it again without a duplicate bubble.
    updateChat((s) => {
      const last = s.messages.at(-1);
      return last?.role === "user" && last.text === text ? { ...s, messages: s.messages.slice(0, -1) } : s;
    });
    send(text);
  };

  const lastAssistant = [...state.messages].reverse().find((msg) => msg.role === "assistant")?.id;
  const empty = ready && state.messages.length === 0 && !pending;
  const off = failure?.kind === "off";

  const offNotice = (
    <div className={cn("px-6 py-12 text-center", !panel && "rounded-3xl border bg-card")} role="status">
      <Inbox className="mx-auto size-8 text-muted-foreground" strokeWidth={1.5} aria-hidden />
      <p className="mt-4 font-semibold tracking-tight">{t("chat.offTitle")}</p>
      <p className="mx-auto mt-1 max-w-sm text-sm text-muted-foreground">{t("chat.offText")}</p>
    </div>
  );

  const conversation = (
    <>
      {!ready ? (
        <div className="h-40 animate-pulse rounded-3xl bg-muted" />
      ) : empty ? (
        <Welcome examples={examples} onPick={(q) => send(q)} compact={panel} examplesAside={!!aside} />
      ) : (
        state.messages.map((msg) =>
          msg.role === "user" ? (
            <UserBubble key={msg.id} text={msg.text} />
          ) : (
            <AssistantBubble
              key={msg.id}
              answer={msg.answer}
              used={msg.used}
              active={msg.id === lastAssistant && !pending}
              onOption={(o) => send(o, { messageId: msg.id })}
              signedIn={signedIn}
              onNavigate={onClose}
            />
          ),
        )
      )}
      {pending && <Typing />}
      {failure && failure.kind !== "invalid" && !off && (
        <m.div initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25, ease }} role="alert" className="flex items-start gap-3 rounded-2xl bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p className="min-w-0 flex-1">{failure.kind === "rate" ? t("rateLimited") : t("failed")}</p>
          <button type="button" onClick={retry} className="inline-flex shrink-0 items-center gap-1 font-medium underline-offset-4 hover:underline">
            <RotateCcw className="size-3.5" aria-hidden /> {t("chat.retry")}
          </button>
        </m.div>
      )}
    </>
  );

  const composer = (
    <Composer
      inputRef={input}
      value={draft}
      onChange={(v) => {
        setDraft(v);
        if (failure?.kind === "invalid") setFailure(null);
      }}
      onSend={() => send(draft)}
      pending={pending}
      invalid={failure?.kind === "invalid"}
      placeholder={t("chat.placeholder")}
      variant={variant}
    />
  );

  if (panel) {
    return (
      <div className="flex h-full min-h-0 flex-col" data-testid="chat">
        <header className="flex items-center gap-3 border-b px-4 py-3">
          <Avatar />
          <div className="min-w-0 flex-1">
            <p className="truncate font-semibold tracking-tight">{t("widget.title")}</p>
            <p className="truncate text-xs text-muted-foreground">{pending ? t("chat.typing") : t("widget.subtitle")}</p>
          </div>
          {state.messages.length > 0 && (
            <IconButton label={t("chat.new")} onClick={reset} disabled={pending} testId="chat-new">
              <MessageSquarePlus />
            </IconButton>
          )}
          <Link href="/tahlilchi" onClick={onClose} aria-label={t("widget.expand")} title={t("widget.expand")} className={iconButton}>
            <Maximize2 />
          </Link>
          <IconButton label={t("widget.close")} onClick={() => onClose?.()} testId="chat-close">
            <X />
          </IconButton>
        </header>
        {off ? (
          <div className="min-h-0 flex-1 overflow-y-auto">{offNotice}</div>
        ) : (
          <>
            <div ref={scroller} className="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain px-4 py-4" aria-live="polite" aria-busy={pending}>
              {conversation}
            </div>
            <div className="border-t px-3 pt-3 pb-[calc(0.5rem+var(--app-safe-bottom))] sm:pb-2">
              {composer}
              <p className="mt-1 px-2 text-center text-[11px] leading-4 text-muted-foreground">{t("disclaimer")}</p>
            </div>
          </>
        )}
      </div>
    );
  }

  if (off) return offNotice;

  const main = (
    <div className="flex min-w-0 flex-col" data-testid="chat">
      <div className="mb-4 flex min-h-9 items-center justify-between gap-3">
        <p className="text-xs text-muted-foreground">{t("disclaimer")}</p>
        {state.messages.length > 0 && (
          <Button variant="outline" size="pill" className="h-9 shrink-0 px-3.5" onClick={reset} disabled={pending} data-testid="chat-new">
            <MessageSquarePlus /> {t("chat.new")}
          </Button>
        )}
      </div>
      <div className="min-h-[40vh] space-y-5 pb-4" aria-live="polite" aria-busy={pending}>
        {conversation}
        <div ref={end} className="scroll-mb-40" />
      </div>
      {composer}
    </div>
  );
  if (!aside) return main;
  return (
    <div className="lg:grid lg:grid-cols-[minmax(0,1fr)_20rem] lg:gap-10 xl:gap-14">
      {main}
      {/* Phones have the same examples in the welcome card; the side column would only push the composer down. */}
      <aside className="hidden lg:block">
        {/* Sticky only when the window is tall enough to show the whole column. */}
        <div className="space-y-4 [@media(min-height:54rem)]:sticky [@media(min-height:54rem)]:top-24">{aside((q) => send(q), pending)}</div>
      </aside>
    </div>
  );
}

const iconButton =
  "grid size-9 shrink-0 place-items-center rounded-full text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50 [&_svg]:size-4";

function IconButton({ label, onClick, disabled, testId, children }: { label: string; onClick: () => void; disabled?: boolean; testId?: string; children: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} disabled={disabled} aria-label={label} title={label} className={iconButton} data-testid={testId}>
      {children}
    </button>
  );
}

function Welcome({
  examples,
  onPick,
  compact,
  examplesAside,
}: {
  examples: string[];
  onPick: (q: string) => void;
  compact: boolean;
  /** The side column lists the examples on wide screens. */
  examplesAside?: boolean;
}) {
  const t = useTranslations("analyst");
  return (
    <m.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease }}
      className={cn(!compact && "rounded-3xl border bg-card p-5 sm:p-7")}
    >
      <span className="grid size-10 place-items-center rounded-2xl bg-primary/10 text-primary">
        <Sparkles className="size-5" aria-hidden />
      </span>
      <h2 className={cn("mt-4 font-semibold tracking-tight", compact ? "text-base" : "text-lg")}>{t("chat.welcomeTitle")}</h2>
      <p className="mt-1 max-w-prose text-sm text-muted-foreground">{t("chat.welcomeText")}</p>
      {examplesAside && <p className="mt-5 hidden text-sm text-muted-foreground lg:block">{t("page.welcomeAside")}</p>}
      <p className={cn("eyebrow mt-5 text-muted-foreground", examplesAside && "lg:hidden")}>{t("examplesTitle")}</p>
      <div className={cn("mt-2.5 grid gap-2", !compact && "sm:grid-cols-2", examplesAside && "lg:hidden")}>
        {examples.map((ex) => (
          <button
            key={ex}
            type="button"
            onClick={() => onPick(ex)}
            className={cn(
              "rounded-2xl border text-left text-sm transition-colors hover:border-foreground/30 hover:bg-muted/50",
              compact ? "px-3.5 py-2.5" : "px-4 py-3",
            )}
          >
            {ex}
          </button>
        ))}
      </div>
    </m.div>
  );
}

function UserBubble({ text }: { text: string }) {
  return (
    <m.div initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25, ease }} className="flex justify-end">
      <p className="max-w-[min(85%,36rem)] rounded-3xl rounded-br-lg bg-foreground px-4 py-2.5 break-words whitespace-pre-line text-background" data-testid="chat-user">
        {text}
      </p>
    </m.div>
  );
}

function Avatar() {
  return (
    <span className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full bg-primary/10 text-primary" aria-hidden>
      <Sparkles className="size-4" />
    </span>
  );
}

function AssistantBubble({
  answer,
  used,
  active,
  onOption,
  signedIn,
  onNavigate,
}: {
  answer: ChatAnswer;
  used?: string;
  active: boolean;
  onOption: (option: string) => void;
  signedIn: boolean;
  onNavigate?: () => void;
}) {
  const t = useTranslations("analyst");
  const kind = answer.kind ?? "answer";
  const options = kind === "clarify" ? (answer.options ?? []) : [];
  const actions = answer.actions ?? [];
  const missing = kind === "answer" && answer.status !== "ok";
  return (
    <m.div initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.3, ease }} className="flex gap-3" data-testid="chat-answer" data-kind={kind}>
      <Avatar />
      {/* Capped so a wide page keeps lines readable; the corner panel is narrower anyway. */}
      <div className="min-w-0 max-w-3xl flex-1 space-y-3">
        <div className="rounded-3xl rounded-tl-lg border bg-card px-4 py-3 sm:px-5 sm:py-4">
          {missing && (
            <p className="mb-2 inline-flex items-center gap-1.5 rounded-full bg-warm-soft px-2.5 py-0.5 text-xs font-medium text-warm-foreground">
              <SearchX className="size-3.5" aria-hidden /> {t(`status.${answer.status as "insufficient_data" | "no_product"}`)}
            </p>
          )}
          {/* Plain text only: the answer is never read as HTML or markdown. */}
          <p className="leading-relaxed break-words whitespace-pre-line">{keepNumbersTogether(answer.text)}</p>
          {kind === "unanswered" && <p className="mt-2 text-xs text-muted-foreground">{t("chat.forwarded")}</p>}
          {answer.fallback === "limit" && <p className="mt-2 text-xs text-muted-foreground">{t("chat.busy")}</p>}
        </div>

        {options.length > 0 && (
          <div className="flex flex-wrap gap-2" role="group" aria-label={t("chat.optionsLabel")}>
            {options.map((o) => (
              <button
                key={o}
                type="button"
                disabled={!active || !!used}
                onClick={() => onOption(o)}
                className={cn(
                  "rounded-full border px-3.5 py-1.5 text-sm transition-colors",
                  used === o ? "border-foreground bg-foreground text-background" : "hover:border-foreground/40 hover:bg-muted/50",
                  "disabled:cursor-default",
                  (!active || used) && used !== o && "opacity-50",
                )}
                data-testid="chat-option"
              >
                {o}
              </button>
            ))}
          </div>
        )}

        {actions.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {actions.map((a) => (
              <ChatActionButton key={`${a.type}-${a.label}`} action={a} signedIn={signedIn} onNavigate={onNavigate} />
            ))}
          </div>
        )}

        <SourceLine sources={answer.sources} />
        {(answer.table?.rows.length || answer.sources.length > 0) && (
          <div className="flex flex-col items-start gap-2">
            {answer.table && <TableDisclosure table={answer.table} />}
            <SourcesDisclosure sources={answer.sources} />
          </div>
        )}
      </div>
    </m.div>
  );
}

function Typing() {
  const t = useTranslations("analyst");
  return (
    <div className="flex gap-3" data-testid="chat-typing">
      <Avatar />
      <div className="flex items-center gap-1 rounded-3xl rounded-tl-lg border bg-card px-4 py-3.5">
        {[0, 1, 2].map((i) => (
          <m.span
            key={i}
            className="size-1.5 rounded-full bg-muted-foreground"
            animate={{ opacity: [0.3, 1, 0.3], y: [0, -2, 0] }}
            transition={{ duration: 1, repeat: Infinity, delay: i * 0.15 }}
          />
        ))}
        <span className="sr-only" role="status">
          {t("chat.typing")}
        </span>
      </div>
    </div>
  );
}

function Composer({
  inputRef,
  value,
  onChange,
  onSend,
  pending,
  invalid,
  placeholder,
  variant,
}: {
  inputRef: React.RefObject<HTMLTextAreaElement | null>;
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  pending: boolean;
  invalid: boolean;
  placeholder: string;
  variant: Variant;
}) {
  const t = useTranslations("analyst");
  const trimmed = value.trim();
  const tooLong = trimmed.length > MAX_LENGTH;
  const panel = variant === "panel";

  // Grow with the text up to a few lines, then scroll inside.
  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, panel ? 120 : 160)}px`;
  }, [value, inputRef, panel]);

  const hint = invalid || tooLong ? t("chat.invalid", { max: MAX_LENGTH }) : panel ? null : <span className="hidden sm:inline">{t("chat.shortcut")}</span>;
  const counter = trimmed.length > MAX_LENGTH - 100;

  return (
    <div
      className={cn(
        !panel &&
          "sticky bottom-[calc(5.25rem+var(--app-safe-bottom))] z-10 -mx-1 bg-linear-to-t from-background via-background to-transparent px-1 pt-4 pb-1 sm:bottom-4",
      )}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          onSend();
        }}
        className={cn(
          "flex items-end gap-2 rounded-3xl border bg-card p-1.5 pl-4 transition-colors focus-within:border-foreground/30",
          !panel && "shadow-[0_12px_40px_-20px_color-mix(in_oklch,var(--foreground)_30%,transparent)]",
          (invalid || tooLong) && "border-destructive/50",
        )}
      >
        <textarea
          ref={inputRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              onSend();
            }
          }}
          rows={1}
          maxLength={MAX_LENGTH + 50}
          placeholder={placeholder}
          aria-label={t("label")}
          aria-invalid={invalid || tooLong}
          className={cn(
            "min-h-10 flex-1 resize-none bg-transparent py-2 text-base leading-6 outline-none placeholder:text-muted-foreground/60",
            panel ? "max-h-30" : "max-h-40",
          )}
          data-testid="chat-input"
        />
        <Button
          type="submit"
          variant="solid"
          size="icon"
          className="size-10 shrink-0 rounded-full"
          disabled={!trimmed || tooLong || pending}
          aria-label={t("submit")}
          data-testid="chat-send"
        >
          <ArrowUp />
        </Button>
      </form>
      {(hint || counter || !panel) && (
        <div className="mt-1.5 flex min-h-4 items-center justify-between gap-3 px-4 text-xs">
          <span className={cn("text-muted-foreground", (invalid || tooLong) && "text-destructive")} role={invalid ? "alert" : undefined}>
            {hint}
          </span>
          {counter && <span className={cn("num text-muted-foreground", tooLong && "text-destructive")}>{`${trimmed.length}/${MAX_LENGTH}`}</span>}
        </div>
      )}
    </div>
  );
}
