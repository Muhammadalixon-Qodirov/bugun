/** The visitor's chosen region lives in a cookie so pages render with it on the server. */
export const REGION_COOKIE = "bugun_region";

export function setRegionCookie(id: number) {
  document.cookie = `${REGION_COOKIE}=${id}; path=/; max-age=${60 * 60 * 24 * 365}; samesite=lax`;
}
