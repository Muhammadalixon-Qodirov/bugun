"use client";

import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

/** A titled settings block; the id is the target of the side navigation. */
export function SettingsSection({
  id,
  title,
  description,
  danger,
  children,
}: {
  id: string;
  title: string;
  description?: string;
  danger?: boolean;
  children: React.ReactNode;
}) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} className={cn("scroll-mt-24 rounded-3xl border bg-card", danger && "border-destructive/25")}>
      <header className="border-b px-6 py-5 sm:px-8">
        <h2 id={`${id}-title`} className={cn("font-semibold tracking-tight", danger && "text-destructive")}>
          {title}
        </h2>
        {description && <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>}
      </header>
      <div className="px-6 py-6 sm:px-8 sm:py-7">{children}</div>
    </section>
  );
}

/** Side menu for a settings page that highlights the section in view. */
export function SectionNav({ items }: { items: { id: string; label: string }[] }) {
  const t = useTranslations("common");
  const [active, setActive] = useState(items[0]?.id);
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]) setActive(visible[0].target.id);
      },
      { rootMargin: "-96px 0px -55% 0px" },
    );
    for (const { id } of items) {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [items]);

  return (
    <nav aria-label={t("sections")} className="-mx-4 flex gap-1 overflow-x-auto px-4 scrollbar-none lg:mx-0 lg:flex-col lg:px-0">
      {items.map((it) => (
        <a
          key={it.id}
          href={`#${it.id}`}
          onClick={(e) => {
            e.preventDefault();
            document.getElementById(it.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
            setActive(it.id);
          }}
          aria-current={active === it.id ? "true" : undefined}
          className={cn(
            "shrink-0 rounded-full px-4 py-2 text-sm transition-colors lg:rounded-xl lg:px-3",
            active === it.id ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {it.label}
        </a>
      ))}
    </nav>
  );
}
