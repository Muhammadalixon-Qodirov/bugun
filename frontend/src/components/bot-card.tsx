"use client";

import { useMutation } from "@tanstack/react-query";
import { m } from "motion/react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { QRCodeSVG } from "qrcode.react";
import { toast } from "sonner";

import { TelegramIcon } from "@/components/telegram-icon";
import { Button, buttonVariants } from "@/components/ui/button";
import { setBotEnabledMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useApiErrors } from "@/lib/errors";
import { cn } from "@/lib/utils";

export function BotCard({ enabled, botUsername }: { enabled: boolean; botUsername: string }) {
  const router = useRouter();
  const t = useTranslations("profile.bot");
  const errors = useApiErrors();
  const toggle = useMutation({
    ...setBotEnabledMutation(),
    onSuccess: (me) => {
      toast.success(me.bot_enabled ? t("connected") : t("disconnected"));
      router.refresh();
    },
    onError: (err) => errors.show(err, "change"),
  });
  const botUrl = botUsername ? `https://t.me/${botUsername}` : null;

  return (
    <div className="flex flex-col gap-6 sm:flex-row sm:items-center">
        <div className="flex-1">
          <div className="flex items-center gap-3">
            <span className="grid size-11 place-items-center rounded-2xl bg-telegram text-white">
              <TelegramIcon className="size-5" />
            </span>
            <div>
              <p className="font-medium">{botUsername ? `@${botUsername}` : t("name")}</p>
              <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                <span className="relative flex size-2">
                  {enabled && <span className="absolute inline-flex size-full animate-ping rounded-full bg-primary/60" />}
                  <span className={cn("relative inline-flex size-2 rounded-full", enabled ? "bg-primary" : "bg-muted-foreground/40")} />
                </span>
                {enabled ? t("on") : t("off")}
              </p>
            </div>
          </div>
          <p className="mt-4 max-w-md text-sm leading-relaxed text-muted-foreground">
            {t("text")}
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            {botUrl && enabled && (
              <a href={botUrl} target="_blank" rel="noreferrer" className={buttonVariants({ variant: "solid", size: "pill" })}>
                {t("open")}
              </a>
            )}
            <Button
              variant={enabled ? "outline" : "solid"}
              size="pill"
              disabled={toggle.isPending}
              onClick={() => toggle.mutate({ body: { enabled: !enabled } })}
            >
              {enabled ? t("disconnect") : t("reconnect")}
            </Button>
          </div>
        </div>
        {botUrl && enabled && (
          <m.div
            initial={{ opacity: 0, scale: 0.96 }}
            animate={{ opacity: 1, scale: 1 }}
            className="hidden rounded-2xl border bg-white p-3 sm:block"
            title={t("scanTitle")}
          >
            <QRCodeSVG value={botUrl} size={116} fgColor="#10201a" />
            <p className="mt-2 text-center text-[11px] text-neutral-500">{t("scan")}</p>
          </m.div>
        )}
    </div>
  );
}
