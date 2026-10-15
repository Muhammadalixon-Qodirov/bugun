# Avtomatik narx

Avtomatik narx — sotuvchi e'lonining narxini o'zi belgilagan oraliq (`price_min`–`price_max`) ichida bozorga qarab kuniga uch marta yangilab turadigan tizim. U bozorga **ergashadi, lekin uni boshqarmaydi**: asos viloyatdagi boshqa sotuvchilarning vaznli medianasi, ustiga talab-taklif bosimi, haftalik trend, e'lonning o'z ko'rsatkichlari (ko'rishlar, bog'lanishlar, sotuvlar, RFQ natijalari, zaxira) kichik foizlar bilan qo'shiladi. Natija sotuvchi oralig'i, bozorning yuqori kvartili (P75) va kunlik ±5% bilan chegaralanadi. Har bir o'zgarish sabablari bilan jurnalga yoziladi, sotuvchiga botda xabar boradi va u bir tugma bilan eski narxni qaytara oladi.

## Mundarija

1. [Sozlamalar](#1-sozlamalar)
2. [Qachon ishlaydi](#2-qachon-ishlaydi)
3. [Qaysi e'lonlar qayta narxlanadi](#3-qaysi-elonlar-qayta-narxlanadi)
4. [Kirish ma'lumotlari](#4-kirish-malumotlari)
5. [Omillar va ularning chegaralari](#5-omillar-va-ularning-chegaralari)
6. [Algoritm qadamma-qadam](#6-algoritm-qadamma-qadam)
7. [To'liq ishlangan misol](#7-toliq-ishlangan-misol)
8. [Jurnal, bildirishnoma va qaytarish](#8-jurnal-bildirishnoma-va-qaytarish)
9. [Medianga ta'siri: `seller_auto`](#9-medianga-tasiri-seller_auto)
10. [Cheklovlar va kuzatilgan nozik joylar](#10-cheklovlar-va-kuzatilgan-nozik-joylar)
11. [Qayerda joylashgan](#qayerda-joylashgan)

---

## 1. Sozlamalar

Har bir e'lon uchun alohida (`listings` jadvali, migratsiya `00017_dynamic_pricing.sql`), API: `PUT /v1/listings/{id}/pricing`.

| Maydon | Qiymatlar | Ma'nosi |
|---|---|---|
| `mode` | `manual` (standart), `auto` | `auto` — tizim narxni oraliq ichida o'zgartira oladi |
| `goal` | `fast`, `balanced` (standart), `profit` | Medianga nisbatan qayerda turish: `fast` ×0.97, `balanced` ×1.00, `profit` ×1.03 |
| `price_min` | 50 … 50 000 000 | Narx hech qachon bundan past tushmaydi |
| `price_max` | 50 … 50 000 000 | Narx hech qachon bundan oshmaydi |
| `paused_until` | faqat javobda | Qo'lda narx o'zgartirilgan bo'lsa, avtomatik rejim shu vaqtgacha kutadi |

**Oraliq qoidalari** (`Pricing.validate`, `backend/internal/market/listings.go`):

1. `goal` bo'sh bo'lsa `balanced` olinadi.
2. `mode` va `goal` ruxsat etilgan qiymatlardan biri bo'lishi kerak (`invalid_choice`).
3. `price_min`/`price_max` berilgan bo'lsa, 50–50 000 000 oralig'ida (`invalid_price`).
4. `auto` rejimida qo'shimcha:
   - ikkala chegara majburiy (`required`);
   - `price_min ≤ price_max` (`below_min`);
   - `price_max ≤ 3 · price_min` (`range_too_wide`, `maxBoundsSpread = 3`) — xato kiritilgan nol tizimga "erkin qo'l" bermasligi uchun;
   - hozirgi narx oraliq ichida bo'lishi kerak (`price_outside_range`).
5. Bazada ham himoya bor: `CHECK (pricing_mode <> 'auto' OR (price_min IS NOT NULL AND price_max IS NOT NULL AND price_min <= price_max))`.

## 2. Qachon ishlaydi

Rejalashtirilgan ish `auto_pricing` (`backend/cmd/api/main.go`), Toshkent vaqti bilan har kuni:

- **07:30** — bozor ochilishidan oldin;
- **12:00** — tushda;
- **16:30** — tushdan keyin.

`Window = 3 soat`: server qayta ishga tushsa, slotdan keyin 3 soat ichida o'tkazib yuborilgan ish baribir bajariladi. Har bir slot `job_runs` jadvalida band qilinadi, shuning uchun ikki nusxa ishlasa ham bir slot bir marta bajariladi (`backend/internal/jobs/jobs.go`). Ish tugagach o'zgargan narxlar ro'yxati botga uzatiladi (8-bo'lim).

## 3. Qaysi e'lonlar qayta narxlanadi

`AutoPricedListings` (`backend/queries/autoprice.sql`):

```
pricing_mode = 'auto'
AND status = 'active'
AND valid_until > now()
AND (manual_price_at IS NULL OR manual_price_at < now() - 24 soat)
```

- **24 soatlik pauza** (`market.ManualPause`): sotuvchi narxni qo'lda o'zgartirsa (`UpsertListing`/`UpdateListing` narx haqiqatan o'zgarganda `manual_price_at = now()` qo'yadi) yoki avtomatik o'zgarishni qaytarsa, e'lon 24 soat davomida tegilmaydi.
- Har bir e'lon **alohida tranzaksiyada** va `FOR UPDATE` qator qulfi ostida ko'rib chiqiladi (`LockAutoPricedListing`). Qulfdan keyin shartlar qayta tekshiriladi: agar shu orada sotuvchi narxni o'zgartirgan bo'lsa, e'lon o'tkazib yuboriladi.
- Bitta e'londagi xato log'ga yoziladi va qolganlarini to'xtatmaydi (`Service.Reprice`, `backend/internal/market/autoprice.go`).

## 4. Kirish ma'lumotlari

`repriceInput` (`market/autoprice.go`) quyidagilarni yig'adi. Hammasi e'lonning **viloyati**, shu mahsulot va shu savdo turi bo'yicha, e'lonning o'zi chiqarib tashlanadi.

| Kirish | Manba | Qoida |
|---|---|---|
| `Current` | e'lon narxi | — |
| `DayStart` | `DayStartPrice` | Bugungi (Toshkent kuni) birinchi avtomatik o'zgarishning `old_price` i; bugun o'zgarish bo'lmagan bo'lsa `Current` |
| `Min`, `Max` | `price_min`, `price_max` | — |
| `Goal` | `pricing_goal` | — |
| `Peers` | `PeerPrices` | Boshqa tirik narxlar soni ([narx metodologiyasi](03-narx-metodologiyasi.md#4-qaysi-narx-tirik-hisoblanadi)) |
| `Median` | `WeightedQuantile(prices, weights, 0.5)` | Vaznli median |
| `P75` | `WeightedQuantile(prices, weights, 0.75)` | Vaznli yuqori kvartil |
| `MedianPrev` | `PeerPricesWeekAgo` | 7 kun oldin tirik bo'lgan narxlarning vaznli medianasi; ular ≥ 5 ta bo'lsa, aks holda 0 (noma'lum) |
| `Pressure`, `PressureOK` | `PressureWindows` + `analytics.Pressure` | Talab-taklif bosim indeksi, viloyat bo'yicha ([tahlil metodologiyasi](04-tahlil-metodologiyasi.md)); oldingi haftalardan kamida 2 tasida ma'lumot bo'lishi kerak |
| `Views`, `Contacts` | `ResponseCounts` | E'lonning oxirgi 7 kundagi `view` va `contact` hodisalari (`listing_events`) |
| `MarketRate` | `ResponseCounts` | Viloyatdagi boshqa e'lonlarning `contacts / views` nisbati; ularning ko'rishlari ≥ 100 bo'lsa (`minMarketViews`), aks holda 0 |
| `RFQOffers`, `RFQPicked` | `SellerRFQOutcomes` | Oxirgi 30 kunda sotuvchining shu mahsulot bo'yicha yakunlangan RFQ'larga javoblari (`declined` dan tashqari) va ulardan nechtasini xaridor tanlagani (`chosen`/`confirmed`) |
| `Sales` | `SalesCounts` | E'lonning oxirgi 7 kundagi bitimlari (`reported` yoki `confirmed`) |
| `MarketSales`, `SalesKnown` | `SalesCounts` | Viloyatdagi boshqa faol e'lonlarning o'rtacha bitim soni; boshqa e'lon bo'lsa `SalesKnown = true` |
| `StockRatio` | `PeerStock` | `e'lon zaxirasi / boshqalar zaxirasining medianasi` (bir xil birlik, ≥ 3 ta boshqa e'lon — `minStockPeers`); zaxira ko'rsatilmagan bo'lsa 0 |

Boshqa narxlar 5 tadan kam bo'lsa, qolgan ma'lumotlar yig'ilmaydi — qaror baribir "kutish" bo'ladi.

## 5. Omillar va ularning chegaralari

Barcha omillar `analytics.Reprice` da (`backend/internal/analytics/repricing.go`). Har bir omil narxning ishorali ulushi (0.02 = +2%).

| Kod | Shart | Ulush | Chegara |
|---|---|---|---|
| `goal` | `fast` yoki `profit` | `g − 1`: −0.03 yoki +0.03 | — (alohida ko'paytiriladi) |
| `pressure` | `PressureOK` | `0.10 · (I − 1)` | [−0.05; +0.05] |
| `trend` | `MedianPrev > 0` | `0.5 · (Median / MedianPrev − 1)` | [−0.03; +0.03] |
| `response` | `Views ≥ 20` va `MarketRate > 0` va `Contacts / Views < 0.5 · MarketRate` | −0.02 | — |
| `rfq` | `RFQOffers ≥ 3` va `RFQPicked = 0` | −0.02 | — |
| `sales` (yuqori) | `Sales ≥ 2` va `SalesKnown` va `Sales ≥ 2 · MarketSales` | +0.02 | — |
| `sales` (past) | aks holda, `Sales = 0` va `Contacts ≥ 5` | −0.02 | — |
| `stock` (ko'p) | `StockRatio ≥ 2` | −0.02 | — |
| `stock` (kam) | `0 < StockRatio ≤ 0.5` | +0.01 | — |
| `cap` | Chegara narxni kesganda | `narx / maqsad − 1` | faqat izoh uchun |

- `I` — bosim indeksi: `(T_hozir / T̄) / (S_hozir / S̄)`, `T` — vaznli talab, `S` — tirik narxli sotuvchilar soni.
- Ma'nosi: talab odatdagidan yuqori → narx biroz ko'tariladi; bozor haftada ko'tarilgan → shu o'sishning yarmicha ergashiladi; ko'p ko'rilib kam bog'lanilsa → narx yuqori bo'lishi mumkin; RFQ'larda doim boshqalar tanlansa → arzonlashtirish; boshqalardan ikki barobar tez sotilsa → biroz ko'tarish; bog'lanishlar bor-u sotuv yo'q → arzonlashtirish; zaxira odatdagidan 2 barobar ko'p → arzonlashtirish, ancha kam → biroz ko'tarish.
- Qo'shiladigan omillar yig'indisining oralig'i: `−0.16 … +0.11`.

## 6. Algoritm qadamma-qadam

`analytics.Reprice(in)`:

1. **Ma'lumot yetarlimi.** `Peers < 5` (`MinRepricePeers`) yoki `Median ≤ 0` yoki `Current ≤ 0` → narx o'zgarmaydi (`Hold = "few_prices"`).
2. **Maqsad koeffitsiyenti** `g`: `fast` 0.97, `balanced` 1.00, `profit` 1.03 (noma'lum qiymat → 1).
3. **Omillar yig'indisi** `Σ` — 5-bo'limdagi `pressure`, `trend`, `response`, `rfq`, `sales`, `stock` ulushlari.
4. **Maqsad narx:**
   ```
   maqsad = Median · g · (1 + Σ)
   ```
5. **Yuqori va pastki chegara:**
   ```
   lo = price_min
   hi = price_max
   agar 0 < P75 < hi bo'lsa:  hi = max(P75, lo)       -- "P75 dan yuqori emas", lekin sotuvchi minimumidan past ham emas
   start = DayStart (0 bo'lsa Current)
   lo = max(lo, start · 0.95)                          -- kunlik −5%
   hi = min(hi, start · 1.05)                          -- kunlik +5%
   ```
   `lo > hi` bo'lsa (kunlik oraliq sotuvchi oralig'idan tashqarida) → o'zgarmaydi (`Hold = "capped"`).
6. **Kesish:** `narx = clamp(maqsad, lo, hi)`. Kesilgan bo'lsa `cap` omili qo'shiladi: `narx / maqsad − 1`.
7. **Bozorcha yaxlitlash** (`roundingStep`):

   | Narx | Qadam |
   |---|---|
   | ≥ 1 000 000 | 10 000 |
   | ≥ 100 000 | 1 000 |
   | ≥ 20 000 | 500 |
   | ≥ 5 000 | 100 |
   | < 5 000 | 50 |

   `yaxlit = round(narx / qadam) · qadam`. `hi` dan oshsa → `floor(hi / qadam) · qadam`; `lo` dan past bo'lsa → `ceil(lo / qadam) · qadam`; oraliq bitta qadamdan tor bo'lsa → yaxlitlanmagan `narx`.
8. **Minimal qadam:** `|yaxlit − Current| / Current < 0.02` (`minChange`) → o'zgarmaydi (`Hold = "small"`), narx "titramasligi" uchun.
9. Aks holda yangi narx `= yaxlit` (butun so'm).

**Qo'llash** (`repriceListing`, `market/autoprice.go`), bitta tranzaksiyada:

1. `SetAutoPrice` — e'lon narxi yangilanadi; `manual_price_at` o'zgarmaydi, `valid_until` ham uzaymaydi.
2. `InsertPriceChange` — `price_changes` ga `old_price`, `new_price`, `reasons` (JSON: `median`, `peers`, `factors`, `contrib`). `contrib` — har bir omilning o'zgarishdagi aniq Shapley ulushi: ular yig'indisi qo'llangan o'zgarishga teng, chegara (cap) va yaxlitlash ham shu jumladan ([ML usullari](12-ml-usullari.md)).
3. `recordPrice(..., SourceSellerAuto)` — eski kuzatuv `expired`, yangi narx `seller_auto` manbasi bilan medianga yoziladi.

Avtomatik narx `Screen` anomaliya tekshiruvidan o'tmaydi: u sotuvchi oralig'i, kunlik 5% va P75 bilan chegaralangan.

**Kunlik ±5% nima uchun `DayStart` dan.** Kun ichidagi uchta ishga tushirishning yig'indisi kun boshidagi narxdan ±5% dan oshmasligi kerak. Shuning uchun chegara joriy narxdan emas, bugungi birinchi avtomatik o'zgarishdan oldingi narxdan hisoblanadi. `DayStartPrice` Toshkent kunining boshini `date_trunc('day', now() AT TIME ZONE 'Asia/Tashkent')` bilan oladi. Minimal 2% qadam esa joriy narxdan hisoblanadi.

## 7. To'liq ishlangan misol

**Vaziyat.** Andijon viloyati, pomidor, ulgurji. Sotuvchi e'loni: `Current = 4 500` so'm/kg, `price_min = 4 000`, `price_max = 6 000`, `goal = profit`. Bugun hali avtomatik o'zgarish bo'lmagan (`DayStart = 4 500`). Soat 07:30 ishi.

**Kirishlar:**

| Kirish | Qiymat |
|---|---|
| `Peers` | 8 |
| `Median` | 4 600 |
| `P75` | 4 900 |
| `MedianPrev` | 4 400 |
| `Pressure` | 1.30 (`PressureOK = true`) |
| `Views`, `Contacts` | 40, 6 |
| `MarketRate` | 0.20 |
| `RFQOffers`, `RFQPicked` | 2, 0 |
| `Sales` | 0 |
| `StockRatio` | 1.0 |

**1. Ma'lumot:** 8 ≥ 5 — davom etamiz.

**2. Maqsad koeffitsiyenti:** `profit` → `g = 1.03`; omil `goal = +0.03`.

**3. Omillar:**

| Omil | Hisob | Ulush |
|---|---|---|
| `pressure` | `0.10 · (1.30 − 1) = 0.030`, chegarada | +0.0300 |
| `trend` | `0.5 · (4 600 / 4 400 − 1) = 0.5 · 0.04545 = 0.0227`, ≤ 0.03 | +0.0227 |
| `response` | `6 / 40 = 0.15`; `0.5 · 0.20 = 0.10`; `0.15 < 0.10` emas | 0 |
| `rfq` | 2 ta taklif, 3 dan kam | 0 |
| `sales` | `Sales = 0` va `Contacts = 6 ≥ 5` | −0.0200 |
| `stock` | `StockRatio = 1.0` — oraliqlarga tushmaydi | 0 |
| **Σ** | | **+0.0327** |

**4. Maqsad narx:** `4 600 · 1.03 · 1.0327 = 4 738 · 1.0327 ≈ 4 893.1`.

**5. Chegaralar:**

- `lo = 4 000`, `hi = 6 000`;
- `P75 = 4 900 < 6 000` → `hi = max(4 900, 4 000) = 4 900`;
- kunlik: `lo = max(4 000, 4 500 · 0.95 = 4 275) = 4 275`; `hi = min(4 900, 4 500 · 1.05 = 4 725) = 4 725`.

**6. Kesish:** `clamp(4 893.1; 4 275; 4 725) = 4 725`. `cap = 4 725 / 4 893.1 − 1 ≈ −0.034`.

**7. Yaxlitlash:** 4 725 < 5 000 → qadam 50. `round(4 725 / 50) = round(94.5) = 95` → 4 750 > `hi` → `floor(4 725 / 50) · 50 = 4 700`.

**8. Minimal qadam:** `|4 700 − 4 500| / 4 500 = 4.4% ≥ 2%` → o'zgaradi.

**Natija:** narx **4 500 → 4 700** so'm/kg. `price_changes.reasons`:

```json
{"median": 4600, "peers": 8,
 "factors": [{"code": "goal", "share": 0.03}, {"code": "pressure", "share": 0.03},
             {"code": "trend", "share": 0.0227}, {"code": "sales", "share": -0.02},
             {"code": "cap", "share": -0.0344}]}
```

**Shu kuni soat 12:00 da** (bozor o'zgarmagan deb faraz qilamiz): `DayStart = 4 500` (bugungi birinchi o'zgarishning eski narxi), demak `hi = 4 725` hali ham. Maqsad yana 4 893 → kesiladi 4 725 → yaxlitlanadi 4 700 → joriy narx bilan farq 0% < 2% → `Hold = "small"`, o'zgarish yo'q. Kunlik ±5% chegarasi shu tarzda kun ichidagi yig'indini ushlab turadi.

## 8. Jurnal, bildirishnoma va qaytarish

**Jurnal.** `price_changes` jadvali: `listing_id`, `old_price`, `new_price`, `status` (`applied` / `reverted`), `reasons` (JSONB), `created_at`, `reverted_at`. API: `GET /v1/me/listings/{id}/price-changes` — oxirgi 30 ta o'zgarish, har birida `median`, `peers`, `factors` (ulushlar 3 xonagacha yaxlitlangan) va `can_revert` (faqat eng oxirgi, `applied` holatidagi va e'lon narxi hali shu o'zgarish qo'ygan narx bo'lsa `true`).

**Bot xabari** (`PricesChanged`, `backend/internal/bot/autoprice.go`). Har bir ishdan keyin har bir sotuvchiga **bitta** xabar: har bir o'zgargan e'lon uchun eski → yangi narx, foiz, viloyat medianasi va narxlar soni, sabablar oddiy so'zlarda (masalan, "talab odatdagidan yuqori", "bog'lanishlar bor, lekin sotuv yo'q", "chegara: oraliqingiz yoki kunlik 5%"). Har bir o'zgarish uchun "Qaytarish" tugmasi (`a:undo:<id>`). Sotuvchilar orasida kichik pauza (`sendPause`) bor.

**Qaytarish** (`RevertAutoPrice`, `market/autoprice.go`; API `POST /v1/me/price-changes/{id}/revert` yoki bot tugmasi):

1. O'zgarish sotuvchiniki ekanligi tekshiriladi (aks holda `ErrNotFound`).
2. Allaqachon qaytarilgan bo'lsa → `ErrReverted`.
3. E'lon narxi shundan keyin yana o'zgargan bo'lsa (joriy narx `new_price` ga teng emas) → `ErrStale`: yangiroq narxni ustidan yozmaslik uchun.
4. Aks holda narx `old_price` ga qaytadi, `manual_price_at = now()` qo'yiladi — **avtomatik rejim 24 soat to'xtaydi**; o'zgarish `reverted` deb belgilanadi; qaytgan narx medianga `seller` manbasi bilan yoziladi (sotuvchining o'z narxi sifatida).

## 9. Medianga ta'siri: `seller_auto`

Avtomatik qo'yilgan narx `price_observations` ga `source = 'seller_auto'` bilan yoziladi va vazni `0.4 · e^(−0.2·t)` — sotuvchining o'z narxidan (`0.8`) ikki barobar kam. Sabab: tizim ergashadigan medianni o'zi surib yubormasligi kerak. Ko'p sotuvchi avtomatik rejimda bo'lsa ham, median asosan odamlar qo'ygan narxlar va haqiqiy RFQ/bitim narxlari bilan belgilanadi. Sotuvchi narxni qo'lda o'zgartirsa yoki avtomatik o'zgarishni qaytarsa, kuzatuv yana `seller` bo'ladi. Batafsil: [narx metodologiyasi](03-narx-metodologiyasi.md#2-vazn-ishonch-va-yangilik).

## 10. Cheklovlar va kuzatilgan nozik joylar

- **Kutish sabablari saqlanmaydi.** `few_prices`, `capped`, `small` faqat `Decision.Hold` da qaytadi; ular jurnalga yozilmaydi va sotuvchiga ko'rsatilmaydi.
- **P75 sotuvchi minimumidan past bo'lsa** `hi = price_min` bo'ladi, ya'ni sotuvchi oralig'i bozor P75 dan ustun turadi.
- **`goal` ulushi** `factors` ro'yxatida `g − 1` sifatida ko'rinadi, lekin hisobda u boshqa omillarga qo'shilmaydi, ko'paytiriladi. `cap` ulushi esa maqsad narxga nisbatan.
- **Avtomatik o'zgarish e'lon muddatini uzaytirmaydi.** E'lon 7 kundan keyin baribir tugaydi va sotuvchi uni yangilashi kerak.
- **`SellerRFQOutcomes`** viloyat bo'yicha emas, sotuvchining shu mahsulot bo'yicha barcha RFQ javoblari bo'yicha hisoblanadi.
- **`ResponseCounts`** dagi "bozor" e'lonlari holati bo'yicha filtrlanmaydi (yashirin yoki muddati o'tgan e'lonlarning hodisalari ham `MarketRate` ga kiradi), `SalesCounts` esa faqat `active` e'lonlarni oladi. So'rov izohida "unique viewers" deyilgan, lekin u hodisalar sonini sanaydi; ko'rish hodisasi bir foydalanuvchi uchun kuniga bir marta yoziladi (e'lon sahifasi API izohi bo'yicha).
- `spec.md` §10.5 dagi "tavsiya oralig'i 25–50-foizli qism" qoidasi avtomatik narxda ishlatilmaydi; kodda asos — vaznli median, yuqori chegara — P75 (`spec.md` boshidagi 2026-09-25 avtomatik narx qarori, 14.4 ni o'zgartiradi).

## Qayerda joylashgan

- `backend/internal/analytics/repricing.go` — `Reprice`, `goalFactor`, `MinRepricePeers`, `DailyStep`, `minChange`, `roundingStep`
- `backend/internal/analytics/weighted.go` — `WeightedQuantile` (median, P75)
- `backend/internal/analytics/pressure.go` — `Pressure`, `DemandWeight`, `MinHistory`
- `backend/internal/market/autoprice.go` — `Service.Reprice`, `repriceListing`, `repriceInput`, `RevertAutoPrice`, `ManualPause`, `minMarketViews`, `minStockPeers`
- `backend/internal/market/pressure.go` — `PressureWindows`
- `backend/internal/market/listings.go` — `Pricing.validate`, `SetPricing`, `maxBoundsSpread`, `SourceSellerAuto`, `recordPrice`
- `backend/queries/autoprice.sql` — `AutoPricedListings`, `LockAutoPricedListing`, `PeerPricesWeekAgo`, `DayStartPrice`, `SellerRFQOutcomes`, `PeerStock`, `ResponseCounts`, `SalesCounts`, `SetAutoPrice`, `InsertPriceChange`, `RevertAutoPrice`, `ListPriceChanges`
- `backend/queries/pressure.sql` — `DemandWindows`, `SupplyWindows`, `DealWindows`
- `backend/migrations/00017_dynamic_pricing.sql` — `listings` ustunlari, `price_changes`, `seller_auto` vazni
- `backend/cmd/api/main.go` — `auto_pricing` ishi (07:30, 12:00, 16:30, oyna 3 soat)
- `backend/internal/jobs/jobs.go` — Toshkent vaqtidagi rejalashtiruvchi
- `backend/internal/bot/autoprice.go` — `PricesChanged`, `factorText`, qaytarish tugmasi
- `backend/internal/httpapi/listings.go` — `PUT /v1/listings/{id}/pricing`, `GET /v1/me/listings/{id}/price-changes`, `POST /v1/me/price-changes/{id}/revert`
