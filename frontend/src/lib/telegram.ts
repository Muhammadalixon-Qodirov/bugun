// Telegram opens Mini Apps with launch parameters in the URL hash
// (#tgWebAppData=…&tgWebAppVersion=…). The hash is gone after the first
// in-app navigation, so the fact is remembered for the rest of the visit.
const FLAG = "bugun-telegram-webapp";

/**
 * The same check as an inline script in the root layout, run while the HTML is
 * parsed. Its hash is allowed by the Content-Security-Policy (lib/csp.ts), so
 * any change here changes the policy with it.
 */
export const telegramFlagScript = `try{var k="${FLAG}";if(/tgWebApp(Data|Version|Platform)=/.test(location.hash))sessionStorage.setItem(k,"1");if(sessionStorage.getItem(k)==="1")document.documentElement.classList.add("tg-webapp")}catch(e){}`;

/** True inside the Telegram bot's Mini App (its in-app web view). */
export function isTelegramWebApp() {
  if (typeof window === "undefined") return false;
  try {
    if (/tgWebApp(Data|Version|Platform)=/.test(window.location.hash)) {
      sessionStorage.setItem(FLAG, "1");
      return true;
    }
    return sessionStorage.getItem(FLAG) === "1";
  } catch {
    return false;
  }
}

/**
 * A light tap of the phone's vibration motor inside the Telegram Mini App, as
 * native apps give on tab and toggle changes. Does nothing in a browser.
 */
export function haptic(kind: "selection" | "success" | "error" = "selection") {
  const h = typeof window === "undefined" ? undefined : window.Telegram?.WebApp?.HapticFeedback;
  if (!h) return;
  if (kind === "selection") h.selectionChanged();
  else h.notificationOccurred(kind);
}

const LOGIN_TRIED = "bugun-telegram-login";

/**
 * Claims the one sign-in attempt from Telegram's launch data for this visit.
 * True only the first time, so a failing or refused sign-in never repeats.
 */
export function claimTelegramLogin() {
  try {
    if (sessionStorage.getItem(LOGIN_TRIED) === "1") return false;
    sessionStorage.setItem(LOGIN_TRIED, "1");
    return true;
  } catch {
    return false;
  }
}

/** The part of window.Telegram.WebApp (telegram-web-app.js) this site uses. */
export type TelegramWebApp = {
  /** Signed launch data for the API (empty when opened outside a bot button or link). */
  initData: string;
  colorScheme: "light" | "dark";
  ready: () => void;
  expand: () => void;
  isVersionAtLeast: (version: string) => boolean;
  disableVerticalSwipes?: () => void;
  setHeaderColor: (color: string) => void;
  setBackgroundColor: (color: string) => void;
  setBottomBarColor?: (color: string) => void;
  onEvent: (event: "themeChanged", handler: () => void) => void;
  offEvent: (event: "themeChanged", handler: () => void) => void;
  HapticFeedback?: {
    selectionChanged: () => void;
    impactOccurred: (style: "light" | "medium" | "heavy" | "rigid" | "soft") => void;
    notificationOccurred: (type: "error" | "success" | "warning") => void;
  };
  BackButton: {
    show: () => void;
    hide: () => void;
    onClick: (handler: () => void) => void;
    offClick: (handler: () => void) => void;
  };
};

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp };
  }
}
