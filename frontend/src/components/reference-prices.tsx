import { ExternalLink, Landmark } from "lucide-react";
import { getTranslations } from "next-intl/server";

import { getFormat } from "@/lib/i18n/format.server";
import { formatSom } from "@/lib/labels";
import { getProductReferences } from "@/lib/server/analytics";
import { cn } from "@/lib/utils";

/**
 * Official reference prices (the commodity exchange, UzEx) beside the market's
 * own: the price per catalog unit, the original quote, its date and a link to
 * the source. Renders nothing when there are none or they cannot be loaded.
 */
export async function ReferencePrices({
  slug,
  region,
  trade,
  perUnit,
  className,
}: {
  slug: string;
  region: number;
  trade?: "ulgurji" | "chakana";
  perUnit: string;
  className?: string;
}) {
  const references = await getProductReferences(slug, region, trade).catch(() => []);
  if (references.length === 0) return null;
  const [t, fmt] = await Promise.all([getTranslations("product.reference"), getFormat()]);

  return (
    <ul className={cn("space-y-2 text-sm", className)} data-testid="reference-prices" aria-label={t("label")}>
      {references.map((r) => (
        <li key={`${r.source}-${r.quote_name}-${r.quoted_on}`} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5" title={t("hint")}>
          <span className="inline-flex items-center gap-1.5 self-center font-medium">
            <Landmark className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            {t(`source.${r.source}`)}
          </span>
          <span className="num font-semibold">
            {formatSom(r.price_som)}
            <span className="ml-1 font-sans text-xs font-normal text-muted-foreground">{perUnit}</span>
          </span>
          <span className="min-w-0 text-xs text-muted-foreground">
            {[
              t("quote", { name: r.quote_name, price: formatSom(r.quote_som), unit: r.quote_unit }),
              fmt.date(r.quoted_on),
              r.national && t("national"),
            ]
              .filter(Boolean)
              .join(" · ")}
          </span>
          {r.source_url && (
            <a
              href={r.source_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
            >
              {t("open")}
              <ExternalLink className="size-3" aria-hidden />
            </a>
          )}
        </li>
      ))}
    </ul>
  );
}
