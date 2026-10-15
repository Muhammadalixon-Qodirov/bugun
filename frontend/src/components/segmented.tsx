"use client";

import { m } from "motion/react";
import { useId } from "react";

import { haptic } from "@/lib/telegram";
import { cn } from "@/lib/utils";

type Option<T extends string> = { value: T; label: React.ReactNode };

/**
 * A pill switch whose highlight glides to the chosen option. Arrow keys move
 * between options like a native radio group.
 */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
  className,
  size = "default",
}: {
  options: Option<T>[];
  value: T;
  onChange: (value: T) => void;
  label: string;
  className?: string;
  size?: "sm" | "default" | "lg";
}) {
  const id = useId();
  const move = (dir: number) => {
    const i = options.findIndex((o) => o.value === value);
    onChange(options[(i + dir + options.length) % options.length].value);
  };
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className={cn("flex rounded-full bg-muted p-1", className)}
      onKeyDown={(e) => {
        const dir = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
        if (dir) {
          e.preventDefault();
          move(dir);
        }
      }}
    >
      {options.map((o) => {
        const on = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={on}
            tabIndex={on ? 0 : -1}
            onClick={() => {
              if (!on) haptic();
              onChange(o.value);
            }}
            className={cn(
              "relative isolate flex-1 rounded-full whitespace-nowrap transition-colors duration-200",
              size === "sm" ? "px-1.5 py-1.5 text-[13px]" : size === "lg" ? "px-4 py-2.5 text-sm" : "px-4 py-2 text-sm",
              on ? "font-medium text-foreground" : "text-muted-foreground hover:text-foreground",
            )}
          >
            {on && (
              <m.span
                layoutId={`segmented-${id}`}
                className="absolute inset-0 -z-10 rounded-full bg-card shadow-sm dark:bg-foreground/15"
                transition={{ type: "spring", stiffness: 520, damping: 38 }}
              />
            )}
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
