"use client";

import { useEffect, useState } from "react";

import { useMarketNav, useMarketParams } from "@/components/market-nav";
import { groupDigits } from "@/lib/labels";

/** Transport cost per unit per 100 km for the arbitrage cards; applied a moment after typing stops. */
export function TransportRate({ basePath, value, label }: { basePath: string; value: number; label: string }) {
  const { go } = useMarketNav();
  const params = useMarketParams();
  const [text, setText] = useState(groupDigits(String(value)));

  useEffect(() => {
    const n = Number(text.replace(/\D/g, ""));
    if (!text || n === value || n > 5000) return;
    const timer = setTimeout(() => {
      const next = new URLSearchParams(params);
      if (n === 60) next.delete("transport");
      else next.set("transport", String(n));
      go(`${basePath}?${next}`);
    }, 600);
    return () => clearTimeout(timer);
  }, [text, value, params, basePath, go]);

  return (
    <label className="flex items-center gap-3 text-sm text-muted-foreground">
      <span className="min-w-0 flex-1">{label}</span>
      <input
        inputMode="numeric"
        value={text}
        onChange={(e) => setText(groupDigits(e.target.value).slice(0, 5))}
        className="num h-10 w-24 rounded-full border bg-card px-3 text-right text-foreground outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      />
    </label>
  );
}
