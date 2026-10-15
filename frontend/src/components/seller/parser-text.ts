"use client";

import { useTranslations } from "next-intl";
import { useCallback, useMemo } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import type { Hint, Item, Problem } from "@/lib/api/gen/types.gen";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";

// Package words the price parser understands, besides the catalog's base units.
const packages = ["qop", "yashik", "karobka", "lotok", "tonna", "gramm", "dona"] as const;

/** The price parser's problems and hints in the visitor's language, from their codes. */
export function useParserText() {
  const t = useTranslations("price.parser");
  const fmt = useFormat();
  const names = useCatalogNames();

  const unitWord = useCallback(
    (u: string) => ((packages as readonly string[]).includes(u) ? t(`units.${u as (typeof packages)[number]}`) : fmt.unit(u)),
    [t, fmt],
  );

  const problem = useCallback(
    (p: Problem) => {
      const product = p.product ? names.product(p.product_slug, p.product) : "";
      switch (p.code) {
        case "no_price":
          // The example stays in Uzbek: the parser reads Uzbek and Russian text.
          return product ? t("problems.no_price", { product, example: (p.product ?? "").toLowerCase() }) : p.message;
        case "unknown_unit":
          return p.unit && p.base_unit
            ? t("problems.unknown_unit", { product, unit: unitWord(p.unit), baseUnit: fmt.unit(p.base_unit) })
            : p.message;
        case "bad_price":
          return p.price != null ? t("problems.bad_price", { product, price: formatSom(p.price) }) : p.message;
        case "bad_quantity":
          return product ? t("problems.bad_quantity", { product }) : p.message;
        case "no_product":
          return t("problems.no_product");
        default:
          return p.message;
      }
    },
    [t, fmt, names, unitWord],
  );

  const hint = useCallback(
    (h: Hint, it?: Item) => {
      if (h.code === "matched" && h.written && it) return t("hints.matched", { written: h.written, product: names.product(undefined, it.product_name) });
      if (h.code === "read_as_thousands" && h.written && h.price != null) {
        return t("hints.read_as_thousands", { written: h.written, price: formatSom(h.price) });
      }
      if (h.code === "converted" && h.unit && h.per_base != null && h.base_unit) {
        return t("hints.converted", { unit: unitWord(h.unit), amount: fmt.decimal(h.per_base, Math.min(3, (String(h.per_base).split(".")[1] ?? "").length)), baseUnit: fmt.unit(h.base_unit) });
      }
      if (h.code === "by_model") return t("hints.by_model");
      return null;
    },
    [t, fmt, names, unitWord],
  );

  /** An item's notes: translated hints when the API sent them, else its Uzbek notes. */
  const notes = useCallback(
    (it: Item) => (it.hints?.length ? it.hints.map((h, i) => hint(h, it) ?? it.notes?.[i] ?? "").filter(Boolean) : (it.notes ?? [])),
    [hint],
  );

  return useMemo(() => ({ problem, notes }), [problem, notes]);
}
