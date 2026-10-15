/** Periods the analytics API accepts, shortest first. */
export const periods = ["30d", "90d", "1y", "3y"] as const;
export type Period = (typeof periods)[number];
