"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, Check, Send } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { useCatalogNames } from "@/components/catalog-names";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { addWatchMutation, getCatalogOptions, listWatchesQueryKey } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { Action } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import { formatSom } from "@/lib/labels";

const num = (v: unknown) => (typeof v === "number" && Number.isFinite(v) ? v : typeof v === "string" && v.trim() !== "" && Number.isFinite(Number(v)) ? Number(v) : 0);
const str = (v: unknown) => (typeof v === "string" ? v : "");

/** Where the request form opens, filled in from the assistant's proposal. */
export function requestFormHref(params: Action["params"]) {
  const q = new URLSearchParams();
  if (str(params.product)) q.set("mahsulot", str(params.product));
  if (num(params.quantity) > 0) q.set("miqdor", String(num(params.quantity)));
  if (num(params.days) > 0) q.set("kun", String(num(params.days)));
  return `/sorovlar${q.size ? `?${q}` : ""}`;
}

/**
 * A change the assistant proposed. Nothing happens on the first press: a
 * dialog says exactly what will be done, and only its confirm button acts.
 */
export function ChatActionButton({ action, signedIn, onNavigate }: { action: Action; signedIn: boolean; onNavigate?: () => void }) {
  const t = useTranslations("analyst.actions");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const fmt = useFormat();
  const router = useRouter();
  const pathname = usePathname();
  const errors = useApiErrors();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [done, setDone] = useState(false);

  const p = action.params;
  const product = str(p.product) ? names.product(str(p.product), str(p.product)) : "";
  const trade = str(p.trade) === "ulgurji" ? tc("wholesaleLower") : str(p.trade) === "chakana" ? tc("retailLower") : "";
  // The province's name is needed only in the dialog, so the catalog is asked then.
  const catalog = useQuery({ ...getCatalogOptions(), enabled: open && num(p.province_id) > 0, staleTime: 60 * 60 * 1000 });
  const province = catalog.data?.regions.find((r) => r.id === num(p.province_id));
  const area = province ? names.region(province.name) : tc("wholeCountry");

  const watch = useMutation({
    ...addWatchMutation(),
    onSuccess: (data) => {
      queryClient.setQueryData(listWatchesQueryKey(), data);
      toast.success(t("watchSaved"));
      setDone(true);
      setOpen(false);
    },
    onError: (err) => errors.show(err, "save"),
  });

  // A watch is saved where the visitor is; after sign-in they come back to that page.
  const target = action.type === "create_request" ? requestFormHref(p) : pathname;
  const confirm = () => {
    if (!signedIn) {
      setOpen(false);
      onNavigate?.();
      router.push(`/kirish?keyin=${encodeURIComponent(target)}`);
      return;
    }
    if (action.type === "create_request") {
      setOpen(false);
      onNavigate?.();
      router.push(target);
      return;
    }
    const trade = str(p.trade);
    watch.mutate({
      body: {
        product_id: num(p.product_id),
        region_id: num(p.province_id) || undefined,
        trade_type: trade === "ulgurji" || trade === "chakana" ? trade : undefined,
      },
    });
  };

  const Icon = action.type === "watch_price" ? Bell : Send;
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        disabled={done}
        className="inline-flex min-h-9 items-center gap-1.5 rounded-full border border-primary/30 bg-primary/5 px-3.5 py-1.5 text-left text-sm font-medium text-primary transition-colors hover:bg-primary/10 disabled:border-border disabled:bg-transparent disabled:text-muted-foreground"
        data-testid={`chat-action-${action.type}`}
      >
        {done ? <Check className="size-4 shrink-0" aria-hidden /> : <Icon className="size-4 shrink-0" aria-hidden />}
        {action.label}
      </DialogTrigger>
      <DialogContent>
        <DialogTitle>{t(`${action.type}.title`)}</DialogTitle>
        <DialogDescription>{t(`${action.type}.description`)}</DialogDescription>
        <dl className="mt-5 divide-y rounded-2xl border px-4 text-sm">
          {product && <Row label={t("product")} value={product} />}
          {action.type === "watch_price" ? (
            <>
              <Row label={t("area")} value={area} />
              {trade && <Row label={t("trade")} value={trade} />}
            </>
          ) : (
            <>
              {num(p.quantity) > 0 && <Row label={t("quantity")} value={`${formatSom(num(p.quantity))} ${str(p.unit) ? fmt.unit(str(p.unit)) : ""}`.trim()} />}
              {num(p.days) > 0 && <Row label={t("days")} value={t("daysValue", { count: num(p.days) })} />}
            </>
          )}
        </dl>
        {!signedIn && <p className="mt-4 rounded-2xl bg-muted px-4 py-3 text-sm text-muted-foreground">{t("signInFirst")}</p>}
        <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <DialogClose render={<Button variant="outline" size="pill" className="h-11 px-5" />}>{t("cancel")}</DialogClose>
          <Button variant="solid" size="pill" className="h-11 px-6" onClick={confirm} disabled={watch.isPending} data-testid="chat-action-confirm">
            {!signedIn ? t("signIn") : action.type === "create_request" ? t("create_request.confirm") : t("watch_price.confirm")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-2.5">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-medium">{value}</dd>
    </div>
  );
}
