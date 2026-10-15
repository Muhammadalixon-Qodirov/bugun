// Shared by the public statistics pages (/statistika, /hudud, /solishtirish, /xarita).

export type Trade = "ulgurji" | "chakana";

export const CATEGORIES = ["sabzavot", "meva", "gosht", "don", "oziq_ovqat", "qurilish", "xizmat"] as const;
export type Category = (typeof CATEGORIES)[number];

export const PERIODS = ["30d", "90d", "1y", "3y"] as const;
export type Period = (typeof PERIODS)[number];

type Param = string | string[] | undefined;
const first = (v: Param) => (Array.isArray(v) ? v[0] : v);

export const parseTrade = (v: Param): Trade => (first(v) === "chakana" ? "chakana" : "ulgurji");
export const parseCategory = (v: Param) => CATEGORIES.find((c) => c === first(v));
export const parsePeriod = (v: Param): Period => PERIODS.find((p) => p === first(v)) ?? "90d";

/** "a,b,,c" -> ["a", "b", "c"] */
export const parseList = (v: Param) =>
  (first(v) ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

/** For search boxes: lower-cased and without apostrophes, so "Farg'ona", "Fargʻona" and "fargona" all match. */
export function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
}

/**
 * Colour steps of "against the country" shares on the heat table: where a
 * share stops being "about the same" (±2%), "a little" (up to 7%), "clearly"
 * (up to 15%) and becomes "much" cheaper or dearer. Display thresholds only:
 * the API has no such rule, so they live here and nowhere else.
 */
export const DIFF_BOUNDS = [0.02, 0.07, 0.15] as const;

/** Diverging steps: −3 (much cheaper) … 0 (within ±DIFF_BOUNDS[0]) … +3 (much dearer). */
export const DIFF_STEPS = [-3, -2, -1, 0, 1, 2, 3] as const;
export function diffStep(diff: number) {
  const a = Math.abs(diff);
  const s = DIFF_BOUNDS.filter((bound) => a >= bound).length;
  return diff < 0 ? -s : s;
}
const STEP_MIX = [0, 30, 55, 80];
/** Cheap green ↔ grey middle ↔ dear warm, from the chart tokens. */
export function diffFill(step: number) {
  if (step === 0) return "var(--viz-mid)";
  const pole = step < 0 ? "var(--viz-cheap)" : "var(--viz-dear)";
  return `color-mix(in oklab, ${pole} ${STEP_MIX[Math.abs(step)]}%, var(--viz-mid))`;
}
