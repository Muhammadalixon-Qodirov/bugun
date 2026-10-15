# Tahlil metodologiyasi

Bu hujjat mahsulot tahlili sahifasi, viloyat sahifasi, issiqlik xaritasi, TOP talab, solishtirish va haftalik PDF hisobotdagi har bir raqam qanday hisoblanishini tushuntiradi. Asosiy hisoblar **deterministik statistika**: medianlar, o'rtachalar, chiziqli regressiya, klassik mavsumiy dekompozitsiya va Pearson korrelyatsiyasi. Ularning ustiga qo'shilgan ML usullari (prognoz usullari musobaqasi, kunlik anomaliyalar, viloyat klasterlari, Local Moran I, Shapley qiymatlari, Isolation Forest) ham deterministik: qat'iy urug' (seed) bilan ishlaydi va bir xil ma'lumotdan har doim bir xil natija chiqadi. Ular [ML usullari](12-ml-usullari.md) hujjatida batafsil yozilgan. Asosiy tamoyil: **raqamlar hech qachon to'qib chiqarilmaydi; ma'lumot yetarli bo'lmasa, ko'rsatkich ko'rsatilmaydi** (JSON'da maydon tushib qoladi yoki `null`/bo'sh ro'yxat bo'ladi). Har bo'limda shu "yetarlilik" chegarasi alohida yozilgan.

Jonli narxlar qanday yig'ilishi, manba ishonchi, vaznli median va kvantillar [narx metodologiyasi](03-narx-metodologiyasi.md)da. Talab signallari qayerdan kelishi [savdo va talab](05-savdo-va-talab.md)da.

## Mundarija

