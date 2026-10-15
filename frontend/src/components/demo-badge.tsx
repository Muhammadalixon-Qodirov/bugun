import { FlaskConical } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";

/** "35%", or "<1%" so a tiny share never reads as none. Whole percents read the same in every locale. */
export function demoPercent(share: number) {
  return share > 0 && share < 0.01 ? "<1%" : `${Math.round(Math.min(1, share) * 100)}%`;
}

/**
 * Marks numbers built partly from demo data: demo sellers or the generated
 * sample history. With `share` (the API's demo_share, 0–1) it says how much
 * and renders nothing at 0; without it, it is a plain "Demo data" mark.
 * `compact` is the small form for a table row or a chart legend.
 */
export function DemoBadge({ share, compact, className }: { share?: number; compact?: boolean; className?: string }) {
  const t = useTranslations("codes");
  if (share != null && !(share > 0)) return null;
  const pct = share != null ? demoPercent(share) : null;
  const hint = pct ? t("demoShareHint", { share: pct }) : t("demoHint");
  return (
    <span
      title={hint}
      data-demo-share={share}
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-full bg-warm-soft font-medium whitespace-nowrap text-warm-foreground",
        compact ? "px-2 py-0.5 text-[11px]" : "px-2.5 py-1 text-xs",
        className,
      )}
    >
      <FlaskConical className={compact ? "size-3" : "size-3.5"} aria-hidden />
      <span aria-hidden>{pct ? (compact ? t("demoShareLabel", { share: pct }) : `${t("demo")} · ${pct}`) : t("demo")}</span>
      <span className="sr-only">{hint}</span>
    </span>
  );
}
