"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, BellRing } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { Segmented } from "@/components/segmented";
import { buttonVariants } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { addWatchMutation, listWatchesOptions, listWatchesQueryKey } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useApiErrors } from "@/lib/errors";
import { groupDigits } from "@/lib/labels";
import { cn } from "@/lib/utils";

type Trade = "ulgurji" | "chakana";

/**
 * "Tell me when it's cheaper": a watch on this product, province and trade.
 * The bot sends the alert, so a disconnected bot is pointed out first.
 */
export function WatchButton({
  product,
  provinces,
  province,
  trade,
  signedIn,
  botEnabled,
  className,
}: {
  product: { id: number; name: string };
  provinces: { id: number; name: string }[];
  /** Preselected province (0 = the whole country). */
  province: number;
  trade: Trade;
  signedIn: boolean;
  botEnabled: boolean;
  className?: string;
}) {
  const t = useTranslations("watch");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const pathname = usePathname();
  const errors = useApiErrors();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [region, setRegion] = useState(String(province));
  const [kind, setKind] = useState<Trade>(trade);
  const [price, setPrice] = useState("");

  const watches = useQuery({ ...listWatchesOptions(), enabled: signedIn });
  const existing = watches.data?.watches.find(
    (w) => w.product_id === product.id && w.trade_type === kind && (w.province_id ?? 0) === Number(region),
  );
  const watching = watches.data?.watches.some((w) => w.product_id === product.id);

  const save = useMutation({
    ...addWatchMutation(),
    onSuccess: (data) => {
      queryClient.setQueryData(listWatchesQueryKey(), data);
      toast.success(t("saved"));
      setOpen(false);
    },
    onError: (err) => errors.show(err, "save"),
  });

  const trigger = cn(buttonVariants({ variant: "outline", size: "pill", className: "h-10 px-4" }), className);
  if (!signedIn) {
    return (
      <Link href={`/kirish?keyin=${encodeURIComponent(pathname)}`} className={trigger}>
        <Bell /> {t("button")}
      </Link>
    );
  }

  const items = [{ value: "0", label: tc("wholeCountry") }, ...provinces.map((p) => ({ value: String(p.id), label: names.region(p.name) }))];
  const maxPrice = Number(price.replace(/\D/g, ""));

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) setPrice(existing?.max_price ? groupDigits(String(existing.max_price)) : "");
      }}
    >
      <DialogTrigger className={trigger}>
        {watching ? <BellRing className="text-primary" /> : <Bell />} {watching ? t("watching") : t("button")}
      </DialogTrigger>
      <DialogContent>
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>{t("description", { product: product.name })}</DialogDescription>
        {!botEnabled && (
          <p className="mt-4 rounded-2xl bg-warm-soft px-4 py-3 text-sm text-warm-foreground">
            {t("botOff")}{" "}
            <Link href="/profil#bot" className="font-medium underline underline-offset-4">
              {t("connectBot")}
            </Link>
          </p>
        )}
        <form
          className="mt-5 space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate({
              body: { product_id: product.id, region_id: Number(region) || undefined, trade_type: kind, max_price: maxPrice || undefined },
            });
          }}
        >
          <label className="block space-y-1.5">
            <span className="text-sm font-medium">{t("region")}</span>
            <Select items={items} value={region} onValueChange={(v) => setRegion(v ?? "0")}>
              <SelectTrigger className="h-11! w-full rounded-xl bg-card">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {items.map((i) => (
                  <SelectItem key={i.value} value={i.value}>
                    {i.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          <div className="space-y-1.5">
            <span className="text-sm font-medium">{t("trade")}</span>
            <Segmented
              label={t("trade")}
              value={kind}
              onChange={setKind}
              options={[
                { value: "ulgurji", label: tc("wholesale") },
                { value: "chakana", label: tc("retail") },
              ]}
            />
          </div>
          <label className="block space-y-1.5">
            <span className="text-sm font-medium">{t("maxPrice")}</span>
            <input
              inputMode="numeric"
              value={price}
              onChange={(e) => setPrice(groupDigits(e.target.value).slice(0, 13))}
              className="num h-11 w-full rounded-xl border bg-card px-3.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            />
            <span className="block text-xs text-muted-foreground">{t("maxPriceHint")}</span>
          </label>
          <button type="submit" disabled={save.isPending} className={buttonVariants({ variant: "solid", size: "pill", className: "h-11 w-full" })}>
            {save.isPending ? tc("saving") : existing ? t("update") : t("save")}
          </button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
