"use client";

import { scaleBand, scaleLinear } from "d3-scale";
import { useState } from "react";

import { ChartTooltip, type TooltipRow } from "@/components/charts/tooltip";
import { useWidth } from "@/components/charts/use-width";

export type ColumnSeries = { id: string; label: string; color: string };
export type Column = {
  key: string;
  /** Axis label under the column. */
  label: string;
  /** Tooltip heading. */
  title: string;
  values: number[];
  /** Marks the column (the current month, the cheapest day). */
  highlight?: boolean;
};

const M = { top: 10, right: 4, bottom: 24, left: 44 };

/** Rounded at the data end, square at the baseline (grows up or down from it). */
function columnPath(x: number, w: number, from: number, to: number) {
  const h = Math.abs(to - from);
  const r = Math.min(4, w / 2, h);
  if (h < 0.5) return "";
  if (to < from) {
    return `M${x},${from}V${to + r}Q${x},${to} ${x + r},${to}H${x + w - r}Q${x + w},${to} ${x + w},${to + r}V${from}Z`;
  }
  return `M${x},${from}V${to - r}Q${x},${to} ${x + r},${to}H${x + w - r}Q${x + w},${to} ${x + w},${to - r}V${from}Z`;
}

/**
 * Columns from a baseline: 0 for counts, or 1 for an index (1.2 = 20% dearer
 * than usual) where columns go up or down. Several series sit side by side
 * with a 2px gap. Each column is its own hover and focus target.
 */
export function ColumnChart({
  data,
  series,
  height = 200,
  baseline = 0,
  colorFor,
  yFormat,
  valueFormat = yFormat,
  ariaLabel,
  integer = false,
}: {
  data: Column[];
  series: ColumnSeries[];
  height?: number;
  baseline?: number;
  /** Per-column colour for single-series charts (e.g. cheap vs dear). */
  colorFor?: (value: number, column: Column) => string;
  yFormat: (v: number) => string;
  valueFormat?: (v: number) => string;
  ariaLabel: string;
  /** Counts: only whole-number ticks on the axis. */
  integer?: boolean;
}) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const [active, setActive] = useState<number | null>(null);

  const innerW = Math.max(0, width - M.left - M.right);
  const innerH = height - M.top - M.bottom;
  const all = data.flatMap((d) => d.values);
  const lo = Math.min(baseline, ...all);
  const hi = Math.max(baseline, ...all);
  const y = scaleLinear().domain([lo, hi]).range([innerH, 0]).nice(4);
  const band = scaleBand<string>()
    .domain(data.map((d) => d.key))
    .range([0, innerW])
    .paddingInner(0.3)
    .paddingOuter(0.15);
  const groupW = band.bandwidth();
  const colW = Math.min(24, (groupW - (series.length - 1) * 2) / series.length);
  const groupUsed = colW * series.length + (series.length - 1) * 2;
  const labelEvery = Math.max(1, Math.ceil(data.length / Math.max(1, Math.floor(innerW / 34))));

  const cur = active == null ? null : data[active];
  const rows: TooltipRow[] = cur ? series.map((s, i) => ({ key: s.id, label: s.label, value: valueFormat(cur.values[i]), color: colorFor ? colorFor(cur.values[i], cur) : s.color })) : [];

  return (
    <div ref={ref} className="relative w-full select-none" style={{ height }}>
      {width > 0 && (
        <svg width={width} height={height} role="img" aria-label={ariaLabel} className="overflow-visible">
          <g transform={`translate(${M.left},${M.top})`}>
            {y
              .ticks(4)
              .filter((v) => !integer || Number.isInteger(v))
              .map((v) => (
              <g key={v} transform={`translate(0,${y(v)})`}>
                <line x2={innerW} className={v === baseline ? "stroke-viz-axis" : "stroke-viz-grid"} />
                <text x={-8} dy="0.32em" textAnchor="end" className="num fill-muted-foreground text-[11px]">
                  {yFormat(v)}
                </text>
              </g>
            ))}
            {data.map((d, i) => {
              const gx = (band(d.key) ?? 0) + (groupW - groupUsed) / 2;
              const on = active === i;
              return (
                <g
                  key={d.key}
                  tabIndex={0}
                  role="img"
                  aria-label={`${d.title}: ${series.map((s, k) => `${s.label} ${valueFormat(d.values[k])}`).join(", ")}`}
                  className="outline-none"
                  onPointerEnter={() => setActive(i)}
                  onPointerDown={() => setActive(i)}
                  onPointerLeave={(e) => e.pointerType === "mouse" && setActive(null)}
                  onFocus={() => setActive(i)}
                  onBlur={() => setActive(null)}
                >
                  <rect x={band(d.key)} y={0} width={groupW} height={innerH} fill="transparent" />
                  {series.map((s, k) => {
                    const v = d.values[k];
                    const color = colorFor ? colorFor(v, d) : s.color;
                    return (
                      <path
                        key={s.id}
                        d={columnPath(gx + k * (colW + 2), colW, y(baseline), y(v))}
                        fill={color}
                        opacity={active == null || on ? 1 : 0.45}
                        className="transition-opacity duration-150"
                      />
                    );
                  })}
                  {d.highlight && <circle cx={(band(d.key) ?? 0) + groupW / 2} cy={innerH + 20} r={2} className="fill-foreground" />}
                  {i % labelEvery === 0 && (
                    <text
                      x={(band(d.key) ?? 0) + groupW / 2}
                      y={innerH + 14}
                      textAnchor="middle"
                      className={d.highlight ? "fill-foreground text-[11px] font-medium" : "fill-muted-foreground text-[11px]"}
                    >
                      {d.label}
                    </text>
                  )}
                </g>
              );
            })}
          </g>
        </svg>
      )}
      {cur && (
        <ChartTooltip
          x={M.left + (band(cur.key) ?? 0) + groupW / 2}
          y={M.top}
          width={width}
          title={cur.title}
          rows={rows}
        />
      )}
    </div>
  );
}
