"use client";

import { useMutation } from "@tanstack/react-query";
import { Phone } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { TelegramIcon } from "@/components/telegram-icon";
import { setContactPrefsMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import { useApiErrors } from "@/lib/errors";
import { cn } from "@/lib/utils";

/** Seller chooses how buyers may reach them (Telegram chat and/or phone). */
export function ContactPrefs({ telegram, phone, username, hasPhone }: { telegram: boolean; phone: boolean; username?: string; hasPhone: boolean }) {
  const router = useRouter();
  const t = useTranslations("profile.contact");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const save = useMutation({
    ...setContactPrefsMutation(),
    onSuccess: () => {
      toast.success(tc("saved"));
      router.refresh();
    },
    onError: (err) => errors.show(err, "save"),
  });
  const set = (next: { telegram: boolean; phone: boolean }) => save.mutate({ body: next });

  const options = [
    {
      key: "telegram" as const,
      icon: TelegramIcon,
      title: t("telegram"),
      text: username ? t("telegramVia", { username }) : t("noUsername"),
      on: telegram,
      disabled: !username,
    },
    {
      key: "phone" as const,
      icon: Phone,
      title: t("phone"),
      text: hasPhone ? t("phoneHint") : t("noPhone"),
      on: phone,
      disabled: !hasPhone,
    },
  ];

  return (
    <div className="grid gap-2.5 sm:grid-cols-2">
        {options.map((o) => (
          <button
            key={o.key}
            type="button"
            role="switch"
            aria-checked={o.on}
            disabled={o.disabled || save.isPending}
            onClick={() => set({ telegram, phone, [o.key]: !o.on })}
            className={cn(
              "flex items-start gap-3 rounded-2xl border p-4 text-left transition-colors disabled:opacity-50",
              o.on ? "border-foreground/80 bg-card" : "hover:border-foreground/25",
            )}
          >
            <o.icon className={cn("mt-0.5 size-5 shrink-0", o.on ? "text-foreground" : "text-muted-foreground")} />
            <span className="min-w-0 flex-1">
              <span className="block font-medium">{o.title}</span>
              <span className="block text-sm text-muted-foreground">{o.text}</span>
            </span>
            <span className={cn("relative mt-0.5 h-5 w-9 shrink-0 rounded-full transition-colors", o.on ? "bg-primary" : "bg-input")}>
              <span className={cn("absolute top-0.5 size-4 rounded-full shadow transition-all", o.on ? "left-[18px] bg-primary-foreground" : "left-0.5 bg-background dark:bg-muted-foreground")} />
            </span>
          </button>
        ))}
    </div>
  );
}
