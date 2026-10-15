"use client";

import { Check, Globe } from "lucide-react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useTransition } from "react";

import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { type Locale, locales, setLocaleCookie } from "@/i18n/config";
import { cn } from "@/lib/utils";

/** Language menu in the header; the choice is kept in a cookie for a year. */
export function LocaleSwitcher({ className }: { className?: string }) {
  const t = useTranslations("common");
  const locale = useLocale();
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  const choose = (next: Locale) => {
    if (next === locale) return;
    setLocaleCookie(next);
    startTransition(() => router.refresh());
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("language")}
        className={cn(
          "flex h-10 shrink-0 items-center gap-1.5 rounded-full px-2.5 text-sm sm:px-3 font-medium uppercase transition-[background-color,opacity] outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 aria-expanded:bg-muted",
          pending && "opacity-60",
          className,
        )}
      >
        <Globe className="hidden size-4 text-muted-foreground sm:block" />
        {locale}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        {locales.map((l) => (
          <DropdownMenuItem key={l} lang={l} onClick={() => choose(l)}>
            <span className="flex-1">{t(`localeNames.${l}`)}</span>
            {l === locale && <Check className="size-4" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
