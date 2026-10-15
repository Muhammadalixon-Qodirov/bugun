"use client";

import { Popover } from "@base-ui/react/popover";
import { Check, Search } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { norm } from "@/lib/stats";
import { cn } from "@/lib/utils";

export type PickerItem = {
  value: string;
  label: string;
  /** Extra text the search also matches (the Uzbek name). */
  alias?: string;
  /** Grey text on the right (category, unit). */
  hint?: string;
};

/**
 * A search box over a list in a popover, for products and provinces. The
 * trigger is any element; picking an item closes the popover.
 */
export function Picker({
  items,
  selected = [],
  disabled,
  onPick,
  trigger,
  triggerClassName,
  triggerLabel,
  placeholder,
  triggerDisabled,
}: {
  items: PickerItem[];
  selected?: string[];
  disabled?: (value: string) => boolean;
  onPick: (value: string) => void;
  trigger: React.ReactNode;
  triggerClassName?: string;
  triggerLabel: string;
  placeholder: string;
  triggerDisabled?: boolean;
}) {
  const tc = useTranslations("common");
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const q = norm(query.trim());
  const visible = q ? items.filter((i) => norm(i.label).includes(q) || (i.alias && norm(i.alias).includes(q))) : items;

  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery("");
      }}
    >
      <Popover.Trigger aria-label={triggerLabel} className={triggerClassName} disabled={triggerDisabled}>
        {trigger}
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Positioner side="bottom" align="start" sideOffset={8} collisionPadding={16} className="z-50">
          <Popover.Popup className="flex max-h-[min(26rem,var(--available-height))] w-[min(20rem,calc(100vw-2rem))] origin-(--transform-origin) flex-col overflow-hidden rounded-2xl border bg-popover text-popover-foreground shadow-xl shadow-black/10 outline-none transition-[opacity,transform] duration-150 data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0">
            <div className="border-b p-1.5">
              <label className="flex h-10 items-center gap-2 rounded-xl bg-muted px-3">
                <Search className="size-4 shrink-0 text-muted-foreground" />
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={placeholder}
                  aria-label={placeholder}
                  className="w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                />
              </label>
            </div>
            <ul className="flex-1 overflow-y-auto p-1.5" role="listbox" aria-label={triggerLabel}>
              {visible.map((i) => {
                const on = selected.includes(i.value);
                const off = !on && disabled?.(i.value);
                return (
                  <li key={i.value}>
                    <button
                      type="button"
                      role="option"
                      aria-selected={on}
                      disabled={off}
                      onClick={() => {
                        onPick(i.value);
                        setOpen(false);
                        setQuery("");
                      }}
                      className={cn(
                        "flex min-h-10 w-full items-center justify-between gap-2 rounded-xl px-2.5 py-2 text-left text-sm transition-colors hover:bg-muted disabled:pointer-events-none disabled:opacity-40",
                        on && "font-medium",
                      )}
                    >
                      <span className="truncate">{i.label}</span>
                      <span className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
                        {i.hint}
                        {on && <Check className="size-4 text-foreground" />}
                      </span>
                    </button>
                  </li>
                );
              })}
              {visible.length === 0 && <li className="px-2.5 py-6 text-center text-sm text-muted-foreground">{tc("notFound")}</li>}
            </ul>
          </Popover.Popup>
        </Popover.Positioner>
      </Popover.Portal>
    </Popover.Root>
  );
}
