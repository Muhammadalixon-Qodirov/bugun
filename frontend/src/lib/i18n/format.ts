import type { Locale } from "@/i18n/config";
import { isEntrepreneur } from "@/lib/roles";

/** A translator scoped to the "format" messages (next-intl's t, from a hook or the server). */
export type FormatT = (key: string, values?: Record<string, string | number>) => string;

const TZ = "Asia/Tashkent";

// Browsers ship uneven "uz" locale data (some print "2026 M09 29"), so Uzbek
// month names are ours; English and Russian use Intl, which has them in full.
const uzMonths = ["yanvar", "fevral", "mart", "aprel", "may", "iyun", "iyul", "avgust", "sentabr", "oktabr", "noyabr", "dekabr"];
const uzMonthsShort = ["yan", "fev", "mar", "apr", "may", "iyn", "iyl", "avg", "sen", "okt", "noy", "dek"];
const uzWeekdays = ["dushanba", "seshanba", "chorshanba", "payshanba", "juma", "shanba", "yakshanba"];
const uzWeekdaysShort = ["Du", "Se", "Ch", "Pa", "Ju", "Sh", "Ya"];
// A Monday, for naming weekdays through Intl (index 0 = Monday, as the API counts).
const MONDAY = Date.UTC(2024, 0, 1, 12);
const tashkentParts = new Intl.DateTimeFormat("en-CA", { timeZone: TZ, year: "numeric", month: "numeric", day: "numeric" });
const tashkentDay = new Intl.DateTimeFormat("en-CA", { timeZone: TZ, year: "numeric", month: "2-digit", day: "2-digit" });

function tashkentDate(iso: string) {
  const parts = Object.fromEntries(tashkentParts.formatToParts(new Date(iso)).map((p) => [p.type, p.value]));
  return { year: Number(parts.year), month: Number(parts.month), day: Number(parts.day) };
}

/**
 * Dates, ages, units and roles in the visitor's language. Dates are stored in
 * UTC and always shown in Asia/Tashkent.
 */
export function makeFormat(locale: Locale, t: FormatT) {
  const intlDate = locale === "uz" ? null : new Intl.DateTimeFormat(locale, { timeZone: TZ, day: "numeric", month: "long", year: "numeric" });
  const intlDay = locale === "uz" ? null : new Intl.DateTimeFormat(locale, { timeZone: TZ, day: "numeric", month: "long" });

  /** "28-sentabr, 2026" / "September 28, 2026" / "28 сентября 2026 г." */
  const date = (iso: string) => {
    if (intlDate) return intlDate.format(new Date(iso));
    const d = tashkentDate(iso);
    return `${d.day}-${uzMonths[d.month - 1]}, ${d.year}`;
  };
  const shortDay = (iso: string) => {
    if (intlDay) return intlDay.format(new Date(iso));
    const d = tashkentDate(iso);
    return `${d.day}-${uzMonths[d.month - 1]}`;
  };

  const decimal = (n: number, digits: number) => {
    const s = n.toFixed(digits).replace(/\.0+$/, "");
    return locale === "en" ? s : s.replace(".", ",");
  };
  const intlMonth = (style: "short" | "long") =>
    locale === "uz" ? null : new Intl.DateTimeFormat(locale, { timeZone: "UTC", month: style });
  const monthShortFmt = intlMonth("short");
  const monthLongFmt = intlMonth("long");
  const weekdayFmt = (style: "short" | "long") => (locale === "uz" ? null : new Intl.DateTimeFormat(locale, { timeZone: "UTC", weekday: style }));
  const weekdayShortFmt = weekdayFmt("short");
  const weekdayLongFmt = weekdayFmt("long");
  const intlDayShort = locale === "uz" ? null : new Intl.DateTimeFormat(locale, { timeZone: TZ, day: "numeric", month: "short" });
  const intlTime = new Intl.DateTimeFormat("en-GB", { timeZone: TZ, hour: "2-digit", minute: "2-digit" });

  return {
    date,
    /** "18-sentabr – 24-sentabr" (spec §7.4). */
    range: (fromIso: string, toIso: string) => {
      const a = shortDay(fromIso);
      const b = shortDay(toIso);
      return a === b ? b : `${a} – ${b}`;
    },
    /** Day-level age ("bugun", "kecha", "3 kun oldin"): stable between server and browser renders. */
    dayAgo: (iso: string, now = new Date()) => {
      const days = Math.round((Date.parse(tashkentDay.format(now)) - Date.parse(tashkentDay.format(new Date(iso)))) / 86_400_000);
      if (days <= 0) return t("today");
      if (days === 1) return t("yesterday");
      return days < 30 ? t("daysAgo", { count: days }) : date(iso);
    },
    until: (iso: string) => t("until", { date: date(iso) }),
    since: (iso: string) => t("since", { date: date(iso) }),
    unit: (unit: string) => (["kg", "litr", "dona", "m2", "m3", "xizmat"].includes(unit) ? t(`units.${unit}`) : unit),
    /** Account role label; pre-merge seller/buyer accounts read as entrepreneurs. */
    role: (role: string) => {
      const key = isEntrepreneur(role) ? "entrepreneur" : role;
      return key === "entrepreneur" || key === "market_admin" || key === "admin" ? t(`roles.${key}`) : role;
    },
    /** Month name, 1 = January: "avgust" / "August" / "август". */
    month: (m: number) => (monthLongFmt ? monthLongFmt.format(Date.UTC(2024, m - 1, 15)) : uzMonths[m - 1]),
    /** Short month for chart axes: "avg" / "Aug" / "авг.". */
    monthShort: (m: number) => (monthShortFmt ? monthShortFmt.format(Date.UTC(2024, m - 1, 15)) : uzMonthsShort[m - 1]),
    /** Weekday, 0 = Monday. */
    weekday: (i: number) => (weekdayLongFmt ? weekdayLongFmt.format(MONDAY + i * 86_400_000) : uzWeekdays[i]),
    weekdayShort: (i: number) => (weekdayShortFmt ? weekdayShortFmt.format(MONDAY + i * 86_400_000) : uzWeekdaysShort[i]),
    /** Day and short month for charts: "27-sen" / "Sep 27" / "27 сент.". */
    dayMonth: (iso: string) => {
      if (intlDayShort) return intlDayShort.format(new Date(iso));
      const d = tashkentDate(iso);
      return `${d.day}-${uzMonthsShort[d.month - 1]}`;
    },
    /** Tashkent clock time: "14:12". */
    time: (iso: string) => intlTime.format(new Date(iso)),
    /** A share as a percentage: 0.032 -> "3,2%", signed -> "+3,2%" (minus sign U+2212). */
    percent: (share: number, opts: { signed?: boolean; digits?: number } = {}) => {
      const v = share * 100;
      const digits = opts.digits ?? (Math.abs(v) < 10 ? 1 : 0);
      const body = decimal(Math.abs(v), digits);
      const sign = opts.signed ? (v > 0.0001 ? "+" : v < -0.0001 ? "\u2212" : "") : v < 0 ? "\u2212" : "";
      return `${sign}${body}%`;
    },
    /** A plain decimal in the visitor's style: 4.3 -> "4,3". */
    decimal,
  };
}

export type Format = ReturnType<typeof makeFormat>;
