# API

Bugun'ning HTTP API'si — Go'da yozilgan REST uslubidagi JSON API. Sayt ham, tashqi integratsiyalar ham shu API orqali ishlaydi; bot esa xuddi shu servislarni jarayon ichida chaqiradi. Bu hujjatda umumiy qoidalar (manzil, kirish, CORS, limitlar, xato konverti, kesh), barcha 82 ta operatsiyaning guruhlangan katalogi va xato kodlari jadvallari bor. To'liq sxema — `backend/openapi.json`.

## Mundarija

1. [Asosiy manzillar](#1-asosiy-manzillar)
2. [Kirish va rollar](#2-kirish-va-rollar)
3. [CORS](#3-cors)
4. [Limitlar](#4-limitlar)
5. [So'rov va javob shakli](#5-sorov-va-javob-shakli)
6. [Xato konverti](#6-xato-konverti)
7. [Kesh](#7-kesh)
8. [Endpointlar katalogi](#8-endpointlar-katalogi)
9. [Xato kodlari](#9-xato-kodlari)
10. [Qayerda joylashgan](#qayerda-joylashgan)

## 1. Asosiy manzillar

| Kim | Manzil |
|---|---|
| Brauzer, production | `NEXT_PUBLIC_API_URL` — saytga qo'shni subdomen; cookie `COOKIE_DOMAIN` orqali ikkalasiga ham yetadi |
| Brauzer, lokal | `/api` — Next.js rewrite `http://localhost:8080` ga uzatadi |
| Sayt serveri | `API_INTERNAL_URL` (standart `http://localhost:8080`) |
| Telegram callback | `API_URL`, bo'sh bo'lsa `WEB_URL + "/api"` |

- Barcha operatsiyalar `/v1/...` ostida, ulardan tashqari `GET /healthz` va `GET /media/{key}`.
- nginx API'ni `/api/` prefiksi bilan ham qabul qiladi, shuning uchun ikkala ko'rinish bir xil ishlaydi.
- CSV va PDF yuklab olish havolalari `publicApiUrl` bilan tuziladi, qattiq yozilgan `/api/v1/...` bilan emas.

## 2. Kirish va rollar

- **Sessiya cookie:** `bugun_session`. Bayroqlar: `HttpOnly`, `SameSite=Lax`, `Path=/`, `Domain=COOKIE_DOMAIN`, `WEB_URL` `https://` bilan boshlansa `Secure`. Muddati `SESSION_TTL` (standart `720h`, 30 kun). Bearer token yo'q.
- `openapi.json` da `security: session` bilan belgilangan operatsiyalar cookie yo'q, noma'lum yoki muddati o'tgan bo'lsa **401 `unauthorized`** qaytaradi.
- **Ixtiyoriy cookie:** `browseProducts`, `getProduct`, `getListing` cookie'siz ham ishlaydi; cookie bo'lsa faqat talab signali va ko'rishlarni to'g'ri hisoblash uchun ishlatiladi.
- **Rollar:**
  - `seller` — `role = seller` **va** `region_id` o'rnatilgan. E'lon, narx matni, RFQ'ga javob. Aks holda 403 `sellers_only`.
  - `buyer` — `role = buyer` va `region_id` o'rnatilgan. RFQ yaratish. Aks holda 403 `buyers_only`.
  - `market_admin`, `org_analyst` — admin tasdiqlashi kerak (`role_approved`). Faqat haftalik hisobot ular uchun ochiq: `org_analyst` — istalgan viloyat, `market_admin` — o'z viloyati. Aks holda 403 `reports_only`.
  - Admin — `role = admin` **yoki** Telegram id'si `ADMIN_TELEGRAM_IDS` da. `/v1/admin/*` uchun; aks holda 403 `admins_only`.
- Egalik tekshiriladigan operatsiyalar (o'z e'loni, o'z RFQ'si) begona obyekt uchun 404 qaytaradi (403 emas), shunda obyekt borligi oshkor bo'lmaydi.

## 3. CORS

- Ruxsat etilgan manbalar: `WEB_URL` va `CORS_ORIGINS` (vergul bilan).
- `credentials` ruxsat etilgan.
- Metodlar: `GET, POST, PUT, PATCH, DELETE, OPTIONS`.
- Ruxsat etilgan so'rov sarlavhasi faqat `Content-Type`; ochiq javob sarlavhasi `Location`.
- Preflight 300 soniya keshlanadi.
- CORS router'ning eng tashqi qatlamida turadi, shuning uchun 429, 500 va timeout javoblarida ham sarlavhalar bor.

## 4. Limitlar

| Limit | Qiymat | Tafsilot |
|---|---|---|
| So'rovlar soni | Daqiqasiga `RATE_LIMIT_PER_MINUTE` (120), har mijoz kaliti uchun | `/media/{key}` bundan mustasno. Oshsa **429** `rate_limited` (problem+json) |
| Mijoz kaliti | Ulanish IP'si. `X-Real-IP` / `X-Forwarded-For` faqat ulanish `TRUSTED_PROXIES` dagi proxy'dan (nginx) kelsa o'qiladi | Sayt serveri `X-Client-IP` + to'g'ri `X-Internal-Secret` yuborsa, kalit tashrif buyuruvchining IP'si bo'ladi. Sir noto'g'ri yoki yo'q bo'lsa, shu serverdagi barcha tashrif buyuruvchilar bitta limitni bo'lishadi |
| Vaqt | Handlerga `REQUEST_TIMEOUT` (15 s), butun javobga `HTTP_WRITE_TIMEOUT` (60 s) | Oshsa JSON tanasiz 504 |
| Tana hajmi | 1 MB (Huma standarti) | Oshsa **413** `body_too_large` |
| Rasm yuklash | `MEDIA_MAX_UPLOAD_BYTES` (10 MB) + 64 KB tana; rasm 40 megapikselgacha | JPEG, PNG, WebP; e'longa `MAX_PHOTOS_PER_LISTING` (5) tagacha |
| Model funksiyalari | Tahlilchi: foydalanuvchi/IP uchun daqiqasiga `ANALYST_PER_MINUTE` (6) model javobi, oshsa qoidali javob; STT va rasm: soatlik chegara, oshsa 429 `ai_limit` | [sozlamalar, 9-bo'lim](13-sozlamalar.md#9-backend-ixtiyoriy-model-funksiyalari) |
| nginx | `client_max_body_size 11m`, `proxy_read_timeout 30s`; IP bo'yicha 10 so'rov/s (burst 40), sayt serveri uchun 100 so'rov/s, rasmlar 50 so'rov/s | `deploy/nginx/` |

Biznes limitlari (RFQ, avtomatik narx, rasmlar, kuzatuvlar, taqqoslash, admin ro'yxatlari) `GET /v1/meta/limits` da; sayt ularni o'zida takrorlamaydi.

## 5. So'rov va javob shakli

- **Ortiqcha maydonlar rad etiladi.** Har bir so'rov tanasi sxemasida `additionalProperties: false`. Noma'lum JSON kaliti 422 `validation_failed` va maydon kodi `unknown_field` beradi. Masalan, `MyListing` obyektini butunligicha `PUT /v1/listings/{id}` ga qaytarib yuborib bo'lmaydi. `readOnly` maydonlar (masalan `pricing.paused_until`) xatosiz qabul qilinadi.
- **Bo'sh massivlar** har doim `[]`, hech qachon `null` emas (`huma.DefaultArrayNullable = false`).
- **Yo'q qiymatlar odatda tashlab ketiladi** (`omitempty`), `null` yuborilmaydi. Istisnolar: `Changes.d1/d7/d30/y1` (doim bor, `null` bo'lishi mumkin) va `HeatRow.cells[]` elementlari (viloyatda 5 tadan kam narx bo'lsa `null`).
- **Pul** — butun so'm, mahsulotning asosiy birligi uchun. Narx chegaralari hamma joyda 50 – 50 000 000; miqdor va zaxira 10 000 000 gacha.
- **Foizlar ikki xil:** `diff_percent` — butun foiz (12 = 12%); tahlildagi `changes`, `trend.weekly`, `share`, `contact_rate`, `demo_share` va shu kabilar — ulush (0.12 = 12%).
- **Vaqt** — RFC 3339 (UTC). **Sanalar** (`YYYY-MM-DD`) — Toshkent kalendari bo'yicha.
- **Hudud parametri** (`region`) tuman ham, viloyat ham bo'lishi mumkin; 0 — butun mamlakat. Tahlil, kuzatuv, top-talab, taqqoslash, voqealar va hisobot tumanni uning viloyatiga aylantiradi; `getProduct` va `getListing` tumandan boshlab, narx yetmasa viloyatga, keyin mamlakatga kengayadi (`price_scope`).

## 6. Xato konverti

Barcha xatolar `Content-Type: application/problem+json` bilan qaytadi:

```json
{
  "status": 422,
  "code": "validation_failed",
  "title": "Unprocessable Entity",
  "detail": "Ma'lumotlarni tekshiring.",
  "errors": [
    {
      "location": "body.price_max",
      "code": "range_too_wide",
      "params": { "max_ratio": 3 },
      "message": "Eng yuqori narx eng pastdan 3 barobardan oshmasin.",
      "value": 400000
    }
  ]
}
```

| Maydon | Ma'nosi |
|---|---|
| `status` | HTTP holati |
| `code` | Barqaror mashina kodi (9.1-jadval). Tarjima shu bo'yicha qilinadi |
| `detail` | O'zbekcha zaxira matn |
| `errors[].location` | `body.<maydon>`, `query.<param>`, `path.<param>` yoki `cookie.<nom>` |
| `errors[].code` | Maydon kodi (9.2-jadval) |
| `errors[].params` | Matnga qo'yiladigan qiymatlar, masalan `{max}`, `{max_ratio}`, `{stock}` |
| `errors[].message` | O'zbekcha zaxira matn |

Qoidalar:

- Mijoz matnni `code`, `errors[].code` va `errors[].params` dan tarjima qiladi; `detail` va `message` — faqat zaxira.
- Servisdagi tekshiruvlar bir vaqtda bitta maydon xatosini qaytaradi; Huma sxema tekshiruvi esa bir nechtasini birga qaytarishi mumkin.
- Ba'zi joylarda aniq kod o'rniga umumiy kod keladi: `productPressure` (noma'lum mahsulot — 404 `not_found`, noma'lum hudud — 422 `validation_failed` `errors[]` siz), `provinceMap` (noma'lum mahsulot — 404 `not_found`), `weeklyReport` (noma'lum hudud — 422 `validation_failed` `errors[]` siz).
- `/media/{key}` dagi 404 — oddiy matn, JSON emas.

## 7. Kesh

**HTTP sarlavhalari** (qolgan endpointlar `Cache-Control` yubormaydi):

| Endpoint | `Cache-Control` |
|---|---|
| `getCatalog` | `public, max-age=300` |
| `metaLimits` | `public, max-age=300` |
| `listListings` | `public, max-age=30` |
| `provinceMap` | `public, max-age=300` |
| `productAnalyticsCsv` | `private, max-age=600` |
| `weeklyReport` | `private, no-store` |
| `/media/{key}` | faol e'lon: `public, max-age=31536000, immutable`; boshqasi: `private, no-store` |

**Server xotirasidagi kesh — 10 daqiqa** (`insight.Service`, `cacheFor = 10 * time.Minute`):

- `productAnalytics` va CSV — (mahsulot, viloyat, savdo turi, davr, transport) bo'yicha;
- `priceHeatmap` — (savdo turi, toifa) bo'yicha;
- `topDemand` — viloyat bo'yicha;
- `regionOverview` — (viloyat, savdo turi) bo'yicha.

`compareProducts` keshlanmaydi. Tahlil qatorlari `price_daily` dan o'qiladi, u esa har soatda qayta quriladi. Demak yangi narx tahlil sahifasida 1 soat + 10 daqiqagacha kechikib ko'rinishi mumkin.

Sayt ham katalogni o'zida 1 soat keshlaydi (`unstable_cache`).

## 8. Endpointlar katalogi

"Kirish" ustuni: **ochiq** — cookie kerak emas; **sessiya** — cookie shart; **sessiya (egasi)** — faqat obyekt egasi, boshqalarga 404. Har bir endpoint bundan tashqari `rate_limited` (429), `internal` (500), `bad_request` (400) va `validation_failed` (422) qaytarishi mumkin.

### 8.1 Tizim va kirish

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| GET `/healthz` | `health` | ochiq | `{status, database: "ok" \| "unavailable"}`; baza javob bermasa **503** (healthcheck'lar shu kodga qaraydi) |
| GET `/v1/meta/limits` | `metaLimits` | ochiq | Biznes limitlari: RFQ, avtomatik narx, rasmlar, kuzatuvlar, taqqoslash chiziqlari, admin ro'yxatlari |
| GET `/v1/ai/features` | `aiFeatures` | ochiq | Qaysi model funksiyalari yoqilgan (`voice`, `voice_max_seconds`, `photo`, `analyst_model`, `embeddings`, `moderation_model`, `advice_model`) |
| GET `/v1/auth/telegram/start` | `telegramLoginStart` | ochiq | 302 Telegram kirishiga. `return_to` ≤200 belgi, faqat lokal yo'l |
| GET `/v1/auth/telegram/callback` | `telegramLoginCallback` | ochiq | Telegram qaytaradigan joy; cookie qo'yadi va saytga 302. Xato bo'lsa `/kirish?xato=bekor\|muddat\|telegram\|sozlanmagan` |
| POST `/v1/auth/telegram/webapp` | `telegramWebAppLogin` | ochiq | Mini App ichida kirish: `init_data` (Telegram.WebApp.initData) imzosi bot tokeni bilan va yoshi (`TELEGRAM_WEBAPP_MAX_AGE`) tekshiriladi, sessiya cookie'si qo'yiladi. Javob: `next`, `needs_onboarding`. Xato: 401 `invalid_init_data`, 503 `telegram_not_configured` |
| POST `/v1/auth/logout` | `logout` | cookie ixtiyoriy | Sessiyani o'chiradi, 204 |

### 8.2 Katalog va profil

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| GET `/v1/catalog` | `getCatalog` | ochiq | Hududlar, bozorlar, mahsulotlar, toifalar |
| GET `/v1/catalog/match` | `matchProduct` | ochiq | Erkin so'zni (`q`, ≤60 belgi, istalgan yozuvda: "pamidor", "помидор") katalogga moslash: tanlangan mahsulot (ishonch ≥ `MATCH_MIN_CONFIDENCE`) yoki nomzodlar |
| GET `/v1/me` | `getMe` | sessiya | Joriy foydalanuvchi profili |
| PUT `/v1/me/profile` | `updateProfile` | sessiya | Rol, tuman, bozor, toifalar, ochiq ism, rozilik (onboarding va sozlamalar) |
| PUT `/v1/me/bot` | `setBotEnabled` | sessiya | Botni ulash yoki uzish |
| PUT `/v1/me/contact` | `setContactPrefs` | sessiya | Xaridorlar qanday bog'lanadi: Telegram va/yoki telefon |
| DELETE `/v1/me` | `deleteMe` | sessiya | Akkaunt va shaxsiy ma'lumotlarni o'chirish, 204 |

### 8.3 Matndan narx (qoralamalar)

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| POST `/v1/prices/drafts` | `createPriceDraft` | sessiya + seller | Erkin matnni tahlil qilib qoralama yaratadi (1 soat yashaydi). Hech narsa topilmasa `id = 0` |
| POST `/v1/prices/drafts/{id}/confirm` | `confirmPriceDraft` | sessiya + seller | Qoralamadagi narxlarni e'lon sifatida saqlaydi (bir martalik) |
| DELETE `/v1/prices/drafts/{id}` | `discardPriceDraft` | sessiya | Qoralamani bekor qilish, 204 |
| POST `/v1/prices/transcribe` | `transcribePrice` | sessiya | Ovozli yozuvdan matn (multipart `file`: OGG, WebM, MP4/M4A, WAV, MP3; `voice_max_seconds` gacha). Matnni `/v1/prices/drafts` ga yuborish kerak. STT sozlanmagan bo'lsa 503 `feature_off` |
| POST `/v1/prices/from-photo` | `priceDraftFromPhoto` | sessiya + seller | Narxlar ro'yxati rasmidan qoralama (multipart `file`: JPEG, PNG, WebP) va rasmdan o'qilgan matn (`transcript`). Qoralama odatdagidek tasdiqlanadi; `id = 0` — hech narsa o'qilmadi |

### 8.4 Sotuvchi e'lonlari

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| POST `/v1/listings` | `createListing` | sessiya + seller | E'lon yaratish yoki shu (mahsulot, nav, savdo turi) uchun mavjudini yangilash |
| GET `/v1/me/listings` | `listMyListings` | sessiya | Mening e'lonlarim: median, o'rin, tekshiruv sababi, admin izohi, 7 kunlik voronka bilan |
| GET `/v1/me/listings/{id}` | `getMyListing` | sessiya (egasi) | Bitta e'lon rasmlari bilan |
| PUT `/v1/listings/{id}` | `updateListing` | sessiya + seller (egasi) | Tahrirlash; muddatni 7 kunga uzaytiradi |
| POST `/v1/listings/{id}/refresh` | `refreshListing` | sessiya (egasi) | Narxni yana bir haftaga saqlash |
| POST `/v1/me/listings/refresh` | `refreshDueListings` | sessiya | Muddati o'tgan yoki 48 soat ichida tugaydigan barcha faol e'lonlarni uzaytirish |
| PUT `/v1/listings/{id}/pricing` | `setListingPricing` | sessiya + seller (egasi) | Avtomatik narxni min–max oralig'ida yoqish yoki o'chirish |
| GET `/v1/me/listings/{id}/price-changes` | `listPriceChanges` | sessiya (egasi) | Avtomatik narx o'zgarishlari va sabablari, ko'pi bilan 30 ta |
| POST `/v1/me/price-changes/{id}/revert` | `revertPriceChange` | sessiya (egasi) | Avtomatik o'zgarishni qaytarish; avtomatika 24 soat to'xtaydi |
| POST `/v1/listings/{id}/sold` | `markListingSold` | sessiya (egasi) | Sotuvni yozish, zaxirani kamaytirish; tugasa e'lon yashiriladi |
| GET `/v1/me/listings/{id}/stats` | `listingStats` | sessiya (egasi) | Kunlik ko'rishlar, kontaktlar, bitimlar (7 yoki 30 kun) va viloyat o'rtachasi |
| PUT `/v1/listings/{id}/visibility` | `setListingVisibility` | sessiya (egasi) | Yashirish yoki ko'rsatish, 204 |
| DELETE `/v1/listings/{id}` | `deleteListing` | sessiya (egasi) | E'lonni rasmlari bilan o'chirish, 204 |
| POST `/v1/listings/{id}/photos` | `uploadListingPhoto` | sessiya (egasi) | Rasm yuklash (multipart `file`) |
| DELETE `/v1/listings/{id}/photos/{photo_id}` | `deleteListingPhoto` | sessiya (egasi) | Rasmni o'chirish, 204 |

### 8.5 Ochiq ko'rish: katalog, lenta, e'lon sahifasi

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| GET `/v1/products` | `browseProducts` | ochiq (cookie ixtiyoriy) | Hudud bo'yicha median narxli mahsulotlar ro'yxati; `q` bo'lsa "search" talab signali |
| GET `/v1/products/{slug}` | `getProduct` | ochiq (cookie ixtiyoriy) | Mahsulot sahifasi: bozor narxi, takliflar (≤100), viloyat medianlari, qiziqish; "view" signali |
| GET `/v1/listings` | `listListings` | ochiq | Filtrli e'lonlar lentasi (sahifalash: `page` ≤500, `per_page` ≤60) |
| GET `/v1/listings/{id}` | `getListing` | ochiq (cookie ixtiyoriy) | E'lon sahifasi: sotuvchi, reyting, bozor narxi, o'xshash e'lonlar |
| GET `/v1/sellers/{id}` | `getSeller` | ochiq | Sotuvchi sahifasi: nomi, bozori, reyting, faol e'lonlari (100 tagacha); telefon va Telegram ko'rsatilmaydi |
| POST `/v1/listings/{id}/contact` | `revealContact` | sessiya | Sotuvchi kontaktini ochish; qiziqish yoziladi va ~24 soatdan keyin botda savol rejalashtiriladi |
| GET `/v1/products/{slug}/pressure` | `productPressure` | ochiq | Talab–taklif bosim indeksi |
| GET `/v1/map/provinces` | `provinceMap` | ochiq | Viloyat chegaralari (GeoJSON), mahsulot bo'yicha median va bosim |
| GET `/v1/products/{slug}/references` | `productReferences` | ochiq | Rasmiy etalon narxlar (UzEx…): har manbaning oxirgi kotirovkasi, asosiy birlikka o'tkazilgan, manba nomi, birligi va sanasi bilan. Medianga kirmaydi — alohida chiziq sifatida ko'rsatiladi. Konnektor yoqilmaguncha bo'sh |

### 8.6 Tahlil

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| GET `/v1/products/{slug}/analytics` | `productAnalytics` | ochiq | Mahsulot tahlili: tarix, o'zgarishlar, rekordlar, trend, mavsum, prognoz, maslahat, ma'lumot sifati |
| GET `/v1/products/{slug}/analytics.csv` | `productAnalyticsCSV` | ochiq | Narx qatorini CSV (BOM bilan, Excel uchun) |
| GET `/v1/regions/{id}/overview` | `regionOverview` | ochiq | Viloyat sharhi: pasport, mahsulotlar, savat indeksi, tanqislik va ortiqchalik, imkoniyatlar |
| GET `/v1/analytics/heatmap` | `priceHeatmap` | ochiq | Mahsulot × viloyat issiqlik xaritasi (14 kun) va haftaning eng katta o'zgarishlari |
| GET `/v1/analytics/top-demand` | `topDemand` | ochiq | Eng ko'p izlanayotgan va talabi o'sayotgan mahsulotlar |
| GET `/v1/analytics/compare` | `compareProducts` | ochiq | Bitta grafikda 1–4 ta chiziq (mahsulotlar × hududlar) |
| GET `/v1/analytics/clusters` | `provinceClusters` | ochiq | Narx profili bo'yicha viloyat guruhlari (K-Means, `k` silhouette bilan) yoki aniq guruh yo'qligi haqida izoh |
| GET `/v1/products/{slug}/advice-text` | `productAdviceText` | ochiq | "Hozir oling / Kuting" maslahati oddiy tilda (`region`, `trade`, `lang`): shablon yoki `ADVICE_TEXT_ENABLED` da model; raqamlar maslahat sabablaridan |
| POST `/v1/ask` | `askAnalyst` | ochiq (cookie ixtiyoriy) | Narx savoliga javob (`question` ≤500, `lang`): matn, uning ortidagi jadval va manbalar (funksiya, parametrlar, sanalar, demo ulushi), `mode` = `model` yoki `rules`. Barcha raqamlar deterministik funksiyalardan; 3 tadan kam narx bo'lsa "ma'lumot kam" deydi |

Tahlil javoblaridagi `demo_share` (va `history_demo_share`) — hisobdagi demo ma'lumot ulushi (0–1); 0 dan katta bo'lsa sayt "Demo ma'lumot" belgisini ko'rsatadi.

Hisob-kitoblar: [tahlil metodologiyasi](04-tahlil-metodologiyasi.md).

### 8.7 So'rovlar (RFQ)

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| POST `/v1/rfqs` | `createRFQ` | sessiya + buyer | Viloyatdagi sotuvchilardan narx so'rash (bir vaqtda ≤3 ochiq) |
| GET `/v1/me/rfqs` | `listMyRFQs` | sessiya | Mening so'rovlarim va takliflar soni (≤50) |
| GET `/v1/rfqs/{id}` | `getRFQ` | sessiya (egasi) | So'rov, takliflar, bozor medianasi va tejash |
| POST `/v1/rfqs/{id}/cancel` | `cancelRFQ` | sessiya (egasi) | Ochiq so'rovni yopish, 204 |
| GET `/v1/me/rfq-inbox` | `rfqInbox` | sessiya + seller | Men javob bera oladigan so'rovlar (xaridor kimligi ko'rsatilmaydi) |
| POST `/v1/rfqs/{id}/offer` | `offerRFQ` | sessiya + seller | Narx taklif qilish yoki "Menda yo'q" |
| POST `/v1/offers/{id}/choose` | `chooseOffer` | sessiya (so'rov egasi) | Xaridor taklifni tanlaydi |
| POST `/v1/offers/{id}/answer` | `answerOffer` | sessiya (taklif egasi) | Sotuvchi rozi bo'ladi (bitim) yoki rad etadi |
| GET `/v1/offers/{id}/contact` | `offerContact` | sessiya (ikki tomon) | Ikkalasi rozi bo'lgandan keyin qarshi tomon kontakti |

### 8.8 Kuzatuvlar

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| POST `/v1/watches` | `addWatch` | sessiya | Mahsulotni kuzatish (≤20); qayta qo'shish yangilaydi |
| GET `/v1/me/watches` | `listWatches` | sessiya | Kuzatayotgan mahsulotlarim |
| DELETE `/v1/watches/{id}` | `deleteWatch` | sessiya | Kuzatishni to'xtatish, 204 |

### 8.9 Hisobotlar

| Metod va yo'l | operationId | Kirish | Vazifasi |
|---|---|---|---|
| GET `/v1/reports/weekly` | `weeklyReport` | sessiya + admin / tasdiqlangan org_analyst / tasdiqlangan market_admin (o'z viloyati) | Haftalik viloyat narx hisoboti, PDF |

### 8.10 Admin

Barchasi: sessiya + admin (`role = admin` yoki `ADMIN_TELEGRAM_IDS`).

| Metod va yo'l | operationId | Vazifasi |
|---|---|---|
| GET `/v1/admin/summary` | `adminSummary` | Boshqaruv paneli sonlari |
| GET `/v1/admin/listings/review` | `adminReviewQueue` | Tekshiruvdagi e'lonlar, eskisi birinchi |
| POST `/v1/admin/listings/{id}/decision` | `adminDecideListing` | Tasdiqlash yoki rad etish (izoh ≤300, sotuvchiga ko'rinadi) |
| GET `/v1/admin/role-requests` | `adminRoleRequests` | Tasdiq kutayotgan `market_admin` va `org_analyst` |
| POST `/v1/admin/role-requests/{id}/decision` | `adminDecideRole` | Rolni tasdiqlash; rad etilsa foydalanuvchi tasdiqlangan `buyer` bo'ladi |
| GET `/v1/admin/deals` | `adminDeals` | Oxirgi bitimlar (≤100), holat bo'yicha filtr |
| GET `/v1/admin/ratings/held` | `adminHeldRatings` | Tekshiruv kutayotgan past baholi izohlar |
| POST `/v1/admin/ratings/{id}/decision` | `adminDecideRating` | Izohni ko'rsatish yoki yashirish (yulduzlar baribir hisoblanadi) |
| GET `/v1/admin/events` | `adminListEvents` | Bozor voqealari (≤500) |
| POST `/v1/admin/events` | `adminCreateEvent` | Voqea qo'shish (sovuq, eksport qoidasi, bayram…) |
| PUT `/v1/admin/events/{id}` | `adminUpdateEvent` | Voqeani o'zgartirish |
| DELETE `/v1/admin/events/{id}` | `adminDeleteEvent` | Voqeani o'chirish, 204 |
| GET `/v1/admin/anomalies` | `adminListAnomalies` | Kunlik narx tarixidagi shubhali viloyat-kunlar, yangisi birinchi (≤300) |
| POST `/v1/admin/anomalies/decision` | `adminDecideAnomaly` | Belgini tasdiqlash (kun grafiklardan chiqib turadi) yoki rad etish (kun qaytadi) |
| GET `/v1/admin/monitoring` | `adminMonitoring` | Ochiq ogohlantirishlar, oxirgi 7 kunda hal qilinganlar, har ishning oxirgi bajarilishi |
| GET `/v1/admin/sellers/suspicious` | `adminSuspiciousSellers` | Boshqalardan farq qiladigan sotuvchilar (Isolation Forest, 60 kun) — tekshirish uchun ishora, hukm emas |
| GET `/v1/admin/ai-usage` | `adminAIUsage` | Model chaqiruvlari kun, funksiya va model bo'yicha: tokenlar, audio, kechikish, narx, xatolar (`days` 1–365, standart 30) |

### 8.11 OpenAPI'dan tashqarida

- `GET /media/{key}` — e'lon rasmi (JPEG). Kalit `[A-Za-z0-9_-]{20,64}.jpg`; kichik nusxa `{kalit}_t.jpg` (480 px), bo'lmasa to'liq rasm beriladi. Rate limitga kirmaydi. Faol e'lon rasmi hammaga (S3 ochiq manzili sozlangan bo'lsa 302 bilan bucket'ga); tekshiruvdagi, yashirilgan yoki rad etilgan e'lon rasmi faqat egasi va admin'ga, boshqalarga 404 (oddiy matn).
- Quyidagilarning HTTP API'si yo'q, ular faqat botda: Telegram kanal va guruhlarni ulash, bitim haqidagi savollarga javob, baho berish, narx o'zgarishi sababi. `/savol` ning API ko'rinishi — `POST /v1/ask`. Qarang: [Telegram bot](07-telegram-bot.md).

## 9. Xato kodlari

### 9.1 Yuqori darajadagi `code`

| code | HTTP | Qachon |
|---|---|---|
| `bad_request` | 400 | JSON noto'g'ri tuzilgan |
| `unauthorized` | 401 | Sessiya yo'q yoki muddati o'tgan |
| `forbidden` | 403 | Umumiy (hozir handlerlar ishlatmaydi) |
| `not_found` | 404 | Marshrut topilmadi; `pressure` va `map` da noma'lum mahsulot |
| `method_not_allowed` | 405 | Metod qo'llab-quvvatlanmaydi |
| `body_too_large` | 413 | Tana limitdan katta |
| `unsupported_media_type` | 415 | Noto'g'ri `Content-Type` |
| `validation_failed` | 422 | Sxema yoki maydon xatolari (9.2) |
| `rate_limited` | 429 | Daqiqasiga 120 so'rovdan oshdi |
| `internal` | 500 (va boshqa 5xx) | Server xatosi |
| `error` | boshqa 4xx | Moslanmagan 4xx uchun zaxira |
| `sellers_only` | 403 | Narx matni, e'lon, RFQ javobi — faqat sotuvchi |
| `buyers_only` | 403 | RFQ yaratish — faqat xaridor |
| `admins_only` | 403 | `/v1/admin/*` |
| `reports_only` | 403 | Hisobotga ruxsat yo'q |
| `rfq_not_eligible` | 403 | So'rov viloyatida shu mahsulotdan faol e'lon yo'q |
| `listing_not_found` | 404 | E'lon yo'q, sizniki emas yoki (ochiq sahifada) muddati o'tgan |
| `product_not_found` | 404 | `getProduct`, tahlil, CSV |
| `seller_not_found` | 404 | `getSeller`: foydalanuvchi yo'q yoki sotuvchi emas |
| `region_not_found` | 404 (overview) / 422 (`getProduct`, `getListing`) | Noma'lum hudud |
| `contact_hidden` | 404 | Qarshi tomon bog'lanish usulini ochmagan |
| `review_not_found` | 404 | E'lon tekshiruvda emas (boshqa admin hal qilgan) |
| `role_request_not_found` | 404 | Rol so'rovi allaqachon hal qilingan |
| `rating_not_found` | 404 | Baho tekshiruvda emas |
| `event_not_found` | 404 | Voqea topilmadi |
| `watch_not_found` | 404 | Kuzatuv topilmadi |
| `rfq_not_found` | 404 | So'rov yoki taklif topilmadi (yoki sizniki emas) |
| `draft_stale` | 409 | Qoralama eskirgan, saqlangan yoki sizniki emas |
| `listing_exists` | 409 | Shu mahsulot, nav va savdo turi uchun e'loningiz bor |
| `price_changed_since` | 409 | Narx keyin yana o'zgargan, qaytarib bo'lmaydi |
| `already_reverted` | 409 | O'zgarish allaqachon qaytarilgan |
| `rfq_limit` | 409 | 3 tadan ortiq ochiq so'rov |
| `rfq_closed` | 409 | So'rov yopilgan yoki bekor qilingan |
| `offer_locked` | 409 | Taklif tanlangan, endi o'zgartirib bo'lmaydi |
| `offer_state` | 409 | Taklifning hozirgi holatida bu amal mumkin emas |
| `too_many_watches` | 409 | 20 tadan ortiq kuzatuv |
| `photo_too_large` | 413 | Rasm 10 MB yoki 40 MP dan katta |
| `photo_unsupported` | 415 | JPEG, PNG yoki WebP emas |
| `photo_limit` | 422 | E'longa 5 tadan ortiq rasm |
| `unknown_product` | 422 | Mahsulot katalogda yo'q |
| `unknown_region` | 422 | Hudud topilmadi (kuzatuv, tahlil, top-talab, taqqoslash) |
| `too_many_lines` | 422 | Taqqoslashda 0 yoki 4 tadan ortiq chiziq |
| `region_required` | 422 | Hisobot uchun viloyat tanlanmagan |
| `price_text_invalid` | 422 | Narx matni bo'sh yoki 2000 belgidan uzun |
| `contact_method_required` | 422 | Kamida bitta bog'lanish usuli qolishi kerak |
| `telegram_username_missing` | 422 | Telegram'da username yo'q |
| `phone_missing` | 422 | Telefon raqami saqlanmagan |
| `invalid_init_data` | 401 | Mini App `initData` imzosi noto'g'ri yoki eskirgan |
| `telegram_not_configured` | 503 | Bot tokeni yo'q, Mini App orqali kirib bo'lmaydi |
| `feature_off` | 503 | So'ralgan model funksiyasi (STT, rasm, tahlilchi) yoqilmagan |
| `ai_limit` | 429 | STT yoki rasmdan o'qishning soatlik chegarasi |
| `stt_failed` | 502 | Ovozni matnga o'girib bo'lmadi |
| `audio_too_long` | 413 | Yozuv `voice_max_seconds` dan uzun |
| `audio_unsupported` | 415 | Ovoz formati o'qilmadi |
| `photo_unreadable` | 422 | Rasmdan narxlarni o'qib bo'lmadi |
| `question_invalid` | 422 | Savol bo'sh yoki 500 belgidan uzun |

Faqat mijozda: `network` (tarmoq xatosi, `frontend/src/lib/errors.ts`).

### 9.2 Maydon kodlari (`validation_failed` ichida, `errors[].code`)

**Huma sxema kodlari** (`body.*`, `query.*`, `path.*`):

| code | params | Ma'nosi |
|---|---|---|
| `required` | – | To'ldirilishi shart |
| `too_long` | `{max}` | Ko'pi bilan `max` belgi |
| `too_short` | `{min}` | Kamida `min` belgi |
| `too_small` | `{min}` | `min` dan kichik bo'lmasin. Diqqat: "> 0" cheklovlarida ham `{min: 0}` keladi |
| `too_large` | `{max}` | `max` dan katta bo'lmasin |
| `too_many` | `{max}` | Ko'pi bilan `max` ta element |
| `invalid_choice` | – | Ro'yxatdagi qiymatlardan biri emas |
| `invalid_type` | – | Qiymat turi noto'g'ri |
| `unknown_field` | – | Noma'lum maydon |
| `invalid` | – | Format noto'g'ri (sana, uuid, takroriy element va h.k.) |

**Handler kodlari:**

| code | params | Maydonlar | Ma'nosi |
|---|---|---|---|
| `consent_required` | – | `consent` | Shaxsiy ma'lumotlarga rozilik kerak |
| `unknown_category` | `{value}` | `categories` | Noma'lum toifa |
| `market_required` | – | `market_id` | `market_admin` uchun bozor shart |
| `categories_required` | – | `categories` | Sotuvchi kamida bitta toifa tanlaydi |
| `region_not_found` | – | `region_id` | Hudud topilmadi |
| `district_required` | – | `region_id` | Viloyat emas, tuman yoki shahar tanlang |
| `market_not_found` | – | `market_id` | Bozor topilmadi |
| `market_outside_region` | – | `market_id` | Bozor tanlangan hududda emas |
| `invalid_price` | – | `price_som`, `price_min`, `price_max`, `max_price_som` | Narx noto'g'ri |
| `invalid_amount` | – | `min_order`, `stock`, `quantity` | Miqdor noto'g'ri (>0 va ≤10 000 000) |
| `below_min` | – | `price_max` | Eng yuqori narx eng pastdan kam |
| `range_too_wide` | `{max_ratio: 3}` | `price_max` | Oraliq 3 barobardan keng |
| `price_outside_range` | `{price}` | `price_min` | Hozirgi narx oraliqdan tashqarida |
| `more_than_stock` | `{stock}` | `quantity` | Zaxiradagidan ko'p sotib bo'lmaydi |
| `out_of_range` | `{min: 1, max: 7}` | `days` | RFQ muddati 1–7 kun |
| `unknown_product` | – | `product_id` | Mahsulot katalogda yo'q |
| `invalid_date` | – | `starts_on`, `ends_on` | Sana `YYYY-MM-DD` emas (odatda avval Huma'ning `invalid` kodi chiqadi) |
| `before_start` | – | `ends_on` | Tugash sanasi boshlanishidan oldin |
| `invalid_url` | – | `source_url` | Havola `http://` yoki `https://` bilan boshlanmagan |
| `unknown_region` | – | `region_id` | Hudud topilmadi (voqealar) |

## Qayerda joylashgan

- `backend/openapi.json` — to'liq sxema (82 operatsiya)
- `backend/internal/httpapi/server.go` — router, CORS, rate limit, timeout
- `backend/internal/httpapi/apierror.go`, `errors.go` — xato konverti va kodlar
- `backend/internal/httpapi/session.go` — cookie va `currentUser`
- `backend/internal/httpapi/*.go` — har bir guruhning handlerlari (`listings.go`, `browse.go`, `feed.go`, `rfq.go`, `watches.go`, `insight.go`, `admin.go`, `events.go`, `report.go`, `media.go`, `meta.go`, `references.go`, `webapp.go`, `ai.go`, `anomalies.go`, `monitoring.go`, `suspicious.go`)
- `backend/internal/httpapi/realip.go` — ishonchli proxy'lar
- `backend/internal/insight/analytics.go` — 10 daqiqalik kesh (`cacheFor`)
- `frontend/src/lib/api/gen/` — yaratilgan TypeScript mijoz
- `frontend/src/lib/errors.ts` — mijozda xatolarni o'qish
