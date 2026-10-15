import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { headers } from "next/headers";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getTranslations } from "next-intl/server";

import { ChatWidget } from "@/components/analyst/chat-widget";
import { CatalogNamesProvider } from "@/components/catalog-names";
import { LimitsProvider } from "@/components/limits";
import { Providers } from "@/components/providers";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { ThemeRoot } from "@/components/theme";
import { Toaster } from "@/components/ui/sonner";
import { siteUrl } from "@/lib/env";
import { getCatalogNameMaps } from "@/lib/i18n/catalog.server";
import { getCurrentUser, getLimitsOrNull } from "@/lib/server-api";
import { telegramFlagScript } from "@/lib/telegram";
import { themeColors } from "@/lib/theme";
import "./globals.css";

const sans = Geist({ variable: "--font-sans", subsets: ["latin", "latin-ext", "cyrillic"] });
const mono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

const ogLocale: Record<string, string> = { uz: "uz_UZ", en: "en_US", ru: "ru_RU" };

export async function generateMetadata(): Promise<Metadata> {
  const [t, locale] = await Promise.all([getTranslations("site"), getLocale()]);
  return {
    metadataBase: new URL(siteUrl()),
    title: { default: t("title"), template: "%s · Bugun" },
    description: t("description"),
    applicationName: "Bugun",
    // Link previews for pages without their own openGraph (which replaces this one as a whole).
    openGraph: { type: "website", siteName: "Bugun", title: t("title"), description: t("description"), locale: ogLocale[locale] },
    twitter: { card: "summary_large_image" },
  };
}

export const viewport: Viewport = {
  // Lets the layout reach under the iPhone home indicator; --app-safe-* (globals.css) keeps controls clear of it.
  viewportFit: "cover",
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: themeColors.light },
    { media: "(prefers-color-scheme: dark)", color: themeColors.dark },
  ],
};

export default async function RootLayout({ children }: LayoutProps<"/">) {
  const [locale, catalogNameMaps, limits, nonce, signedIn] = await Promise.all([
    getLocale(),
    getCatalogNameMaps(),
    getLimitsOrNull(),
    headers().then((h) => h.get("x-nonce") ?? undefined),
    // Unknown (API trouble) counts as signed in: the Mini App then leaves sign-in alone.
    getCurrentUser().then(
      (me) => !!me,
      () => true,
    ),
  ]);
  return (
    <html lang={locale} suppressHydrationWarning className={`${sans.variable} ${mono.variable} h-full antialiased`}>
      <head>
        {/*
          Marks the Telegram Mini App while the HTML is parsed, before first paint,
          so its safe areas and height apply at once (see lib/telegram.ts). A
          next/script "beforeInteractive" runs only after the Next runtime loads.
          The Content-Security-Policy allows it by its hash (lib/csp.ts).
        */}
        <script dangerouslySetInnerHTML={{ __html: telegramFlagScript }} />
      </head>
      {/* Browser extensions (e.g. ColorZilla) add attributes to <body> before hydration. */}
      <body suppressHydrationWarning className="flex min-h-full flex-col">
        <ThemeRoot nonce={nonce}>
          <NextIntlClientProvider>
          <CatalogNamesProvider maps={catalogNameMaps}>
          <LimitsProvider limits={limits}>
          <Providers signedIn={signedIn}>
            <SiteHeader />
            <main className="flex-1 overflow-x-clip pb-[calc(7rem+var(--app-safe-bottom))] sm:pb-0">{children}</main>
            <SiteFooter />
            <Toaster richColors position="top-center" mobileOffset={{ top: "calc(12px + var(--app-safe-top))" }} />
            <ChatWidget signedIn={signedIn} />
          </Providers>
          </LimitsProvider>
          </CatalogNamesProvider>
          </NextIntlClientProvider>
        </ThemeRoot>
      </body>
    </html>
  );
}
