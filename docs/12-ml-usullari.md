# Mashinaviy o'qitish va robust statistika usullari

Bu hujjat backend'ga qo'shilgan yetti usulni tushuntiradi: robust statistika (median, MAD, robust z), prognoz usullari musobaqasi, aniq Shapley qiymatlari (prognoz `drivers` va avtomatik narx `contrib`), kunlik narx anomaliyalari, sotuvchilar uchun Isolation Forest, viloyatlarni K-Means bilan guruhlash, issiqlik xaritasida Local Moran's I, tizim monitoringi va ixtiyoriy til modellari (kanal postlari, tahlilchi, ovoz, rasm, katalogga moslash, maslahat matni, moderatsiya). Hammasi Go'da, tashqi kutubxonasiz yozilgan (`backend/internal/ml`). Model faylga saqlanmaydi: har ishga tushishda ma'lumotdan qayta hisoblanadi. Tasodifiy tanlovlar doim bir xil urug'dan (`ml.Seed = 42`) olinadi, shuning uchun bir xil ma'lumot har doim bir xil natija beradi. Ma'lumot yetarli bo'lmasa, natija chiqmaydi (maydon tushib qoladi yoki bo'sh ro'yxat qaytadi). Chegaralar (anomaliya, prognoz, klasterlar) muhit o'zgaruvchilarida sozlanadi: [sozlamalar, 4-bo'lim](13-sozlamalar.md#4-backend-algoritm-chegaralari).

**Muhim:** tarixiy tahlillarning ko'pi hozircha `seed -history` yaratgan sintetik demo tarix ustida ishlaydi. Har natija o'z `demo_share` ini qaytaradi va sayt "Demo ma'lumot" belgisini ko'rsatadi.

Asosiy tahlil (kunlik tarix, trend, mavsumiylik, bosim indeksi va boshqalar) [tahlil metodologiyasi](04-tahlil-metodologiyasi.md)da, avtomatik narx algoritmi [avtomatik narx](06-avtomatik-narx.md)da, kanal postlarini qoidalar bilan o'qish [Telegram bot](07-telegram-bot.md#8-kanallar-va-guruhlar)da. Bu yerda faqat yangi usullar va ularning o'sha tizimlarga qanday ulanishi yozilgan.

## Mundarija

1. [Robust statistika](#1-robust-statistika)
2. [Prognoz usullari musobaqasi](#2-prognoz-usullari-musobaqasi)
3. [Avtomatik narxda aniq Shapley hissalari (`contrib`)](#3-avtomatik-narxda-aniq-shapley-hissalari-contrib)
4. [Kunlik narx anomaliyalari](#4-kunlik-narx-anomaliyalari)
5. [Isolation Forest: g'ayrioddiy sotuvchilar](#5-isolation-forest-gayrioddiy-sotuvchilar)
6. [K-Means: o'xshash viloyatlar](#6-k-means-oxshash-viloyatlar)
7. [Local Moran's I: issiq va sovuq nuqtalar](#7-local-morans-i-issiq-va-sovuq-nuqtalar)
8. [Tizim monitoringi](#8-tizim-monitoringi)
9. [Ixtiyoriy til modellari](#9-ixtiyoriy-til-modellari)
10. [Usul → qayerda ko'rinadi](#10-usul--qayerda-korinadi)
11. [Kod va hujjatlar orasidagi farqlar](#11-kod-va-hujjatlar-orasidagi-farqlar)
12. [Qayerda joylashgan](#qayerda-joylashgan)

---

## 1. Robust statistika

**G'oya.** O'rtacha qiymat va standart og'ish bitta xato narxga juda sezgir: xato narxning o'zi o'rtachani o'ziga tortadi va og'ishni kattalashtiradi, natijada u "unchalik g'alati emas" bo'lib ko'rinadi (bu "maskalanish" deyiladi). Median va MAD esa chetdagi qiymatga deyarli qaramaydi. Shuning uchun barcha yangi usullar shu ikkitasiga tayanadi.

**Formulalar** (`backend/internal/ml/robust.go`):

```
Median(xs)   — tartiblangan qatorning o'rtasi (juft sonda ikki o'rtadagining o'rtachasi)
MAD(xs)      = Median(|x_i − Median(xs)|)
robust_z(x)  = (x − Median(ref)) / (1.4826 × MAD(ref))
```

- `x` — tekshirilayotgan qiymat, `ref` — solishtiriladigan qiymatlar to'plami.
- `1.4826` (`madScale`) — normal taqsimotda MAD ni standart og'ishga aylantiradigan koeffitsiyent. Shunda robust z odatdagi z bilan bir xil shkalada bo'ladi.
- `RobustZ` natija bermaydi (`ok = false`), agar `ref` da **5 tadan kam** qiymat bo'lsa yoki `MAD = 0` bo'lsa (tarqoqlik yo'q).

**Nega log narx.** Narxlar ko'paytma bo'yicha o'zgaradi: 5 000 dan 10 000 ga o'tish ham, 10 000 dan 5 000 ga tushish ham "ikki barobar". Log shkalada bu ikki harakat bir xil kattalikda (`±ln 2`), shuning uchun anomaliya qidirish va viloyat profillarida `ln(narx)` yoki `ln(narx / respublika)` ishlatiladi. Log farqni foizga qaytarish: `e^d − 1`.

**`Standardize`.** Har ustunni median 0 va robust tarqoqlik 1 ga keltiradigan funksiya ham bor (MAD 0 bo'lsa o'rtacha absolyut og'ishga o'tadi). Hozir u faqat testlarda ishlatiladi; ishlab turgan usullarning hech biri uni chaqirmaydi (6-bo'limga qarang).

**Misol** (kod izohidagi). Qiymatlar `[4, 5, 5, 6, 100]`, `100` ni tekshiramiz.

- O'rtacha va standart og'ish bilan (100 o'zi ichida): o'rtacha 24, og'ish 38, `z = (100 − 24) / 38 = 2.0` — "oddiy" ko'rinadi.
- Median 5, og'ishlar `[1, 0, 0, 1, 95]` → MAD 1. `z = (100 − 5) / (1.4826 × 1) ≈ 64` — aniq chetdagi qiymat.

Eski narx tekshiruvidagi (`analytics/stats.go`) robust z haqida [narx metodologiyasi](03-narx-metodologiyasi.md#82-robust-z-ball-robustz-isoutlier-analyticsstatsgo)ga qarang; `ml.RobustZ` shu g'oyaning umumiy nusxasi.

---

## 2. Prognoz usullari musobaqasi

**G'oya.** Bitta formulaga ishonish o'rniga uchta oddiy usul bir xil sharoitda sinaladi: har biri o'tgan yilning har haftasida "o'sha kungacha bo'lgan ma'lumot bilan" 1–4 hafta oldinga prognoz qiladi va haqiqiy narx bilan solishtiriladi. Eng kam xato qilgani tanlanadi. Agar murakkab usul oddiy "narx o'zgarmaydi" usulidan yaxshi bo'lmasa, oddiysi ko'rsatiladi. Bu [tahlil metodologiyasi, 8-bo'lim](04-tahlil-metodologiyasi.md#8-prognoz-va-backtest)dagi yagona modelning o'rnini egalladi (o'sha bo'limdagi `season_trend` formulasi hali ham usullardan biri sifatida ishlaydi).

Kod: `backend/internal/insight/forecast.go`, `ForecastOf`, `predict`.

### 2.1 Uchta usul

Hammasi bitta asosdan boshlanadi: `baza = oxirgi 7 kunlik kunlik medianlar o'rtachasi` (`windowMean(ps, end, 7)`). Har bir usul bazani bir yoki bir nechta ko'paytuvchiga ko'paytiradi:

```
prognoz = baza × Π (1 + ulush_k)
```

| Usul | Ko'paytuvchilar (`ulush_k`) | Qachon ishlay olmaydi |
|---|---|---|
| `naive` | yo'q: `prognoz = baza` | oxirgi 7 kunda narx yo'q |
| `last_year` | `last_year = keyin / o'shanda − 1`, bu yerda `o'shanda` — 364 kun oldingi 7 kunlik o'rtacha, `keyin` — 364 kun oldingidan `h` hafta keyingi 7 kunlik o'rtacha | o'tgan yilning shu haftalarida narx yo'q |
| `season_trend` | `season = S(maqsad) / S(hozir) − 1`; `trend = clamp(Σ_{k=1..h} weekly × 0.8^k, −0.15, +0.15)` | mavsumiy indeks yo'q |

- `h` — necha hafta oldinga (1..4).
- `S(kun)` — oylik mavsumiy indeks kunga chiziqli interpolyatsiya qilingan (`seasonAt`); `maqsad` — maqsad haftaning o'rtasi (`end + 7h − 3` kun), `hozir` — baza haftasining o'rtasi (`end − 3` kun).
- `weekly` — 28 kunlik trend sur'ati (`TrendOf`, [tahlil, 5-bo'lim](04-tahlil-metodologiyasi.md#5-trend)). Trend hisoblanmasa, `season_trend` faqat `season` ko'paytuvchisi bilan ishlaydi.
- `trendDamping = 0.8`, `maxTrendEffect = 0.15`.
- `364` kun = aynan 52 hafta, shuning uchun hafta kunlari mos keladi.

### 2.2 Backtest: bir xil haftalarda sinash

1. **Kelib chiqish nuqtalari (origin):** `bugun − 365` kundan `bugun − 28` kungacha, 7 kunlik qadam bilan (taxminan 49 ta).
2. Har origin uchun faqat o'sha kungacha bo'lgan nuqtalar olinadi (`past`), har usul `h = 1..4` hafta uchun prognoz qiladi. Haqiqiy qiymat — origin'dan `7h` kun keyin tugaydigan 7 kunlik o'rtacha.
3. Xato: `e = |prognoz / haqiqiy − 1|`. Origin usul uchun faqat **to'rttala** gorizont hisoblangan bo'lsa qabul qilinadi.
4. **Umumiy haftalar:** usullar faqat hammasi prognoz qila olgan originlarda solishtiriladi (adolatli musobaqa). Umumiy haftalar **20 tadan kam** (`minBacktests`) bo'lsa, eng kam hafta qamragan usul (`naive` hech qachon) chiqarib tashlanadi va qayta sanaladi. Oxirida ham 20 dan kam bo'lsa, prognoz yo'q.
5. **MAPE:** `MAPE_usul = Σ e / (umumiy_haftalar × 4)` — o'rtacha nisbiy xato, 0.08 = 8%.
6. **G'olib:** usullar MAPE bo'yicha o'sish tartibida saralanadi (`methods`), va bugungi kundan to'rttala haftani prognoz qila oladigan birinchi usul tanlanadi (`method`).
7. **`skill`:** `skill = 1 − MAPE_g'olib / MAPE_naive`. G'olib `naive` bo'lsa `skill = 0`. `0.3` = oddiy usuldan 30% aniqroq. G'olib ro'yxatdagi birinchi "ishlay oladigan" usul bo'lgani va `naive` doim ishlay olgani uchun `skill` manfiy bo'lmaydi.
8. **`mape`** (massiv) — g'olib usulning har gorizont bo'yicha o'rtacha xatosi; **`tested`** — umumiy haftalar soni.

`last_year` usuli backtestda ham o'tgan yilni ko'rishi kerak, ya'ni to'liq sinov uchun taxminan ikki yillik tarix kerak. `season_trend` uchun mavsumiy indeks butun tarixdan tuzilgani sababli uning backtestdagi aniqligi biroz optimistik; bu API `note` matnida ochiq yozilgan.

### 2.3 Oraliq (`low`, `high`)

Har gorizont `h` uchun g'olib usulning umumiy haftalardagi xatolari tartiblanadi va 80-persentil olinadi: indeks `min(n − 1, ceil(0.8 × n) − 1)`, qiymati `q`.

```
low  = prognoz × (1 − q)
high = prognoz × (1 + q)
```

Ya'ni o'tgan yil xatolarining 80% shu oraliqqa sig'gan. `median`, `low`, `high` va `base` `roundPrice` bilan yaxlitlanadi: ≥100 000 → 1 000 ga, ≥10 000 → 100 ga, ≥1 000 → 50 ga, aks holda 10 ga.

### 2.4 Shapley `drivers`: prognozni nima suryapti

**G'oya.** `season_trend` yoki `last_year` prognozi bazadan farq qiladi. Bu farqning qancha qismi mavsumdan, qanchasi trenddan? Ko'paytuvchilar bir-biriga ko'payadi, shuning uchun "mavsum +4%, trend +2%" yig'indisi umumiy +6.08% ga teng emas; qo'shimcha 0.08% qaysi biriga tegishli ekanini adolatli bo'lish kerak. Buni Shapley qiymati qiladi.

**O'yinchilar** — usulning ko'paytuvchilari: `season`, `trend` (`season_trend` da) yoki `last_year`. `naive` da o'yinchi yo'q, `drivers` bo'sh.

**Qiymat funksiyasi:** koalitsiya `M` (o'yinchilar to'plami) uchun `v(M) = Π_{k∈M} (1 + ulush_k)`, ya'ni faqat `M` dagi ko'paytuvchilar qo'llanganda narx bazaning necha barobari bo'lishi. `v(∅) = 1`.

**Aniq Shapley** (`ml.Shapley`, `backend/internal/ml/shapley.go`): barcha `2^n` koalitsiya sanab chiqiladi (tanlama yo'q, tasodif xatosi yo'q):

```
φ_i = Σ_{M ⊆ N∖{i}}  |M|! (n − |M| − 1)! / n!  × (v(M ∪ {i}) − v(M))
```

- `n` — o'yinchilar soni, `N` — hammasi, `φ_i` — `i`-o'yinchi hissasi (bazaning ulushi sifatida).
- **Yig'indi xossasi:** `Σ φ_i = v(N) − v(∅) = prognoz / baza − 1`.
- `n ≤ 12` (`MaxShapleyPlayers`, 4 096 koalitsiya); ko'prog'ida funksiya panic qiladi. Prognozda o'yinchilar 1–2 ta.
- Har `share` 3 xonagacha yaxlitlanadi; 0 ga yaxlitlanganlari ro'yxatdan tushadi.

**Misol.** `baza = 10 000`, `season = +0.04`, `trend = +0.02`.

| Koalitsiya | `v` |
|---|---|
| ∅ | 1 |
| {season} | 1.04 |
| {trend} | 1.02 |
| {season, trend} | 1.0608 |

`n = 2` da vaznlar 1/2 va 1/2:

```
φ_season = ½(1.04 − 1) + ½(1.0608 − 1.02) = 0.0200 + 0.0204 = 0.0404
φ_trend  = ½(1.02 − 1) + ½(1.0608 − 1.04) = 0.0100 + 0.0104 = 0.0204
yig'indi = 0.0608 = 1.0608 − 1
```

Prognoz `10 000 × 1.0608 = 10 608` → yaxlitlangan `median = 10 600`. `drivers = [{season, 0.04}, {trend, 0.02}]` (3 xonaga yaxlitlangan). E'tibor bering: `median` yaxlitlangani uchun `drivers` yig'indisi `median / base − 1` ga taxminan teng, aynan emas (11-bo'lim).

### 2.5 API ko'rinishi

`GET /v1/products/{slug}/analytics` javobidagi `forecast`:

```json
{"method": "season_trend", "base": 10000, "skill": 0.31,
 "methods": [{"method": "season_trend", "mape": 0.062}, {"method": "last_year", "mape": 0.081}, {"method": "naive", "mape": 0.09}],
 "weeks": [{"week": "2026-09-27", "median": 10200, "low": 9700, "high": 10700,
            "drivers": [{"code": "season", "share": 0.015}, {"code": "trend", "share": 0.005}]}],
 "mape": [0.041, 0.055, 0.068, 0.084], "tested": 44, "note": "..."}
```

(Raqamlar namuna uchun. `skill = 1 − 0.062 / 0.09 ≈ 0.31`.)

---

## 3. Avtomatik narxda aniq Shapley hissalari (`contrib`)

**G'oya.** Avtomatik narxda `factors` har bir omilning *maqsad narxga* ta'sirini ko'rsatadi, lekin keyin chegara (sotuvchi oralig'i, P75, kunlik ±5%) va yaxlitlash harakatni kesadi. Natijada "omillar +8%, narx +4.4% o'zgardi" bo'lib qoladi va farq qayerga ketgani noma'lum. `contrib` haqiqiy harakatni (`yangi / eski − 1`) o'yinchilar orasida aniq Shapley qiymatlari bilan bo'ladi, chegara hisobga olingan holda.

Kod: `contributions` (`backend/internal/analytics/repricing.go`). Algoritmning qolgan qismi [avtomatik narx](06-avtomatik-narx.md#6-algoritm-qadamma-qadam)da.

**O'yinchilar:**

1. `market` — joriy narxdan viloyat medianiga o'tish;
2. `goal` — faqat `g ≠ 1` bo'lsa (`fast` yoki `profit`);
3. qolgan nolga teng bo'lmagan omillar: `pressure`, `trend`, `response`, `rfq`, `sales`, `stock`.

`cap` o'yinchi emas: chegaralar har doim qo'llanadi.

**Qiymat funksiyasi** (koalitsiya `M` uchun narx):

```
asos  = M da market bo'lsa Median, aks holda Current
g_M   = M da goal bo'lsa g, aks holda 1
Σ_M   = M dagi omillar ulushlari yig'indisi
v(M)  = clamp(asos × g_M × (1 + Σ_M), lo, hi)
```

- `lo`, `hi` — algoritmning 5-qadamidagi yakuniy chegaralar (sotuvchi oralig'i, P75, kunlik ±5%).
- `v(∅) = clamp(Current, lo, hi) = Current` (kunlik chegara joriy narx atrofida bo'lgani uchun odatda shunday).

**Hisob.** `φ_i = ml.Shapley(...)`, keyin har biri eski narxga bo'linadi: `contrib_i = φ_i / Current`. Yaxlitlash alohida qator bo'ladi:

```
rounding = (yaxlit_narx − v(N)) / Current
```

Shunda `Σ contrib_i + rounding = (v(N) − v(∅) + yaxlit − v(N)) / Current = yaxlit / Current − 1`, ya'ni aynan qo'llangan o'zgarish (agar `v(∅) = Current`). Chegara omillar orasida ularning marginal ta'siri bo'yicha avtomatik taqsimlanadi.

Kichik hissalar (`|contrib| < 0.0005`) ro'yxatga kirmaydi. `contrib` faqat narx o'zgarganda to'ldiriladi.

**Misol** ([avtomatik narx, 7-bo'lim](06-avtomatik-narx.md#7-toliq-ishlangan-misol)dagi Andijon pomidori). `Current = 4 500`, `Median = 4 600`, `g = 1.03`, `pressure = +0.03`, `trend = +0.0227`, `sales = −0.02`, `lo = 4 275`, `hi = 4 725`, yaxlit narx 4 700. 5 ta o'yinchi, 32 koalitsiya. Natija:

| O'yinchi | `φ` (so'm) | `contrib` |
|---|---|---|
| `market` | +59.0 | +0.0131 |
| `goal` | +78.2 | +0.0174 |
| `pressure` | +78.6 | +0.0175 |
| `trend` | +60.4 | +0.0134 |
| `sales` | −51.3 | −0.0114 |
| `rounding` | 4 700 − 4 725 = −25 | −0.0056 |
| **Yig'indi** | +200 | **+0.0444** = 4 700 / 4 500 − 1 |

Chegarasiz bo'lganda hissalar (+0.0229, +0.0308, +0.0308, +0.0233, −0.0205) yig'indisi +0.087 bo'lardi; chegara har birini proporsional emas, marginal ta'siriga qarab kamaytirdi. `factors` dagi `cap = −0.034` esa maqsad narxga nisbatan, u boshqa narsani o'lchaydi.

**Qayerda ko'rinadi.** `price_changes.reasons` JSON'ida `contrib` (2026-09-26 dan oldingi o'zgarishlarda yo'q), `GET /v1/me/listings/{id}/price-changes` javobida `contrib` (3 xonaga yaxlitlangan), botdagi avtomatik narx xabarida sabablar `contrib` bo'yicha kattadan kichikka foiz bilan (`contribText`, `backend/internal/bot/autoprice.go`).

---

## 4. Kunlik narx anomaliyalari

**G'oya.** Kunlik narx tarixida (`price_daily`, [tahlil, 2-bo'lim](04-tahlil-metodologiyasi.md#2-kunlik-narx-tarixi-price_daily)) ba'zan bitta viloyat-kun xato bo'ladi: noto'g'ri birlik, bitta sotuvchining xatosi, spam. Bunday kun grafikni, trendni, mavsumiylikni buzadi. Tizim ikki qatlamda tekshiradi va shubhali kunni belgilaydi; aniq belgilangan kun grafik va statistikadan chiqadi, admin esa qarorni o'zgartira oladi. Bu jonli narxlarni kirishda tekshiradigan `Screen` dan ([narx metodologiyasi, 8-bo'lim](03-narx-metodologiyasi.md#8-anomaliyalarni-aniqlash-va-tekshiruv)) alohida: bu yerda allaqachon yig'ilgan kunlik medianlar tekshiriladi.

Kod: `DetectAnomalies`, `RefreshAnomalies`, `explainedBy` (`backend/internal/insight/anomaly.go`). Qator = mahsulot × savdo turi × viloyat, qiymat = kunlik median.

### 4.1 1-qatlam: `temporal` (viloyatning o'z o'tmishiga nisbatan)

Har kun `d` uchun:

1. `ref` — shu qatorning `d` dan oldingi **28 kun** (`temporalRefDays`) ichidagi kunlarining `ln(median)` qiymatlari. **10 tadan kam** (`minTemporalRef`) bo'lsa, tekshirilmaydi.
2. `z = RobustZ(ln(median_d), ref)`; `odatiy = e^{Median(ref)}`.
3. Belgilanadi, agar hammasi bajarilsa:
   - `|z| > 3.5` (`anomalyZ`);
   - `|median_d / odatiy − 1| ≥ 0.25` (`minTemporalMove`) — statistik jihatdan g'alati, lekin 25% dan kichik farq belgilanmaydi (juda barqaror qatorlarda MAD kichik bo'lib, z oson kattalashadi);
   - kun "davomli daraja" emas (4.3);
   - yaqinida voqea yoki bayram yo'q (4.3).
4. Yoziladi: `value = median_d`, `reference = odatiy`, `z`.

### 4.2 2-qatlam: `cross_section` (boshqa viloyatlarga nisbatan)

Hamma viloyatda narx birga ko'tarilsa (masalan, kurs yoki mavsum), bu xato emas. Faqat bitta viloyat sakrab, boshqalar joyida tursa, shubhali.

1. Har viloyat-kun uchun o'z oxirgi haftasiga nisbatan log o'zgarish: `c = ln(median_d) − Median(ln(oldingi 7 kun))`. Oldingi 7 kunda **kamida 4 kun** (`minCrossRef`) bo'lishi kerak.
2. Bir mahsulot, savdo turi va kun bo'yicha viloyatlar guruhlanadi. Guruhda **kamida 5 viloyat** (`minCrossProvinces`) bo'lishi kerak.
3. Har viloyat uchun `z = RobustZ(c, boshqalarning c lari)`.
4. Belgilanadi, agar `|z| > 3.5` va `|c − Median(boshqalar)| ≥ ln(1.3) ≈ 0.262` (`minCrossMove`: boshqalardan kamida 30% farq), kun davomli daraja emas va voqea bilan izohlanmasa.
5. `reference = haftalik_median × e^{Median(boshqalar)}` — viloyat boshqalar kabi harakat qilganida bo'lishi kerak bo'lgan narx.

Bir kun ikkala qatlamda ham belgilanishi mumkin (jadval kaliti `layer` ni ham o'z ichiga oladi).

### 4.3 Istisnolar

- **Davomli daraja (`lasting`).** Keyingi 4 kun ichidagi kunlardan **kamida 3 tasi** yoki oldingi 4 kun ichidagi kunlardan kamida 3 tasi shu kun narxidan **±10%** (`lastingBand`) ichida bo'lsa, bu yangi narx darajasi, xato emas. Xato yolg'iz turadi, haqiqiy o'zgarish davom etadi.
- **Voqea yoki bayram.** `market_events` jadvalidagi (bayram taqvimi ham shu yerda) voqea kunni izohlaydi, agar: voqea **14 kundan uzun bo'lmasa** (`maxExplainDays`; to'y mavsumi kabi oylab davom etadigan davr har qanday xatoni "oqlab" yubormasligi uchun), mahsulot, kategoriya va viloyat mos kelsa (bo'sh maydon = hammasiga) va kun `[boshlanish − 3; tugash + 3]` oralig'ida bo'lsa (`eventMargin = 3`).

### 4.4 Vaqtinchalik va yakuniy belgilar

`settled = (kun + 4 kun) ≤ ma'lumotdagi oxirgi kun`. Oxirgi 4 kun uchun keyingi kunlar hali yo'q, ya'ni "davomli daraja"ni tekshirib bo'lmaydi. Shuning uchun bu kunlar faqat **belgilanadi**, lekin grafikda qoladi: haqiqiy yangilik hech qachon yashirilmaydi. 4 kun o'tgach, keyingi tekshiruv uni yakuniy qiladi yoki (narx davom etgan bo'lsa) olib tashlaydi.

### 4.5 Saqlash va `price_daily_clean`

`price_anomalies` jadvali (`migrations/00033_price_anomalies.sql`): `product_id`, `trade_type`, `province_id`, `day`, `layer`, `value`, `reference`, `z`, `settled`, `status` (`open` / `confirmed` / `dismissed`), `found_at`, `decided_at`, `decided_by`.

`price_daily_clean` ko'rinishi (view) — `price_daily` dan quyidagi kunlarni chiqarib tashlaydi:

```
status = 'confirmed'  YOKI  (status = 'open' VA settled)
```

Grafiklar, trend, mavsumiylik, prognoz, viloyat matritsasi, issiqlik xaritasi, klasterlar va sotuvchi belgilari (`queries/insight.sql`, `regions.sql`, `sellers.sql`) shu ko'rinishdan o'qiydi. `dismissed` belgi kunni qaytaradi; vaqtinchalik belgi kunni chiqarmaydi.

### 4.6 Jadval va admin qarori

- `price_daily` job: **har soatda** kunlik tarixni yangilagach, shu yangi qatorlar ustida oxirgi **7 kun** (`AnomalyHourDays`) anomaliyalarini ham tekshiradi.
- `price_anomalies_year` job: **har kuni 03:30** (Toshkent) oxirgi **365 kun** (`AnomalyNightDays`).
- Har ishda davr ichidagi `open` belgilar o'chiriladi va qaytadan yoziladi; admin qaror qilgan (`confirmed`/`dismissed`) belgilar `ON CONFLICT DO NOTHING` bilan o'zgarmay qoladi. Keyin tahlil keshi tozalanadi.
- `GET /v1/admin/anomalies?status=` — oxirgi 300 ta, yangi kunlar va katta `|z|` birinchi.
- `POST /v1/admin/anomalies/decision` — `confirmed` (kun grafikdan butunlay chiqadi) yoki `dismissed` (kun qaytadi). Topilmasa `404 anomaly_not_found`, noto'g'ri sana `422 bad_date`.
- Mahsulot tahlilida `anomalies` ro'yxati (`dismissed` lar kirmaydi): `date`, `province_id`, `province_name`, `layer`, `value`, `reference`, `diff`, `excluded` (`confirmed` yoki `settled` bo'lsa `true`).

### 4.7 Misol

Samarqand, pomidor, ulgurji. Oldingi 12 kun medianlari (so'm/kg): 7 800, 8 000, 8 200, 7 900, 8 100, 8 000, 8 300, 7 700, 8 000, 8 100, 7 900, 8 200. Bugungi median — 12 000.

- `ref` = shu 12 ta qiymatning logarifmi (≥ 10 — yetarli). `Median(ref) = ln 8 000`, `MAD ≈ 0.01258`.
- `z = (ln 12 000 − ln 8 000) / (1.4826 × 0.01258) = 0.4055 / 0.01865 ≈ 21.7` → `|z| > 3.5`.
- `12 000 / 8 000 − 1 = 0.50 ≥ 0.25`.
- Keyingi kunlar 8 100, 7 900, 8 000 bo'lsa — davomli emas → `temporal` belgi, `value = 12 000`, `reference = 8 000`.
- Keyingi kunlar 11 800, 12 100, 11 900 bo'lsa — 3 kun ±10% ichida → yangi daraja, belgilanmaydi.

**`cross_section` misoli.** Shu kuni 8 viloyatning haftaga nisbatan log o'zgarishlari: boshqa yettitasi `0.02, 0.03, −0.01, 0.00, 0.04, 0.01, 0.02`, Namanganniki `0.45`. Boshqalar medianasi 0.02, MAD 0.01. `z = (0.45 − 0.02) / (1.4826 × 0.01) ≈ 29`, farq `0.43 ≥ 0.262` → belgilanadi. Namanganning haftalik medianasi 8 000 bo'lsa, `reference = 8 000 × e^0.02 ≈ 8 160`.

---

## 5. Isolation Forest: g'ayrioddiy sotuvchilar

**G'oya.** Boshqalardan keskin farq qiladigan sotuvchini tasodifiy "kesishlar" bilan tez ajratib olish mumkin: masalan, narxlarining yarmi tekshiruvga tushadigan yagona sotuvchi bitta-ikkita kesishda yolg'iz qoladi, oddiy sotuvchini ajratish uchun esa ko'p kesish kerak. Isolation Forest (Liu, Ting, Zhou, 2008) yuzlab tasodifiy daraxtlarda o'rtacha "ajratish chuqurligi"ni o'lchaydi. U belgilangan misollarsiz (yorliqsiz) ishlaydi. U yolg'iz nuqtalarni topadi; bir-biriga o'xshash katta guruh esa "naqsh" hisoblanadi, anomaliya emas.

Bu **faqat maslahat**: admin qarab chiqishi uchun ro'yxat. Hech kim avtomatik bloklanmaydi; katta ulgurji sotuvchi g'ayrioddiy, lekin halol bo'lishi mumkin.

Kod: `backend/internal/ml/iforest.go` (`NewForest`, `Score`), `backend/internal/insight/sellers.go` (`ScoreSellers`, `RefreshSellerScores`), so'rov `SellerFeatures` (`backend/queries/sellers.sql`).

### 5.1 Belgilar (oxirgi 60 kun)

Hisobga olinadi: `source` = `seller` yoki `seller_auto` bo'lgan narxlar, oxirgi 60 kun, **kamida 3 ta narxi** bor sotuvchilar. Kunlik median bilan solishtirish `price_daily_clean` dagi o'sha mahsulot, savdo turi, viloyat va Toshkent kuni bo'yicha.

| Belgi | Ma'nosi | Forestga kiradi |
|---|---|---|
| `prices` | 60 kundagi narxlar soni | `ln(1 + prices)` |
| `review_share` | `review` yoki `rejected` holatidagi narxlar ulushi | ha |
| `deviation` | `|ln(narx / kunlik_median)|` ning medianasi (0.2 ≈ 22% farq) | ha |
| `below_share` | kunlik P25 dan past narxlar ulushi | ha |
| `above_share` | kunlik P75 dan yuqori narxlar ulushi | ha |
| `per_listing` | narxlar soni / turli e'lonlar soni: narx qanchalik tez-tez o'zgartiriladi | `ln(1 + per_listing)` |
| `max_listings_day` | bir kunda yaratilgan eng ko'p e'lon | `ln(1 + max_listings_day)` |
| `products` | turli mahsulotlar soni | yo'q, faqat ko'rsatiladi |

Katta sonlar (`prices`, `per_listing`, `max_listings_day`) log bilan siqiladi, aks holda ular ulushlardan ustun kelardi.

### 5.2 Algoritm

1. Sotuvchilar **30 tadan kam** (`MinScoredSellers`) bo'lsa — hisob yo'q, eski ballar o'chiriladi (taqqoslash uchun hamroh yo'q).
2. **100 daraxt** (`forestTrees`). Har daraxt uchun qatorlar tasodifiy aralashtiriladi va birinchi `ψ = min(256, n)` tasi olinadi (`forestSample = 256`).
3. Daraxt o'stirish: tugunda hali o'zgaradigan belgilardan tasodifiy biri, uning `[min, max]` oralig'idan tasodifiy kesish nuqtasi; `< kesish` chapga, qolganlari o'ngga. To'xtash: chuqurlik `ceil(log2 ψ)` ga yetsa yoki tugunda ≤ 1 nuqta qolsa yoki hech bir belgi o'zgarmasa.
4. Yo'l uzunligi: `h(x) = barg chuqurligi + c(barg_hajmi)`, bu yerda

   ```
   c(1) = 0,  c(2) = 1
   c(m) = 2·(ln(m − 1) + 0.5772156649) − 2·(m − 1)/m      (m > 2)
   ```

   `c(m)` — `m` nuqtali ikkilik qidiruv daraxtida muvaffaqiyatsiz qidiruvning o'rtacha yo'li (barg ichida "qolgan" chuqurlik).
5. **Anomaliya balli:**

   ```
   s(x) = 2^( − E[h(x)] / c(ψ) )
   ```

   `E[h(x)]` — 100 daraxt bo'yicha o'rtacha yo'l. 1 ga yaqin — oson ajraladi (g'ayrioddiy), 0.5 atrofi va past — oddiy.
6. **Chegara:** `score ≥ 0.6` (`SuspiciousScore`) bo'lsa admin ro'yxatiga chiqadi.

### 5.3 Sabab kodlari

Har sotuvchi uchun forestdagi belgilar (`prices` dan tashqari) boshqalar bilan solishtiriladi: `RobustZ(belgi, barcha sotuvchilar ustuni) > 2.5` (`reasonZ`) bo'lsa, sabab qo'shiladi. Deyarli hamma uchun 0 bo'lgan belgida MAD = 0 bo'ladi; unda qiymat `median + 0.25` dan katta bo'lsa sabab hisoblanadi. Kodlar: `review_share`, `deviation`, `below_share`, `above_share`, `per_listing`, `max_listings_day`. Faqat **yuqori** qiymat sabab bo'ladi. Ro'yxat bo'sh bo'lsa — alohida belgi emas, belgilarning g'ayrioddiy *birikmasi*.

### 5.4 Misol

40 sotuvchi (`ψ = 40`): `c(40) ≈ 6.53`, daraxt chuqurligi chegarasi `ceil(log2 40) = 6`.

- Oddiy sotuvchining o'rtacha yo'li ≈ 6.5 → `s = 2^(−6.5/6.53) ≈ 0.50`.
- Narxlarining 45% i tekshiruvga tushgan (boshqalarda ~2%) va bir kunda 30 ta e'lon yaratgan sotuvchi o'rtacha 3 kesishda ajraladi → `s = 2^(−3/6.53) ≈ 0.73 ≥ 0.6` → ro'yxatga chiqadi, sabablari `review_share`, `max_listings_day`.
- O'rtacha yo'l 4 bo'lsa `s ≈ 0.65`, 5.3 bo'lsa `s ≈ 0.57` (chegaradan past).

### 5.5 Saqlash va ko'rsatish

- `seller_scores` (`migrations/00034_seller_scores.sql`): `seller_id`, `score`, `features` (JSONB), `reasons` (TEXT[]), `computed_at`. Har ishda jadval to'liq qayta yoziladi.
- `seller_scores` job: **har kuni 04:00** (tungi anomaliya tekshiruvidan keyin).
- `GET /v1/admin/sellers/suspicious` — `score ≥ 0.6`, kattasidan, 100 tagacha; har birida `name`, `username`, `demo`, `score`, `reasons`, `features`, `live_listings`; umumiy `scored`, `min_sellers` (30), `threshold` (0.6), `computed_at`.

---

## 6. K-Means: o'xshash viloyatlar

**G'oya.** Har viloyatning "narx profili" bor: qaysi mahsulot turi respublikaga nisbatan arzon, qaysi biri qimmat. Profili o'xshash viloyatlarni guruhlash savdogarga "qayerdan olib, qayerga sotish" yoki "qaysi viloyat biznikiga o'xshaydi" degan savolga javob beradi. K-Means har doim `k` ta guruh qaytaradi, hatto ma'lumotda guruh bo'lmasa ham; shuning uchun silhouette bahosi guruhlar haqiqiymi yoki yo'qligini aytadi va zaif bo'lsa, tizim guruh o'ylab topmaydi.

Kod: `backend/internal/ml/kmeans.go` (`KMeans`, `Silhouette`, `BestK`, `Quality`), `backend/internal/insight/clusters.go` (`profiles`, `clusterProfiles`, `byCategory`, `similarTo`).

### 6.1 Profil (belgilar vektori)

1. **Matritsa:** har mahsulot × viloyat uchun oxirgi **14 kun**dagi kunlik medianlarning narxlar soni bilan vaznlangan o'rtachasi (`price_daily_clean`, `ProductProvinceMatrix`); **5 tadan kam** narxli katak tashlanadi (issiqlik xaritasi bilan bir xil).
2. **Mahsulotlar:** viloyatlarning kamida **70%** ida (`profileCoverage`) narxi bor mahsulotlar profilga kiradi.
3. **Viloyatlar:** shu mahsulotlarning kamida **50%** ida (`provinceCoverage`) narxi bo'lmagan viloyat chiqariladi (`excluded`).
4. Har katak: `ln(viloyat_narxi / respublika)`, respublika — viloyat medianlarining narxlar soni bilan vaznlangan o'rtachasi (`countryOf`). Narxi yo'q katak 0 (= respublika darajasi).
5. **Kategoriya bo'yicha o'rtacha** (`byCategory`): K-Means'ga har kategoriyaning o'rtacha log farqi beriladi (masalan, `[sabzavot, meva, go'sht, ...]`). Yuzlab shovqinli mahsulot naqshni yashirmasligi uchun.

**Normallashtirish.** Qo'shimcha standartlashtirish yo'q: log nisbat allaqachon birliksiz va hamma ustun bir xil shkalada (`ml.Standardize` bu yerda chaqirilmaydi).

Minimal ma'lumot: **≥ 6 viloyat** (`minProfiles`) va **≥ 5 mahsulot** (`minProfileItems`), aks holda `quality = "none"` va izoh.

### 6.2 K-Means va `k` tanlash

- **k-means++ boshlanishi:** birinchi markaz tasodifiy; har keyingisi mavjud markazlardan masofa kvadratiga proporsional ehtimol bilan tanlanadi.
- **Lloyd iteratsiyasi:** har nuqta eng yaqin markazga (Evklid masofa kvadrati), markazlar — guruh o'rtachasi; o'zgarish bo'lmasa yoki **100 iteratsiya**dan keyin to'xtaydi.
- **20 marta qayta ishga tushirish** (`kmeansRestarts`), guruh ichi masofalar yig'indisi eng kichigi olinadi.
- **`k = 2..5`** (`maxClusters`), lekin `k ≤ viloyatlar − 1`; eng yuqori silhouette'li `k` qoladi (`BestK`).

### 6.3 Silhouette va 0.25 qoidasi

Har nuqta `i` uchun:

```
a(i) = i dan o'z guruhidagi boshqa nuqtalargacha o'rtacha masofa
b(i) = i dan eng yaqin boshqa guruh nuqtalarigacha o'rtacha masofa
s(i) = (b − a) / max(a, b)
silhouette = o'rtacha s(i)      (−1..1; guruhida yolg'iz nuqta 0 hisoblanadi)
```

| Silhouette | `quality` | Natija |
|---|---|---|
| ≥ 0.5 (`StrongSilhouette`) | `strong` | guruhlar aniq |
| ≥ 0.25 (`MinSilhouette`) | `weak` | guruhlar zaif, lekin ishlatsa bo'ladi |
| < 0.25 | `none` | `groups` bo'sh, izoh: "farqlar uzluksiz" |

Rousseeuw (1987) chegaralari. 0.25 dan past bo'lsa guruhlar bir-biriga shunchalik kirib ketganki, ularni nomlash noto'g'ri bo'lardi.

### 6.4 Natija

`GET /v1/analytics/clusters?trade=ulgurji|chakana`:

- `groups[]`: `provinces`, `level` (guruhdagi barcha mahsulotlar log farqlari o'rtachasi, `e^x − 1`; −0.05 = 5% arzon), `cheaper` / `dearer` — guruh eng arzon / eng qimmat sotadigan **3 tagacha** mahsulot, faqat farq **3% dan katta** (`notableGap`) bo'lsa. Guruhlar `level` bo'yicha saralanadi, eng arzoni `id = 1`.
- `k`, `silhouette`, `quality`, `products` (profildagi mahsulotlar soni), `excluded`, `note`.

**`similar` (viloyat sahifasi).** `GET /v1/regions/{id}/overview` → `similar`: profili eng yaqin **3 viloyat**. Masofa bu yerda kategoriya o'rtachasi emas, **mahsulot darajasidagi** to'liq vektor bo'yicha: `gap = e^{√(Σ(d_j)² / m)} − 1` (`m` — mahsulotlar soni), ya'ni odatdagi narx farqi; `same_group` — `clusters` bo'yicha bir guruhdami.

### 6.5 Misol

6 viloyat, ikki kategoriya bo'yicha profil `[sabzavot, meva]`:

| Viloyat | sabzavot | meva |
|---|---|---|
| A | −0.10 | −0.08 |
| B | −0.12 | −0.06 |
| C | −0.09 | −0.10 |
| D | +0.08 | +0.06 |
| E | +0.10 | +0.09 |
| F | +0.07 | +0.05 |

`k = 2`: {A, B, C} va {D, E, F}. A uchun `a ≈ 0.025`, `b ≈ 0.235`, `s(A) = (0.235 − 0.025) / 0.235 ≈ 0.89`. O'rtacha silhouette ≈ 0.86 → `strong`. 1-guruhning `level ≈ e^{−0.092} − 1 ≈ −8.8%` (arzon viloyatlar).

---

## 7. Local Moran's I: issiq va sovuq nuqtalar

**G'oya.** Issiqlik xaritasida ([tahlil, 18-bo'lim](04-tahlil-metodologiyasi.md#18-issiqlik-xaritasi)) har katak viloyat narxining respublikaga nisbati. Local Moran's I (Anselin, 1995) har viloyat uchun: "u va uning qo'shnilari o'xshashmi?" deb so'raydi. Qimmat qo'shnilar orasidagi qimmat viloyat — issiq nuqta (`hot`), arzonlar orasidagi arzon — sovuq nuqta (`cold`), arzonlar orasidagi qimmat — `high_outlier`, qimmatlar orasidagi arzon — `low_outlier`. Bu tasodif emasligini tekshirish uchun qo'shnilar tasodifiy almashtiriladi.

Kod: `backend/internal/ml/moran.go` (`LocalMoran`), `backend/internal/insight/heatmap.go` (`markSpots`, `provinceNeighbours`), `backend/internal/geo/geo.go` (`Neighbors`).

### 7.1 Qo'shnilar

Viloyat chegaralari (GeoJSON) bitta manbadan olingan, shuning uchun umumiy chegarada nuqtalar bir xil. Koordinatalar `0.01°` (~1 km) katakchalarga bo'linadi (`neighbourGrid`); ikki viloyat chegarasining kamida **2 ta nuqtasi** bir katakka tushsa, ular qo'shni. Qo'shnisi topilmagan viloyat (masalan, Toshkent viloyati ichidagi Toshkent shahri) chegarasi eng yaqin viloyatni qo'shni oladi. Hisobda faqat shu mahsulot uchun narxi bor qo'shnilar ishlatiladi; bunday qo'shnisi yo'q viloyat baholanmaydi.

### 7.2 Hisob

Mahsulot qatori uchun, narxi bor viloyatlar bo'yicha (`n` ta):

```
x_i  = ln(1 + diff_i) = ln(narx_i / respublika)
z_i  = x_i − x̄
m2   = Σ z_i² / n
lag_i = (1/|N_i|) Σ_{j∈N_i} z_j           (qator bo'yicha standartlangan vaznlar)
I_i  = z_i / m2 × lag_i
```

`N_i` — `i` ning qo'shnilari. `I_i > 0` — viloyat qo'shnilariga o'xshaydi, `I_i < 0` — farq qiladi.

**Shartli almashtirish (999 marta, `moranPermutations`).** `i` ning qiymati joyida qoladi; boshqa viloyatlardan tasodifiy `|N_i|` tasi "qo'shni" sifatida tanlanadi va `I_i` qayta hisoblanadi. Kuzatilgan `I_i` yo'nalishida undan kam bo'lmagan (≥ yoki manfiyda ≤) natijalar soni `E`:

```
p = (E + 1) / (999 + 1)
```

Eng kichik mumkin bo'lgan `p = 0.001`. Tur (`Kind`) faqat `p < 0.05` (`ml.MoranAlpha`) bo'lsa beriladi: `z > 0` va `lag > 0` → `hot`; `z < 0` va `lag < 0` → `cold`; `z > 0` (qo'shnilar arzon) → `high_outlier`; aks holda `low_outlier`.

### 7.3 Issiqlik xaritasidagi qat'iyroq qoidalar

~100 mahsulot × 14 viloyat — minglab test. `p < 0.05` da kataklarning taxminan 5% i tasodifan belgilanib qolardi. Shuning uchun `markSpots` qo'shimcha talab qiladi:

- mahsulotda **≥ 8 viloyat** narxi bo'lishi (`minSpotProvinces`);
- `p < 0.01` (`spotAlpha`), ya'ni 999 tasodifiy almashtirishdan ko'pi bilan 8 tasi kuzatilgandek chetda;
- katakning respublikadan farqi **≥ 5%** (`spotGap`): `|diff| ≥ 0.05`.

Natija katakdagi `spot` maydonida (`hot`, `cold`, `high_outlier`, `low_outlier`); belgilanmaganda maydon yo'q.

### 7.4 Misol

8 viloyat, `x = [0.10, 0.08, 0.09, 0.00, −0.02, −0.05, −0.08, −0.06]`. `x̄ = 0.0075`, `m2 ≈ 0.00462`. 1-viloyat qo'shnilari 2 va 3: `z_1 = 0.0925`, `lag_1 = (0.0725 + 0.0825)/2 = 0.0775`, `I_1 = 0.0925 / 0.00462 × 0.0775 ≈ 1.55`. Qimmat viloyat qimmat qo'shnilar orasida. Tasodifiy 2 ta "qo'shni" bunday katta `lag` ni kamdan-kam beradi; `p < 0.01` chiqsa va narx respublikadan ≥ 5% yuqori bo'lsa (`e^0.10 − 1 ≈ 10.5%`), katak `hot` bo'ladi.

---

## 8. Tizim monitoringi

**G'oya.** Platformaning o'zi to'g'ri ishlayotganini kuzatish: narxlar har manbadan odatdagidek kelyaptimi, rejalashtirilgan ishlar (job) bajarilyaptimi, kurs va ob-havo yangimi. Har muammo "signal", signallar ogohlantirishga (alert) aylanadi, jiddiylari botda adminlarga yuboriladi.

Kod: `backend/internal/monitor/monitor.go`, so'rovlar `backend/queries/monitoring.sql`, bot `backend/internal/bot/monitoring.go`, jadval `migrations/00032_monitoring_alerts.sql`.

### 8.1 Tekshiruvlar

**Manba oqimi** (`FlowSignals`). `price_observations` `created_at` bo'yicha hozirdan orqaga 24 soatlik bo'laklarga bo'linadi (`days_ago` 0..14; 0 = oxirgi 24 soat); demo sotuvchilar hisobga olinmaydi. Har manba uchun `odatiy = Median(1..14-kunlar soni)` (narx kelmagan kun 0 hisoblanadi), `oxirgi = 0-kun soni`.

| Tur (`kind`) | Shart | Daraja |
|---|---|---|
| `source_silent` | `odatiy ≥ 10` va `oxirgi = 0` | `critical` |
| `source_drop` | `odatiy ≥ 10` va `oxirgi < 0.3 × odatiy` | `high` |
| `source_spike` | `oxirgi ≥ 50` va `oxirgi > 5 × max(odatiy, 1)` (spam yoki ikki marta yuklash) | `high` |

Kuniga 10 tadan kam narx keladigan manbaning jimligi hech narsa anglatmaydi (`minFlowBaseline`).

**Tekshiruv ulushi** (`review_share`, `high`). Barcha manbalar bo'yicha: oxirgi 24 soatda **≥ 20 narx** bo'lsa, `ulush = review / jami`; `odatiy_ulush = 14 kunlik review / 14 kunlik jami`. `ulush > max(0.3, 3 × odatiy_ulush)` bo'lsa signal: parser yoki etalon narx buzilgan bo'lishi mumkin. (Faqat `status = 'review'` sanaladi, `rejected` emas.)

**Joblar** (`JobSignals`, `job_runs` dagi har jobning oxirgi ishga tushishi):

| Tur | Shart | Daraja |
|---|---|---|
| `job_stuck` | tugamagan va **2 soatdan** ko'p oldin boshlangan | `critical` |
| `job_failed` | oxirgi ish xato bilan tugagan (xato matni 200 belgigacha) | `high` |
| `job_overdue` | oxirgi boshlanishdan jobning `MaxGap` idan ko'p vaqt o'tgan | `high` |

`MaxGap` jadvaldagi har job uchun belgilangan: masalan, `price_daily` 3 soat, `auto_pricing` 18 soat, kunlik joblar (`price_anomalies_year`, `seller_scores`, `fx_daily`, `weather_daily`, `rating_prompts`, `seller_deal_digest`) 30 soat, `watch_alerts` 18 soat, `deal_prompts` 1 soat, `weekly_reminder` 8 kun. `monitoring` jobining o'zida `MaxGap` yo'q.

**Yangilik** (`FreshnessSignals`, Toshkent sanasi bo'yicha):

| Tur | Shart | Daraja |
|---|---|---|
| `fx_missing` / `weather_missing` | jadvalda hali bironta kun yo'q | `medium` |
| `fx_stale` | oxirgi kurs **4 kundan** eski (Markaziy bank dam olish kunlari e'lon qilmaydi) | `high` |
| `weather_stale` | oxirgi ob-havo **3 kundan** eski | `high` |

### 8.2 Ogohlantirish hayoti

1. **Yozish** (`UpsertAlert`): har tur + subyekt (`kind`, `subject`) uchun ko'pi bilan **bitta ochiq** ogohlantirish (qisman unique indeks). Mavjud bo'lsa `level`, `value`, `baseline`, `message`, `updated_at` yangilanadi. Daraja **og'irlashsa** (`medium` → `high` → `critical`), `notified_at` tozalanadi — bu yangi xabar.
2. **Hal qilish** (`ResolveAlertsExcept`): joriy tekshiruvda ko'rinmagan ochiq ogohlantirishlarga `resolved_at = now()`.
3. **Bildirishnoma** (`AlertsToNotify`): ochiq, hali yuborilmagan, darajasi `high` yoki `critical`. Matn (`monitor.Text`) botda yuboriladi: `role = 'admin'` foydalanuvchilar va `ADMIN_TELEGRAM_IDS` dagi id'lar. Keyin `notified_at` qo'yiladi. `medium` botga yuborilmaydi. Bot o'chiq bo'lsa, ogohlantirishlar faqat bazada va API'da.
4. **Jadval:** `monitoring` job **har soatda**.

**API:** `GET /v1/admin/monitoring` — ochiq va oxirgi 7 kunda hal qilingan ogohlantirishlar (200 tagacha; avval ochiqlar, og'irroq daraja birinchi) va har jobning oxirgi ishga tushishi (`jobs`).

### 8.3 Misol

`telegram` manbasidan oxirgi 14 kunda kunlik sonlar medianasi 120. Oxirgi 24 soatda 25 ta narx keldi: `25 < 0.3 × 120 = 36` → `source_drop`, `high`, `value = 25`, `baseline = 120`, xabar: "«telegram» manbasidan narxlar keskin kamaydi: oxirgi 24 soatda 25 ta, odatda 120 ta." Adminlar botda bir marta oladi; keyingi soatlarda holat saqlansa, faqat `value` va `updated_at` yangilanadi, qayta yuborilmaydi. Keyingi 24 soatlik oynada umuman narx kelmasa, `source_silent` (`critical`) — boshqa tur, demak yangi ogohlantirish ochiladi va yuboriladi, `source_drop` esa hal qilinadi. Oqim tiklansa, ochiq ogohlantirish keyingi tekshiruvda hal qilinadi.

---

## 9. Ixtiyoriy til modellari

Barcha model funksiyalari standart o'chiq, OpenAI-mos API orqali ishlaydi va o'chiq bo'lganda deterministik yoki qoidali yo'l javob beradi. Umumiy qoida — **model raqam bermaydi**: u matnni o'qiydi, funksiyani tanlaydi yoki tayyor natijani so'z bilan ifodalaydi; har son deterministik kod bilan tekshiriladi. Model xatosi, kechikishi yoki limiti asosiy oqimni hech qachon to'xtatmaydi. Har chaqiruv `ai_log` ga yoziladi (9.10).

9.1–9.3 kanal postlari haqida, 9.4–9.9 boshqa funksiyalar.

**Kanal postlari. G'oya.** Ulangan Telegram kanal yoki guruh postlari avval qoidalar bilan o'qiladi ([Telegram bot, 8.2](07-telegram-bot.md#82-postdan-narx-olish-channelsingest)). Ba'zi postlar erkin yozilgan va qoidalar hech narsa topmaydi. Shunda, sozlangan bo'lsa, OpenAI bilan mos chat API (Groq, Gemini, OpenAI va boshqalar) orqali til modelidan narxlarni so'raladi. Model faqat **taklif qiladi**; har taklif sotuvchining o'z qatoridek qat'iy tekshiriladi va o'ylab topilgan raqam hech qachon narx bo'lmaydi.

Kod: `backend/internal/llm/llm.go` (`New`, `Extract`, `parseItems`), `backend/internal/pricing/candidates.go` (`Verify`, `CatalogText`), `backend/internal/channels/channels.go` (`Ingest`).

### 9.1 Qachon ishlaydi

Hammasi bajarilishi kerak:

1. Bot yoqilgan (`BOT_ENABLED`, `TELEGRAM_BOT_TOKEN`) — kanal xizmati faqat botda yaratiladi.
2. `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY` uchalasi berilgan. Bittasi bo'sh bo'lsa, `llm.New` `nil` qaytaradi — **sukut bo'yicha o'chiq**.
3. Post faol, ulangan kanal yoki guruhdan.
4. Qoidalar **bironta ham** narx topmagan.
5. Matnda kamida bitta raqam bor.
6. Soatlik chegara oshmagan: `LLM_MAX_PER_HOUR` (sukut 100). Oshsa `ErrLimit`, post o'qilmay qoladi.

So'rov: `temperature = 0`, `response_format = json_object`, tizim ko'rsatmasida katalog (slug, nom, 4 tagacha boshqa nom, birliklar) va javob formati `{"items":[{"product","price","unit","trade"}]}`. Vaqt chegarasi 20 soniya, javob 1 MB gacha o'qiladi. Xato bo'lsa faqat status kodi log'ga yoziladi (javob tanasi so'rovni takrorlashi mumkin, u log'ga chiqmaydi).

### 9.2 Xavfsizlik qoidalari (`Verify`)

Har nomzod:

1. **Katalogda bor** — `product` slug katalogdagi mahsulotga aynan mos; `price > 0`.
2. **Raqam postda yozilgan** — qiymat postdan qoidalar parseri topgan sonlardan biriga teng bo'lishi shart:
   - model `4.5` qaytarsa va postda "4.5" (o'nli, "ming"siz, 1 000 dan kichik) bo'lsa → 4 500 deb o'qiladi (qoidalar ham shunday o'qiydi); 100 dan kichik butun son ham minglarda;
   - model `4500` qaytarsa va postda "4.5" yozilgan bo'lsa → 4 500 qabul qilinadi;
   - boshqa hollarda raqam postda bo'lmasa — tashlanadi.
3. **Birlik ma'lum** — bo'sh birlik = mahsulotning asosiy birligi; aks holda ma'lum birlik so'zi yoki "har biri" shakli bo'lishi, va mahsulot uchun asosiy birlikka o'tkazilishi (`toBase`) kerak. Noma'lum birlik — tashlanadi (taxmin qilinmaydi).
4. **Narx chegarasi** — asosiy birlikka o'tkazilgan narx 50 … 50 000 000 so'm.
5. **Savdo turi** — faqat aynan `ulgurji` ulgurji, qolgani `chakana`.
6. Bir mahsulot va savdo turi uchun **bitta** qator.

O'tganlar `Item` bo'ladi: izoh `NoteByModel` ("model o'qidi, raqam postda bor" mazmunida), `hints: [{code: by_model}]`. Keyin ular boshqa kanal narxlari kabi `screen` dan o'tadi (bozordan juda uzoq bo'lsa `review`). `price_observations.ai_confidence = 0.5` yoziladi (qoidalar o'qigan narxlarda bo'sh). Egasining kanali `update_listings` yoqilgan bo'lsa, model o'qigan narxlar ham e'lonlarni yangilaydi.

### 9.3 Misol

Post: "Bugun kartoshkani 4.5 dan beryapmiz, piyoz ham arzonlashdi". Faraz qilaylik, qoidalar hech narsa topmadi. Model qaytardi:

```json
{"items": [{"product": "kartoshka", "price": 4500, "unit": "kg", "trade": "chakana"},
           {"product": "piyoz", "price": 3000, "unit": "kg", "trade": "chakana"}]}
```

- `kartoshka`: postda "4.5" bor, model 4 500 yozgan → qabul, 4 500 so'm/kg.
- `piyoz`: postda 3 000 ham, "3" ham yo'q → **tashlanadi** (model o'ylab topgan).

(Slug nomlari misol uchun.)

### 9.4 Tahlilchi (AI-5): `/savol` va `POST /v1/ask`

Kod: `backend/internal/analyst`. Yoqish: `ANALYST_ENABLED=true` + `LLM_*`.

1. Model tizim ko'rsatmasi va 8 ta funksiya tavsifini oladi: `price_stats`, `price_trend`, `cheapest_offers`, `pressure_index`, `compare_regions`, `forecast`, `anomalies`, `top_demand`. Har funksiya sayt ishlatadigan servislarni chaqiradi.
2. Model funksiyalarni tanlaydi (raundda ko'pi bilan 4 ta chaqiruv, `ANALYST_MAX_TOOL_ROUNDS` = 4 raund), natijalarni oladi va javob yozadi.
3. **Raqam tekshiruvi** (`internal/numcheck`): javob matnidagi har son funksiyalar qaytargan sonlardan biri bo'lishi kerak. Aks holda — yoki model hech funksiya chaqirmay son yozgan bo'lsa — matn shablon javob bilan almashtiriladi.
4. Javob: `text`, oxirgi funksiyaning jadvali (`table`), har chaqirilgan funksiya parametrlari, sana oralig'i, kuzatuvlar soni va `demo_share` bilan (`sources`), `mode` = `model` / `rules`. 3 tadan kam narx bo'lsa "ma'lumot kam" deydi.
5. Model o'chiq, xato qilgan yoki foydalanuvchi daqiqasiga `ANALYST_PER_MINUTE` (6) dan ko'p so'ragan bo'lsa — qoidali marshrut (`Route`) xuddi shu funksiyalarni chaqiradi va shablon matn yozadi.

### 9.5 Ovozdan matn (AI-1)

Kod: `backend/internal/speech`, `backend/internal/bot/ai.go`, `POST /v1/prices/transcribe`. Yoqish: `STT_*`. Yozuv formati birinchi baytlardan (OGG, WebM, MP4/M4A, WAV, MP3), OGG uzunligi oxirgi sahifasidan aniqlanadi; `STT_MAX_SECONDS` dan uzun yozuv modelga yuborilmaydi. Olingan matn odatiy qoidali parserdan o'tadi — ovoz faqat klaviaturaning o'rnini bosadi, narx tasdiqsiz saqlanmaydi.

### 9.6 Rasmdan narx (AI-3)

Kod: `backend/internal/photoprice`, `POST /v1/prices/from-photo`. Yoqish: `VISION_MODEL`. Model rasmdagi matnni ko'chiradi (`transcript`) va narxlarni taklif qiladi. Avval qoidali parser shu matnni o'qiydi; model takliflari kanal postlaridagi `Verify` qoidalaridan o'tadi (katalogda, birlik ma'lum, raqam `transcript` da aynan yozilgan). Natija qoralama, sotuvchi tasdiqlaguncha narx emas, tasdiqdan keyin odatiy anomaliya tekshiruvi.

### 9.7 Katalogga moslash (AI-4)

Kod: `backend/internal/catalogmatch`, `GET /v1/catalog/match`. Qatlamlar: nomlar va boshqa nomlar lug'ati → trigram o'xshashligi (`pg_trgm` yoki Go'da xuddi shu o'lchov) → ixtiyoriy embeddings (`EMBEDDINGS_MODEL`, vektorlar `product_embeddings` da, katalog o'zgarsa qayta hisoblanadi). Ishonch `MATCH_MIN_CONFIDENCE` (0.6) dan past bo'lsa avtomatik moslanmaydi: sotuvchi `MATCH_MIN_CANDIDATE` (0.3) dan yuqori nomzodlardan tanlaydi.

### 9.8 Maslahat matni (AI-6)

Kod: `backend/internal/advice`, `GET /v1/products/{slug}/advice-text`. "Hozir oling / Kuting" qarori va sabablari `insight.AdviceOf` da hisoblanadi. Standart holatda matn shu sabablar ustidagi shablon; `ADVICE_TEXT_ENABLED=true` da model ularni oddiy tilda qayta yozadi, sabablarda yo'q son bo'lsa shablon qaytadi.

### 9.9 Moderatsiya (AI-7)

Kod: `backend/internal/moderation`. Kalit so'z qoidalari doim ishlaydi; `MODERATION_ENABLED=true` da model ham matnni tasniflaydi: spam, haqorat, taqiqlangan mahsulot, e'lon mahsulotiga aloqasiz matn (`review_reason` = `content_*`). Belgi e'lonni faqat tekshiruvga ushlaydi (admin hal qiladi), hech qachon rad etmaydi; model xatosi yoki `MODERATION_TIMEOUT` (4 s) dan kechikishi hech narsani ushlamaydi.

### 9.10 `ai_log` va o'lchov

Har model chaqiruvi (`internal/ailog`): funksiya, model, kirish/chiqish tokenlari, audio soniyalari, kechikish, taxminiy narx (`AI_MODEL_PRICES` bo'yicha), natija va xato kodi, foydalanuvchi, kirishning kalitli qisqa xeshi. So'rov va javob matni **saqlanmaydi**. `AI_LOG_RETENTION_DAYS` (90) kundan keyin o'chiriladi. Admin: `GET /v1/admin/ai-usage`.

O'lchov: `make eval` (`backend/cmd/eval`, to'plamlar `backend/eval/`), natija `backend/eval/RESULTS.md`. 2026-09-26 holati (qoidali yo'llar):

| Komponent | To'plam | Natija |
|---|---|---|
| Narx ajratish (AI-2) | 72 xabar, 73 narx | Narx aniq to'g'ri 97.3%, mahsulot 100%, birlik va hajm 95.9%, aniqlik 95.9% |
| Katalogga moslash (AI-4) | 60 so'z (5 tasi katalogda yo'q) | To'g'ri qaror 90%, top-3 ichida 92.7%, noto'g'ri avtomatik moslash 0 |
| Tahlilchi yo'naltirishi (AI-5) | 40 savol, 8 funksiya | 100% (belgilarni qoidalar mualliflari yozgan — optimistik baho) |

STT va rasmdan o'qish hali o'lchanmagan: yozib olingan ovozlar va narx ro'yxati rasmlari to'plami kerak.

---

## 10. Usul → qayerda ko'rinadi

| Usul | Saqlanadi | API maydoni | Sahifa / kanal |
|---|---|---|---|
| Robust statistika | — | anomaliya `z`, sabab kodlari | — (boshqa usullar ichida) |
| Prognoz musobaqasi | hisoblanadi, keshda | `GET /v1/products/{slug}/analytics` → `forecast.method`, `skill`, `methods`, `mape`, `tested`, `weeks[].median/low/high` | Mahsulot tahlili: prognoz chizig'i va oraliq, tanlangan usul va uning xatosi naive bilan solishtirib, har hafta omillari |
| Prognoz Shapley | — | `forecast.weeks[].drivers` | Tahlil sahifasi, prognoz bloki |
| Avtomatik narx Shapley | `price_changes.reasons.contrib` | `GET /v1/me/listings/{id}/price-changes` → `contrib` | Botdagi avtomatik narx xabari (sabablar foiz bilan) |
| Kunlik anomaliyalar | `price_anomalies`, `price_daily_clean` | `analytics.anomalies`; `GET /v1/admin/anomalies`, `POST /v1/admin/anomalies/decision` | Barcha grafik va statistikalar tozalangan tarixdan; tahlil grafigida ✕/? belgilari; admin paneli |
| Isolation Forest | `seller_scores` | `GET /v1/admin/sellers/suspicious` | Admin paneli |
| K-Means | keshda | `GET /v1/analytics/clusters` → `groups`, `quality`, `silhouette`; `GET /v1/regions/{id}/overview` → `similar` | `/statistika` (guruhlar, xaritada rangda), `/hudud/{id}` (o'xshash viloyatlar) |
| Local Moran's I | keshda | `GET /v1/analytics/heatmap` → `rows[].cells[].spot` | `/statistika` issiqlik xaritasi katagi |
| Tizim monitoringi | `monitoring_alerts`, `job_runs` | `GET /v1/admin/monitoring` → `alerts`, `jobs` | Botda adminlarga (`high`, `critical`); admin paneli |
| Til modeli (kanallar) | `price_observations.ai_confidence = 0.5` | narx qatorida `hints: by_model` | Kanal/guruh postlari |
| Tahlilchi | `ai_log` | `POST /v1/ask` | Bot `/savol`; saytda `/tahlilchi` |
| Ovoz, rasm | `ai_log` | `POST /v1/prices/transcribe`, `POST /v1/prices/from-photo` | Bot; saytda `/narx` tugmalari |
| Katalogga moslash | `product_embeddings` | `GET /v1/catalog/match` | Bot ("qaysi mahsulot?" tugmalari) |
| Maslahat matni | — | `GET /v1/products/{slug}/advice-text` | Sayt: tahlil sahifasi |
| Moderatsiya | `listings.review_reason = content_*` | admin navbatida `reason` | Admin tekshiruv navbati |

Rejalashtirilgan ishlar (Toshkent vaqti):

| Job | Vaqt | Nima qiladi |
|---|---|---|
| `price_daily` | har soat | kunlik tarix, keyin oxirgi 7 kun anomaliyalari |
| `price_anomalies_year` | har kuni 03:30 | oxirgi 365 kun anomaliyalari |
| `seller_scores` | har kuni 04:00 | Isolation Forest ballari |
| `monitoring` | har soat | tekshiruvlar, ogohlantirishlar, bot xabari |
| `ai_log_cleanup` | har kuni 04:45 | eski `ai_log` qatorlari |

Prognoz, klasterlar va Moran nuqtalari so'rov paytida hisoblanadi va tahlil keshida saqlanadi (kesh qoidalari [tahlil, 22-bo'lim](04-tahlil-metodologiyasi.md#22-kesh-va-yangilik)da); anomaliya jobi va admin qarori keshni tozalaydi.

---

## 11. Kod va hujjatlar orasidagi farqlar

- **[Tahlil metodologiyasi](04-tahlil-metodologiyasi.md) eskirgan joylari.** Kirish qismida "ML modeli yo'q" deyilgan va 8-bo'limda bitta prognoz modeli, "`tested` — kamida bitta gorizont hisoblangan originlar" deb yozilgan. Kodda endi uchta usul musobaqasi bor va origin faqat to'rttala gorizont hisoblangan bo'lsa sanaladi. 1-bo'limdagi "Prognoz: ≥ 20 tekshirilgan hafta" chegarasi hali ham to'g'ri, lekin endi umumiy (hamma usul uchun) haftalar sanaladi. 18-bo'limda `spot` maydoni yo'q. Grafiklar `price_daily` emas, `price_daily_clean` dan o'qishi ham u yerda aytilmagan.
- **[Avtomatik narx](06-avtomatik-narx.md)** jurnal tavsifida `reasons` faqat `median`, `peers`, `factors` deb yozilgan; kodda `contrib` ham bor.
- **`cross_section` minimal viloyatlar soni.** `minCrossProvinces = 5`, lekin har viloyat qolgan viloyatlar bilan `RobustZ` orqali solishtiriladi va u **kamida 5** ta qiymat talab qiladi. Demak amalda 2-qatlam **6 va undan ko'p** viloyatda ishlaydi.
- **`drivers` yig'indisi.** API izohida "shares add up to median/base − 1" deyilgan. Shapley qiymatlari yaxlitlanmagan prognozga nisbatan aniq qo'shiladi, lekin `median` va `base` `roundPrice` bilan, ulushlar 3 xonagacha yaxlitlanadi, shuning uchun yig'indi taxminan teng.
- **`contrib` yig'indisi.** Aniq tenglik `v(∅) = Current` bo'lganda bajariladi. `|contrib| < 0.0005` bo'lgan hissalar ro'yxatdan tushadi va API 3 xonagacha yaxlitlaydi, shu sababli ko'rsatilgan yig'indida mayda farq bo'lishi mumkin.
- **Moran alfa.** `ml.MoranAlpha = 0.05` (tur shu chegarada beriladi), issiqlik xaritasi esa `p < 0.01` talab qiladi. Ikkalasi mos, lekin `ml` paketini boshqa joyda ishlatsangiz 0.05 ishlashini unutmang. `spec.md` §10.8 da 0.01 yozilgan, bu xaritaga tegishli.
- **Isolation Forest belgilari.** `seller_auto` narxlari ham `prices` va `per_listing` ga kiradi, shuning uchun avtomatik narxni yoqqan sotuvchida `per_listing` tabiiy ravishda yuqori bo'ladi va `per_listing` sababi bilan ro'yxatga chiqishi mumkin. Sabab uchun robust z hisoblanganda solishtirish to'plamiga sotuvchining o'zi ham kiradi (≥ 30 sotuvchida ta'siri kichik).
- **`ml.Standardize`** kodda bor, lekin hech bir ishlab turgan usul uni chaqirmaydi.
- **`ai_confidence`** saqlanadi, lekin narx vaznida (medianda) hisobga olinmaydi: model o'qigan narx qoidalar o'qigan kanal narxi bilan bir xil vaznda.
- **Monitoring.** Hech qachon ishga tushmagan job `job_runs` da yo'q, shuning uchun `job_overdue` uni sezmaydi. `monitoring` jobining o'zi kuzatilmaydi. Tekshiruv ulushi faqat `review` holatini sanaydi.
- **Sayt.** Admin panelida monitoring, anomaliyalar (tasdiqlash/rad etish) va shubhali sotuvchilar bo'limlari bor; tahlil sahifasida prognoz usuli, aniqligi va `drivers`, statistika sahifasida klasterlar va `spot`, viloyat sahifasida `similar`, sotuvchi narx tarixida `contrib` ko'rsatiladi. Frontend `drivers`, `methods`, `contrib`, `anomalies`, `similar` maydonlari doim keladi deb hisoblaydi (backend ularni doim bo'sh bo'lsa ham massiv sifatida yuboradi).

---

## Qayerda joylashgan

- `backend/internal/ml/robust.go` — `Seed`, `Median`, `MAD`, `RobustZ`, `Standardize`
- `backend/internal/ml/shapley.go` — `Shapley`, `MaxShapleyPlayers`
- `backend/internal/ml/iforest.go` — `NewForest`, `Score`, `avgPath`, `forestTrees`, `forestSample`
- `backend/internal/ml/kmeans.go` — `KMeans`, `Silhouette`, `BestK`, `Quality`, `MinSilhouette`, `StrongSilhouette`
- `backend/internal/ml/moran.go` — `LocalMoran`, `moranPermutations`, `MoranAlpha`
- `backend/internal/ml/ml_test.go` — har usul uchun testlar
- `backend/internal/insight/forecast.go` — `ForecastOf`, `predict`, `parts.drivers`, `roundPrice`
- `backend/internal/insight/anomaly.go` — `DetectAnomalies`, `lasting`, `RefreshAnomalies`, `explainedBy`
- `backend/internal/insight/sellers.go` — `SellerFeatures`, `ScoreSellers`, `RefreshSellerScores`
- `backend/internal/insight/clusters.go` — `profiles`, `clusterProfiles`, `byCategory`, `similarTo`
- `backend/internal/insight/heatmap.go` — `markSpots`, `provinceNeighbours`, `spotAlpha`, `spotGap`
- `backend/internal/insight/analytics.go` — `forecast` va `anomalies` maydonlari
- `backend/internal/insight/regions.go` — `similar`, `matrix`, `countryOf`
- `backend/internal/geo/geo.go` — `Neighbors`
- `backend/internal/analytics/repricing.go` — `contributions`
- `backend/internal/bot/autoprice.go` — `contribText`
- `backend/internal/monitor/monitor.go` — `FlowSignals`, `JobSignals`, `FreshnessSignals`, `Run`, `Text`
- `backend/internal/bot/monitoring.go` — `MonitoringAlerts`
- `backend/internal/llm/llm.go`, `client.go` — `New`, `Extract`, `parseItems`; chat, tool calling, STT, vision, embeddings mijozi
- `backend/internal/analyst/`, `numcheck/`, `speech/`, `photoprice/`, `catalogmatch/`, `advice/`, `moderation/`, `ailog/` — model funksiyalari
- `backend/cmd/eval/`, `backend/eval/` — o'lchov va natijalar
- `backend/internal/pricing/candidates.go` — `Verify`, `CatalogText`, `NoteByModel`
- `backend/internal/channels/channels.go` — `Ingest`, `SetExtractor`, `modelConfidence`
- `backend/internal/httpapi/anomalies.go`, `monitoring.go`, `suspicious.go`, `insight.go`, `listings.go` — API
- `backend/queries/anomalies.sql`, `monitoring.sql`, `sellers.sql`, `regions.sql` — SQL
- `backend/migrations/00032_monitoring_alerts.sql`, `00033_price_anomalies.sql`, `00034_seller_scores.sql` — jadvallar va `price_daily_clean`
- `backend/cmd/api/main.go` — job jadvali, `LLM_*` sozlamalari
- `backend/internal/config/config.go` — `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY`, `LLM_MAX_PER_HOUR`, `ADMIN_TELEGRAM_IDS`

## ML prognoz: gradient boosting (PROGNOZ_ML.md)

**Nima.** Uchta oddiy usulga (`naive`, `last_year`, `season_trend`) to'rtinchisi qo'shildi: `ml`. Bu gradient boosting daraxtlari (`backend/internal/ml/gbm.go`, LightGBM g'oyasi Go'da): belgilar 255 ta qutiga bo'linadi, daraxtlar gistogrammada o'stiriladi, yo'qotish Huber (`Delta = 0.05`, keskin sakrashlar modelni burib yubormaydi), yo'q qiymat (NaN) har bo'linishda o'z tomonini o'rganadi, erta to'xtash.

**Belgilar** (`backend/internal/insight/mlfeatures.go`, 33 ta, haftalik, faqat o'tmishdan): tarix (1/2/4/8/13 hafta o'zgarishi, 52 hafta darajasi, tebranish, mamlakatdan farqi, qo'shnilar, kategoriya, ulgurji), mavsum (o'tgan yilning 1–4 haftasi, hafta sinus/kosinus, oy), bayram (kunlari, turi), ob-havo (sovuq, issiq, yomg'ir, minimal harorat anomaliyasi), dollar (1/4 hafta o'zgarishi, import mahsulot), talab-taklif (taklif va talab nisbati, talab darajasi). Maqsad: `log(h haftadan keyingi o'rtacha narx / joriy narx)`, h = 1..4, har h uchun alohida model.

**Kechasi** (`forecast_ml` ishi, 04:40): 3 yillik haftalar yuklanadi; oxirgi 12 hafta daraxtlar sonini tanlaydi, keyin model hamma ma'lumotga qayta o'rgatiladi va har seriyaning oxirgi haftasi prognoz qilinadi → `forecast_ml`. Keyin sinov: har h uchun model faqat bir yil oldingi kesimgacha bo'lgan maqsadlarda o'rgatiladi va o'tgan yilning har haftasini prognoz qiladi; xato `|exp(prognoz − haqiqat) − 1|` → `forecast_ml_backtest`. Model saqlanmaydi, har kecha qayta o'rganiladi.

**Sabablar.** TreeSHAP (`backend/internal/ml/treeshap.go`, Lundberg 2018) har belgining aniq Shapley qiymatini beradi; ular guruhlarga qo'shiladi (`history`, `season`, `holiday`, `weather`, `fx`, `demand_supply`), modelning kutilgan qiymati `history`ga, va `exp(change) − 1` ga teng bo'lishi uchun masshtablanadi.

**Musobaqa** (`backend/internal/insight/mlcompete.go`). ML xatolari hafta (dushanba–yakshanba) bo'yicha; boshqa usullar shu haftalarning yakshanbasida qayta hisoblanadi, va hammasi bir xil haftalarda solishtiriladi (kamida `FORECAST_MIN_BACKTESTS`). ML yutsa: `method: "ml"`, haftalar dushanbadan boshlanadi, oraliq o'tgan yilgi xatolarning 80% kvantilidan. Yutmasa: eski prognoz qoladi, `methods` ro'yxatida ML bali ko'rinadi.

**Admin.** `GET /v1/admin/forecast/model`: oxirgi o'rgatish va sinov, har hafta uchun daraxtlar, xato va eng muhim belgilar.
