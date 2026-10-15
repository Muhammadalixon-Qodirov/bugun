"use client";

import { Popover } from "@base-ui/react/popover";
import { Check, ChevronDown, ChevronLeft, ChevronRight, MapPin, Search } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useMemo, useState, useTransition } from "react";

import { useCatalogNames } from "@/components/catalog-names";
import type { Region } from "@/lib/api/gen/types.gen";
import { setRegionCookie } from "@/lib/region";
import { cn } from "@/lib/utils";

function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
}

/** Region menu in the header: province first, then its districts, with a filter box. */
export function RegionPicker({ regions, current }: { regions: Region[]; current: number }) {
  const t = useTranslations("region");
  const tc = useTranslations("common");
  const names = useCatalogNames();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [province, setProvince] = useState<Region | null>(null);
  const [query, setQuery] = useState("");
  const [pending, startTransition] = useTransition();

  const byId = useMemo(() => new Map(regions.map((r) => [r.id, r])), [regions]);
  const selected = byId.get(current);
  const label = selected
    ? selected.parent_id
      ? `${names.region(selected.name)}, ${names.region(byId.get(selected.parent_id)?.name ?? "")}`
      : names.region(selected.name)
    : tc("wholeCountry");

  const items = province ? regions.filter((r) => r.parent_id === province.id) : regions.filter((r) => r.parent_id == null);
  // Matches the name in the visitor's language and the Uzbek one.
  const visible = items.filter((r) => !query || [names.region(r.name), r.name].some((n) => norm(n).includes(norm(query))));

  const reset = () => {
    setProvince(null);
    setQuery("");
  };
  const choose = (id: number) => {
    setRegionCookie(id);
    setOpen(false);
    reset();
    startTransition(() => router.refresh());
  };
  const openProvince = (r: Region) => {
    setProvince(r);
    setQuery("");
  };

  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) reset();
      }}
    >
      <Popover.Trigger
        className={cn(
          "flex h-10 min-w-0 max-w-[11rem] items-center gap-1.5 rounded-full px-2.5 sm:px-3 text-sm font-medium transition-colors outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 data-[popup-open]:bg-muted sm:max-w-[16rem]",
          pending && "opacity-60",
        )}
        aria-label={t("choose")}
      >
        <MapPin className="size-4 shrink-0 text-muted-foreground" />
        <span className="truncate">{label}</span>
        <ChevronDown className="hidden size-3.5 shrink-0 text-muted-foreground sm:block" />
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Positioner side="bottom" align="end" sideOffset={8} collisionPadding={16} className="z-50">
          <Popover.Popup className="flex max-h-[min(30rem,var(--available-height))] w-[min(22rem,calc(100vw-2rem))] origin-(--transform-origin) flex-col overflow-hidden rounded-2xl border bg-popover text-popover-foreground shadow-xl shadow-black/10 outline-none transition-[opacity,transform] duration-150 data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0">
            <div className="border-b p-1.5">
              {province && (
                <button
                  type="button"
                  onClick={reset}
                  className="mb-1 flex w-full items-center gap-1.5 rounded-xl px-2.5 py-2 text-left text-sm font-medium hover:bg-muted"
                >
                  <ChevronLeft className="size-4" /> {names.region(province.name)}
                </button>
              )}
              <label className="flex h-10 items-center gap-2 rounded-xl bg-muted px-3">
                <Search className="size-4 shrink-0 text-muted-foreground" />
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={province ? t("searchDistrict") : t("searchProvince")}
                  aria-label={t("search")}
                  className="w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                />
              </label>
            </div>
            <ul className="flex-1 overflow-y-auto p-1.5" role="listbox" aria-label={province ? names.region(province.name) : t("provinces")}>
              {!query && (
                <li>
                  <Row
                    label={province ? t("whole", { name: names.region(province.name) }) : tc("wholeCountry")}
                    active={current === (province?.id ?? 0)}
                    onClick={() => choose(province?.id ?? 0)}
                    strong
                  />
                </li>
              )}
              {visible.map((r) => (
                <li key={r.id}>
                  <Row
                    label={names.region(r.name)}
                    active={current === r.id || (!province && selected?.parent_id === r.id)}
                    arrow={!province}
                    onClick={() => (province ? choose(r.id) : openProvince(r))}
                  />
                </li>
              ))}
              {visible.length === 0 && <li className="px-2.5 py-6 text-center text-sm text-muted-foreground">{tc("notFound")}</li>}
            </ul>
          </Popover.Popup>
        </Popover.Positioner>
      </Popover.Portal>
    </Popover.Root>
  );
}

function Row({ label, active, arrow, strong, onClick }: { label: string; active: boolean; arrow?: boolean; strong?: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      role="option"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "flex w-full items-center justify-between gap-2 rounded-xl px-2.5 py-2 text-left text-sm transition-colors hover:bg-muted",
        (active || strong) && "font-medium",
      )}
    >
      <span className="truncate">{label}</span>
      {arrow ? (
        <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
      ) : active ? (
        <Check className="size-4 shrink-0" />
      ) : null}
    </button>
  );
}
