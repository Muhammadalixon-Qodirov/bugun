"use client";

import { useMutation } from "@tanstack/react-query";
import { MessageCircle, Phone } from "lucide-react";
import { AnimatePresence, m } from "motion/react";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { TelegramIcon } from "@/components/telegram-icon";
import { Button, buttonVariants } from "@/components/ui/button";
import { revealContactMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import { problemOf, useApiErrors } from "@/lib/errors";
import { cn } from "@/lib/utils";

/** Reveals a seller's contacts on demand; the phone is only shown after a tap and sign-in. */
export function ContactButton({
  listingId,
  hasTelegram,
  hasPhone,
  block,
}: {
  listingId: number;
  hasTelegram: boolean;
  hasPhone: boolean;
  /** Full-width, tall buttons for the listing page's main panel. */
  block?: boolean;
}) {
  const size = block ? "h-12 flex-1 px-6 text-[15px]" : "h-9 px-4";
  const t = useTranslations("contact");
  const router = useRouter();
  const errors = useApiErrors();
  const pathname = usePathname();
  const reveal = useMutation({
    ...revealContactMutation(),
    onError: (err) => {
      if (problemOf(err).status === 401) {
        router.push(`/kirish?keyin=${encodeURIComponent(pathname)}`);
        return;
      }
      errors.show(err, "contact");
    },
  });

  if (!hasTelegram && !hasPhone) {
    return <p className="text-xs text-muted-foreground">{t("hidden")}</p>;
  }

  return (
    <AnimatePresence mode="wait" initial={false}>
      {reveal.data ? (
        <m.div key="shown" initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} className={cn("flex flex-wrap gap-2", block && "w-full")}>
          {reveal.data.telegram_url && (
            <a href={reveal.data.telegram_url} target="_blank" rel="noreferrer" className={buttonVariants({ className: cn("rounded-full bg-telegram text-white hover:bg-telegram-hover", size) })}>
              <TelegramIcon className="size-4" /> {t("telegram")}
            </a>
          )}
          {reveal.data.phone && (
            <a href={`tel:${reveal.data.phone}`} className={buttonVariants({ variant: "outline", className: cn("num rounded-full", size) })}>
              <Phone /> {reveal.data.phone}
            </a>
          )}
        </m.div>
      ) : (
        <m.div key="hidden" exit={{ opacity: 0 }} className={cn(block && "w-full")}>
          <Button className={cn("rounded-full", size, block && "w-full")} disabled={reveal.isPending} onClick={() => reveal.mutate({ path: { id: listingId } })}>
            <MessageCircle /> {reveal.isPending ? t("opening") : t("contact")}
          </Button>
        </m.div>
      )}
    </AnimatePresence>
  );
}
