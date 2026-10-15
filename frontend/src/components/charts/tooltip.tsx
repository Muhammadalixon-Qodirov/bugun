"use client";

import { cn } from "@/lib/utils";

export type TooltipRow = { key: string; label: string; value: string; color?: string; dashed?: boolean };

/**
 * The chart readout: title, then one row per series with the value leading and
 * a short line key in the series colour. Placed beside the pointer and flipped
 * to stay inside the chart.
 */
export function ChartTooltip({
  x,
  y,
  width,
  title,
  rows,
  notes,
}: {
  x: number;
  y: number;
  width: number;
  title: string;
  rows: TooltipRow[];
  notes?: string[];
}) {
  const flip = x > width / 2;
  return (
    <div
      role="status"
      className={cn(
        "pointer-events-none absolute top-0 z-10 w-max max-w-[15rem] rounded-xl border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg shadow-black/10",
        flip ? "-translate-x-[calc(100%+12px)]" : "translate-x-3",
      )}
      style={{ left: x, top: Math.max(0, y) }}
    >
      <p className="mb-1 font-medium text-muted-foreground">{title}</p>
      <ul className="space-y-0.5">
        {rows.map((r) => (
          <li key={r.key} className="flex items-center gap-2">
            {r.color && (
              <span
                aria-hidden
                className="h-0.5 w-3 shrink-0 rounded-full"
                style={
                  r.dashed
                    ? { backgroundImage: `linear-gradient(90deg, ${r.color} 60%, transparent 60%)`, backgroundSize: "4px 2px" }
                    : { backgroundColor: r.color }
                }
              />
            )}
            <span className="num font-semibold text-foreground">{r.value}</span>
            <span className="truncate text-muted-foreground">{r.label}</span>
          </li>
        ))}
      </ul>
      {notes && notes.length > 0 && (
        <ul className="mt-1.5 space-y-0.5 border-t pt-1.5 text-muted-foreground">
          {notes.map((n) => (
            <li key={n} className="leading-snug">
              {n}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
