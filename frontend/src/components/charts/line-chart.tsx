"use client";

import { bisector, extent } from "d3-array";
import { scaleLinear, scaleTime } from "d3-scale";
import { area, curveMonotoneX, line } from "d3-shape";
import { useMemo, useState } from "react";

import { ChartTooltip, type TooltipRow } from "@/components/charts/tooltip";
import { useWidth } from "@/components/charts/use-width";

export type LinePoint = { x: number; y: number | null };
/** A null range leaves a gap in the band, like a null y does in the line. */
export type BandPoint = { x: number; lo: number | null; hi: number | null };

export type LineSeries = {
  id: string;
  label: string;
  /** Any CSS colour, usually a token: "var(--primary)". */
  color: string;
  points: LinePoint[];
  /** A shaded range drawn under the line (p25–p75, forecast low–high). */
  band?: BandPoint[];
  /** Dashed: a projection, not observed data. */
  dashed?: boolean;
};

/** A single day or a span shown behind the lines (holidays, events, jumps). */
export type ChartMark = { id: string; from: number; to?: number; label: string };

/** A flagged date on a strip along the x-axis: ✕ for one left out of the line, ? for one still checked. */
export type ChartDot = { id: string; x: number; kind: "excluded" | "checking" };

const M = { top: 12, right: 12, bottom: 26, left: 52 };

/** Dots too close to tell apart share one glyph; ✕ goes first so it is the one kept. */
function placed(dots: ChartDot[], x: (v: number) => number) {
  const kept: ChartDot[] = [];
  for (const d of [...dots].sort((a, b) => (a.kind === b.kind ? 0 : a.kind === "excluded" ? -1 : 1))) {
    if (kept.every((k) => Math.abs(x(k.x) - x(d.x)) >= 12)) kept.push(d);
  }
  return kept;
}

/**
 * Time series with an optional band per line, marks for events, a crosshair
 * that snaps to the nearest date, and keyboard stepping with the arrow keys.
 * One y-axis only: a second measure belongs in its own chart.
 */
