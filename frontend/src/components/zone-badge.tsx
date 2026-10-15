import { ArrowDownRight, ArrowUpRight, Minus } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";

export type Zone = "surplus" | "balanced" | "shortage";

const tone: Record<Zone, string> = {
  surplus: "bg-primary/10 text-primary",
  balanced: "bg-secondary text-secondary-foreground",
  shortage: "bg-warm-soft text-warm-foreground",
};
const icon = { surplus: ArrowDownRight, balanced: Minus, shortage: ArrowUpRight };

/**
 * The demand–supply pressure zone as a label with an icon, never colour alone.
 * Without a zone (not enough history) it says the data is still coming in.
 */
export function ZoneBadge({ zone, withHint, className }: { zone?: Zone | string; withHint?: boolean; className?: string }) {
  const t = useTranslations("codes");
  if (zone !== "surplus" && zone !== "balanced" && zone !== "shortage") {
    return <span className={cn("inline-flex items-center rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground", className)}>{t("zone.none")}</span>;
  }
  const Icon = icon[zone];
  return (
    <span className={cn("inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium", tone[zone], className)} title={t(`zone.${zone}.hint`)}>
      <Icon className="size-3.5" />
      {t(`zone.${zone}.label`)}
      {withHint && <span className="font-normal opacity-80">· {t(`zone.${zone}.hint`)}</span>}
    </span>
  );
}
