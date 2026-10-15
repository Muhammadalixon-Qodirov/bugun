"use client";

import { useMutation } from "@tanstack/react-query";
import { ChartColumn, Eye, EyeOff, HandCoins, MoreHorizontal, Pencil, RefreshCw, Trash2 } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { SoldDialog } from "@/components/seller/sold-dialog";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  deleteListingMutation,
  refreshListingMutation,
  setListingVisibilityMutation,
} from "@/lib/api/gen/@tanstack/react-query.gen";
import type { MyListing } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { needsRefresh } from "@/lib/listings";

export function ListingActions({ l }: { l: MyListing }) {
  const router = useRouter();
  const t = useTranslations("mine.actions");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const [confirming, setConfirming] = useState(false);
  const [selling, setSelling] = useState(false);
  const done = (msg: string) => () => {
    toast.success(msg);
    router.refresh();
  };
  const refresh = useMutation({ ...refreshListingMutation(), onSuccess: done(t("renewed")), onError: (err) => errors.show(err, "renew") });
  const visibility = useMutation({ ...setListingVisibilityMutation(), onSuccess: done(tc("saved")), onError: (err) => errors.show(err, "change") });
  const remove = useMutation({ ...deleteListingMutation(), onSuccess: done(t("deleted")), onError: (err) => errors.show(err, "delete") });
  const hidden = l.status === "hidden";

  if (confirming) {
    return (
      <div className="flex gap-1.5">
        <Button size="pill-sm" variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate({ path: { id: l.id } })}>
          {tc("delete")}
        </Button>
        <Button size="pill-sm" variant="ghost" onClick={() => setConfirming(false)}>
          {tc("cancel")}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1.5">
      {needsRefresh(l) && (
        <Button variant="solid" size="pill-sm" disabled={refresh.isPending} onClick={() => refresh.mutate({ path: { id: l.id } })}>
          <RefreshCw className={refresh.isPending ? "animate-spin" : ""} /> {t("renew")}
        </Button>
      )}
      <Link href={`/mahsulotlarim/${l.id}`} className={buttonVariants({ size: "pill-sm", variant: "outline" })}>
        <Pencil /> <span className="hidden sm:inline">{t("edit")}</span>
      </Link>
      <DropdownMenu>
        <DropdownMenuTrigger className="grid size-8 place-items-center rounded-full border hover:bg-muted" aria-label={t("more")}>
          <MoreHorizontal className="size-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-52">
          {l.status === "active" && (
            <DropdownMenuItem onClick={() => setSelling(true)}>
              <HandCoins /> {t("sold")}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem onClick={() => router.push(`/mahsulotlarim/${l.id}?bolim=statistika`)}>
            <ChartColumn /> {t("stats")}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => refresh.mutate({ path: { id: l.id } })}>
            <RefreshCw /> {t("renewWeek")}
          </DropdownMenuItem>
          {/* Listings awaiting review or rejected by an admin stay as they are: showing one would skip the review. */}
          {(l.status === "active" || l.status === "hidden") && (
            <DropdownMenuItem onClick={() => visibility.mutate({ path: { id: l.id }, body: { visible: hidden } })}>
              {hidden ? <Eye /> : <EyeOff />} {hidden ? t("show") : t("hide")}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onClick={() => setConfirming(true)}>
            <Trash2 /> {tc("delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <SoldDialog l={l} open={selling} onOpenChange={setSelling} />
    </div>
  );
}
