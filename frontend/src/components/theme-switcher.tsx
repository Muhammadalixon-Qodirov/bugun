"use client";

import { Check, Monitor, Moon, Sun } from "lucide-react";
import { useTranslations } from "next-intl";

import { useAppTheme } from "@/components/theme";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

const options = [
  { value: "light", icon: Sun },
  { value: "dark", icon: Moon },
  { value: "system", icon: Monitor },
] as const;

/** Light / dark / system menu in the header. */
export function ThemeSwitcher({ className }: { className?: string }) {
  const t = useTranslations("common");
  const { choice, setChoice } = useAppTheme();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t("theme")}
        className={cn(
          "grid size-9 shrink-0 place-items-center sm:size-10 rounded-full text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 aria-expanded:bg-muted aria-expanded:text-foreground",
          className,
        )}
      >
        {/* Chosen by the .dark class, so the server HTML already shows the right icon. */}
        <Sun className="size-[1.125rem] dark:hidden" />
        <Moon className="hidden size-[1.125rem] dark:block" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        {options.map(({ value, icon: Icon }) => (
          <DropdownMenuItem key={value} onClick={() => setChoice(value)}>
            <Icon />
            <span className="flex-1">{t(`themes.${value}`)}</span>
            {choice === value && <Check className="size-4" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
