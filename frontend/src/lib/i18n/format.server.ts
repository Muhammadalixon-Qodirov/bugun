import "server-only";

import { getLocale, getTranslations } from "next-intl/server";

import { type FormatT, makeFormat } from "@/lib/i18n/format";

/** Locale-aware dates, ages, units and roles for server components. */
export async function getFormat() {
  const [locale, t] = await Promise.all([getLocale(), getTranslations("format")]);
  return makeFormat(locale, t as unknown as FormatT);
}
