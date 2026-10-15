"use client";

import { geoArea, geoMercator, geoPath, type GeoPermissibleObjects } from "d3-geo";
import { useMemo, useState } from "react";

import { ChartTooltip, type TooltipRow } from "@/components/charts/tooltip";
import { useWidth } from "@/components/charts/use-width";

type Ring = [number, number][];

/**
 * d3-geo reads rings on the sphere: a polygon wound the other way round (as
 * RFC 7946 GeoJSON is) covers the whole globe except the province. Such
 * polygons get their rings reversed; everything else passes through.
 */
function rewind(geometry: GeoPermissibleObjects): GeoPermissibleObjects {
  const g = geometry as { type: string; coordinates?: unknown };
  const fix = (rings: Ring[]) => (geoArea({ type: "Polygon", coordinates: rings }) > 2 * Math.PI ? rings.map((r) => [...r].reverse()) : rings);
  if (g.type === "Polygon") return { type: "Polygon", coordinates: fix(g.coordinates as Ring[]) };
  if (g.type === "MultiPolygon") return { type: "MultiPolygon", coordinates: (g.coordinates as Ring[][]).map(fix) };
  return geometry;
}

export type MapFeature = {
  id: string;
  geometry: GeoPermissibleObjects;
  /** Name for the tooltip and the accessible label. */
  name: string;
  fill: string;
  rows: TooltipRow[];
  notes?: string[];
};

/**
 * Provinces as an SVG map, fitted to the container. Each province is a hover,
 * focus and click target; the selected one is outlined.
 */
export function Choropleth({
  features,
  selected,
  onSelect,
  ariaLabel,
  aspect = 0.56,
}: {
  features: MapFeature[];
  selected?: string | null;
  onSelect?: (id: string) => void;
  ariaLabel: string;
  /** Height as a share of the width. */
  aspect?: number;
}) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<{ id: string; x: number; y: number } | null>(null);
  const height = Math.round(width * aspect);

  const paths = useMemo(() => {
    if (!width) return [];
    const shapes = features.map((f) => ({ id: f.id, geometry: rewind(f.geometry) }));
    const collection = { type: "FeatureCollection", features: shapes.map((s) => ({ type: "Feature", properties: {}, geometry: s.geometry })) };
    const projection = geoMercator().fitSize([width, height], collection as GeoPermissibleObjects);
    const path = geoPath(projection);
    return shapes.map((s) => ({ id: s.id, d: path(s.geometry) ?? "", centroid: path.centroid(s.geometry) }));
  }, [features, width, height]);

  const byId = new Map(features.map((f) => [f.id, f]));
  const current = hover ? byId.get(hover.id) : null;

  return (
    <div ref={ref} className="relative w-full" style={{ height: height || undefined, aspectRatio: width ? undefined : `${1 / aspect}` }}>
      {width > 0 && (
        <svg width={width} height={height} role="img" aria-label={ariaLabel} className="overflow-visible">
          {paths.map((p) => {
            const f = byId.get(p.id)!;
            const on = selected === p.id;
            return (
              <path
                key={p.id}
                d={p.d}
                fill={f.fill}
                stroke="var(--card)"
                strokeWidth={on ? 0 : 1.5}
                strokeLinejoin="round"
                tabIndex={0}
                role="button"
                aria-label={f.name}
                aria-pressed={onSelect ? on : undefined}
                className="cursor-pointer outline-none transition-[filter,opacity] duration-150 hover:brightness-95 focus-visible:brightness-90 dark:hover:brightness-125"
                onPointerMove={(e) => setHover({ id: p.id, x: e.nativeEvent.offsetX, y: e.nativeEvent.offsetY })}
                onPointerLeave={(e) => e.pointerType === "mouse" && setHover(null)}
                onFocus={() => setHover({ id: p.id, x: p.centroid[0], y: p.centroid[1] })}
                onBlur={() => setHover(null)}
                onClick={(e) => {
                  setHover({ id: p.id, x: e.nativeEvent.offsetX, y: e.nativeEvent.offsetY });
                  onSelect?.(p.id);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect?.(p.id);
                  }
                }}
              />
            );
          })}
          {selected && paths.find((p) => p.id === selected) && (
            <path d={paths.find((p) => p.id === selected)!.d} fill="none" className="pointer-events-none stroke-foreground" strokeWidth={2} strokeLinejoin="round" />
          )}
        </svg>
      )}
      {current && hover && (
        <ChartTooltip x={hover.x} y={Math.max(0, hover.y - 40)} width={width} title={current.name} rows={current.rows} notes={current.notes} />
      )}
    </div>
  );
}
