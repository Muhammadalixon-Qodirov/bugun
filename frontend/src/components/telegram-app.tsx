"use client";

import { usePathname, useRouter } from "next/navigation";
import Script from "next/script";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { useAppTheme, useSetTelegramScheme } from "@/components/theme";
import { telegramWebAppLogin } from "@/lib/api/gen/sdk.gen";
import { problemOf } from "@/lib/errors";
import { claimTelegramLogin, isTelegramWebApp, type TelegramWebApp } from "@/lib/telegram";
import { themeColors } from "@/lib/theme";

/**
 * Makes the site behave like a native Mini App inside the Telegram bot: full
 * height at once, no accidental close while scrolling, Telegram's own back
 * button, Telegram's light/dark theme, and sign-in from Telegram's own launch
 * data. Outside Telegram it loads nothing.
 */
export function TelegramApp({ signedIn }: { signedIn: boolean }) {
  const [inTelegram] = useState(isTelegramWebApp);
  const [app, setApp] = useState<TelegramWebApp | null>(null);
  const setTelegramScheme = useSetTelegramScheme();
  const { shown } = useAppTheme();
  const router = useRouter();
  const pathname = usePathname();
  const t = useTranslations("errors");

  // A visitor without a session is signed in from the Mini App's launch data,
  // once per visit, and lands where they were headed (the login page's ?keyin=).
  useEffect(() => {
    const initData = app?.initData;
    if (signedIn || !initData || !claimTelegramLogin()) return;
    telegramWebAppLogin({ body: { init_data: initData, return_to: returnPath() } })
      .then(({ data, error }) => {
        if (data) {
          router.replace(data.next);
          router.refresh();
        } else if (problemOf(error).code === "invalid_init_data") {
          toast.error(t("codes.invalid_init_data"));
        }
      })
      .catch(() => {});
    // claimTelegramLogin() allows one try per visit, whatever re-runs this.
  }, [app, signedIn, router, t]);

  // The "system" theme follows Telegram's theme (see components/theme.tsx).
  useEffect(() => {
    if (!app) return;
    const sync = () => setTelegramScheme(app.colorScheme);
    sync();
    app.onEvent("themeChanged", sync);
    return () => app.offEvent("themeChanged", sync);
  }, [app, setTelegramScheme]);

  // Telegram's header and background match the theme on screen.
  useEffect(() => {
    if (!app) return;
    app.setHeaderColor(themeColors[shown]);
    app.setBackgroundColor(themeColors[shown]);
    if (app.isVersionAtLeast("7.10")) app.setBottomBarColor?.(themeColors[shown]);
  }, [app, shown]);

  // Telegram's back button instead of a browser one: shown everywhere but home.
  useEffect(() => {
    if (!app) return;
    const back = () => (window.history.length > 1 ? router.back() : router.push("/"));
    if (pathname === "/") app.BackButton.hide();
    else app.BackButton.show();
    app.BackButton.onClick(back);
    return () => app.BackButton.offClick(back);
  }, [app, pathname, router]);

  if (!inTelegram) return null;
  return (
    <Script
      src="https://telegram.org/js/telegram-web-app.js"
      strategy="afterInteractive"
      onReady={() => {
        const wa = window.Telegram?.WebApp;
        if (!wa) return;
        wa.ready();
        wa.expand();
        // Scrolling down must not swipe the Mini App closed (Bot API 7.7+).
        if (wa.isVersionAtLeast("7.7")) wa.disableVerticalSwipes?.();
        setApp(wa);
      }}
    />
  );
}

/** This page, or the page the login screen was asked to return to; local paths only. */
function returnPath() {
  const { pathname, search } = window.location;
  const path = pathname === "/kirish" ? (new URLSearchParams(search).get("keyin") ?? "/") : pathname + search;
  return path.startsWith("/") && !path.startsWith("//") ? path : "/";
}