export function LineChart({
  series,
  height = 280,
  yFormat,
  xFormat,
  titleFormat,
  marks = [],
  dots = [],
  ariaLabel,
  endLabel = true,
}: {
  series: LineSeries[];
  height?: number;
  yFormat: (v: number) => string;
  /** Axis tick label for a timestamp. */
  xFormat: (x: number) => string;
  /** Tooltip heading for a timestamp. */
  titleFormat: (x: number) => string;
  marks?: ChartMark[];
  /** Flagged dates on a strip along the x-axis; the crosshair stops on them. */
  dots?: ChartDot[];
  ariaLabel: string;
  /** Write the last value of the first series at the line's end. */
  endLabel?: boolean;
}) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const [active, setActive] = useState<number | null>(null);

  const model = useMemo(() => {
    const xs = [...new Set([...series.flatMap((s) => s.points.map((p) => p.x)), ...dots.map((d) => d.x)])].sort((a, b) => a - b);
    const ys = series.flatMap((s) => [
      ...s.points.flatMap((p) => (p.y == null ? [] : [p.y])),
      ...(s.band ?? []).flatMap((b) => (b.lo == null || b.hi == null ? [] : [b.lo, b.hi])),
    ]);
    const [x0, x1] = extent(xs) as [number, number];
    const [y0, y1] = extent(ys) as [number, number];
    return { xs, x0, x1, y0, y1 };
  }, [series, dots]);

  const innerW = Math.max(0, width - M.left - M.right);
  const innerH = height - M.top - M.bottom;
  const empty = model.xs.length === 0 || model.y0 == null;

  const x = scaleTime()
    .domain([model.x0 ?? 0, model.x1 ?? 1])
    .range([0, innerW]);
  const pad = empty ? 1 : Math.max((model.y1 - model.y0) * 0.08, model.y1 * 0.02);
  // Padded to the data, not "niced" to round ends: coarse rounding would stretch
  // a 5 000–13 000 range down to 0 and flatten the line.
  const y = scaleLinear()
    .domain(empty ? [0, 1] : [Math.max(0, model.y0 - pad), model.y1 + pad])
    .range([innerH, 0]);

  const linePath = line<LinePoint>()
    .defined((p) => p.y != null)
    .x((p) => x(p.x))
    .y((p) => y(p.y ?? 0))
    .curve(curveMonotoneX);
  const bandPath = area<BandPoint>()
    .defined((b) => b.lo != null && b.hi != null)
    .x((b) => x(b.x))
    .y0((b) => y(b.lo ?? 0))
    .y1((b) => y(b.hi ?? 0))
    .curve(curveMonotoneX);

  const xTicks = width ? x.ticks(Math.max(4, Math.floor(innerW / 80))) : [];
  const yTicks = y.ticks(4);

  const pick = (px: number) => {
    const t = x.invert(px).getTime();
    const i = bisector((v: number) => v).center(model.xs, t);
    setActive(Math.max(0, Math.min(model.xs.length - 1, i)));
  };

  const at = active == null ? null : model.xs[active];
  const valueAt = (s: LineSeries, t: number) => s.points.find((p) => p.x === t)?.y ?? null;
  const rows: TooltipRow[] =
    at == null
      ? []
      : series.flatMap((s) => {
          const v = valueAt(s, at);
          if (v == null) return [];
          const b = s.band?.find((p) => p.x === at);
          const range = b && b.lo != null && b.hi != null ? ` · ${yFormat(b.lo)}–${yFormat(b.hi)}` : "";
          return [{ key: s.id, label: `${s.label}${range}`, value: yFormat(v), color: s.color, dashed: s.dashed }];
        });
  const notes = at == null ? [] : marks.filter((m) => at >= m.from - 43_200_000 && at <= (m.to ?? m.from) + 43_200_000).map((m) => m.label);

  const first = series[0];
  const last = first ? [...first.points].reverse().find((p) => p.y != null) : undefined;

  return (
    <div ref={ref} className="relative w-full select-none" style={{ height }}>
      {width > 0 && !empty && (
        <svg
          width={width}
          height={height}
          role="img"
          aria-label={ariaLabel}
          tabIndex={0}
          className="overflow-visible outline-none focus-visible:rounded-lg focus-visible:ring-3 focus-visible:ring-ring/40"
          onKeyDown={(e) => {
            if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
            e.preventDefault();
            const step = e.key === "ArrowRight" ? 1 : -1;
            setActive((i) => Math.max(0, Math.min(model.xs.length - 1, (i ?? model.xs.length - 1) + step)));
          }}
          onBlur={() => setActive(null)}
        >
          <g transform={`translate(${M.left},${M.top})`}>
            {marks.map((m) => {
              const a = x(m.from);
              const b = m.to ? x(m.to) : a;
              if (b < 0 || a > innerW) return null;
              const left = Math.max(0, a);
              const w = Math.max(1, Math.min(innerW, b) - left);
              return m.to && w > 2 ? (
                <rect key={m.id} x={left} y={0} width={w} height={innerH} className="fill-foreground/[0.035]" />
              ) : (
                <line key={m.id} x1={left} x2={left} y1={0} y2={innerH} className="stroke-foreground/15" />
              );
            })}
            {yTicks.map((v) => (
              <g key={v} transform={`translate(0,${y(v)})`}>
                <line x2={innerW} className="stroke-viz-grid" />
                <text x={-10} dy="0.32em" textAnchor="end" className="num fill-muted-foreground text-[11px]">
                  {yFormat(v)}
                </text>
              </g>
            ))}
            <line y1={innerH} y2={innerH} x2={innerW} className="stroke-viz-axis" />
            {xTicks.map((t) => (
              <text key={+t} x={x(t)} y={innerH + 18} textAnchor="middle" className="fill-muted-foreground text-[11px]">
                {xFormat(+t)}
              </text>
            ))}
            {series.map((s) =>
              s.band ? <path key={`${s.id}-band`} d={bandPath(s.band) ?? ""} fill={s.color} fillOpacity={0.1} /> : null,
            )}
            {series.map((s) => (
              <path
                key={s.id}
                d={linePath(s.points) ?? ""}
                fill="none"
                stroke={s.color}
                strokeWidth={2}
                strokeLinejoin="round"
                strokeLinecap="round"
                strokeDasharray={s.dashed ? "5 4" : undefined}
              />
            ))}
            {placed(dots, x).map((d) => {
              const cx = x(d.x);
              const cy = innerH - 9;
              return d.kind === "excluded" ? (
                <g key={d.id} transform={`translate(${cx},${cy})`} className="stroke-muted-foreground" strokeWidth={2} strokeLinecap="round">
                  <circle r={7} fill="var(--card)" stroke="none" />
                  <line x1={-4} y1={-4} x2={4} y2={4} />
                  <line x1={-4} y1={4} x2={4} y2={-4} />
                </g>
              ) : (
                <g key={d.id} transform={`translate(${cx},${cy})`}>
                  <circle r={7} fill="var(--card)" stroke="var(--warm)" strokeWidth={2} />
                  <text dy="0.35em" textAnchor="middle" className="fill-warm-foreground text-[10px] font-semibold">
                    ?
                  </text>
                </g>
              );
            })}
            {endLabel && first && last && last.y != null && at == null && (
              <g transform={`translate(${x(last.x)},${y(last.y)})`}>
                <circle r={4} fill={first.color} stroke="var(--card)" strokeWidth={2} />
              </g>
            )}
            {at != null && (
              <g>
                <line x1={x(at)} x2={x(at)} y1={0} y2={innerH} className="stroke-foreground/30" />
                {series.map((s) => {
                  const v = valueAt(s, at);
                  return v == null ? null : <circle key={s.id} cx={x(at)} cy={y(v)} r={4} fill={s.color} stroke="var(--card)" strokeWidth={2} />;
                })}
              </g>
            )}
            <rect
              width={innerW}
              height={innerH}
              fill="transparent"
              style={{ touchAction: "pan-y" }}
              onPointerMove={(e) => pick(e.nativeEvent.offsetX - M.left)}
              onPointerDown={(e) => pick(e.nativeEvent.offsetX - M.left)}
              onPointerLeave={(e) => e.pointerType === "mouse" && setActive(null)}
            />
          </g>
        </svg>
      )}
      {at != null && (rows.length > 0 || notes.length > 0) && (
        <ChartTooltip x={M.left + x(at)} y={M.top} width={width} title={titleFormat(at)} rows={rows} notes={notes} />
      )}
    </div>
  );
}
