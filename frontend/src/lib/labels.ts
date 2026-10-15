// Number formatting shared by every locale: so'm are grouped with spaces.

/** "4500" -> "4 500" while typing in a number field; everything but digits is dropped. */
export function groupDigits(v: string) {
  return v.replace(/\D/g, "").replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

/** 4500 -> "4 500" (so'm are whole numbers). */
export function formatSom(n: number) {
  return Math.round(n).toLocaleString("ru-RU").replace(/ /g, " ");
}
