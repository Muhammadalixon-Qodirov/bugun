# Sozlamalar ma'lumotnomasi

Bugun'ning barcha sozlamalari muhit o'zgaruvchilarida. Bu hujjat ularning to'liq ro'yxatini standart qiymatlari va ma'nosi bilan beradi. Manba — kodning o'zi: backend uchun `backend/internal/config/config.go`, `settings.go`, `ai.go`; frontend uchun `frontend/src/lib/env.ts`, `src/lib/sentry.ts`, `next.config.ts`; server skriptlari uchun `deploy/lib.sh` va `deploy/*.sh`. Shablonlar: `backend/.env.example`, `frontend/.env.example`, `deploy/.env.example` (bo'sh qiymatlar bilan).

API ishga tushganda barcha qiymatlarni tekshiradi (`Config.validate`, `validateSettings`, `AI.validate`). Noto'g'ri qiymat — masalan manfiy vaqt, `RFQ_MIN_DAYS > RFQ_DEFAULT_DAYS` yoki `MEDIA_STORAGE=s3` kalitsiz — bo'lsa, dastur ishga tushmaydi va barcha xatolarni bitta xabarda chiqaradi. Bu imlo xatosi xulq-atvorni jimgina o'zgartirib yubormasligi uchun.

## Mundarija

1. [Backend: asosiy](#1-backend-asosiy)
2. [Backend: HTTP, limitlar, bot](#2-backend-http-limitlar-bot)
3. [Backend: rejali ishlar](#3-backend-rejali-ishlar)
4. [Backend: algoritm chegaralari](#4-backend-algoritm-chegaralari)
5. [Backend: RFQ va rasmlar](#5-backend-rfq-va-rasmlar)
6. [Backend: rasm ombori (S3)](#6-backend-rasm-ombori-s3)
7. [Backend: kuzatuv (heartbeat, Sentry)](#7-backend-kuzatuv-heartbeat-sentry)
8. [Backend: tashqi manbalar](#8-backend-tashqi-manbalar)
9. [Backend: ixtiyoriy model funksiyalari](#9-backend-ixtiyoriy-model-funksiyalari)
10. [Frontend](#10-frontend)
11. [Server: compose, zaxira, deploy](#11-server-compose-zaxira-deploy)
12. [Qaysi qiymatlar ikki joyda mos bo'lishi kerak](#12-qaysi-qiymatlar-ikki-joyda-mos-bolishi-kerak)

Vaqt qiymatlari Go formatida: `15s`, `5m`, `24h`. Jadval vaqtlari — **Toshkent vaqti**, `HH:MM`, bir nechtasi vergul bilan; haftalik — `Mon 09:00`.

## 1. Backend: asosiy

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `DATABASE_URL` | **majburiy** | PostgreSQL ulanish manzili |
| `DATA_ENCRYPTION_KEY` | **majburiy** | 32 bayt, base64 (`openssl rand -base64 32`). Telefonni shifrlash va HMAC xesh kaliti. Yo'qolsa saqlangan telefonlarni ochib bo'lmaydi |
| `APP_ENV` | `development` | `production` da loglar JSON, `.env` o'qilmaydi deb hisoblanadi (Docker `env_file` beradi) |
| `HTTP_ADDR` | `:8080` | API tinglaydigan manzil |
| `WEB_URL` | `http://localhost:3000` | Sayt manzili: kirishdan keyin qaytish, CORS, bot havolalari; `https://` bo'lsa cookie `Secure` |
| `API_URL` | bo'sh → `WEB_URL/api` | Brauzer ko'radigan API manzili, Telegram callback shu yerga keladi |
| `CORS_ORIGINS` | bo'sh | Qo'shimcha ruxsat etilgan manbalar, vergul bilan |
| `COOKIE_DOMAIN` | bo'sh (aniq host) | Sayt va API bo'lishadigan cookie domeni |
| `INTERNAL_SECRET` | bo'sh (o'chiq) | Sayt serveri bilan umumiy sir (frontenddagi `INTERNAL_API_SECRET`) |
| `ADMIN_TELEGRAM_IDS` | bo'sh | Admin Telegram id'lari, vergul bilan |
| `SESSION_TTL` | `720h` | Sessiya muddati (30 kun) |
| `MEDIA_DIR` | `data/media` | Rasmlar papkasi (`MEDIA_STORAGE=disk` da) |
| `MIGRATE_ON_START` | `true` | API ishga tushganda migratsiyalarni qo'llash. Productionda `false`: `deploy.sh` ularni alohida qadamda qo'llaydi |
| `BOT_ENABLED` | `true` | Botni ishga tushirish. Bitta tokenni faqat bitta jarayon o'qiy oladi |
| `TELEGRAM_BOT_TOKEN` | bo'sh | Bot tokeni. Bo'sh bo'lsa bot va botga bog'liq ishlar o'chadi |
| `TELEGRAM_BOT_USERNAME` | bo'sh | Bot nomi |
| `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET` | bo'sh | @BotFather → Bot Settings → Login (OpenID Connect) |
| `TELEGRAM_WEBAPP_MAX_AGE` | `1h` | Mini App `initData` qancha eski bo'lishi mumkin (1m–168h) |

## 2. Backend: HTTP, limitlar, bot

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `REQUEST_TIMEOUT` | `15s` | Bitta handler uchun vaqt; oshsa 504 |
| `HTTP_WRITE_TIMEOUT` | `60s` | Butun javob (PDF, CSV) uchun; `REQUEST_TIMEOUT` dan uzun bo'lishi shart |
| `HTTP_IDLE_TIMEOUT` | `120s` | Bo'sh keep-alive ulanish |
| `RATE_LIMIT_PER_MINUTE` | `120` | Bitta mijoz kaliti uchun daqiqasiga so'rovlar (`/media` dan tashqari) |
| `TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | `X-Forwarded-For` / `X-Real-IP` ga ishoniladigan proxy'lar (CIDR). Docker compose'da `172.16.0.0/12` |
| `BOT_MESSAGES_PER_MINUTE` | `20` | Bitta foydalanuvchi botga daqiqasiga yuboradigan xabar va tugma bosishlar |

## 3. Backend: rejali ishlar

| O'zgaruvchi | Standart | Ish |
|---|---|---|
| `SCHEDULE_AUTO_PRICING` | `07:30,12:00,16:30` | `auto_pricing` — avtomatik narx |
| `SCHEDULE_ANOMALIES` | `03:30` | `price_anomalies_year` — oxirgi 365 kun anomaliyalari |
| `SCHEDULE_SELLER_SCORES` | `04:00` | `seller_scores` — Isolation Forest |
| `SCHEDULE_SESSIONS_CLEANUP` | `04:30` | `sessions_cleanup` |
| `SCHEDULE_FX` | `10:15` | `fx_daily` — Markaziy bank kursi |
| `SCHEDULE_WEATHER` | `06:30` | `weather_daily` — ob-havo |
| `SCHEDULE_SELLER_DIGEST` | `19:00` | `seller_deal_digest` (bot) |
| `SCHEDULE_WATCH_ALERTS` | `08:00,17:00` | `watch_alerts` (bot) |
| `SCHEDULE_RATING_PROMPTS` | `11:00` | `rating_prompts` (bot) |
| `SCHEDULE_WEEKLY_REMINDER` | `Mon 09:00` | `weekly_reminder` (bot) |
| `SCHEDULE_UZEX` | `18:30` | `uzex_quotes` (faqat `UZEX_ENABLED=true`) |
| `SCHEDULE_AI_LOG_CLEANUP` | `04:45` | `ai_log_cleanup` — eski model jurnali |
| `SCHEDULE_PRICE_DAILY_EVERY` | `1h` | `price_daily` — kunlik tarix |
| `SCHEDULE_DEAL_PROMPTS_EVERY` | `15m` | `deal_prompts` (bot) |
| `SCHEDULE_MONITORING_EVERY` | `1h` | `monitoring` |
| `JOB_HEARTBEAT` | `1m` | Ishlayotgan ish lease'ini shu oraliqda yangilaydi |
| `JOB_LEASE_TIMEOUT` | `5m` | Tugallanmagan ish shu vaqtdan eski lease bilan qolsa bo'shatiladi; kamida `2 × JOB_HEARTBEAT` |

Oraliqli ishlar (`*_EVERY`) 1 daqiqadan 24 soatgacha bo'lishi va sutkani butun bo'lishi kerak (masalan `7m` bo'lmaydi), aks holda slotlar har kuni boshqa vaqtga tushadi.

## 4. Backend: algoritm chegaralari

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `ANOMALY_Z` | `3.5` | Kunlik anomaliya uchun robust z chegarasi |
| `ANOMALY_MIN_MOVE` | `0.25` | Viloyatning o'z o'tmishiga nisbatan eng kichik siljish (25%) |
| `ANOMALY_MIN_CROSS_MOVE` | `0.3` | Boshqa viloyatlarga nisbatan eng kichik farq (30%) |
| `ANOMALY_MIN_PROVINCES` | `5` | Solishtirish uchun kerakli viloyatlar |
| `REPRICE_GOAL_FAST` | `0.97` | Avtomatik narx maqsadi "tez sotish" — medianga koeffitsient |
| `REPRICE_GOAL_BALANCED` | `1.00` | "Muvozanat" |
| `REPRICE_GOAL_PROFIT` | `1.03` | "Foyda" |
| `REPRICE_DAILY_STEP` | `0.05` | Bir Toshkent kunida narx ko'pi bilan qancha o'zgaradi (5%) |
| `AUTO_PRICE_MANUAL_PAUSE` | `24h` | Qo'lda o'zgartirilgan narxga avtomatika tegmaydigan vaqt |
| `AUTO_PRICE_MIN_MARKET_VIEWS` | `100` | Kontakt ulushini solishtirish uchun bozordagi kamida ko'rishlar |
| `FORECAST_WEEKS` | `4` | Prognoz necha hafta oldinga (1–12) |
| `FORECAST_TREND_DAMPING` | `0.8` | Trendning har haftalik so'nishi |
| `FORECAST_MAX_TREND_EFFECT` | `0.15` | Trend ta'sirining yuqori chegarasi |
| `FORECAST_MIN_BACKTESTS` | `20` | Usullar sinalishi kerak bo'lgan o'tgan haftalar |
| `FORECAST_INTERVAL_QUANTILE` | `0.8` | Oraliq ichiga tushadigan o'tgan xatolar ulushi |
| `CLUSTER_PROFILE_COVERAGE` | `0.7` | Mahsulot profilga kirishi uchun kerakli viloyatlar ulushi |
| `CLUSTER_PROVINCE_COVERAGE` | `0.5` | Viloyat guruhlanishi uchun kerakli mahsulotlar ulushi |
| `CLUSTER_MIN_PROVINCES` | `6` | Guruhlash uchun eng kam viloyatlar |
| `CLUSTER_MIN_PRODUCTS` | `5` | Eng kam mahsulotlar |
| `CLUSTER_MAX_GROUPS` | `5` | Sinab ko'riladigan eng ko'p guruhlar (`k`) |
| `CLUSTER_TOP` | `3` | Har guruhda nomlanadigan mahsulotlar |
| `CLUSTER_NOTABLE_GAP` | `0.03` | Nomlashga arziydigan eng kichik farq |

Metodologiya: [ML usullari](12-ml-usullari.md), [avtomatik narx](06-avtomatik-narx.md).

## 5. Backend: RFQ va rasmlar

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `RFQ_MAX_OPEN` | `3` | Bitta xaridorning bir vaqtdagi ochiq so'rovlari |
| `RFQ_DEFAULT_DAYS` | `3` | Muddat tanlanmasa |
| `RFQ_MIN_DAYS`, `RFQ_MAX_DAYS` | `1`, `7` | Ruxsat etilgan muddat oralig'i |
| `MEDIA_MAX_UPLOAD_BYTES` | `10485760` | Bitta rasm (10 MiB). nginx `client_max_body_size` undan katta bo'lishi kerak |
| `MEDIA_MAX_PIXELS` | `40000000` | Eni × bo'yi (40 MP) |
| `MAX_PHOTOS_PER_LISTING` | `5` | E'longa rasmlar |

Sayt bu limitlarni o'zida takrorlamaydi: `GET /v1/meta/limits` dan oladi ([API](09-api.md)).

## 6. Backend: rasm ombori (S3)

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `MEDIA_STORAGE` | `disk` | `disk` (`MEDIA_DIR`) yoki `s3` |
| `S3_ENDPOINT` | bo'sh | `host[:port]` yoki `http(s)://host[:port]` (AWS, MinIO, R2, Spaces) |
| `S3_REGION` | bo'sh | Mintaqa |
| `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | bo'sh | `s3` da majburiy |
| `S3_USE_PATH_STYLE` | `false` | O'zi o'rnatilgan xizmatlar (MinIO) uchun odatda `true` |
| `S3_PUBLIC_BASE_URL` | bo'sh | Bucket'ning ochiq manzili. Berilsa faol e'lon rasmlari shu yerga yo'naltiriladi (302); boshqa rasmlar baribir API tekshiruvi orqali beriladi |

Diskdan S3 ga o'tish: avval `go run ./cmd/mediasync` (yoki `-dry-run`) mavjud rasmlarni nusxalaydi, keyin `MEDIA_STORAGE=s3`.

## 7. Backend: kuzatuv (heartbeat, Sentry)

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `HEARTBEAT_URL` | bo'sh (o'chiq) | Rejalashtiruvchi tirik ekanida shu manzilga ping yuboradi (masalan healthchecks.io) |
| `HEARTBEAT_INTERVAL` | `5m` | Ping oralig'i |
| `SENTRY_DSN` | bo'sh (o'chiq) | Xatolarni kuzatish |
| `SENTRY_ENVIRONMENT` | `APP_ENV` | Sentry'dagi muhit nomi |
| `SENTRY_TRACES_SAMPLE_RATE` | `0` | Unumdorlik izlari ulushi (0–1) |

## 8. Backend: tashqi manbalar

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `UZEX_ENABLED` | `false` | UzEx birja kotirovkalarini kunlik yuklash |
| `UZEX_URL` | `https://uzex.uz/Quote/GetQuotes` | Kotirovkalar ro'yxati manzili |

Markaziy bank va Open-Meteo manzillari kodda qotirilgan (kalit kerak emas).

## 9. Backend: ixtiyoriy model funksiyalari

Hammasi standart o'chiq. Har bir funksiya OpenAI-mos API bilan ishlaydi (Groq, Gemini, OpenAI va boshqalar). O'chiq bo'lsa qoidali yo'l javob beradi. Har bir chaqiruv `ai_log` ga yoziladi (matnsiz).

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY` | bo'sh | Chat modeli. Uchalasi berilmaguncha o'chiq. Kanal postlari, tahlilchi, moderatsiya va maslahat matni shuni ishlatadi |
| `LLM_MAX_PER_HOUR` | `100` | Chat, vision va embeddings chaqiruvlarining soatlik chegarasi (har mijoz uchun alohida) |
| `LLM_TIMEOUT` | `20s` | Bitta chat chaqiruvi |
| `ANALYST_ENABLED` | `false` | `/savol` va `POST /v1/ask` da model funksiyalarni tanlaydi (tool calling) |
| `ANALYST_MAX_TOOL_ROUNDS` | `4` | Bitta savolga funksiya chaqirish raundlari (1–10) |
| `ANALYST_PER_MINUTE` | `6` | Bitta foydalanuvchi (yoki IP) uchun daqiqasiga model javoblari; oshsa qoidali javob |
| `STT_BASE_URL`, `STT_MODEL`, `STT_API_KEY` | bo'sh | Ovozdan matn (bot ovozli xabarlari, `POST /v1/prices/transcribe`) |
| `STT_LANGUAGE` | `uz` | Tanish tili |
| `STT_MAX_SECONDS` | `60` | Eng uzun yozuv (5–600) |
| `STT_MAX_PER_HOUR` | `200` | Soatlik chegara |
| `VISION_MODEL` | bo'sh | Rasmdan narx o'qish modeli; `VISION_BASE_URL` / `VISION_API_KEY` bo'sh bo'lsa `LLM_*` ishlatiladi |
| `EMBEDDINGS_MODEL` | bo'sh | Katalogga moslash uchun embeddings; `EMBEDDINGS_BASE_URL` / `EMBEDDINGS_API_KEY` bo'sh bo'lsa `LLM_*` |
| `MATCH_MIN_CONFIDENCE` | `0.6` | Shundan yuqori moslik avtomatik qabul qilinadi |
| `MATCH_MIN_CANDIDATE` | `0.3` | Shundan yuqori nomzodlar sotuvchiga tanlash uchun ko'rsatiladi |
| `ADVICE_TEXT_ENABLED` | `false` | Maslahatni model oddiy tilda qayta yozadi (raqamsiz) |
| `MODERATION_ENABLED` | `false` | E'lon matnini model ham tekshiradi; faqat tekshiruvga ushlaydi, hech qachon rad etmaydi |
| `MODERATION_TIMEOUT` | `4s` | Moderatsiya kutish vaqti; oshsa e'lon to'xtatilmaydi |
| `AI_MODEL_PRICES` | bo'sh | `ai_log.cost_usd` uchun narxlar: `model=in:out[:audio_minute]` (1M token va audio daqiqa uchun USD), vergul bilan |
| `AI_LOG_RETENTION_DAYS` | `90` | `ai_log` qatorlari saqlanadigan kunlar |
| `EVAL_DATABASE_URL` | `DATABASE_URL` | Faqat `make eval`: katalog o'qiladigan baza |

## 10. Frontend

| O'zgaruvchi | Development standarti | Ma'nosi |
|---|---|---|
| `API_INTERNAL_URL` | `http://localhost:8080` | Next.js serveri (proxy, server komponentlari) chaqiradigan Go API. **Productionda majburiy** |
| `NEXT_PUBLIC_API_URL` | `/api` | Brauzer uchun API: `/api` (saytning o'z proxy'si) yoki API subdomeni. Uning origin'i CSP va `next/image` ruxsatlariga qo'shiladi. Build vaqtida qotadi |
| `NEXT_PUBLIC_SITE_URL` | `http://localhost:3000` | Saytning ochiq manzili: canonical, sitemap, link preview. Vercel'da standart — production domeni. **Productionda majburiy** |
| `INTERNAL_API_SECRET` | bo'sh | Backenddagi `INTERNAL_SECRET` bilan bir xil; faqat serverda |
| `NEXT_PUBLIC_TELEGRAM_BOT_USERNAME` | bo'sh | Profil sahifasidagi bot nomi (`@` siz) |
| `E2E_BASE_URL` | `http://localhost:3100` | Playwright ochadigan sayt |
| `NEXT_PUBLIC_SENTRY_DSN` | bo'sh (o'chiq) | Brauzer va server xatolari; build vaqtida qotadi |
| `NEXT_PUBLIC_SENTRY_ENVIRONMENT` | `NODE_ENV` | Sentry muhit nomi |
| `NEXT_PUBLIC_SENTRY_TRACES_SAMPLE_RATE` | `0` | Unumdorlik izlari ulushi |
| `SENTRY_AUTH_TOKEN`, `SENTRY_ORG`, `SENTRY_PROJECT` | bo'sh | Faqat build: source map'larni Sentry'ga yuklash. Tokensiz build Sentry'ga murojaat qilmaydi |

Production build yoki server `API_INTERNAL_URL` yoki sayt manzilisiz ishga tushmaydi — jimgina localhost'ga murojaat qilish o'rniga xato bilan to'xtaydi (`src/lib/env.ts`).

## 11. Server: compose, zaxira, deploy

`deploy/.env.example` — serverdagi `.env` shabloni. U docker compose'ga, API konteyneriga va deploy/zaxira skriptlariga beriladi. `DATABASE_URL`, `APP_ENV`, `HTTP_ADDR` va `MEDIA_DIR` ni `docker-compose.yml` o'zi qo'yadi.

| O'zgaruvchi | Standart | Ma'nosi |
|---|---|---|
| `POSTGRES_PASSWORD` | **majburiy** | Postgres konteyneri paroli |
| `API_PORT` | `8090` | API faqat `127.0.0.1:API_PORT` da ochiladi; `deploy/nginx/bugun.conf` dagi upstream bilan bir xil bo'lishi kerak |
| `MIGRATE_ON_START` | `false` (compose'da) | Migratsiyani `deploy.sh` alohida qadamda qiladi |
| `TRUSTED_PROXIES` | `172.16.0.0/12` (compose'da) | nginx konteynerga Docker ko'prigi orqali keladi |
| `DEPLOY_DIR` | skriptlar turgan repo ildizi | Server checkout'i (masalan `/srv/bugun/app`) |
| `BUGUN_ENV_FILE` | `$DEPLOY_DIR/../.env` | Server sirlari fayli |
| `BACKUP_DIR` | `$DEPLOY_DIR/../backups` | Dump va rasm arxivlari |
| `BACKUP_RETENTION_DAYS` | `14` | Lokal nusxalar saqlanadigan kunlar |
| `RCLONE_REMOTE` | bo'sh (o'chiq) | Offsite nusxa, masalan `b2:bugun-backups/prod` |
| `RCLONE_RETENTION_DAYS` | `BACKUP_RETENTION_DAYS` | Offsite nusxalar; `0` — hech qachon o'chirilmaydi |
| `BACKUP_HEARTBEAT_URL` | bo'sh (o'chiq) | Har muvaffaqiyatli zaxiradan keyin ping, xatoda `<url>/fail` |
| `BACKUP_TAR_IMAGE` | Postgres image'i | Rasm arxivini yozadigan vaqtinchalik konteyner image'i |
| `DEPLOY_BRANCH` | `main` | Server kuzatadigan branch (faqat fast-forward) |
| `HEALTH_TIMEOUT` | `60` | Yangi image'dan `/healthz` = 200 ni kutish (soniya) |
| `KEEP_IMAGES` | `5` | Qo'lda qaytish uchun saqlanadigan SHA teglar |
| `COMPOSE_PROJECT` | `bugun` | Compose loyiha nomi (hajmlar shu nomga tegishli) |
| `BUGUN_SSH_HOST` | `bugun-server` | `remote-deploy.sh`: SSH host yoki `~/.ssh/config` taxallusi (sizning kompyuteringizda) |
| `BUGUN_REMOTE_DIR` | `Bugun/app` | `remote-deploy.sh`: serverdagi checkout (masalan `/srv/bugun/app`) |

## 12. Qaysi qiymatlar ikki joyda mos bo'lishi kerak

| Qiymat | Joy 1 | Joy 2 |
|---|---|---|
| Ichki sir | backend `INTERNAL_SECRET` | frontend `INTERNAL_API_SECRET` |
| API porti | server `.env` `API_PORT` | `deploy/nginx/bugun.conf` `upstream bugun_api` |
| Rasm hajmi | `MEDIA_MAX_UPLOAD_BYTES` | nginx `client_max_body_size` (11m = 10 MiB + multipart) |
| Sayt manzili | backend `WEB_URL` | frontend `NEXT_PUBLIC_SITE_URL` |
| API manzili | backend `API_URL` | frontend `NEXT_PUBLIC_API_URL` |
| Proxy manzillari | `TRUSTED_PROXIES` | nginx qayerdan ulanishi (host yoki Docker ko'prigi) |
