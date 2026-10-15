import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import { defaultLocale, isLocale, LOCALE_COOKIE } from "@/i18n/config";

/**
 * The visitor's language comes from a cookie (set by the language switcher),
 * so links and the Telegram Mini App URL are the same in every language.
 * Catalog names (products, regions, markets) are translated separately, see
 * lib/i18n/catalog.ts.
 */
export default getRequestConfig(async () => {
  const saved = (await cookies()).get(LOCALE_COOKIE)?.value;
  const locale = isLocale(saved) ? saved : defaultLocale;
  const messages = (await import(`../messages/${locale}/index.ts`)).default;
  return { locale, messages, timeZone: "Asia/Tashkent" };
});
