"use client";

import { useTranslations } from "next-intl";
import { createContext, use, useCallback, useMemo } from "react";

import { catalogNames, type CatalogNameMaps, emptyCatalogNames } from "@/lib/i18n/catalog";
import type stats from "@/messages/uz/stats.json";

const CatalogNamesContext = createContext<CatalogNameMaps>(emptyCatalogNames);

/** Hands the server-built name maps to client components. */
export function CatalogNamesProvider({ maps, children }: { maps: CatalogNameMaps; children: React.ReactNode }) {
  return <CatalogNamesContext value={maps}>{children}</CatalogNamesContext>;
}

/** Catalog name helpers for client components. */
export function useCatalogNames() {
  const maps = use(CatalogNamesContext);
  return useMemo(() => catalogNames(maps), [maps]);
}

type ShortKey = keyof (typeof stats)["provinceShort"];

/**
 * A province's short name for tight spots (map labels, table headers), found by
 * its id; other regions, or one without a short name, keep `name`.
 */
export function useShortRegion() {
  const { provinceSlugs } = use(CatalogNamesContext);
  const t = useTranslations("stats.provinceShort");
  return useCallback(
    (id: number | undefined, name: string) => {
      const slug = (id == null ? undefined : provinceSlugs[id]) as ShortKey | undefined;
      return slug && t.has(slug) ? t(slug) : name;
    },
    [provinceSlugs, t],
  );
}
