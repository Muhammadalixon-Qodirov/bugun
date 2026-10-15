"use client";

import { createTranslator } from "next-intl";
import { useEffect, useSyncExternalStore } from "react";

import { defaultLocale, isLocale, LOCALE_COOKIE, type Locale } from "@/i18n/config";
import type { ErrorProps } from "@/components/route-error";
import { captureBoundaryError } from "@/lib/sentry";
import en from "@/messages/en/site.json";
import ru from "@/messages/ru/site.json";
import uz from "@/messages/uz/site.json";

import "./globals.css";

const messages = { uz, en, ru };

// The root layout failed, so there is no next-intl provider: the language comes
// straight from the switcher's cookie once in the browser.
const noSubscribe = () => () => {};
function cookieLocale(): Locale {
  const saved = document.cookie.match(new RegExp(`(?:^|; )${LOCALE_COOKIE}=([^;]+)`))?.[1];
  return isLocale(saved) ? saved : defaultLocale;
}

/** Replaces the whole page, root layout included, when even that fails. */
export default function GlobalError({ error, retry }: ErrorProps) {
  const locale = useSyncExternalStore(noSubscribe, cookieLocale, () => defaultLocale);
  useEffect(() => captureBoundaryError(error), [error]);
  const t = createTranslator({ locale, messages: { site: messages[locale] }, namespace: "site" });
  return (
    <html lang={locale}>
      <body className="grid min-h-dvh place-items-center px-4 py-16 text-center">
        <title>{t("error.title")}</title>
        <div className="max-w-md">
          <p className="eyebrow">{t("error.eyebrow")}</p>
          <h1 className="display mt-3 text-4xl sm:text-5xl">{t("error.title")}</h1>
          <p className="mt-4 leading-relaxed text-muted-foreground">{t("error.text")}</p>
          <div className="mt-8 flex flex-wrap justify-center gap-2">
            <button type="button" onClick={() => retry()} className="h-11 rounded-full bg-foreground px-6 text-sm font-medium text-background">
              {t("error.retry")}
            </button>
            {/* A full page load: the client router may be what broke. */}
            {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
            <a href="/" className="inline-flex h-11 items-center rounded-full border px-6 text-sm font-medium">
              {t("backToMarket")}
            </a>
          </div>
          {error.digest && <p className="num mt-8 text-xs text-muted-foreground">{t("error.code", { digest: error.digest })}</p>}
        </div>
      </body>
    </html>
  );
}
