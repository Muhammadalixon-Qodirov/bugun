export const locales = ["uz", "en", "ru"] as const;
export type Locale = (typeof locales)[number];

/** Uzbek unless the visitor picked another language. */
export const defaultLocale: Locale = "uz";

/** Where the chosen language is kept; the URL stays the same in every language. */
export const LOCALE_COOKIE = "NEXT_LOCALE";

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (locales as readonly string[]).includes(value);
}

export function setLocaleCookie(locale: Locale) {
  document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=${60 * 60 * 24 * 365}; samesite=lax`;
}