1. [Umumiy tamoyillar](#1-umumiy-tamoyillar)
2. [Kunlik narx tarixi (`price_daily`)](#2-kunlik-narx-tarixi-price_daily)
3. [Demo tarix](#3-demo-tarix)
4. [O'zgarishlar: d1, d7, d30, y1](#4-ozgarishlar-d1-d7-d30-y1)
5. [Trend](#5-trend)
6. [Mavsumiylik](#6-mavsumiylik)
7. [Hafta kunlari](#7-hafta-kunlari)
8. [Prognoz va backtest](#8-prognoz-va-backtest)
9. [Talab–taklif bosimi indeksi](#9-talabtaklif-bosimi-indeksi)
10. [Xarid/sotish maslahati](#10-xaridsotish-maslahati)
11. [Hududlar, arbitraj, chakana ustama, savdolashish](#11-hududlar-arbitraj-chakana-ustama-savdolashish)
12. [Rekordlar, keskin sakrashlar va voqealar](#12-rekordlar-keskin-sakrashlar-va-voqealar)
13. [Ob-havo](#13-ob-havo)
14. [Valyuta kurslari](#14-valyuta-kurslari)
15. [Sotuvchilar aytgan sabablar](#15-sotuvchilar-aytgan-sabablar)
16. [Taklif (supply) bloki va ma'lumot sifati](#16-taklif-supply-bloki-va-malumot-sifati)
17. [Viloyat sahifasi](#17-viloyat-sahifasi)
18. [Issiqlik xaritasi](#18-issiqlik-xaritasi)
19. [TOP talab](#19-top-talab)
20. [Solishtirish](#20-solishtirish)
21. [Haftalik PDF hisobot](#21-haftalik-pdf-hisobot)
22. [Kesh va yangilik](#22-kesh-va-yangilik)
23. [Qayerda joylashgan](#qayerda-joylashgan)

---

## 1. Umumiy tamoyillar

- **Vaqt zonasi.** Barcha "kun" va "hafta"lar `Asia/Tashkent` bo'yicha. `insight.Day()` vaqtni Toshkent kalendar kuniga aylantiradi. Hafta dushanbadan boshlanadi (Postgres `date_trunc('week', ...)` va Go'dagi `weekly()` ham shunday).
- **Ulush (share) ko'rinishi.** Deyarli barcha nisbiy qiymatlar kasr sifatida qaytadi: `0.12` = +12%, `-0.08` = −8%. Ular 3 xonagacha yaxlitlanadi (`round3`, `diff`).
- **`diff(x, ref)`** — `x / ref − 1`; `x` yoki `ref` nolga teng yoki manfiy bo'lsa `nil` (ko'rsatilmaydi). Bu yordamchi hamma joyda ishlatiladi (`insight/regions.go`).
- **Respublika qiymati.** "O'zbekiston bo'yicha" narx — viloyat medianlarining **narxlar soni bilan vaznlangan o'rtachasi**:
  `respublika = Σ(median_v × offers_v) / Σ offers_v`, bu yerda `v` — viloyat, `offers_v` — shu viloyatdagi narxlar soni.
- **Minimal hajm chegaralari** (hammasi koddan):

  | Chegara | Qiymat | Qayerda |
  |---|---|---|
  | Bir viloyat-kun `price_daily`ga yozilishi uchun | ≥ 3 narx | `BuildPriceDaily` (`HAVING count(*) >= 3`) |
  | "Hozirgi" statistika (`now`) | ≥ 3 jonli narx | `analytics.MinObservations = 3` |
  | Viloyat reytingi, xarita katagi | ≥ 5 narx 14 kunda | `minProvinceOffers = 5` |
  | Tuman va bozor reytingi | ≥ 3 narx 14 kunda | `analytics.MinObservations` |
  | Savdolashish chegirmasi | ≥ 5 tasdiqlangan savdo 90 kunda | `minDeals = 5` |
  | Trend | 28 kundan ≥ 7 nuqta | `TrendOf` |
  | Hafta kunlari | 12 haftadan ≥ 8 to'liq hafta | `minWeekdayWeeks = 8` |
  | Mavsumiylik | ≥ 12 namuna, har oy kamida bir marta | `minSeasonSamples = 12` |
  | Prognoz | ≥ 20 tekshirilgan hafta | `minBacktests = 20` |
  | Bosim indeksi | ≥ 2 oldingi hafta ma'lumotli | `analytics.MinHistory = 2` |
  | Valyuta korrelyatsiyasi | ≥ 20 nuqta | `minCorrelationPoints = 20` |
  | TOP talab | ball ≥ 5 | `minDemandScore = 5` |

---

## 2. Kunlik narx tarixi (`price_daily`)

**G'oya.** Tahlil sahifasi yillar davomidagi narxni ko'rsatadi. Har so'rovda millionlab kuzatuvlarni qayta hisoblamaslik uchun har bir *mahsulot × savdo turi × viloyat × kun* uchun bitta tayyor qator saqlanadi.

**Grafiklar tozalangan tarixni o'qiydi.** Tahlil sahifasi va hisoblar `price_daily_clean` dan foydalanadi: bu `price_daily` ning o'zi, faqat anomaliya deb belgilangan kunlarsiz: admin tasdiqlagan (`confirmed`) yoki ochiq, lekin "aniq" (`settled`, oxirgi 4 kundan eski) belgilar chiqariladi; admin rad etgan (`dismissed`) kun qaytadi ([ML usullari](12-ml-usullari.md), kunlik anomaliyalar bo'limi).

**Jadval** (`migrations/00024_price_daily.sql`, `00025_price_daily_synthetic.sql`): `product_id`, `trade_type` (`ulgurji`/`chakana`), `province_id`, `day`, `median`, `p25`, `p75`, `min_price`, `max_price`, `offers`, `demo_share`, `synthetic`. Asosiy kalit: `(product_id, trade_type, province_id, day)`.

**Qanday quriladi** (`queries/insight.sql` → `BuildPriceDaily`):

1. Olinadigan narxlar: `price_observations` dan `status IN ('active','expired')` va `source_trust(source) > 0`. Ya'ni tekshiruvda ushlangan (`review`) va rad etilgan narxlar hamda rasmiy benchmarklar (ishonchi 0) kirmaydi.
2. Narx **amal qilgan har bir Toshkent kuniga** yoziladi: `observed_at` kunidan `valid_until − 1 soniya` kunigacha (`generate_series`). 5 kun amal qilgan narx 5 kunlik qatorga kiradi.
3. Tuman narxi o'z viloyatiga yig'iladi: `COALESCE(r.parent_id, r.id)`.
4. Kun ichida **faqat manba ishonchi bilan** vaznlanadi (`source_trust`), yangilik bo'yicha pasayish qo'llanmaydi — aks holda tarix buziladi:

   | Manba | `source_trust` |
   |---|---|
   | `rfq_offer`, `deal` | 1.0 |
   | `seller` | 0.8 |
   | `telegram_channel` | 0.6 |
   | `olx` | 0.5 |
   | `seller_auto` | 0.4 |
   | boshqalar (benchmark) | 0 (chiqariladi) |

5. `median`, `p25`, `p75` — `weighted_quantile(narxlar, vaznlar, q)` bilan (0.5, 0.25, 0.75). `min`, `max`, `offers = count(*)`.
6. Viloyat-kunda **kamida 3 narx** bo'lmasa qator yozilmaydi.
7. `demo_share` = shu qatordagi narxlardan demo sotuvchilarnikining ulushi (`avg(is_demo)`).

**Misol.** Andijon, pomidor, ulgurji, 12-may: narxlar 5 000 (`seller`, 0.8), 5 200 (`seller`, 0.8), 4 800 (`rfq_offer`, 1.0), 6 000 (`seller_auto`, 0.4). Tartiblangan: 4 800 (1.0), 5 000 (0.8), 5 200 (0.8), 6 000 (0.4); jami vazn 3.0, yarmi 1.5. Yig'ma vazn: 1.0 → 1.8 — 1.5 ga 5 000 da yetadi, demak median 5 000 (interpolyatsiyasiz "pastki vaznli kvantil" qoidasi, [narx metodologiyasi](03-narx-metodologiyasi.md)ga qarang). `offers = 4`, `min = 4 800`, `max = 6 000`, demo sotuvchi bo'lmasa `demo_share = 0`.

**Soatlik qayta qurish.** `price_daily` rejalashtirilgan ish (`cmd/api/main.go`, nomi `price_daily`) har soatda (`jobs.Every(time.Hour)`) `insight.Service.RefreshRecent`ni chaqiradi: **kecha va bugun** qayta quriladi. `RebuildDaily` bitta tranzaksiyada: avval shu kunlarning `synthetic = FALSE` qatorlari o'chiriladi (`DeletePriceDaily`), keyin qayta yoziladi — o'quvchi yarim qurilgan holatni ko'rmaydi. Ish `job_runs` jadvalidagi slot bilan himoyalangan: ikki server bo'lsa ham bir soatda bir marta ishlaydi.

> Eslatma: faqat oxirgi 2 kun qayta quriladi. Agar eski narx keyinroq rad etilsa, uning eski kunlardagi izi `price_daily`da qoladi (qo'lda `RebuildDaily` chaqirilmaguncha).

**Bucket (guruhlash).** `PriceSeries` so'rovi kunlik qatorlarni kerakli bucketga yig'adi (`date_trunc(bucket, day)`), viloyatlar bo'yicha narxlar soni bilan vaznlab:

| Davr (`period`) | Kunlar | Bucket |
|---|---|---|
| `30d` | 30 | kun |
| `90d` | 90 | kun |
| `1y` | 365 | kun |
| `3y` | 1095 | hafta (dushanba) |

Mavsumiylik uchun ichkarida `month` bucket ham ishlatiladi.

**`demo_share`.** Grafikdagi har nuqtada va butun davr uchun (`quality.demo_share`) — demo narxlar ulushi, narxlar soni bilan vaznlangan. 0 dan katta bo'lsa, interfeys "Demo" belgisini ko'rsatishi kerak.

---

## 3. Demo tarix

Platformada hali yillik haqiqiy narxlar yo'q, shuning uchun tahlil sahifasi **generatsiya qilingan demo tarix** ustida ko'rsatiladi (`cmd/seed/history.go`). Muhim kafolatlar:

- Demo tarix **faqat `price_daily`ga**, `synthetic = TRUE` va `demo_share = 1` bilan yoziladi. `price_observations`ga hech narsa yozilmaydi, demak jonli medianlarga ta'sir qilmaydi.
- Soatlik qayta qurish `synthetic` qatorlarni o'chirmaydi; lekin qaysi viloyat-kunda ≥ 3 haqiqiy narx bo'lsa, `BuildPriceDaily`ning `ON CONFLICT ... synthetic = FALSE` qismi demo qatorni haqiqiysi bilan almashtiradi.
- Seed paytida ham `INSERT ... ON CONFLICT DO NOTHING`: haqiqiy kunlar saqlanib qoladi.

**Ishga tushirish:** `go run ./cmd/seed -history N` (N yil) yoki `-history-only` (sotuvchi/e'lonlarga tegmay faqat tarixni almashtirish). `-purge` demo ma'lumotlar bilan birga `demo_share > 0` qatorlarni va `is_demo` voqealarni o'chiradi.

**Qanday yasaladi.** Har mahsulot va viloyat uchun har kungi daraja:

```
daraja = narx × (1 + inflyatsiya)^yil × mavsum(kun) / mavsum(bugun)
         × (1 + viloyat_farqi) × (1 + shovqin) × (1 + voqea_ta'siri)
```

- `narx` — mahsulotning hozirgi jonli ulgurji medianasi (≥ 3 jonli narx bo'lsa), aks holda seed narxi. Shunday qilib tarix bugungi jonli bozor bilan sakrashsiz tutashadi.
- `inflyatsiya = 0.10` yiliga; `yil` — bugungacha bo'lgan masofa (manfiy).
- `mavsum(kun) = 1 + A × cos(2π × (oy − cho'qqi_oy) / 12)`. Kategoriyalar bo'yicha: `sabzavot` A=0.35, cho'qqi 2.5 (fevral–mart); `meva` 0.30, 4.5; `don` 0.08, 5.5; `gosht` 0.06, 3.5; `oziq_ovqat` 0.04, 1.5; `qurilish` 0.05, 7; `xizmat` 0.05, 9.
- `viloyat_farqi` — mahsulot+SOATO xeshidan ±10% doimiy farq; Toshkent shahri (1726) uchun qo'shimcha +8%.
- `shovqin` — ikki AR(1) jarayon yig'indisi (mahsulot uchun umumiy + viloyatning o'zi): `x = 0.93·x + 0.012·N(0,1)`.
- `voqea_ta'siri` — 8 ta demo voqea (sovuq, eksport, dizel, Ramazon, rekord hosil, yo'llar yopilishi, Navro'z, jazirama), har biri 3 kunda o'sadi va ikkinchi yarmida so'nadi.
- Chakana = ulgurji × (1 + mahsulot yoki kategoriya ustamasi).
- `p25/p75` = median × (1 ∓ s), `min/max` = median × (1 − 2s) / (1 + 2.2s), bu yerda `s` 0.06–0.10 oralig'ida tasodifiy; `offers` 3–15.

**Zichlik.** Oxirgi 1 yil kunma-kun, undan eskisi faqat **dushanbalar** (haftalik) yoziladi — bu hajmni kichik tutadi; haftalik va oylik bucketlar bunga mos keladi.

**Belgilash.** Demo voqealar `market_events`ga `is_demo = TRUE` va izoh `Demo voqea: demo narx tarixini tushuntirish uchun` bilan yoziladi; API'da `events[].demo = true`.

---

## 4. O'zgarishlar: d1, d7, d30, y1

Barchasi oxirgi ~400 kunlik **kunlik** seriyadan (`ChangesOf`, `insight/stats.go`). `windowMean(end, n)` — `(end − n kun, end]` oralig'idagi kunlik medianlar o'rtachasi.

| Maydon | Formula |
|---|---|
| `d1` | `oxirgi_kun / undan_oldingi_kun − 1` |
| `d7` | `o'rtacha(oxirgi 7 kun) / o'rtacha(7 kun oldin tugagan 7 kun) − 1` |
| `d30` | `o'rtacha(oxirgi 7 kun) / o'rtacha(30 kun oldin tugagan 7 kun) − 1` |
| `y1` | `o'rtacha(oxirgi 7 kun) / o'rtacha(365 kun oldin tugagan 7 kun) − 1` |

Taqqoslanadigan oynada ma'lumot bo'lmasa maydon `null`.

**Misol.** Oxirgi 7 kun o'rtachasi 5 250, bir oy oldingi hafta o'rtachasi 5 000 → `d30 = 5250/5000 − 1 = 0.05` (+5%).

---

## 5. Trend

**G'oya.** Oxirgi 4 hafta narxi qaysi tomonga va qanchalik tez ketyapti.

**Qoida** (`TrendOf`):

1. Oxirgi **28 kun** nuqtalari olinadi; **7 tadan kam** bo'lsa trend ko'rsatilmaydi.
2. Eng kichik kvadratlar bilan to'g'ri chiziq: `y = a + b·x`, `x` — kun raqami, `y` — kunlik median.
3. Haftalik sur'at: `weekly = b × 7 / ȳ`, bu yerda `ȳ` — shu 28 kunlik o'rtacha narx.
4. Yo'nalish: `weekly > 0.01` → `up`, `weekly < −0.01` → `down`, aks holda `flat` (haftasiga 1% dan sekin).

**Misol.** 28 kunda chiziq qiyaligi kuniga +15 so'm, o'rtacha narx 5 000. `weekly = 15 × 7 / 5000 = 0.021` → `up`, "haftasiga taxminan 2%".

---

## 6. Mavsumiylik

**G'oya.** Qaysi oyda mahsulot odatda qimmat yoki arzon. Inflyatsiya va uzoq muddatli trendni chiqarib tashlash uchun **klassik multiplikativ dekompozitsiya** ishlatiladi.

**Qoida** (`SeasonalIndex`, `insight/season.go`):

1. Oxirgi 4 yilning oylik medianlari olinadi (`PriceSeries`, `bucket = month`).
2. Har oy `m` uchun **markazlashtirilgan 2×12 harakatlanuvchi o'rtacha**:
   `MA(m) = (0.5·P(m−6) + P(m−5) + … + P(m+5) + 0.5·P(m+6)) / 12`
   (13 oy, ikki chetdagisi yarim vazn bilan; jami vazn 12). Agar ±6 oydan birortasi yo'q bo'lsa, bu oy tashlanadi.
3. Nisbat: `r(m) = P(m) / MA(m)`.
4. Nisbatlar kalendar oy bo'yicha o'rtachalanadi (barcha yanvarlar, barcha fevrallar...).
5. 12 qiymat o'rtachasi **1 ga normallanadi**: `S_k = r̄_k / mean(r̄_1..r̄_12)`.
6. **Kamida 12 namuna** (to'liq ±6 oyli oylar) bo'lishi va 12 oyning har biri kamida bir marta uchrashi kerak; aks holda `seasonality` yo'q. Amalda bu taxminan 2 yillik tarix degani.

Natija: `months` (12 qiymat, yanvardan), `cheapest_month`, `dearest_month`, `samples`.

**Misol.** Mart oyi uchun: `P = 6 000`, markaziy o'rtacha `MA = 5 000` → `r = 1.20`. Uch yil martlari: 1.20, 1.16, 1.18 → o'rtacha 1.18. 12 oy o'rtachasi 1.00 ga yaqin bo'lsa, `S_mart ≈ 1.18`: "martda narx odatda yillik darajadan 18% yuqori".

---

## 7. Hafta kunlari

**Qoida** (`WeekdaysOf`):

1. Oxirgi **84 kun (12 hafta)** olinadi va dushanbadan boshlanadigan haftalarga bo'linadi.
2. Faqat **7 kuni ham bor to'liq haftalar** hisobga olinadi.
3. Har kun o'z haftasi o'rtachasiga bo'linadi: `k = P(kun) / o'rtacha(hafta)`.
4. Hafta kuni bo'yicha o'rtachalanadi → `days` (7 qiymat, dushanbadan).
5. Har bir hafta kuni **kamida 8 haftada** uchrashi shart (`minWeekdayWeeks = 8`), aks holda blok yo'q.
6. `cheapest_day`, `dearest_day` (1 = dushanba … 7 = yakshanba), `spread = eng_qimmat / eng_arzon − 1`. `spread < 0.01` bo'lsa, amalda hafta kuni qonuniyati yo'q.

**Misol.** `days = [0.98, 0.99, 1.00, 1.00, 1.01, 1.03, 0.99]` → eng arzon dushanba, eng qimmat shanba, `spread = 1.03/0.98 − 1 = 0.051` (5%).

---

## 8. Prognoz va backtest

**Model** (`insight/forecast.go`). Kod uchta usulni (`naive`, `last_year`, `season_trend`) o'tgan yilning bir xil haftalarida sinaydi va eng kichik MAPE bergan usulni tanlaydi. Natijada `method`, har bir usulning xatosi (`methods`), oddiy usulga nisbatan yutuq (`skill`) va har hafta uchun Shapley `drivers` beriladi. Musobaqa va Shapley [ML usullari](12-ml-usullari.md) hujjatida yozilgan. Quyidagi formula — `season_trend` usuli: mavsumiy indeks va so'nuvchi (damped) trend bilan oxirgi hafta narxini oldinga surish.

**Keyingi 4 hafta** uchun (`h = 1..4`):

```
baza   = o'rtacha(oxirgi 7 kun)
f      = baza × S(maqsad_hafta_o'rtasi) / S(joriy_hafta_o'rtasi)
effekt = Σ_{k=1..h} weekly × 0.8^k        (cheklov: −0.15 ≤ effekt ≤ +0.15)
f      = f × (1 + effekt)
```

- `S(kun)` — oylik mavsumiy indeks kunga chiziqli interpolyatsiya qilingan (har oy qiymati oyning o'rtasida turadi).
- `weekly` — 5-bo'limdagi trend (bo'lmasa trend qismi qo'shilmaydi).
- `trendDamping = 0.8`, `maxTrendEffect = 0.15`.

**Backtest.** Usul o'tgan yilning har haftasida sinab ko'riladi: kelib chiqish nuqtasi (origin) `bugun − 365` dan `bugun − 28` gacha 7 kunlik qadam bilan. Har origin uchun faqat o'sha kungacha bo'lgan ma'lumot bilan 1–4 hafta oldinga prognoz qilinadi va haqiqiy haftalik o'rtachaga solishtiriladi: `xato = |f / haqiqiy − 1|`.

- `mape[h]` — `h` hafta oldinga bo'lgan xatolar o'rtachasi (MAPE).
- `tested` — barcha to'rt gorizont (1–4 hafta) hisoblangan originlar soni. **20 dan kam** bo'lsa prognoz yo'q.
- **Oraliq (`low`–`high`)**: har gorizont uchun xatolarning **80-persentili** `q` (tartiblangan ro'yxatda indeks `ceil(0.8·n) − 1`): `low = f × (1 − q)`, `high = f × (1 + q)`. Ya'ni o'tmishdagi xatolarning 80% shu oraliqqa sig'gan.
- Natijalar "haddan tashqari aniq" ko'rinmasligi uchun yaxlitlanadi (`roundPrice`): ≥100 000 → 1 000 ga; ≥10 000 → 100 ga; ≥1 000 → 50 ga; aks holda 10 ga.

**Qachon yo'q:** mavsumiylik yo'q bo'lsa, backtest 20 haftadan kam bo'lsa, yoki biror gorizontda umuman xato hisoblanmasa.

**Halollik izohi** (API'dagi `note`): mavsumiy indeks butun tarixdan tuziladi, shuning uchun backtestdagi aniqlik biroz optimistik ko'rinishi mumkin (kelajak ma'lumoti indeksga kirgan).

**Misol.** `baza = 5 000`, `S(hozir) = 0.95`, `S(4 hafta keyin) = 1.00`, `weekly = 0.02`.
`effekt(4) = 0.02 × (0.8 + 0.64 + 0.512 + 0.4096) = 0.0472`.
`f = 5000 × 1.00/0.95 × 1.0472 ≈ 5 511` → yaxlitlangan **5 500**.
Agar 4-hafta xatolarining 80-persentili `q = 0.09` bo'lsa: `low ≈ 5 015 → 5 000`, `high ≈ 6 007 → 6 000`.

---

## 9. Talab–taklif bosimi indeksi

**G'oya.** Bu hafta mahsulotga qiziqish sotuvchilar soniga nisbatan odatdagidan ko'pmi yoki kammi.

**Talab ballari** (`analytics.DemandWeight`, `internal/analytics/pressure.go`):

| Signal | Vazn |
|---|---|
| `view` (sahifa ko'rish) | 1 |
| `search` (qidiruv) | 2 |
| `question` (savol) | 2 |
| `contact` (bog'lanish) | 3 |
| `watch` (kuzatuvga olish) | 3 |
| `rfq` (xaridor so'rovi) | 5 |
| `deal` (tasdiqlangan savdo) | 8 |

**Oynalar** (`queries/pressure.sql`, `market.PressureWindows`): beshta 7 kunlik oyna — `0` = oxirgi 7 kun, `1..4` = undan oldingi 4 hafta.

- `T_w` = oynadagi `Σ vazn × hodisalar_soni` (`demand_events` + `deals.status = 'confirmed'`).
- `S_w` = oynada biror vaqt jonli narxi (`seller`, `seller_auto` yoki `rfq_offer`) bo'lgan **alohida sotuvchilar** soni.

**Formula** (`analytics.Pressure`):

```
I = (T_0 / T̄) / (S_0 / S̄)
```

`T̄`, `S̄` — 1..4 oynalardan **faqat 0 dan katta** qiymatlar o'rtachasi.

**Yetarli tarix qoidasi:** talab uchun ham, taklif uchun ham kamida **2 ta** oldingi oynada ma'lumot bo'lishi va `S_0 > 0` bo'lishi shart; aks holda indeks ko'rsatilmaydi (shovqinni o'lchab qolmaslik uchun).

**Zonalar:** `I < 0.8` → `surplus` (taklif ko'p, xarid uchun qulay); `0.8 ≤ I ≤ 1.2` → `balanced`; `I > 1.2` → `shortage` (talab yuqori).

**Misol.** Bu hafta: 40 ko'rish, 5 qidiruv, 3 bog'lanish, 2 RFQ → `T_0 = 40 + 10 + 9 + 10 = 69`. Oldingi haftalar: 50, 45, 55, 0 → `T̄ = 50` (0 hisobga olinmaydi). Sotuvchilar: hozir 10, oldin 12, 12, 11, 13 → `S̄ = 12`.
`I = (69/50) / (10/12) = 1.38 / 0.833 ≈ 1.66` → `shortage`.

> Eslatma: talab signallari savdo turiga (`ulgurji`/`chakana`) bo'linmaydi — `DemandWindows` `trade_type` bo'yicha filtrlamaydi; taklif esa tanlangan savdo turi bo'yicha sanaladi.

---

## 10. Xarid/sotish maslahati

**G'oya.** "Hozir olaymi yoki kutaymi?" (xaridor) va "hozir sotaymi yoki ushlab turaymi?" (sotuvchi) savoliga oddiy javob va uning sabablari. Har omil ballga +1 (narx oshadi yoki hozir past → hozir ol) yoki −1 (narx tushadi yoki hozir baland → kut) qo'shadi (`AdviceOf`, `insight/season.go`).

| Omil | Shart | Ball | `advice.items[].code` |
|---|---|---|---|
| Trend | `direction = up` | +1 | `trend_up` (`share` = haftalik sur'at) |
| | `direction = down` | −1 | `trend_down` |
| Mavsum | `S(keyingi oy)/S(bu oy) − 1 > 0.03` | +1 | `season_dearer` (`share`, `from_month`, `month`) |
| | `< −0.03` | −1 | `season_cheaper` |
| Bosim | `surplus` | +1 | `surplus` |
| | `shortage` | 0 (faqat ogohlantirish) | `shortage` |
| Daraja | `o'rtacha(7 kun)/o'rtacha(365 kun) − 1 < −0.05` | +1 | `below_year` (`share`) |
| | `> +0.10` | −1 | `above_year` |
| Bayram | 21 kun ichida boshlanadi | +1 | `holiday_soon` (`title`, `days`) |
| | hozir davom etmoqda | 0 | `holiday_now` (`title`, `until`) |

Bayramlar mahsulot kategoriyasiga bog'langan `market_events` (`kind = 'holiday'`) dan olinadi (`HolidaysAround`, `holidayHorizon = 21`); bir xil nomli bayram bir marta hisoblanadi.

**Hukm:** `ball ≥ 1` → `buyer = buy_now`, `seller = hold`; `ball ≤ −1` → `wait`, `sell_now`; aks holda ikkalasi `neutral`.

`reasons` — o'zbekcha tayyor gaplar; `items` — o'sha sabablar kod va raqamlar bilan, **xuddi shu tartibda**, boshqa tillarda o'zi matn tuzadigan mijozlar uchun.

**Misol.** Trend `up` (+1), mavsum: oktabr→noyabr −5% (−1), bosim `surplus` (+1), daraja yillikdan 7% past (+1) → ball = 2 → `buy_now` / `hold`.

---

## 11. Hududlar, arbitraj, chakana ustama, savdolashish

Kod: `insight/regions.go`, so'rovlar `queries/insight.sql`.

### Viloyat, tuman va bozor medianlari

- **Viloyatlar** (`ProvincePrices`): oxirgi **14 kun**dagi `price_daily` qatorlari bo'yicha narxlar soni bilan vaznlangan o'rtacha median. Faqat `offers ≥ 5`. Arzondan qimmatga tartiblanadi.
- **Tumanlar** va **bozorlar** (`DistrictPrices`, `MarketPrices`) — faqat viloyat tanlanganda: oxirgi 14 kundagi kuzatuvlardan to'g'ridan-to'g'ri `source_trust` bilan vaznli median, kamida **3** narx.
- **`diff`**: tanlangan viloyat bo'lsa, uning medianiga nisbatan; respublika ko'rinishida — viloyatlarning vaznli o'rtachasiga nisbatan. `−0.11` = 11% arzon.

> Nozik nuqta: `price_daily.offers` narx har amal qilgan kuni qayta sanaladi, shuning uchun 14 kunlik `offers` yig'indisi aslida "narx-kun"lar soni. "5 narx" chegarasi shu ma'noda tushunilishi kerak.

### Arbitraj

**G'oya.** Bir viloyatda sotib olib, boshqasida sotish — yo'l xarajatidan keyin foyda qoladimi.

- **Masofa** (`geo.RoadKm`): viloyat markazlari orasidagi katta doira (haversine) masofasi × **1.3** (odatiy yo'l aylanmasi koeffitsienti). Markaz — viloyat chegarasi (`viloyatlar.geojson`) eng katta poligoni tashqi halqasi uchlari o'rtachasi (`geo.Centroids`).
- **Transport tarifi**: `transport` so'rov parametri (so'm / birlik / 100 km), standart **60** (`DefaultTransport`), ruxsat 0–5000.
- Formula har juftlik (xarid `b`, sotish `s`, `P_s > P_b`) uchun:
  ```
  transport = tarif × km / 100
  net       = P_s − P_b − transport
  net_pct   = net / P_b
  ```
- Faqat `net > 0`; `net_pct` bo'yicha kamayish tartibida **top 5**. Viloyat tanlangan bo'lsa — faqat shu viloyat qatnashgan juftliklar.

**Misol.** Surxondaryoda 3 000, Toshkent shahrida 4 200 so'm/kg, taxminiy yo'l 460 km. `transport = 60 × 460/100 = 276`, `net = 1 200 − 276 = 924`, `net_pct = 924/3000 = 0.308` (+30.8%).

### Chakana ustama

`retail_markup = chakana_median / ulgurji_median − 1`, oxirgi **30 kun** `price_daily`dan (`TradeMedians`), har bir savdo turida `offers ≥ 5` bo'lsa. Masalan ulgurji 4 000, chakana 5 360 → `0.34`.

### Savdolashish chegirmasi

`DealDiscounts`: oxirgi **90 kun**dagi tasdiqlangan, narxi va e'loni bor savdolar. Har savdo uchun kelishilgan narx e'lonning savdodan oldingi oxirgi so'ralgan narxi (`seller`/`seller_auto`) bilan solishtiriladi:
`avg_discount = o'rtacha(kelishilgan / so'ralgan − 1)`. **Kamida 5** savdo bo'lsa ko'rsatiladi. Masalan 5 savdo: −3%, −5%, 0%, −4%, −5.5% → `−0.035`.

---

## 12. Rekordlar, keskin sakrashlar va voqealar

### Rekordlar (`PriceRecords`)

Tanlangan davrda har yil uchun eng qimmat va eng arzon **viloyat-kun** (faqat `offers ≥ 3`), qaysi viloyatda bo'lgani bilan. `records.max`/`min` — butun davr bo'yicha, `by_year` — yil bo'yicha. Teng qiymatda erta sana olinadi.

> Respublika ko'rinishida rekord respublika o'rtachasi emas, balki eng chekka bitta viloyat-kun ekanini unutmang.

### Keskin sakrashlar (`JumpsOf`, `insight/jumps.go`)

1. Kunlik seriya dushanbadan boshlanadigan haftalik o'rtachalarga aylantiriladi (3y davrida seriya allaqachon haftalik).
2. Ketma-ket ikki hafta uchun `o'zgarish = hafta / oldingi_hafta − 1`.
3. `|o'zgarish| > 0.15` (qat'iy katta) bo'lsa sakrash deb belgilanadi.
4. Har sakrashga **±7 kun** atrofidagi voqealar biriktiriladi: oralig'i `[dushanba − 7, dushanba + 6 + 7]` bilan kesishgan voqealar, shuningdek eng og'ir 3 ta ob-havo ekstremumi (13-bo'lim).

Bu "ehtimoliy sabablar", isbotlangan sabab emas — API hujjatida ham shunday yozilgan.

### Bozor voqealari jurnali (`market_events`)

Admin kiritadigan voqealar (`migrations/00026_market_events.sql`). Turlari: `frost`, `heat`, `rain`, `export`, `import`, `fuel`, `currency`, `holiday`, `harvest`, `roads`, `policy`, `other`. Voqea viloyat, mahsulot yoki kategoriyaga bog'lanishi mumkin; bo'sh bo'lsa — hamma joy / hamma narsa. `EventsFor` mahsulotga bevosita, kategoriyasi orqali yoki umumiy tegadigan va davr bilan kesishgan voqealarni oladi.

### Bayram taqvimi 2023–2027 (`migrations/00029_holiday_calendar.sql`)

Haqiqiy (demo emas) `holiday` yozuvlari, izohi `Taqvim` bilan boshlanadi:

| Bayram | Sanalar | Kategoriyalar |
|---|---|---|
| Ramazon oyi | oy taqvimi bo'yicha (masalan 2026: 18-fev – 19-mar) | `gosht`, `meva`, `oziq_ovqat` |
| Ramazon hayiti | 4 kun (2026: 17–20-mar) | `gosht`, `oziq_ovqat`, `meva` |
| Qurbon hayiti | 8 kun (2026: 22–29-may) | `gosht` |
| Navro'z | har yili 10–21-mart | `don`, `sabzavot`, `gosht` |
| Yangi yil | 25-dekabr – 1-yanvar | `meva`, `gosht` |
| To'y mavsumi | 15-sentabr – 30-noyabr | `gosht`, `oziq_ovqat`, `sabzavot`, `don` |

Oy taqvimidagi sanalar rasmiy sanadan 1 kunga farq qilishi mumkin (izohda yozilgan).

---

## 13. Ob-havo

**Manba.** Open-Meteo (kalitsiz, bepul) — `internal/external/weather.go`. Har viloyat **markazi** (`geo.Centroids`) uchun kunlik `temperature_2m_min`, `temperature_2m_max`, `precipitation_sum`, `timezone = Asia/Tashkent`.

- Arxiv API ~5 kun orqada qoladi, shuning uchun `bugun − 6` kungacha arxivdan, oxirgi 7 kun esa prognoz API'ning `past_days` qismidan olinadi.
- Birinchi ishga tushishda 3 yil (`History = 1095` kun) orqaga to'ldiriladi, keyin oxirgi saqlangan kundan davom etadi.
- Ish `weather_daily` har kuni **06:30** Toshkent (20 soatlik oyna). Bitta viloyat xatosi keyingi safar qayta uriniladi; hamma viloyat xato bo'lsa ish xato bilan tugaydi.

**Seriya** (`WeatherSeries`): viloyat tanlanganda — uning markazi; respublika uchun — barcha viloyatlarning har kungi **o'rtachasi** (`scope = "O'zbekiston (o'rtacha)"`). Bucket ichida: eng past minimum, eng yuqori maksimum, yog'ingarchilik yig'indisi.

**Ekstremumlar** (`WeatherExtremes`), har viloyat bo'yicha:

- **Sovuq (`frost`)**: `tmin ≤ −5°C`, yoki mart–may oylarida `tmin ≤ 0°C` (gullash va ko'chatlar uchun xavfli).
- **Jazirama (`heat`)**: `tmax ≥ 40°C`.
- Og'irlik: jazirama uchun `t − 40`, sovuq uchun `−t`. Davr bo'yicha eng og'ir **20 tasi** ko'rsatiladi (sana bo'yicha tartiblangan); har sakrashga ±7 kun ichidagi eng og'ir **3 tasi** biriktiriladi.

---

## 14. Valyuta kurslari

**Manba.** O'zbekiston Markaziy banki arxivi (`https://cbu.uz/uz/arkhiv-kursov-valyut/json/all/<sana>/`), `internal/external/fx.go`. Saqlanadigan valyutalar: **USD, RUB, KZT**; kurs `Rate / Nominal` (1 birlik uchun so'm). Ish `fx_daily` har kuni **10:15**; oxirgi 3 yilning dollar kursi yo'q kunlarini to'ldiradi. Ketma-ket 10 xato bo'lib bironta kun saqlanmasa, "cbu unreachable" xatosi.

**Tahlilda** (`insight/fx.go`): har valyuta uchun oxirgi kurs (`now`) va davr boshiga nisbatan o'zgarish (`change`); USD uchun narx seriyasi bilan bir xil bucketdagi nuqtalar.

**Korrelyatsiya** (`correlation_usd`): narx va dollar kursi sanalari mos kelgan juftliklar bo'yicha Pearson `r`:

```
r = Σ(x−x̄)(y−ȳ) / √(Σ(x−x̄)² · Σ(y−ȳ)²)
```

`x` — mahsulot medianasi, `y` — dollar kursi. **Kamida 20 nuqta** va ikkala seriyada o'zgaruvchanlik bo'lishi shart. 2 xonagacha yaxlitlanadi. **Birga harakatlanish sabab emas** — bu faqat bog'liqlik ko'rsatkichi (inflyatsiya ikkalasini ham ko'tarishi mumkin).

---

## 15. Sotuvchilar aytgan sabablar

Sotuvchi e'lon narxini **10% yoki undan ko'p** o'zgartirsa (`reasonThreshold = 0.10`, `internal/bot/reasons.go`), bot "Nega?" deb so'raydi va tayyor javoblarni beradi. Bir e'lon uchun kuniga bitta javob saqlanadi (`price_reasons`, `UNIQUE (listing_id, day)`); sotuvchi ismi ko'rsatilmaydi.

| Yo'nalish | Kodlar |
|---|---|
| Oshganda (`up`) | `supply_low` (Mahsulot kam), `demand_up` (Talab oshdi), `costs_up` (Yo'l va xarajat qimmatladi), `export` (Eksportga ketyapti), `market`, `other` |
| Tushganda (`down`) | `supply_high` (Mahsulot ko'p), `demand_down` (Talab kam), `sell_fast` (Tez sotish kerak), `import` (Import kirdi), `market` (Bozorda hamma shunday qildi), `other` (Boshqa sabab) |

Tahlil sahifasida (`reasons`) oxirgi **30 kun** bo'yicha mahsulot (va tanlangan viloyat) uchun kod + yo'nalish bo'yicha sanoq, ko'pidan kamiga.

---

## 16. Taklif (supply) bloki va ma'lumot sifati

- **`now`** — hozir jonli narxlar bo'yicha vaznli median, p25, p75, min, max (`LiveStats`, jonli median qoidalari bilan: yangilik bo'yicha pasayish bilan `price_weight`). **3 tadan kam** jonli narx bo'lsa yo'q.
- **`supply`**: jonli e'lonlar, alohida sotuvchilar, `sellers_change` (bir hafta oldin jonli narxi bo'lgan sotuvchilarga nisbatan), `stock` (mahsulotning asosiy birligida berilgan zaxiralar yig'indisi; hech kim bermagan bo'lsa yo'q), `stocked`.
- **`quality`**: `offers_now`, `by_source` (oxirgi 30 kun narxlari manba bo'yicha), `confirmed_deals_30d`, `demo_share`, `last_price_at`.

---

## 17. Viloyat sahifasi

`GET /v1/regions/{id}/overview` → `insight.RegionOverview` (`insight/regional.go`). Tuman id berilsa, uning viloyati olinadi.

- **Pasport** (`RegionPassport`): jonli e'lonli sotuvchilar, e'lonlar, bozorlar soni; oxirgi 7 kun va undan oldingi 7 kun bo'yicha RFQ, tasdiqlangan savdolar va talab signallari (`diff` bilan).
- **Mahsulotlar**: viloyatda 14 kunda ≥ 5 narxi bor mahsulotlar: median, `vs_country` (respublika vaznli o'rtachasiga nisbatan), `week` (oxirgi 7 kun oldingi 7 kunga; ikkala yarmida ham ≥ 5 bo'lsa), bosim zonasi, sotuvchilar, `demo_share`.
- **Tanqislik / ortiqchalik**: zonasi `shortage` / `surplus` bo'lgan mahsulotlar, narxlar soni bo'yicha top 10.
- **Imkoniyatlar**: shu viloyatdan boshqa viloyatga olib borib sotish. Har mahsulot uchun eng yuqori sof foyda beradigan viloyat, standart tarif 60 so'm/100 km bilan: `net_pct = (P_u − P_bu − 60 × km/100) / P_bu`. **≥ 10%** (`opportunityGap = 0.10`) bo'lsa ro'yxatga kiradi; `xizmat` kategoriyasi chiqariladi (xizmatni tashib bo'lmaydi); top 10.
- **Oziq-ovqat savati narx indeksi** (`basketIndex`):
  1. Savat kategoriyalari: `sabzavot`, `meva`, `gosht`, `don`, `oziq_ovqat`.
  2. Oxirgi ~1 yil (371 kun) haftalik medianlar (`WeeklyProductPrices`).
  3. Mahsulot savatga kiradi, agar **birinchi haftada** narxi bo'lsa va haftalarning **≥ 90%** ida narxi bo'lsa (`basketCoverage = 0.9`).
  4. Har hafta: `indeks_w = 100 × o'rtacha_p(P_p,w / P_p,birinchi)` — **birinchi hafta = 100**.
  5. Xuddi shu usulda respublika chizig'i (`country`). `index_basket` — savatdagi mahsulotlar soni.

  **Misol.** Savatda 3 mahsulot, hafta nisbatlari 1.10, 0.95, 1.05 → indeks `100 × 3.10/3 = 103.3`.
- **Faollik**: tumanlar va bozorlar bo'yicha jonli e'lonlar va sotuvchilar.

---

## 18. Issiqlik xaritasi

`GET /v1/analytics/heatmap?trade=&category=` (`insight/heatmap.go`).

- Qatorlar — mahsulotlar, ustunlar — viloyatlar. Katak = 14 kunlik vaznli median; **5 tadan kam** narx bo'lsa katak `null`.
- `country` — ko'rsatilgan viloyatlarning narxlar soni bilan vaznlangan o'rtachasi; katakdagi `diff` = `katak / country − 1`.
- `week` — respublika bo'yicha oxirgi 7 kun oldingi 7 kunga (ikkala yarmida ≥ 5).
- `up` / `down` — shu `week` bo'yicha eng ko'p oshgan / tushgan 10 tadan mahsulot.
- `spot` — Local Moran I bo'yicha "issiq" (qo'shnilari bilan birga qimmat) yoki "sovuq" (birga arzon) katak; faqat p < 0,01 va farq ≥ 5% bo'lsa belgilanadi ([ML usullari](12-ml-usullari.md)).

---

## 19. TOP talab

`GET /v1/analytics/top-demand?region=` (`insight/demand.go`).

- Har mahsulot uchun oxirgi 35 kunning talab signallari va tasdiqlangan savdolari 7 kunlik oynalarga bo'linadi (`win = floor(yosh / 7 kun)`, 0..4), 9-bo'limdagi vaznlar bilan.
- `score` = oxirgi 7 kun balli; `usual = (w1 + w2 + w3 + w4) / 4` (bu yerda nol haftalar ham hisobga olinadi — bosim indeksidan farqli); `growth = score / usual − 1` (`usual = 0` bo'lsa yo'q).
- `score < 5` bo'lgan mahsulotlar tashlanadi.
- `top` — ball bo'yicha top 10; `rising` — `growth > 0` bo'lganlardan o'sish bo'yicha top 10. Narx ma'lum bo'lsa `median` (14 kunlik ulgurji).

**Misol.** Bu hafta 30, oldingi to'rt hafta 10, 12, 8, 10 → `usual = 10`, `growth = 2.0` (+200%).

---

## 20. Solishtirish

`GET /v1/analytics/compare?products=&regions=&trade=&period=` (`insight/compare.go`).

- Chiziqlar soni = mahsulotlar × hududlar, **1 dan 4 gacha** (`MaxLines = 4`); oshsa `too_many_lines` xatosi.
- Har chiziq: medianlar va **indeks** — chiziqning birinchi nuqtasi = 100 (`indeks = 100 × P_t / P_birinchi`), turli narxdagi mahsulotlarni bir grafikda solishtirish uchun.
- `change` — oxirgi nuqta birinchisiga nisbatan; `now` — oxirgi nuqta.
- Bucket: `30d`, `90d` — kun; `1y` va `3y` — **hafta** (bir yillik kunlik chiziqlar juda zich).

---

## 21. Haftalik PDF hisobot

`GET /v1/reports/weekly?region=` (`internal/httpapi/report.go`, `internal/report/report.go`, `queries/report.sql`). A4 PDF, `Cache-Control: private, no-store`.

**Kirish huquqi** (`canReport`):

| Rol | Qaysi viloyat |
|---|---|
| Admin | istalgan |
| Tasdiqlangan `org_analyst` | istalgan |
| Tasdiqlangan `market_admin` | faqat o'z viloyati |
| Boshqalar | `403 reports_only` |

`region` berilmasa foydalanuvchining o'z viloyati olinadi; u ham bo'lmasa `422 region_required`.

**Tarkibi:**

1. Sarlavha: viloyat, oxirgi 7 kun oralig'i (Toshkent vaqti).
2. 5 ta ko'rsatkich: faol sotuvchilar, faol narxlar, so'rovlar (7 kun), talab signallari (7 kun), tasdiqlangan savdolar (7 kun — **5 tadan kam** bo'lsa "—").
3. Ulgurji narxlar jadvali: viloyatda **≥ 5 jonli narx**i bor mahsulotlar (ko'pi 30 ta): vaznli median (jonli qoidalar bilan), o'tgan haftaga o'zgarish (o'tgan haftada ham ≥ 5 narx bo'lsa, aks holda "—"), eng past – eng yuqori, narxlar soni, bosim zonasi.
4. Tasdiqlangan savdolar jadvali: haftada **≥ 5** savdosi bor mahsulotlar — soni, hajmi, median kelishilgan narx (narxi ko'rsatilgan savdolar ≥ 5 bo'lsa).
5. Izohlar: metodika qisqacha va tuzilgan vaqt.

Hech bir sotuvchi nomi ko'rsatilmaydi — faqat agregatlar.

---

## 22. Kesh va yangilik

| Nima | Qanchalik yangi |
|---|---|
| `price_daily` | har soatda kecha + bugun qayta quriladi |
| Tahlil sahifasi (`Analytics`) | parametrlar kombinatsiyasi bo'yicha xotirada **10 daqiqa** (`cacheFor`) |
| Viloyat sahifasi, issiqlik xaritasi, TOP talab | shu kesh, kalit bo'yicha (`region|…`, `heatmap|…`, `demand|…`), 10 daqiqa |
| Solishtirish | keshlanmaydi |
| CSV eksport (`/analytics.csv`) | `Cache-Control: private, max-age=600` |
| Valyuta | kuniga bir marta (10:15) |
| Ob-havo | kuniga bir marta (06:30) |

Kesh bitta server jarayoni xotirasida; restartdan keyin bo'sh boshlanadi (ma'lumotlar bazada qoladi, faqat qayta hisoblanadi).

---

## Qayerda joylashgan

- `backend/internal/insight/daily.go` — `Day`, `RebuildDaily`, `RefreshRecent`, kichik kesh
- `backend/internal/insight/analytics.go` — tahlil sahifasini yig'ish, davrlar va bucketlar, rekordlar, `now`, `outlook`, `supply`, `cacheFor`
- `backend/internal/insight/stats.go` — `ChangesOf`, `TrendOf`, `WeekdaysOf`, `Correlation`
- `backend/internal/insight/season.go` — `SeasonalIndex`, `AdviceOf`, `AdviceItem`
- `backend/internal/insight/forecast.go` — `ForecastOf`, `predict`, `roundPrice`
- `backend/internal/insight/regions.go` — viloyat/tuman/bozor reytingi, `arbitrage`, chakana ustama, savdolashish, `diff`
- `backend/internal/insight/jumps.go` — `weekly`, `JumpsOf`, voqealarni bog'lash
- `backend/internal/insight/weather.go`, `fx.go`, `reasons.go` — ob-havo, valyuta, sabablar bloklari
- `backend/internal/insight/regional.go` — viloyat sahifasi, savat indeksi, imkoniyatlar
- `backend/internal/insight/heatmap.go`, `demand.go`, `compare.go` — issiqlik xaritasi, TOP talab, solishtirish
- `backend/internal/analytics/pressure.go` — `DemandWeight`, `Pressure`, zonalar
- `backend/internal/market/pressure.go` — `PressureWindows`
- `backend/internal/external/weather.go`, `fx.go` — Open-Meteo va Markaziy bank sinxronizatsiyasi
- `backend/internal/geo/geo.go` — `Centroids`, `RoadKm`
- `backend/internal/report/report.go`, `backend/internal/httpapi/report.go` — PDF va kirish huquqi
- `backend/internal/httpapi/insight.go` — tahlil endpointlari va parametrlar
- `backend/internal/bot/reasons.go` — narx o'zgarishi sababini so'rash
- `backend/internal/jobs/jobs.go`, `backend/cmd/api/main.go` — rejalashtirilgan ishlar
- `backend/cmd/seed/history.go` — demo tarix va demo voqealar
- `backend/queries/insight.sql`, `pressure.sql`, `regions.sql`, `external.sql`, `events.sql`, `reasons.sql`, `report.sql`
- `backend/migrations/00024_price_daily.sql`, `00025_price_daily_synthetic.sql`, `00026_market_events.sql`, `00027_fx_daily.sql`, `00028_weather_daily.sql`, `00029_holiday_calendar.sql`, `00030_price_reasons.sql`
