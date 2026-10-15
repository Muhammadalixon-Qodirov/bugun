"use client";

import { Sparkles } from "lucide-react";
import dynamic from "next/dynamic";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

// The conversation code loads only when the widget is first opened.
const Chat = dynamic(() => import("@/components/analyst/chat").then((mod) => mod.Chat), {
  ssr: false,
  loading: () => (
    <div className="flex h-full flex-col gap-3 p-4" aria-busy>
      <div className="h-10 animate-pulse rounded-2xl bg-muted" />
      <div className="flex-1 animate-pulse rounded-3xl bg-muted/60" />
      <div className="h-12 animate-pulse rounded-3xl bg-muted" />
    </div>
  ),
});

const exampleKeys = ["price", "basket", "forecast", "movers"] as const;

/** Pages that already show the conversation, or where a floating button would get in the way. */
function hiddenOn(pathname: string) {
  return pathname.startsWith("/tahlilchi") || pathname.startsWith("/kirish") || pathname.startsWith("/boshlash");
}

/**
 * The assistant in the corner of every page: a button that opens the chat as
 * a window (full screen on phones). It shares the conversation with the
 * /tahlilchi page, and stays mounted once opened so a reply that arrives
 * after closing is not lost.
 */
export function ChatWidget({ signedIn }: { signedIn: boolean }) {
  const t = useTranslations("analyst");
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [unread, setUnread] = useState(false);
  const hidden = hiddenOn(pathname);

  const show = () => {
    setLoaded(true);
    setOpen(true);
    setUnread(false);
  };
  const close = useCallback(() => setOpen(false), []);
  // A reply that lands while the window is closed marks the button.
  const visible = useRef(false);
  useEffect(() => {
    visible.current = open && !hidden;
  }, [open, hidden]);
  const onReply = useCallback(() => {
    if (!visible.current) setUnread(true);
  }, []);

  // Escape closes the window.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !document.querySelector("[role=dialog][data-open]:not([data-chat-widget])")) setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  // On phones the chat covers the page: keep the page behind it from scrolling.
  useEffect(() => {
    if (!open || hidden || !window.matchMedia("(max-width: 639px)").matches) return;
    const root = document.documentElement;
    const before = root.style.overflow;
    root.style.overflow = "hidden";
    return () => {
      root.style.overflow = before;
    };
  }, [open, hidden]);

  return (
    <>
      {loaded && (
        <section
          role="dialog"
          aria-modal={false}
          aria-label={t("widget.title")}
          data-chat-widget
          data-open={open && !hidden}
          hidden={!open || hidden}
          className={cn(
            "fixed inset-0 z-45 flex flex-col overflow-hidden bg-background pt-(--app-safe-top)",
            "sm:inset-auto sm:right-5 sm:bottom-5 sm:h-[min(680px,calc(100dvh-2.5rem))] sm:w-100 sm:rounded-3xl sm:border sm:pt-0 sm:shadow-[0_24px_80px_-24px_color-mix(in_oklch,var(--foreground)_35%,transparent)]",
            "animate-in fade-in-0 slide-in-from-bottom-4 duration-200 sm:zoom-in-95 sm:origin-bottom-right",
          )}
          data-testid="chat-widget"
        >
          <Chat
            variant="panel"
            open={open && !hidden}
            onClose={close}
            onReply={onReply}
            signedIn={signedIn}
            examples={exampleKeys.map((k) => t(`examples.${k}`))}
          />
        </section>
      )}
      {!open && !hidden && (
        <button
          type="button"
          onClick={show}
          aria-label={t("widget.open")}
          aria-expanded={false}
          className={cn(
            "group fixed right-4 bottom-[calc(5.75rem+var(--app-safe-bottom))] z-40 flex h-14 items-center gap-2 rounded-full bg-foreground text-background",
            "w-14 justify-center shadow-[0_12px_32px_-8px_color-mix(in_oklch,var(--foreground)_45%,transparent)] transition-transform duration-200 hover:scale-105 active:scale-95",
            "outline-none focus-visible:ring-4 focus-visible:ring-ring/50 sm:right-5 sm:bottom-5 sm:w-auto sm:px-5",
            "animate-in fade-in-0 zoom-in-90 duration-300",
          )}
          data-testid="chat-launcher"
        >
          <Sparkles className="size-5 shrink-0 transition-transform duration-300 group-hover:rotate-12" aria-hidden />
          <span className="hidden font-medium sm:inline">{t("widget.launcher")}</span>
          {unread && (
            <span className="absolute top-0.5 right-0.5 size-3.5 rounded-full border-2 border-background bg-primary" aria-label={t("widget.unread")} data-testid="chat-unread" />
          )}
        </button>
      )}
    </>
  );
}
