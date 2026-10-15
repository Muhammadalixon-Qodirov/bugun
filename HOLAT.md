# HOLAT — fazalar holati

| Faza | Natija | Holat |
|---|---|---|
| M0 | Repo, spec, baza sxemasi, katalog, Telegram orqali kirish, profil, bosh sahifa | bajarildi |
| M1 | Matn va ovozdan narx ajratish, tasdiq kartochkasi | bajarildi (matn qoidalar bilan; ovoz — ixtiyoriy STT, botda) |
| M2 | Median, anomaliya, "bozordagi o'rin", RFQ oqimi | bajarildi |
| M3 | Veb: mahsulot sahifasi, AI tahlilchi | mahsulot sahifasi va tahlil bajarildi; tahlilchi backend, bot va saytda (`/tahlilchi`) bajarildi |
| M4 | Konnektorlar, bosim indeksi, xarita, eval | bosim indeksi, xarita, eval bajarildi; konnektor — faqat UzEx (3 mahsulot) |
| M5 | PDF hisobot, dushanba eslatmasi, demo ma'lumot | bajarildi |
| POC | README, CI, testlar, xavfsiz deploy, zaxira, monitoring | bajarildi (serverda qo'llash — pastda) |

Oxirgi yangilanish: 2026-09-26. Batafsil hujjatlar: [docs/](docs/README.md), hakamlar uchun: [README.md](README.md).

## M0 — qadamlar
- [x] Repo, spec.md, HOLAT.md, .gitignore
- [x] Go API skeleti (config, log, healthz, OpenAPI /docs)
- [x] Baza sxemasi (goose migratsiyalar)
- [x] Katalog seed: 12 tuman/shahar, 8 bozor, 30 mahsulot, birliklar jadvali
- [x] Katalog API (`GET /v1/catalog`)
- [x] Telegram OIDC orqali kirish, bazadagi sessiyalar, `/v1/me`
- [x] Onboarding: rol, hudud, bozor, toifalar, rozilik; botni uzish; ma'lumotlarni o'chirish
- [x] Next.js 16 + shadcn/ui, OpenAPI dan tiplar (Hey API)
- [x] Veb: kirish (W-02), onboarding, profil (W-11)
- [x] Veb: bosh sahifa (W-01), katalog API dan
- [x] Telegram kirish haqiqiy bot bilan tekshirildi (tunnel orqali)
- [x] Bot: /start, /yordam, /profil, /uzish (long polling; webhook kerak emas deb qaror qilindi)

## M1 — qadamlar
- [x] Qoida asosidagi narx parseri (lotin/kirill, so'z bilan raqam, qop/yashik -> kg, optom) + 23 ta test
- [x] Qoralama -> tasdiq: tasdiqlanmagan narx saqlanmaydi, ikki marta bosish bir marta saqlaydi (tranzaksiya)
- [x] Yangi narx sotuvchining eski faol narxini almashtiradi (unique index)
- [x] Bot: matn -> kartochka -> [To'g'ri]/[Tuzatish]
- [x] Veb: /narx (W-06) va /kabinet (W-07, faol narxlarim)
- [x] Ovozli xabar (STT) va rasmdan narx — OpenAI-mos provayder, standart o'chiq; natija oddiy qoralama (tasdiqsiz saqlanmaydi)

## Market bosqichi — qadamlar
- [x] Butun respublika hududlari (14 + 210, SOATO)
- [x] E'lonlar: katalogga bog'langan, rasm (5 tagacha, JPEG, EXIF o'chiriladi), 7 kunlik muddat, yashirish
- [x] Anomaliya: 5+ narxda robust z > 3,5; 3–4 narxda mediandan ±50% -> "tekshirilsin"
- [x] Anomaliya kengaytirildi: sotuvchining oxirgi qabul qilingan narxidan 5 barobar sakrash; sotuvchi kam bo'lsa rasmiy etalondan (UzEx, narxtahlil) ±50%; sabab `listings.review_reason` da va admin navbatida (`reason`)
- [x] O'chirilgan e'lon narxi mediandan darhol chiqadi (DB trigger)
- [x] Ochiq API: katalog (median, filtr, saralash), mahsulot sahifasi (tuman -> viloyat -> respublika), bog'lanish
- [x] Veb: katalog, mahsulot sahifasi, "Mahsulotlarim", e'lon formasi
- [x] Veb: yagona dizayn — bosh sahifa = market, monoxrom toifalar va kartochkalar, e'lon/mahsulot/profil/forma sahifalari bir uslubda
- [x] Veb: hudud tanlash popover, 2–3 qadamli onboarding, e'lon formasi jonli preview bilan, 404 va xato sahifalari
- [x] Veb: silliqlik (grid joyida qoladi, faqat opacity animatsiyalar, Lenis scroll), yagona xato xabarlari (lib/errors.ts), profil sozlamalar sahifasi, sotuvchi kabineti (tablar, uzaytirish), rozilikka bog'liq «Boshlash»
- [x] API: CORS preflight tuzatildi (CORS router darajasida), har bir xato barqaror `code` va maydon `params` bilan; sayt matnni `lib/i18n/errors.ts` lug'atidan oladi
- [x] API: «Mahsulotlarim»da bozor mediani va farqi, `POST /v1/me/listings/refresh` bitta tranzaksiyada; nginx JSON gzip; logda `via` (site/direct)
- [x] Veb: rasmlar yuklashdan oldin brauzerda kichraytiriladi, narxda minglik bo'luvchi, Telegram/OG previewlar, WCAG kontrast tuzatildi
- [x] Veb: optimistik filtrlar (toifa, savdo turi, saralash darhol almashadi), sirpanuvchi toggle'lar, filtr paneli mahsulot chip'lari bilan, rasm lupasi va to'liq ekranli ko'rish
- [x] "Bozordagi o'rin": tasdiqdan keyin bot va sayt "12 sotuvchi ichida 9-o'rindasiz" deydi; «Mahsulotlarim»da `position` va `review_reason`
- [x] Bot: /mening — faol narxlar va bozordagi o'rin
- [x] Vaznli median (spec 10.1): `price_weight` × `weighted_quantile` barcha market medianlari va P25/P75 da; etalon va anomaliya vaznsiz
- [x] RFQ (spec 8.4): xaridor so'rovi → viloyatdagi mos sotuvchilar → taklif / «Menda yo'q» → xaridor tanlaydi → sotuvchi rozi → kontakt; API (`/v1/rfqs`, `/v1/me/rfq-inbox`, `/v1/offers/...`) va bot (/sorov, tugmalar); taklif narxi medianga `rfq_offer` (1,0); tejash hisobi (10.6)
- [x] Talab signallari (`demand_events`): mahsulot ko'rish, qidiruv, kontakt ochish (Telegram ham), RFQ — kuniga bir marta, IP saqlanmaydi
- [x] Rejali vazifalar (`job_runs`, Toshkent vaqti) va dushanba 09:00 eslatmasi (spec 8.3): «Ha, o'zgarmadi» barcha narxlarni 7 kunga uzaytiradi
- [x] Bosim indeksi (spec 10.3): oxirgi 7 kun / oldingi 4 hafta, talab `demand_events`dan, taklif — faol sotuvchilar; `/v1/products/{slug}/pressure`
- [x] Xarita (W-04) API: `/v1/map/provinces` — 14 viloyat chegarasi (hudud.stat.uz, binarda) + median va zona
- [x] Bot: /savol va oddiy savol matni — AI'siz, qoidalar bilan (mahsulot, hudud, niyat); median, oraliq, eng arzonlar yoki hududlar solishtiruvi; `question_log` + talab signali
- [x] Haftalik viloyat hisoboti PDF (W-10): `/v1/reports/weekly` — tashkilot/admin istalgan viloyat, bozor ma'muriyati o'z viloyati; faqat ≥5 narxli agregatlar
- [x] Avtomatik narx ([docs/06](docs/06-avtomatik-narx.md), spec boshidagi 14.4 qarori): sotuvchi min–max oraliq va maqsad beradi (`PUT /v1/listings/{id}/pricing`); `auto_pricing` vazifasi 07:30/12:00/16:30 da vaznli median, P75, bosim, haftalik trend, ko'rish/kontakt, RFQ va zaxiraga qarab narxni kuniga ≤5% siljitadi; qo'lda narx yoki «Qaytarish» 24 soat pauza; har o'zgarish `price_changes` da sabablari bilan, medianga `seller_auto` (0,4); bot xabari + «Qaytarish»; `/v1/me/listings/{id}/price-changes`, `/v1/me/price-changes/{id}/revert`
- [x] Savdolar (`deals`): RFQ «Roziman» → tasdiqlangan savdo; kontaktdan 24 soat keyin xaridorga, 19:00 da sotuvchiga bot so'rovi, ikki tomon moslashtiriladi; «Sotildi» tugmasi (`POST /v1/listings/{id}/sold`, `/mening`) zaxirani kamaytiradi, tugasa yashiradi
- [x] E'lon qiziqishi (`listing_events`: noyob ko'rish va kontakt, kuniga bir marta); voronka `/v1/me/listings/{id}/stats`, `funnel_7d`; mahsulot sahifasida `interest` (≥5); admin `/v1/admin/deals`
- [x] Tasdiqlangan savdo narxi medianga `deal` (1,0); dinamik narxda `sales` omili, bosimda savdo (8) va kuzatish (3)
- [x] Kuzatish (`/v1/watches`, bot /kuzatuv va «🔔 Kuzatish»): arzon narx yoki median 5% tushsa 08:00/17:00 da xabar
- [x] Reyting: tasdiqlangan savdodan 3 kun keyin bot ⭐1–5 so'raydi; e'lon sahifasida ≥3 bahoda; past izohlar admin moderatsiyasida
- [x] Haftalik PDF'da tasdiqlangan savdolar (≥5)
- [x] Tuzatildi: yashirilgan e'lon narxi bozordagi o'rin va avtomatik narx hisobiga kirib qolardi
- [x] Tahlil ([docs/04](docs/04-tahlil-metodologiyasi.md)): kunlik narx tarixi (`price_daily`), 3 yillik demo tarix (`seed -history 3`, belgilangan), `/v1/products/{slug}/analytics`: grafik, o'zgarishlar, rekordlar, trend, viloyatlar, tuman/bozor, arbitraj, ustama, savdolashuv, mavsum, bosim, «Hozir oling/Kuting», hafta kunlari, taklif, keskin o'zgarishlar + voqealar, kurs, ob-havo, prognoz, sotuvchi sabablari; CSV
- [x] Hududiy statistika: `/v1/regions/{id}/overview` (pasport, mahsulotlar, oziq-ovqat indeksi, imkoniyatlar), `/v1/analytics/heatmap`, `/v1/analytics/top-demand`, `/v1/analytics/compare`
- [x] Tashqi bepul ma'lumot: Markaziy bank kursi (USD/RUB/KZT) va Open-Meteo ob-havosi (kunlik, 3 yil orqaga); bayramlar taqvimi 2023–2027; admin voqealar jurnali
- [x] Bot: narx ≥10% o'zgarsa «Nega o'zgardi?», `/mening` da «narxingiz qimmat»
- [x] Telegram kanal va guruhlarni ulash ([docs/07 §8](docs/07-telegram-bot.md#8-kanallar-va-guruhlar)): bot qo'shilganda egasiga bog'lanadi, postlardagi narxlar `telegram_channel` (0,6) bo'lib tushadi, tahrir almashtiradi, uzoq narx tekshiruvga; ixtiyoriy e'lonlarni yangilash; `/kanallar`; guruhda bot hech qachon javob yozmaydi
- [x] ML ([docs/12](docs/12-ml-usullari.md)): prognoz musobaqasi (`naive`/`last_year`/`season_trend`, `skill`, Shapley `drivers`); avtomatik narxda aniq Shapley `contrib`; tizim monitoringi (`/v1/admin/monitoring`, botda adminlarga); kunlik anomaliyalar (`price_anomalies`, `price_daily_clean`, `/v1/admin/anomalies`); Isolation Forest bilan g'ayrioddiy sotuvchilar (`/v1/admin/sellers/suspicious`); K-Means bilan viloyat guruhlari (`/v1/analytics/clusters`, hudud `similar`); issiqlik xaritasida LISA `spot`; kanal postlari uchun ixtiyoriy LLM zaxira parseri (`LLM_*`, default o'chiq)
- [x] Narx matnini tushunish (MATN_TUSHUNISH.md): har bir urinish `parse_log` ga yoziladi, `/v1/admin/parser` statistikasi va dushanba kungi hisobot; xato yozilgan so'zlar, yangi birliklar; tanilmagan qator va ikki marta yozilgan mahsulot ochiq ko'rsatiladi; «120 ming yashik» yashik narxi deb o'qiladi; qoidalar tushunmagan qism ixtiyoriy LLM'ga (Groq) boradi, sotuvchi tasdiqlagan yangi nomlar o'rganiladi (3 sotuvchidan keyin hammaga)
- [x] ML prognoz (PROGNOZ_ML.md): gradient boosting (`internal/ml/gbm.go`, Huber, yo'q qiymatlar, erta to'xtash) + aniq TreeSHAP; 33 ta belgi (tarix, mavsum, bayram, ob-havo, dollar, talab-taklif); har kecha 04:40 `forecast_ml` ishi 1–4 hafta modelini o'rgatadi va o'tgan yilda sinaydi (`forecast_ml`, `forecast_ml_backtest`, `forecast_ml_runs`); tahlil sahifasida 4-usul `ml` bo'lib bir xil haftalarda musobaqalashadi, yutsa `method: ml` va yangi `drivers` kodlari (`history`, `holiday`, `weather`, `fx`, `demand_supply`); `GET /v1/admin/forecast/model`
- [x] AI yordamchi (CHATBOT_REJA.md): `POST /v1/chat` va botda suhbat; 30 ta asbob (narx, tarix, viloyatlar, tuman va bozorlar, takliflar, prognoz, maslahat, mavsum, hafta kuni, sabablar, voqealar, hudud holati, arbitraj, ulgurji-chakana, haftalik o'zgarishlar, mahsulotlarni solishtirish, savat va retsept narxi, kurs, ob-havo, sotuvchi, platforma yordami, so'rov va kuzatuv takliflari); raqamlar faqat asbobdan (numcheck); aniqlashtirish (`ask_user`, tugmalar), javob bera olmaganda adminlarga (`chat_unanswered`, kunlik hisobot, monitoring); zaxira modellar (`LLM_FALLBACK_MODELS`); yoqish: `ANALYST_ENABLED=true`
- [x] Sayt: har sahifada o'ng pastki burchakdagi AI yordamchi (telefonda to'liq ekran) va `/tahlilchi` — bir xil suhbat (`POST /v1/chat`): suhbat brauzerda saqlanadi, «Yangi suhbat», aniqlashtirish tugmalari, `actions` faqat tasdiqlash oynasidan keyin (kuzatuv yoki to'ldirilgan so'rov formasi), manba izohi, jadval, Demo belgisi; admin panelida «Yordamchi» bo'limi (statistika va javobsiz savollar)
- [x] Demo e'lon rasmlari qo'lda tanlangan (87 mahsulot, 200 foto, CC0/public domain, manbasi yozilgan); mos rasm topilmagan mahsulotga rasm qo'yilmaydi; `seed -photos-only` faqat rasmlarni almashtiradi
- [x] Telegram Mini App orqali e'lon qo'shish: bot `/elon` e'lon formasini Mini App'da ochadi
- [x] Mini App ichida `initData` bilan avtomatik kirish (`POST /v1/auth/telegram/webapp`, sayt bir marta chaqiradi)
- [x] Veb: uch til (uz/en/ru, next-intl, `npm run i18n:check`), yorug'/qorong'i/tizim mavzusi, Telegram Mini App mavzusi va xavfsiz hududlari
- [x] Veb: mahsulot tahlili `/mahsulot/{slug}/tahlil` — maslahat, grafik (p25–p75, voqealar, prognoz), mavsum, hafta kunlari, viloyatlar, arbitraj, rekordlar, keskin o'zgarishlar, ob-havo, kurs, CSV
- [x] Veb: `/statistika` (issiqlik xaritasi, haftaning o'zgarishlari, TOP talab), `/hudud/{id}`, `/solishtirish`, `/xarita` (jonli narx kam bo'lsa 30 kunlik median bilan)
- [x] Veb: RFQ — `/sorovlar`, `/sorovlar/{id}`; `/kabinet` rolga qarab: sotuvchiga kelgan so'rovlar, tashkilotga haftalik PDF
- [x] Veb: sotuvchi — bozordagi o'rin, 7 kunlik voronka, «Sotildi», statistika, avtomatik narx va tarixi (qaytarish), `/narx` natijasi; kuzatuv («Narx tushsa xabar ber»), reyting, admin: savdolar, baholar, voqealar jurnali
- [x] API: maslahat sabablari (`advice.items`) va parser xabarlari (`problems[].code`, `items[].hints`) kod bilan; tekshiruvdagi e'lonni yashirish/ko'rsatish orqali faollashtirib bo'lmaydi; rad etish izohi sotuvchiga ko'rinadi
- [x] Admin: "tekshirilsin" navbati va rol so'rovlari (`/admin`, `ADMIN_TELEGRAM_IDS`)
- [x] Serverga deploy (backend + Postgres + rasmlar): Docker Compose, nginx, sayt Vercel'da; eski sxema bo'yicha (`/root/Bugun`) server ishlagan

## POC bosqichi — qadamlar (2026-09-26)
- [x] Ildizda `README.md` (muammo, yechim, demo yo'li, arxitektura, cheklovlar, litsenziyalar); `docs/13-sozlamalar.md`
- [x] `/healthz` baza ishlamasa 503; Docker healthcheck va deploy tekshiruvi shunga qaraydi
- [x] Sozlamalar env'ga: jadvallar (`SCHEDULE_*`), anomaliya, avtomatik narx, RFQ, rasmlar, prognoz, klasterlar, timeout'lar, limitlar; ishga tushishda tekshiriladi; `GET /v1/meta/limits` (sayt limitlarni takrorlamaydi)
- [x] Yagona vaqt zonasi paketi (`internal/tz`); rejali ishlarga lease (00038); migratsiya advisory lock ostida, alohida `cmd/migrate`
- [x] Yashirin e'lon rasmlari faqat egasi va admin'ga; S3 rasm ombori (`MEDIA_STORAGE=s3`, `cmd/mediasync`)
- [x] Ishonchli proxy'lar (`TRUSTED_PROXIES`), botda foydalanuvchi limiti, `WriteTimeout`/`IdleTimeout`, shutdown'da bot va scheduler kutiladi
- [x] Sentry (ixtiyoriy, shaxsiy ma'lumot tozalanadi), tashqi heartbeat (`HEARTBEAT_URL`)
- [x] UzEx konnektori (`reference_prices`, `GET /v1/products/{slug}/references`), standart o'chiq
- [x] Model funksiyalari (standart o'chiq): tahlilchi (`POST /v1/ask`, tool calling, raqam tekshiruvi), STT, rasmdan narx, katalogga moslash, maslahat matni, moderatsiya; `ai_log` va `/v1/admin/ai-usage`; `make eval` (natija `backend/eval/RESULTS.md`)
- [x] Sayt: bo'lim `error.tsx`/`loading.tsx`, `global-error.tsx`, productionda manzillar majburiy, nonce'li CSP, xavfsizlik sarlavhalari, sitemap/robots/OG, `next/image`, viloyatlar ID bo'yicha, demo belgilari
- [x] Playwright smoke testlari (`npm run test:e2e`, desktop va 390 px)
- [x] CI: backend (Postgres bilan), frontend, e2e, Docker image'lar, gitleaks
- [x] nginx: TLS, HSTS, xavfsizlik sarlavhalari, `limit_req`; backup (baza + rasmlar, rclone offsite, heartbeat), `restore.sh`, systemd timer; deploy — SHA teglar, alohida migratsiya, avtomatik orqaga qaytish, root bo'lmagan `deploy` foydalanuvchisi
- [x] Tiklash sinovi: 14 MB dump ~4 s, tiklash ~9 s, 35 jadval qatorlari mos
- [ ] Yangi deploy tuzilmasini serverda qo'llash: `/srv/bugun` + `deploy` foydalanuvchisi, certbot + yangi nginx konfiguratsiyasi, systemd timer, UptimeRobot va healthchecks.io monitorlari
- [x] Sayt: `/tahlilchi`, narx kiritishda (`/narx`) ovoz/rasm tugmalari, maslahat matni, admin model sarfi bo'limi
- [ ] Toza o'rnatish sinovi: repo'ni yangi papkaga clone qilib faqat README bo'yicha ko'tarish

## Qarorlar
- Bot long polling bilan ishlaydi (webhook emas): alohida HTTPS endpoint va sir kerak emas, bitta jarayonda `BOT_ENABLED` bilan bitta poller.
- Model hech qachon raqam bermaydi: raqamlar deterministik funksiyalardan, model matni tekshiriladi; barcha model funksiyalari standart o'chiq.
- Orqaga qaytish migratsiyani qaytarmaydi, shuning uchun har relizda faqat qo'shuvchi migratsiyalar.
- Kirish faqat Telegram OIDC orqali. `telegram:bot_access` scope bot yozishiga ruxsat beradi, shuning uchun alohida "botni ulash" tokeni (7.3) kerak emas.
- Sayt `/api/*` so'rovlarini Go API ga proxy qiladi: cookie birinchi tomon, Telegram bitta domenga qaytadi.
- Sessiyalar bazada (token xeshi), restartdan keyin saqlanadi. Telefon AES-GCM bilan shifrlangan + HMAC xesh.
- Bozorlar ro'yxati va qop/yashik og'irliklari boshlang'ich qiymat — bozor safarida tasdiqlanadi.
- Dizayn: iliq fon, to'q zumrad yashil faqat narx signali va asosiy amal uchun; tanlovlar qora (foreground), toifa ikonkalari monoxrom.
- Sanalar: oy nomlari o'zimizda (brauzerlar "uz" locale'ni turlicha qo'llab-quvvatlaydi), kun Asia/Tashkent bo'yicha.
- Xatolar: API `code` qaytaradi, matn tarjimasi klientda. Yangi til = `lib/i18n/errors.ts` ga yangi lug'at.
- Vaznli median — pastki kvantil (interpolatsiyasiz): jami vaznning yarmiga yetgan eng past narx. SQL va Go bir xil hisoblaydi (test bor).
