/**
 * Holidays are stored with Uzbek titles (seeded, the same every year). Known
 * ones map to a translation key; anything else is shown as written.
 */
const keys: Record<string, string> = {
  "navro'z": "navruz",
  "navro'z oldidan talab oshdi": "navruzDemand",
  "qurbon hayiti": "eidAdha",
  "ramazon hayiti": "eidFitr",
  "ramazon hayiti oldidan talab": "eidFitrDemand",
  "ramazon oyi": "ramadan",
  "to'y mavsumi": "weddings",
  "yangi yil": "newYear",
};

export function holidayKey(title: string) {
  return keys[title.toLowerCase().replace(/[ʻʼ‘’`]/g, "'").trim()];
}
