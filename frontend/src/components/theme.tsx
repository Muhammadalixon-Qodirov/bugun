"use client";

import { ThemeProvider, useTheme } from "next-themes";
import { createContext, use, useEffect, useState } from "react";

import { type Scheme, themeColors } from "@/lib/theme";

const TelegramSchemeContext = createContext<(scheme: Scheme | null) => void>(() => {});

/**
 * Light, dark or system theme, kept in localStorage by next-themes. Inside the
 * Telegram Mini App "system" means Telegram's own theme rather than the phone's,
 * while an explicit light or dark choice still wins.
 */
export function ThemeRoot({ nonce, children }: { nonce?: string; children: React.ReactNode }) {
  const [telegramScheme, setTelegramScheme] = useState<Scheme | null>(null);
  const [forced, setForced] = useState<Scheme | undefined>();
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange forcedTheme={forced} nonce={nonce}>
      <TelegramSchemeContext value={setTelegramScheme}>
        <TelegramSystemTheme scheme={telegramScheme} onForce={setForced} />
        <BrowserThemeColor />
        {children}
      </TelegramSchemeContext>
    </ThemeProvider>
  );
}

/** Lets the Mini App report Telegram's colour scheme (null outside Telegram). */
export function useSetTelegramScheme() {
  return use(TelegramSchemeContext);
}

/** The theme actually on screen, and the visitor's choice ("light" | "dark" | "system"). */
export function useAppTheme() {
  const { theme, setTheme, forcedTheme, resolvedTheme } = useTheme();
  const shown = (forcedTheme ?? resolvedTheme) === "dark" ? "dark" : "light";
  return { choice: theme ?? "system", setChoice: setTheme, shown: shown as Scheme };
}

function TelegramSystemTheme({ scheme, onForce }: { scheme: Scheme | null; onForce: (s: Scheme | undefined) => void }) {
  const { theme } = useTheme();
  useEffect(() => {
    onForce(scheme && (theme ?? "system") === "system" ? scheme : undefined);
  }, [scheme, theme, onForce]);
  return null;
}

/**
 * The layout's theme-color tags follow the phone's setting; once the visitor
 * picks a theme, the browser bar follows that instead.
 */
function BrowserThemeColor() {
  const { shown } = useAppTheme();
  useEffect(() => {
    for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) meta.content = themeColors[shown];
  }, [shown]);
  return null;
}
