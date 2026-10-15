import { createTranslator } from "next-intl";

import { getProduct } from "@/lib/api/gen/sdk.gen";
import { formatSom } from "@/lib/labels";
import { ogCard, ogSize } from "@/lib/og";
import format from "@/messages/uz/format.json";
import product from "@/messages/uz/product.json";
import site from "@/messages/uz/site.json";

// Link previews are fetched without the visitor's language cookie, so they are in Uzbek.
export const alt = site.title;
export const size = ogSize;
export const contentType = "image/png";

const t = createTranslator({ locale: "uz", messages: { product, format } });

/** The product's name and today's median wholesale price. */
export default async function Image({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const { data } = await getProduct({ path: { slug }, query: { region: 0, trade: "ulgurji" } });
  if (!data) return ogCard({ eyebrow: site.title, title: t("product.metaFallback"), text: site.description });
  const name = data.product.name;
  const unit = data.product.base_unit in format.units ? t(`format.units.${data.product.base_unit as keyof typeof format.units}`) : data.product.base_unit;
  const text = data.price.enough
    ? t("product.metaPrice", { price: formatSom(data.price.median ?? 0), unit, offers: data.price.offers })
    : t("product.metaOffers", { name });
  return ogCard({ eyebrow: site.title, title: t("product.metaTitle", { name }), text });
}
