import Link from "next/link";

import { cn } from "@/lib/utils";

export type BarRow = {
  key: string;
  label: string;
  /** Bar length, compared with the longest row. */
  value: number;
  /** The number written at the bar's end. */
  display: string;
  /** Secondary note under or beside the value ("+3%", "96 narx"). */
  note?: string;
  noteTone?: "cheap" | "dear" | "muted";
  /** Any CSS colour for the bar; the brand colour by default. */
  color?: string;
  highlight?: boolean;
  href?: string;
};

/**
 * Horizontal bars for rankings. Plain HTML: every value is written out, so it
 * reads the same with or without the bars.
 */
export function BarList({ rows, className, max }: { rows: BarRow[]; className?: string; max?: number }) {
  const top = max ?? Math.max(...rows.map((r) => Math.abs(r.value)), 1);
  return (
    <ul className={cn("space-y-2.5", className)}>
      {rows.map((r) => {
        const body = (
          <>
            <div className="flex items-baseline justify-between gap-3 text-sm">
              <span className={cn("min-w-0 truncate", r.highlight ? "font-medium text-foreground" : "text-muted-foreground")}>{r.label}</span>
              <span className="flex shrink-0 items-baseline gap-2">
                {r.note && (
                  <span
                    className={cn(
                      "num text-xs",
                      r.noteTone === "cheap" ? "text-primary" : r.noteTone === "dear" ? "text-warm-foreground" : "text-muted-foreground",
                    )}
                  >
                    {r.note}
                  </span>
                )}
                <span className="num font-medium">{r.display}</span>
              </span>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className={cn("h-full rounded-full transition-[width] duration-500", !r.color && (r.highlight ? "bg-primary" : "bg-foreground/60"))}
                style={{ width: `${Math.max(2, (100 * Math.abs(r.value)) / top)}%`, backgroundColor: r.color }}
              />
            </div>
          </>
        );
        return (
          <li key={r.key}>
            {r.href ? (
              <Link href={r.href} className="-mx-2 block rounded-xl px-2 py-1 transition-colors hover:bg-muted/60">
                {body}
              </Link>
            ) : (
              body
            )}
          </li>
        );
      })}
    </ul>
  );
}
