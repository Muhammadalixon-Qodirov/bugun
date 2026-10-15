"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useOptimistic, useTransition } from "react";

import { Segmented } from "@/components/segmented";

type Trade = "ulgurji" | "chakana";

/** Product page wholesale/retail switch: flips at once, the prices follow when loaded. */
export function TradeSwitch({ slug, value }: { slug: string; value: Trade }) {
  const t = useTranslations("market");
  const tc = useTranslations("common");
  const router = useRouter();
  const params = useSearchParams();
  const [, startTransition] = useTransition();
  const [shown, setShown] = useOptimistic(value);
  return (
    <Segmented
      label={t("tradeType")}
      size="lg"
      className="w-full sm:w-72"
      value={shown}
      onChange={(t) =>
        startTransition(() => {
          setShown(t);
          // Other parameters (a region from a bot link) stay.
          const next = new URLSearchParams(params);
          if (t === "ulgurji") next.delete("trade");
          else next.set("trade", t);
          const qs = next.toString();
          router.replace(qs ? `/mahsulot/${slug}?${qs}` : `/mahsulot/${slug}`, { scroll: false });
        })
      }
      options={[
        { value: "ulgurji", label: tc("wholesale") },
        { value: "chakana", label: tc("retail") },
      ]}
    />
  );
}
