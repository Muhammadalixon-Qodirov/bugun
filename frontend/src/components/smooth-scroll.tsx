"use client";

import type Lenis from "lenis";
import { useEffect } from "react";

import { isTelegramWebApp } from "@/lib/telegram";

// Scrollable popups keep their own native scrolling.
const nativeScroll = "[data-lenis-prevent],[role=dialog],[role=menu],[role=listbox],[data-slot=sheet-content]";

/**
 * Eases wheel and trackpad scrolling on desktop. Touch screens, the Telegram
 * Mini App and "reduce motion" users keep native scrolling, and never download
 * Lenis at all.
 */
export function SmoothScroll() {
  useEffect(() => {
    // Touch screens (and the Telegram web view) already scroll natively and
    // smoothly; Lenis would only add work there.
    const fine = window.matchMedia("(hover: hover) and (pointer: fine)").matches;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!fine || reduced || isTelegramWebApp()) return;

    let lenis: Lenis | undefined;
    let observer: MutationObserver | undefined;
    let cancelled = false;
    import("lenis").then(({ default: Lenis }) => {
      if (cancelled) return;
      const instance = new Lenis({
        lerp: 0.14,
        autoRaf: true,
        stopInertiaOnNavigate: true,
        prevent: (node) => node.closest(nativeScroll) != null,
      });
      lenis = instance;

      // Menus and sheets lock the page with overflow: hidden on <html> or <body>;
      // pause smoothing while they are open so the page behind them stays put.
      const html = document.documentElement;
      const locked = () => [html, document.body].some((el) => /hidden|clip/.test(el.style.overflowY || el.style.overflow));
      observer = new MutationObserver(() => (locked() ? instance.stop() : instance.start()));
      observer.observe(html, { attributes: true, attributeFilter: ["style"] });
      observer.observe(document.body, { attributes: true, attributeFilter: ["style"] });
    });

    return () => {
      cancelled = true;
      observer?.disconnect();
      lenis?.destroy();
    };
  }, []);
  return null;
}
