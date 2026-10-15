"use client";

import { useLocale, useTranslations } from "next-intl";
import { useMemo } from "react";

import { type FormatT, makeFormat } from "@/lib/i18n/format";

/** Locale-aware dates, ages, units and roles for client components. */
export function useFormat() {
  const locale = useLocale();
  const t = useTranslations("format");
  return useMemo(() => makeFormat(locale, t as unknown as FormatT), [locale, t]);
}
