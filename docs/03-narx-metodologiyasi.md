# Narx metodologiyasi: kuzatuvdan bozor narxigacha

Bugun'da "bozor narxi" bitta sotuvchining so'zi emas, balki bir nechta manbadan kelgan narx kuzatuvlarining **vaznli medianasi**. Har bir narx `price_observations` jadvaliga yoziladi, manbasi va yangiligiga qarab vazn oladi, shubhali narxlar tekshiruvga (`review`) olinadi va medianga kirmaydi. Median faqat yetarli narx bo'lsa ko'rsatiladi; tumanda narx kam bo'lsa hisob viloyatga, keyin respublikaga kengayadi. Ushbu hujjat har bir qoidani koddagi aniq raqamlar bilan tushuntiradi.

## Mundarija

1. [Narx kuzatuvi va manbalar](#1-narx-kuzatuvi-va-manbalar)
2. [Vazn: ishonch va yangilik](#2-vazn-ishonch-va-yangilik)
3. [Vaznli median va kvantillar](#3-vaznli-median-va-kvantillar)
4. [Qaysi narx "tirik" hisoblanadi](#4-qaysi-narx-tirik-hisoblanadi)
5. [E'lon muddati va yangilash](#5-elon-muddati-va-yangilash)
6. [Minimal kuzatuvlar soni](#6-minimal-kuzatuvlar-soni)
7. [Hudud kengayishi va `price_scope`](#7-hudud-kengayishi-va-price_scope)
8. [Anomaliyalarni aniqlash va tekshiruv](#8-anomaliyalarni-aniqlash-va-tekshiruv)
9. [Sotuvchining bozordagi o'rni](#9-sotuvchining-bozordagi-orni)
10. [`diff_percent` hisoblash](#10-diff_percent-hisoblash)
11. [Rasmiy etalonlar (UzEx, narxtahlil)](#11-rasmiy-etalonlar-uzex-narxtahlil)
12. [O'lchov birliklari](#12-olchov-birliklari)
13. [spec.md bilan farqlar va ochiq masalalar](#13-specmd-bilan-farqlar-va-ochiq-masalalar)
14. [Qayerda joylashgan](#qayerda-joylashgan)

---

## 1. Narx kuzatuvi va manbalar

Har bir narx — bu bitta **kuzatuv** (`price_observations` qatori): mahsulot, narx (so'mda, asosiy birlik uchun), savdo turi (`ulgurji` yoki `chakana`), hudud, manba, kuzatilgan vaqt (`observed_at`), amal qilish muddati (`valid_until`) va holat (`active`, `expired`, `review`, `rejected`).

Ulgurji va chakana narxlar hech qachon aralashtirilmaydi: barcha so'rovlar `trade_type` bo'yicha alohida hisoblanadi.

| Manba (`source`) | Qayerdan keladi | Amal qilish muddati | Kirishdagi tekshiruv |
|---|---|---|---|
| `seller` | Sotuvchi saytda, Mini App'da yoki botda narx kiritdi (e'lon yaratildi yoki narxi o'zgardi) | E'lon bilan birga, 7 kun | To'liq `Screen` tekshiruvi (8-bo'lim) |
| `seller_auto` | Avtomatik narx tizimi e'lon narxini o'zgartirdi ([avtomatik narx](06-avtomatik-narx.md)) | E'lonning `valid_until` qiymati | Yo'q (narx sotuvchi oralig'i va bozor P75 bilan cheklangan) |
| `rfq_offer` | Sotuvchi xaridor so'roviga (RFQ) javoban aniq narx taklif qildi | RFQ muddati (`deadline`) gacha | `IsOutlier` (faqat bozor bilan solishtirish) |
| `deal` | Tasdiqlangan, narxi ko'rsatilgan, RFQ'dan tashqari bitim | 7 kun | Narx e'lon narxining 0,5–2 barobari oralig'ida bo'lmasa `review` |
| `telegram_channel` | Ulangan Telegram kanal yoki guruh posti | 7 kun | Viloyat medianasidan 2 barobardan ko'p farq qilsa `review` |
| `olx` | Vazn belgilangan, lekin kodda import yo'q | — | — |
| `uzex`, `narxtahlil` | Rasmiy etalon; medianga kirmaydi (vazn 0) | — | — |

Muhim: RFQ orqali bo'lgan bitim (`deals.source = 'rfq'`) alohida `deal` kuzatuvi sifatida qo'shilmaydi, chunki uning narxi medianga allaqachon `rfq_offer` sifatida kirgan (`InsertDealObservation`, `queries/deals.sql`).

Har bir yangi narx avvalgi kuzatuvni "iste'fo"ga chiqaradi: e'lon narxi o'zgarganda `ExpireListingObservation` eski qatorni `expired` qiladi va yangisi yoziladi (`recordPrice`, `backend/internal/market/listings.go`). Shu tariqa bitta e'lon medianda bir vaqtda faqat bitta narx bilan qatnashadi. RFQ taklifi, bitim va Telegram posti uchun ham xuddi shunday (`ExpireOfferObservation`, `ExpireDealObservation`, `ExpireTelegramPost`).

## 2. Vazn: ishonch va yangilik

**G'oya.** Xaridorga aniq aytilgan narx yoki haqiqiy bitim narxi e'londagi "so'ralgan" narxdan ishonchliroq. Kechagi narx bir hafta oldingisidan dolzarbroq. Shuning uchun har bir kuzatuv vazn oladi.

**Qoida** (SQL funksiyasi `price_weight`, oxirgi versiyasi migratsiya `00021_deal_prices.sql`):

```
w = s(manba) · e^(−0.2 · t)
```

- `w` — kuzatuvning mediandagi vazni;
- `s(manba)` — manbaga ishonch koeffitsiyenti (jadval pastda);
- `t` — kuzatuv yoshi kunlarda: `(now() − observed_at) / 86400 sekund`, manfiy bo'lsa 0 olinadi;
- `0.2` — yangilik so'nish tezligi (λ, 1/kun).

| Manba | `s` |
|---|---|
| `rfq_offer` | 1.0 |
| `deal` | 1.0 |
| `seller` | 0.8 |
| `telegram_channel` | 0.6 |
| `olx` | 0.5 |
| `seller_auto` | 0.4 |
| `uzex`, `narxtahlil` va boshqalar | 0.0 |

Yangilik koeffitsiyenti `e^(−0.2·t)`:

| Yoshi (kun) | 0 | 1 | 2 | 3 | 5 | 7 |
|---|---|---|---|---|---|---|
| Koeffitsiyent | 1.000 | 0.819 | 0.670 | 0.549 | 0.368 | 0.247 |

Ya'ni 7 kunlik narx yangi narxning taxminan to'rtdan biricha og'irlikka ega.

**`seller_auto` nima uchun 0.4.** Avtomatik narx bozor medianasiga ergashadi. Agar u sotuvchining o'z narxi (0.8) bilan teng vaznda bo'lsa, tizim o'zi ergashayotgan medianni o'zi surib yuborishi mumkin edi. Shuning uchun avtomatik narx sotuvchi narxining yarmi og'irligida hisoblanadi (izoh migratsiya `00017_dynamic_pricing.sql` da).

**Kunlik tarix uchun boshqa vazn.** Tahlil sahifasidagi kunlik tarix (`price_daily`) `source_trust(source)` funksiyasidan foydalanadi: xuddi shu `s` koeffitsiyentlari, lekin **yangilik so'nishisiz**, aks holda o'tgan kunlar tarixi buzilgan bo'lardi (migratsiya `00024_price_daily.sql`). Batafsil: [tahlil metodologiyasi](04-tahlil-metodologiyasi.md).

## 3. Vaznli median va kvantillar

**G'oya.** Narxlarni o'sish tartibida qo'yamiz va vaznlarni ketma-ket qo'shib boramiz. Jami vaznning yarmiga birinchi yetgan narx — median. Chorakdagi (25%) va to'rtdan uch qismdagi (75%) narxlar — P25 va P75.

**Aniq qoida — "pastki vaznli kvantil, interpolyatsiyasiz":**

```
Q(q) = eng kichik narx p_k, bunda  Σ_{i ≤ k} w_i  ≥  q · W · (1 − 1e−9)
```

- `p_1 ≤ p_2 ≤ … ≤ p_n` — o'sish tartibidagi narxlar;
- `w_i` — ularning vaznlari (faqat `w > 0` bo'lganlari hisobga olinadi);
- `W = Σ w_i` — jami vazn;
- `q` — kvantil darajasi: median uchun 0.5, P25 uchun 0.25, P75 uchun 0.75;
- `(1 − 1e−9)` — suzuvchi nuqta xatosidan himoya: yig'indi `q·W` ga "deyarli teng" bo'lsa ham shart bajarilgan deb olinadi.

Natija har doim haqiqiy kuzatilgan narxlardan biri bo'ladi; ikki narx o'rtasidagi o'rtacha qiymat olinmaydi. Masalan, vazni teng ikki narx (4 000 va 5 000) uchun median 4 000 bo'ladi, 4 500 emas. Vaznli narx bo'lmasa, SQL `NULL` qaytaradi (so'rovlarda `COALESCE(..., 0)`), Go versiyasi 0 qaytaradi.

**SQL va Go bir xil.** Qoida ikki joyda amalga oshirilgan va ular bir xil natija beradi:

- SQL: `weighted_quantile(prices, weights, q)` — migratsiya `00012_weighted_median.sql`. Oynali `sum(w) OVER (ORDER BY p)` bilan kumulyativ vazn hisoblanadi va shartni bajargan eng kichik `p` olinadi.
- Go: `analytics.WeightedQuantile(values, weights, q)` — `backend/internal/analytics/weighted.go`. `weights = nil` bo'lsa hamma narx 1 vaznda.

Ommaviy mahsulot sahifasi, katalog, lenta, sotuvchi kabineti, hisobot va boshqalar SQL versiyasini ishlatadi; bozordagi o'rin va avtomatik narx Go versiyasini ishlatadi (vaznlar baribir SQL `price_weight` dan keladi, `PeerPrices` so'rovi).

**Eslatma.** `analytics.Median` va `analytics.Percentile` (oddiy, vaznsiz, juft sonli tanlovda o'rtacha va chiziqli interpolyatsiya bilan) ham mavjud, lekin ular bozor narxi uchun emas, balki anomaliya tekshiruvi (robust z, 8-bo'lim) va boshqa ichki hisoblar uchun ishlatiladi.

### Ishlangan misol: 5 ta narx

Toshkent viloyatida kartoshka, ulgurji, so'm/kg:

| # | Narx | Manba | Yoshi | `s` | `e^(−0.2t)` | `w` | Kumulyativ |
|---|---|---|---|---|---|---|---|
| A | 4 200 | `deal` | 0 kun | 1.0 | 1.000 | 1.000 | 1.000 |
| B | 4 400 | `seller` | 0 kun | 0.8 | 1.000 | 0.800 | 1.800 |
| C | 4 500 | `rfq_offer` | 2 kun | 1.0 | 0.670 | 0.670 | 2.470 |
| D | 4 800 | `telegram_channel` | 0,5 kun | 0.6 | 0.905 | 0.543 | 3.013 |
| E | 5 500 | `seller` | 6 kun | 0.8 | 0.301 | 0.241 | 3.254 |

`W = 3.254`.

- Median: `0.5 · 3.254 = 1.627`. Kumulyativ vazn birinchi marta B da (1.800) shu qiymatdan oshadi → **median = 4 400**. Oddiy (vaznsiz) median C = 4 500 bo'lardi; eski va kamroq ishonchli E narxi natijani kamroq tortadi.
- P25: `0.25 · 3.254 = 0.814` → A da (1.000) → **P25 = 4 200**.
- P75: `0.75 · 3.254 = 2.441` → C da (2.470) → **P75 = 4 500**.

## 4. Qaysi narx "tirik" hisoblanadi

Medianga faqat quyidagi shartlarning **hammasini** bajaradigan kuzatuv kiradi (`browse.sql`, `listings.sql` dagi `PeerPrices`, `ListSellerListings` va boshqalar):

```
o.status = 'active'
AND o.valid_until > now()
AND (o.listing_id IS NULL
     OR EXISTS (SELECT 1 FROM listings x WHERE x.id = o.listing_id AND x.status = 'active'))
```

Ya'ni:

1. Kuzatuv tekshiruvda (`review`) yoki rad etilgan (`rejected`) bo'lmasligi, eskirmagan (`expired`) bo'lishi kerak.
2. Muddati o'tmagan bo'lishi kerak.
3. Agar u e'longa tegishli bo'lsa, e'lonning o'zi `active` bo'lishi kerak.

**E'lon medianni qanday tark etadi:**

| Holat | Nima bo'ladi | Qayerda |
|---|---|---|
| Sotuvchi e'lonni yashirdi (`hidden`) | Kuzatuv `active` qoladi, lekin `EXISTS` sharti uni chiqarib tashlaydi. Qayta ko'rsatilsa darhol qaytadi | `SetVisible`, `SetListingStatus` |
| Zaxira 0 ga tushdi ("sotildi" tugmasi) | E'lon `hidden` bo'ladi, yuqoridagidek | `MarkSold`, `market/sold.go` |
| E'lon o'chirildi (yoki sotuvchi akkaunti bilan birga) | `BEFORE DELETE` trigger `listings_retire_prices` kuzatuvlarni darhol `expired` qiladi, `valid_until = LEAST(valid_until, now())` | migratsiya `00008_retire_deleted_listing_prices.sql` |
| RFQ taklifi o'chirildi (RFQ yoki akkaunt bilan) | `offers_retire_prices` trigger xuddi shunday | migratsiya `00014_retire_deleted_offer_prices.sql` |
| Narx tekshiruvga tushdi | Kuzatuv `status = 'review'` bilan yoziladi va medianga kirmaydi | `recordPrice` |
| Admin rad etdi | E'lon `rejected`, kuzatuv `rejected` | `Review`, `SetReviewedObservationStatus` |
| Muddati tugadi | `valid_until ≤ now()` → avtomatik chiqadi | — |
| Bitim bekor qilindi yoki o'zgardi | `SyncDealPrice` eski qatorni `expired` qiladi, faqat tasdiqlangan bitim qayta yoziladi | `market/sold.go`, `queries/deals.sql` |
| Telegram posti tahrirlandi | Postning eski narxlari `expired`, yangilari yoziladi | `ExpireTelegramPost` |

Triggerlar muhim, chunki o'chirilgan e'lonning narxi 7 kunlik muddati tugaguncha medianda "arvoh" bo'lib qolmasligi kerak.

## 5. E'lon muddati va yangilash

- **Amal qilish muddati:** `market.Validity = 7 · 24 soat`. Har bir yaratish yoki tahrirlashda `valid_until = now + 7 kun`.
- **Yangilash oynasi:** `market.RenewWindow = 48 soat`. Muddati tugagan yoki 48 soat ichida tugaydigan e'lonlar kabinetda "yangilash kerak" deb belgilanadi.
- **Yangilash usullari:**
  - `Refresh` — bitta e'lonni yana 7 kunga uzaytiradi ("narx o'zgarmadi").
  - `RefreshDue` — sotuvchining 48 soat ichida tugaydigan barcha faol e'lonlarini bitta tranzaksiyada uzaytiradi.
  - `RenewAll` — barcha faol e'lonlarini uzaytiradi (haftalik "narxlar o'zgarmadi" javobi).
- Yangilashda e'lonning `valid_until` bilan birga uning faol kuzatuvining `valid_until` ham yangilanadi (`SetListingObservationValidity`, `SetListingsObservationValidity`). **`observed_at` o'zgarmaydi**, ya'ni yangilangan, lekin narxi o'zgarmagan e'lon medianda qolaveradi, ammo uning vazni yoshiga qarab so'nishda davom etadi. Faqat narxning o'zi yangilanganda yangi kuzatuv (va `t = 0`) paydo bo'ladi.
- Faqat `active` e'lonlar yangilanadi; tekshiruvdagi yoki yashirin e'lon bu yo'l bilan uzaytirilmaydi (`RefreshDueListings`).

Bot orqali eslatmalar: [Telegram bot](07-telegram-bot.md).

## 6. Minimal kuzatuvlar soni

Kam narxdan chiqarilgan median tasodifiy bo'ladi va bitta sotuvchini "fosh qilib" qo'yishi mumkin. Shuning uchun chegaralar:

| Chegara | Qiymat | Nima uchun ishlatiladi | Qayerda |
|---|---|---|---|
| `analytics.MinObservations` | 3 | Median ko'rsatish (`enough = true`), viloyat medianalari ro'yxati, bozordagi o'rin, lenta kartochkasidagi median, kabinetdagi median, `/savol` javoblari, kuzatuvlar | `analytics/stats.go`; `HAVING count(*) >= 3` SQL'da |
| `analytics.OutlierMinSample` | 5 | Robust z-ball faqat ≥ 5 ta boshqa narx bo'lsa | `analytics/stats.go` |
| 3–4 ta boshqa narx | ±50% | Robust z o'rniga median atrofida ±50% chegarasi | `IsOutlier` |
| `analytics.MinRepricePeers` | 5 | Avtomatik narx ishlashi uchun | `analytics/repricing.go` |
| Viloyat hisoboti | 5 | Mahsulot haftalik hisobotda faqat ≥ 5 narx bilan | `queries/report.sql` |
| `minPublic` (httpapi) | 5 | Ommaviy qiziqish/sotuv sonlari 5 dan kam bo'lsa ko'rsatilmaydi | `httpapi/funnel.go` |
| `analytics.MinHistory` | 2 | Bosim indeksi uchun oldingi haftalar soni | `analytics/pressure.go` |

E'tibor bering: `offers` (kuzatuvlar soni) va chegara solishtiruvi **kuzatuvlar** soni bo'yicha, noyob sotuvchilar bo'yicha emas. Bitta sotuvchining e'loni va uning RFQ taklifi ikki kuzatuv hisoblanadi.

## 7. Hudud kengayishi va `price_scope`

Hududlar ikki darajali: viloyat (`regions.parent_id IS NULL`) va tuman (`parent_id` = viloyat). Kuzatuvning viloyati `COALESCE(r.parent_id, r.id)`.

**Mahsulot sahifasi va e'lon sahifasi** (`marketPrice`, `backend/internal/httpapi/browse.go`):

1. Foydalanuvchi tumanni tanlagan bo'lsa — avval shu tuman narxlari (`ProductStats`, `region_id = tuman`).
2. Kamida 3 ta bo'lmasa — tumanning viloyati (viloyat va uning barcha tumanlari).
3. Yana 3 ta bo'lmasa — butun respublika.
4. Viloyat tanlangan bo'lsa, 1-qadam o'tkazib yuboriladi; hech narsa tanlanmagan bo'lsa, darhol respublika.

Javobdagi `price_scope` — median qaysi hudud uchun hisoblanganini bildiradi (`id`, `slug`, `name`, `kind`). Respublika bo'yicha hisoblanganda maydon **yo'q** (omitted). Hech bir darajada 3 ta narx bo'lmasa, `price.enough = false` va median ko'rsatilmaydi, `price.offers` esa oxirgi (respublika) darajadagi sonni beradi. E'lon sahifasida region tanlanmagan bo'lsa, e'lonning o'z tumanidan boshlanadi.

`price` ichida: `median`, `p25`, `p75`, `min`, `max`, `offers`, `first_seen`, `last_seen` (hammasi shu scope uchun, so'mga yaxlitlangan).

**Boshqa joylar kengayishni boshqacha qiladi:**

- **Sotuvchi kabineti** (`ListSellerListings`): e'lon viloyatida ≥ 3 narx bo'lsa viloyat medianasi, aks holda respublika medianasi; `median_in_province` qaysi biri ekanini aytadi. Tuman darajasi yo'q.
- **Lenta** (`FeedListings`): tanlangan hudud (yoki butun respublika) bo'yicha ≥ 3 narxli median; kengayish yo'q.
- **Bozordagi o'rin, anomaliya tekshiruvi, avtomatik narx**: har doim e'lonning **viloyati**, kengayishsiz.

## 8. Anomaliyalarni aniqlash va tekshiruv

**G'oya.** Eng ko'p uchraydigan xato — ortiqcha yoki tushib qolgan nol ("45 000" o'rniga "4 500"). Bunday narx medianni buzmasligi uchun u saqlanadi, lekin `review` holatida bo'ladi: sotuvchi o'z e'lonini ko'radi, xaridorlar va median ko'rmaydi, admin tekshiradi.

### 8.1 Sotuvchi narxi uchun `Screen` (`analytics/anomaly.go`)

Kirish: `Previous` — shu e'lonning oxirgi **qabul qilingan** narxi (`LastAcceptedPrice`: `active` yoki `expired` holatidagi oxirgi kuzatuv; tekshiruvdagi xato narx o'tkazib yuboriladi, shunda uni tuzatish ham ushlanib qolmaydi); `Peers` — viloyatdagi boshqa tirik narxlar (shu e'londan tashqari, `PeerPrices`); `Reference` — rasmiy etalon (11-bo'lim). Qiymat 0 bo'lsa "noma'lum" va tekshiruv o'tkazib yuboriladi.

Tartib (birinchi mos kelgani qaytariladi):

1. **Oldingi narxdan sakrash** (`jump_from_previous`), `Previous > 0` bo'lsa:
   ```
   r = narx / Previous;   r ≥ 5  yoki  r ≤ 1/5  → review
   ```
   (`MaxJump = 5.0`, chegaralar kiradi.)
2. **≥ 5 ta boshqa narx** (`OutlierMinSample`) bo'lsa — robust z-ball (`far_from_market`), pastda.
3. **Aks holda, etalon bor bo'lsa** (`far_from_reference`):
   ```
   |narx − Reference| / Reference > 0.5  → review
   ```
   (`ReferenceBand = 0.5`.)
4. **Aks holda, 3–4 ta boshqa narx bo'lsa** — ularning oddiy medianasi atrofida ±50% (`far_from_market`):
   ```
   m = Median(peers);   m > 0  va  |narx − m| / m > 0.5  → review
   ```
5. 0–2 ta boshqa narx va etalon yo'q — narx o'tadi.

### 8.2 Robust z-ball (`RobustZ`, `IsOutlier`, `analytics/stats.go`)

```
x̃   = Median(peers)                         (vaznsiz, juft sonda ikki o'rtaning o'rtachasi)
MAD = Median(|peers_i − x̃|)
MAD' = max(MAD, 0.02 · |x̃|)
z   = 0.6745 · (x − x̃) / MAD'
|z| > 3.5  → review
```

- `x` — tekshirilayotgan narx;
- `0.6745` — normal taqsimotda MAD ni standart og'ishga keltiruvchi koeffitsiyent (Iglewicz va Hoaglin);
- `0.02 · |x̃|` — MAD uchun pastki chegara: ko'p sotuvchi bir xil narx aytsa MAD = 0 bo'lib, har qanday kichik farq "cheksiz" ko'rinmasligi uchun.

E'tibor: robust z **vaznsiz** medianni ishlatadi; vaznli median faqat ko'rsatish va o'rin uchun.

**Ishlangan misol.** Viloyatda 6 ta boshqa narx: 4 000, 4 200, 4 300, 4 500, 4 600, 5 000.

- `x̃ = (4 300 + 4 500) / 2 = 4 400`.
- Og'ishlar: 400, 200, 100, 100, 200, 600 → tartibda 100, 100, 200, 200, 400, 600 → `MAD = (200 + 200) / 2 = 200`. Pastki chegara `0.02 · 4 400 = 88`, demak `MAD' = 200`.
- Yangi narx 6 000: `z = 0.6745 · 1 600 / 200 = 5.40` → **review** (`far_from_market`).
- Yangi narx 5 200: `z = 0.6745 · 800 / 200 = 2.70` → o'tadi.
- Yuqori chegara: `4 400 + 3.5 · 200 / 0.6745 ≈ 5 438` so'm.

**MAD = 0 misoli.** 5 ta sotuvchi hammasi 5 000 so'm. `MAD = 0`, `MAD' = 100`. 5 500 uchun `z = 0.6745 · 500 / 100 = 3.37` → o'tadi; 5 600 uchun `z = 4.05` → review. Ya'ni taxminan ±10,4% gacha ruxsat.

**Sakrash misoli.** Oldingi qabul qilingan narx 4 500. Yangi 45 000: `r = 10 ≥ 5` → `jump_from_previous`. Yangi 900: `r = 0.2 ≤ 0.2` → ham review.

### 8.3 Boshqa manbalar uchun tekshiruv

| Manba | Qoida | Qayerda |
|---|---|---|
| `rfq_offer` | Faqat `IsOutlier(peers, narx)` (≥5 narx: robust z; 3–4: ±50%). Sakrash va etalon tekshirilmaydi. Peers — viloyatdagi ulgurji tirik narxlar | `recordOfferPrice`, `rfq/rfq.go` |
| `telegram_channel` | Kanal viloyatining tirik vaznli medianasi (≥ 3 narx) bo'lsa: `narx / median > 2` yoki `< 1/2` → review (`farFactor = 2.0`) | `screen`, `channels/channels.go` |
| `deal` | E'lon narxining `0.5 × … 2 ×` oralig'idan tashqari → review; e'lonsiz bitim doim `active` | `InsertDealObservation`, `queries/deals.sql` |
| `seller_auto` | Tekshirilmaydi: narx sotuvchi oralig'i, kunlik ±5% va bozor P75 bilan chegaralangan | [avtomatik narx](06-avtomatik-narx.md) |

### 8.4 Tekshiruv va rad etish oqimi

1. `reviewStatus` (`market/listings.go`) `Screen` natijasiga ko'ra e'lonni `active` yoki `review` qiladi va sababni `listings.review_reason` ga yozadi (`jump_from_previous`, `far_from_market`, `far_from_reference`).
2. Kuzatuv `review` holatida yoziladi — medianga kirmaydi, bozordagi o'rin hisoblanmaydi (`PositionOf` faqat `active` e'lon uchun).
3. Sotuvchiga javobda `status: "review"` qaytadi; e'lon unga ko'rinadi.
4. Admin navbatida sabab bilan ko'rinadi. `Review(id, approve, note)`:
   - **tasdiqlash** → e'lon `active`, kuzatuv `active` — narx darhol medianga kiradi;
   - **rad etish** → e'lon `rejected`, kuzatuv `rejected`, admin izohi saqlanadi va sotuvchiga ko'rsatiladi.
5. Rad etilgan e'lon keyingi tahrirlarda ham `rejected` qoladi (`UpsertListing`, `UpdateListing` dagi `CASE WHEN status = 'rejected'`), va uni "ko'rsatish" tugmasi bilan qayta faollashtirib bo'lmaydi (`SetListingStatus` faqat `active` ↔ `hidden`).
6. Yashirin e'lon tahrirlansa ham yashirin qoladi (`Update`).

## 9. Sotuvchining bozordagi o'rni

**G'oya.** "Viloyatda 12 ta narx, siz 9-o'rindasiz, medianadan 7% qimmat."

**Qoida** (`analytics.PositionOf`, `analytics/position.go`; chaqiruvchi `market.PositionOf`):

- Faqat `active` e'lon uchun hisoblanadi (tekshiruvdagi, yashirin, rad etilgan — yo'q).
- `peers` — e'lon **viloyatidagi** shu mahsulot va savdo turining boshqa tirik narxlari (shu e'londan tashqari) va ularning vaznlari (`PeerPrices`).
- `len(peers) < 3` bo'lsa → `Enough = false`, API'da `position` maydoni yo'q.
- Aks holda:
  ```
  rank        = (peers ichida narxdan qat'iy arzonlari soni) + 1      (1 = eng arzon; teng narxlar bir o'rinda)
  total       = len(peers) + 1
  median      = WeightedQuantile(peers, weights, 0.5)                  (boshqalarning vaznli medianasi)
  diff_percent = round(100 · (narx − median) / median)
  ```

API maydonlari (`pricing.MarketPosition`): `rank`, `total`, `median` (so'mga yaxlitlangan), `diff_percent`. U narx tasdiqlangandan keyin (`Confirm`, bot va sayt) va sotuvchi kabinetida qaytariladi.

**Ishlangan misol.** Boshqa narxlar: 4 000, 4 200, 4 300, 4 500, 4 600, 5 000 — hammasi yangi `seller` (w = 0.8). Sotuvchi narxi 4 700.

- Arzonlari: 4 000, 4 200, 4 300, 4 500, 4 600 → 5 ta → `rank = 6`, `total = 7`.
- Vaznli median: `W = 4.8`, yarmi 2.4. Kumulyativ: 0.8, 1.6, **2.4** (4 300 da) → `median = 4 300` (pastki kvantil qoidasi; oddiy median 4 400 bo'lardi).
- `diff_percent = round(100 · (4 700 − 4 300) / 4 300) = round(9.30) = 9`.

Natija: "7 ta sotuvchi, siz 6-o'rindasiz, medianadan 9% qimmat".

## 10. `diff_percent` hisoblash

Barcha joyda formula bir xil:

```
diff_percent = round(100 · (narx − median) / median)
```

- `narx` — e'lon narxi;
- `median` — tegishli median; manfiy natija "bozordan arzon" degani;
- `round` — eng yaqin butun songa (Go `math.Round`, 0,5 noldan uzoqqa).

Farq — qaysi median:

| Joy | Median |
|---|---|
| Bozordagi o'rin (`position.diff_percent`) | Viloyatdagi **boshqa** sotuvchilarning vaznli medianasi (e'lonning o'zi kirmaydi) |
| Mahsulot sahifasidagi takliflar (`offers[].diff_percent`) | `marketPrice` scope medianasi, so'mga yaxlitlangan (e'lonning o'z narxi ham kiradi) |
| E'lon sahifasi (`listing.diff_percent`) | Xuddi shunday, `marketPrice` dan |
| Lenta kartochkasi (`ListingCard.diff_percent`) | `FeedListings` dagi median (tanlangan hudud yoki respublika, ≥ 3 narx), yaxlitlanmagan holda; kodi `httpapi/feed.go` |

Median ko'rsatilmasa (`enough = false`), `diff_percent` ham yo'q.

## 11. Rasmiy etalonlar (UzEx, narxtahlil)

- `uzex` va `narxtahlil` manbalarining vazni **0**, shuning uchun ular hech qachon vaznli medianga ta'sir qilmaydi.
- Ular faqat bitta joyda ishlatiladi: sotuvchi narxini tekshirishda (8.1, 3-qadam), viloyatda 5 tadan kam boshqa narx bo'lsa.
- `ReferencePrice` (`queries/listings.sql`): oxirgi **14 kun** ichidagi etalon narxlar shu mahsulot va savdo turi bo'yicha — `reference_prices` jadvali (konnektorlar yozadi; hududsiz qator — respublika narxi) va `price_observations` dagi `uzex`/`narxtahlil` qatorlari (`rejected` dan tashqari). Avval viloyat medianasi, bo'lmasa respublika medianasi, bo'lmasa 0. Bu yerda median **vaznsiz** va interpolyatsiyali (`percentile_cont(0.5)`).

**Hozirgi holat:** UzEx konnektori (`backend/internal/external/uzex.go`, `UZEX_ENABLED=true`, har kuni 18:30) birja kotirovkalarini `reference_prices` ga yozadi, lekin faqat 3 mahsulot uchun: bug'doy (3-sinf), armatura (12 mm), qopdagi sement — tonna narxi mahsulot asosiy birligiga o'tkaziladi. Boshqa mahsulotlar ataylab olinmagan (masalan un: bitta nav kotirovkasi katalogdagi aralash navlardan 30% farq qiladi). narxtahlil konnektori yo'q. Demak ko'p mahsulotlarda `Reference` 0 va 3-qadam ishlamaydi. Etalonlar saytga `GET /v1/products/{slug}/references` orqali alohida chiziq sifatida beriladi. Seed ma'lumotlaridagi narxlar bu manbalarga asoslangan, lekin `seller` kuzatuvlari sifatida yoziladi (`cmd/seed/data.go`).

## 12. O'lchov birliklari

- Har bir mahsulotning **asosiy birligi** bor (`products.base_unit`: `kg`, `litr`, `dona` va h.k.). Barcha narxlar shu birlikdagi 1 dona/kg/litr uchun saqlanadi; e'lonning `unit` maydoni doim mahsulotning asosiy birligi (`UpsertTx`).
- **Qadoq birliklari** `unit_conversions` jadvalida: `(product_id, unit, base_quantity)`, masalan kartoshka `qop = 50` kg, boshqa mahsulotlar uchun `qop = 40`, `yashik = 20`, `15`, `18`, `10` kg, tuxum uchun `karobka = 360` dona (migratsiya `00003_seed_catalog.sql`).
- Matn parseri (`toBase`, `backend/internal/pricing/parser.go`):
  ```
  narx_asosiy = round(narx_yozilgan / o'lcham)
  ```
  Masalan, "kartoshka qopi 225 000" → `225 000 / 50 = 4 500` so'm/kg; sotuvchiga "1 qop = 50 kg bo'yicha hisoblandi" izohi ko'rsatiladi.
- `tonna` (×1 000) va `gramm` (÷1 000) faqat asosiy birligi `kg` bo'lgan mahsulotlar uchun.
- Mahsulot uchun noma'lum qadoq (masalan, sementning "qopi" jadvalda yo'q) → xato: "«qop» necha kg ekanini bilmaymiz. Narxni 1 kg uchun yozing."
- "4.5" yoki "4,5" kabi kichik sonlar (`< 1 000`, ko'paytiruvchi so'zsiz, kasr yoki `< 100`) 4 500 so'm deb o'qiladi va tasdiqlash uchun ko'rsatiladi.
- Narx chegarasi: 50 dan 50 000 000 so'mgacha (asosiy birlik uchun); tashqarisi xato.

Parser haqida batafsil: [Telegram bot](07-telegram-bot.md).

## 13. spec.md bilan farqlar va ochiq masalalar

Kod va `spec.md` §10 farq qiladigan joylar (kod ustun):

- **Manbalar:** spec'da `deal` (1.0) va `seller_auto` (0.4) yo'q; kodda bor.
- **Anomaliya:** spec "5 tadan kam bo'lsa etalondan ±50%" deydi. Kodda avval sotuvchining o'z oldingi narxidan ×5 sakrash tekshiriladi; etalon bo'lmasa 3–4 ta narx medianasidan ±50%; MAD uchun 2% pastki chegara qo'shilgan.
- **Bozordagi o'rin:** spec "foizli o'rin (percentile)" deydi; kod tartib raqami (`rank` / `total`) va medianadan farqni beradi.

Kodda ko'rilgan nozik joylar (xato bo'lishi mumkin, tekshirish kerak):

- `PeerPrices` va `ProductStats` manbani filtrlamaydi: agar bazada `uzex`/`narxtahlil` kuzatuvlari paydo bo'lsa, ular vazni 0 bo'lsa ham `offers` soniga, robust z uchun `peers` ro'yxatiga va `rank`/`total` ga kiradi. Konnektor `reference_prices` ga yozgani uchun (`price_observations` ga emas) bu hozir ta'sir qilmaydi.
- `PeerPrices` sotuvchining **boshqa** kuzatuvlarini (masalan, o'z RFQ taklifi yoki boshqa navdagi e'loni) chiqarib tashlamaydi — ular "boshqa sotuvchi" sifatida hisoblanadi.
- `deal` kuzatuvi `review` holatiga tushsa, uni tasdiqlash uchun alohida admin yo'li kodda ko'rinmadi (`Review` faqat e'lon kuzatuvlarini o'zgartiradi).
- Yashirin e'lonning kuzatuvi `active` qoladi va faqat `EXISTS` sharti bilan chiqariladi; `EXISTS` ni unutgan yangi so'rov yashirin narxni medianga qo'shib yuborishi mumkin.

## Qayerda joylashgan

- `backend/migrations/00012_weighted_median.sql` — `price_weight`, `weighted_quantile` (birinchi versiya)
- `backend/migrations/00017_dynamic_pricing.sql`, `00021_deal_prices.sql` — `seller_auto`, `deal` va `price_weight` ning oxirgi versiyasi
- `backend/migrations/00024_price_daily.sql` — `source_trust` (yangiliksiz vazn)
- `backend/migrations/00008_retire_deleted_listing_prices.sql`, `00014_retire_deleted_offer_prices.sql` — o'chirishda narxni chiqaruvchi triggerlar
- `backend/migrations/00002_core_schema.sql`, `00003_seed_catalog.sql` — `price_observations`, `unit_conversions`
- `backend/internal/analytics/weighted.go` — `WeightedQuantile`
- `backend/internal/analytics/stats.go` — `MinObservations`, `Median`, `MAD`, `RobustZ`, `IsOutlier`
- `backend/internal/analytics/anomaly.go` — `Screen`, `MaxJump`, `ReferenceBand`
- `backend/internal/analytics/position.go` — `PositionOf`
- `backend/internal/market/listings.go` — `Validity`, `RenewWindow`, `reviewStatus`, `PositionOf`, `peerPrices`, `recordPrice`, `Review`, `Refresh*`
- `backend/internal/market/sold.go` — `MarkSold`, `SyncDealPrice`
- `backend/internal/pricing/service.go` — `Confirm`, `MarketPosition`
- `backend/internal/pricing/parser.go` — birlik konvertatsiyasi (`toBase`)
- `backend/internal/rfq/rfq.go` — `recordOfferPrice`
- `backend/internal/channels/channels.go` — `screen`, `farFactor`
- `backend/internal/httpapi/browse.go` — `marketPrice`, `price_scope`
- `backend/internal/httpapi/feed.go` — e'lon sahifasi median va `diff_percent`
- `backend/queries/browse.sql` — `ProductStats`, `BrowseProducts`, `FeedListings`, `ProductProvinceMedians`
- `backend/queries/listings.sql` — `PeerPrices`, `LastAcceptedPrice`, `ReferencePrice`, `ListSellerListings`
- `backend/queries/deals.sql`, `rfq.sql`, `channels.sql`, `report.sql` — boshqa manbalar va hisobot chegarasi
