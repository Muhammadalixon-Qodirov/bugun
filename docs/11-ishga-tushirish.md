# Ishga tushirish, test va deploy

Bu hujjat Bugun'ni kompyuterda noldan ishga tushirishni, testlarni, build'ni, production serverni bir martalik sozlashni, deploy va orqaga qaytishni, zaxira va tiklashni hamda monitoringni tushuntiradi. Backend — Go dasturi va PostgreSQL, frontend — Next.js. Hujjatda muhit o'zgaruvchilarining faqat **nomlari** bor; haqiqiy qiymatlar hech qachon repozitoriyga yozilmaydi. Barcha o'zgaruvchilarning to'liq ro'yxati: [sozlamalar](13-sozlamalar.md).

## Mundarija

1. [Kerakli dasturlar](#1-kerakli-dasturlar)
2. [Backend: lokal sozlash](#2-backend-lokal-sozlash)
3. [Demo ma'lumotlar (seed)](#3-demo-malumotlar-seed)
4. [Frontend: lokal sozlash](#4-frontend-lokal-sozlash)
5. [Testlar va CI](#5-testlar-va-ci)
6. [Build va Docker image'lar](#6-build-va-docker-imagelar)
7. [Production server](#7-production-server)
8. [Deploy va orqaga qaytish](#8-deploy-va-orqaga-qaytish)
9. [Frontend (Vercel)](#9-frontend-vercel)
10. [Zaxira va tiklash](#10-zaxira-va-tiklash)
11. [Monitoring](#11-monitoring)
12. [Admin qo'shish](#12-admin-qoshish)
13. [Foydali buyruqlar](#13-foydali-buyruqlar)
14. [Muammolar va yechimlar](#14-muammolar-va-yechimlar)
15. [Qayerda joylashgan](#qayerda-joylashgan)

## 1. Kerakli dasturlar

| Dastur | Versiya | Nima uchun |
|---|---|---|
| Go | 1.26 (`backend/go.mod`) | API, bot, migratsiya, seed |
| PostgreSQL | 16 (productionda `postgres:16-alpine`, digest bilan qotirilgan) | Baza; `pg_trgm` kengaytmasi kerak (migratsiya o'zi yoqadi) |
| Node.js va npm | LTS | Frontend |
| Telegram bot va OIDC mijozi | @BotFather'da | Kirish va bot. Ularsiz ham API ishlaydi, faqat kirish `/kirish?xato=sozlanmagan` ga qaytadi va bot o'chiq bo'ladi |
| Docker, nginx, certbot | Faqat serverda | Production |

## 2. Backend: lokal sozlash

1. Bazani yarating: `createdb bugun`.
2. `backend/.env.example` dan `backend/.env` yarating (`.env` `.gitignore` da).
3. Shifrlash kalitini yarating va `DATA_ENCRYPTION_KEY` ga qo'ying: `openssl rand -base64 32`.
4. Ishga tushiring:
   ```bash
   cd backend
   make run          # go run ./cmd/api
   ```
5. Tekshiring: `curl -i http://localhost:8080/healthz` → `200 {"status":"ok","database":"ok"}`. Baza javob bermasa **503** va `"database":"unavailable"` — Docker healthcheck, deploy tekshiruvi va tashqi monitor shu kodga qaraydi.

**Migratsiyalar.** Lokalda `MIGRATE_ON_START=true` (standart): API ishga tushganda goose barcha migratsiyalarni (`backend/migrations/`, binar ichida) qo'llaydi. Migratsiya Postgres advisory lock ostida ishlaydi, shuning uchun bir vaqtda ikki jarayon (masalan API va `cmd/migrate`) migratsiya qilolmaydi — ikkinchisi birinchisini kutadi. Alohida qadam ham bor:

```bash
make migrate                      # go run ./cmd/migrate up
go run ./cmd/migrate status       # har migratsiya: qo'llangan vaqti yoki "pending"
```

API konteyneri productionda `MIGRATE_ON_START=false` bilan ishlaydi — migratsiyani deploy alohida qiladi (8-bo'lim).

Lokal Telegram kirishi uchun callback manzili (`API_URL/v1/auth/telegram/callback`) internetdan ko'rinishi kerak; frontend `next.config.ts` da `*.trycloudflare.com` dev-tunnel manbasiga ruxsat bor.

## 3. Demo ma'lumotlar (seed)

`cmd/seed` demo sotuvchilar, e'lonlar, rasmlar va ixtiyoriy narx tarixini yaratadi. U o'sha `.env` ni o'qiydi, migratsiyalarni qo'llaydi va har safar avval eski demo ma'lumotni o'chiradi (qayta ishga tushirish xavfsiz).

```bash
cd backend
go run ./cmd/seed                            # 150 sotuvchi, 1000 e'lon, rasmlar bilan
go run ./cmd/seed -history 3                 # + 3 yillik demo narx tarixi (tahlil sahifalari uchun)
go run ./cmd/seed -history 3 -history-only   # faqat tarixni almashtirish
go run ./cmd/seed -purge                     # barcha demo ma'lumotni o'chirish
```

| Bayroq | Standart | Ma'nosi |
|---|---|---|
| `-listings` | 1000 | Demo e'lonlar soni |
| `-sellers` | 150 | Demo sotuvchilar soni |
| `-no-images` | false | Rasmlarni yuklamaslik |
| `-cache` | `/tmp/bugun-seed-images` | Rasm keshi papkasi |
| `-seed` | 2026 | Tasodifiylik urug'i (bir xil natija uchun) |
| `-history` | 0 | Necha yillik demo tarix (`price_daily`, `synthetic = true`) |
| `-history-only` | false | Sotuvchi va e'lonlarga tegmasdan faqat tarixni yangilash |
| `-drop` | bo'sh | Keshdagi ma'lum rasmlarni (`slug/NN.jpg`) demo e'lonlardan va keshdan olib tashlash |
| `-purge` | false | Demo ma'lumotni o'chirib chiqish |
| `-photos-only` | false | Faqat demo e'lonlar rasmlarini qo'lda tanlangan rasmlarga almashtirish |

Demo akkauntlar `users.is_demo = true`, soxta raqamlar (`+998 90 000 xx xx`) bilan va Telegram username'siz. Rasmlar har mahsulot uchun qo'lda tanlangan: faqat fotosuratlar, CC0 yoki public domain, manba sahifasi yozilgan (`backend/cmd/seed/photos_data.go`). Tahlilda demo ulushi `demo_share` sifatida qaytadi va sayt "Demo ma'lumot" belgisini ko'rsatadi.

Serverda seed alohida image bilan ishlaydi (production image'da `seed` yo'q), 13-bo'limga qarang.

## 4. Frontend: lokal sozlash

```bash
cd frontend
npm install
cp .env.example .env.local   # lokalda hammasi bo'sh qolishi mumkin
npm run dev                  # http://localhost:3000
```

Developmentda bo'sh manzillar lokal standartlarga tushadi (`API_INTERNAL_URL=http://localhost:8080`, `NEXT_PUBLIC_API_URL=/api`, sayt `http://localhost:3000`). **Production build yoki server** `API_INTERNAL_URL` yoki sayt manzilisiz ishga tushmaydi (`NEXT_PUBLIC_API_URL` bo'lmasa `/api`) — xato bilan to'xtaydi (`src/lib/env.ts`). O'zgaruvchilar jadvali: [sozlamalar, 10-bo'lim](13-sozlamalar.md#10-frontend).

| Buyruq | Nima qiladi |
|---|---|
| `npm run dev` | Dev server |
| `npm run build`, `npm run start` | Production build va uni ishga tushirish |
| `npm run lint` | ESLint |
| `npm run api:gen` | `../backend/openapi.json` dan `src/lib/api/gen/` TypeScript mijozini qayta yaratadi |
| `npm run i18n:index` | Har til uchun `src/messages/<til>/index.ts` ni yaratadi |
| `npm run i18n:check` | `en` va `ru` tarjimalari `uz` bilan bir xil kalit, argument va teglarga egaligini tekshiradi |
| `npm run test:e2e` | Playwright smoke testlari (5-bo'lim) |

API o'zgarsa tartib: backendda `make openapi`, keyin frontendda `npm run api:gen`, ikkala natija bitta commit'da. CI ikkalasining yangiligini tekshiradi.

## 5. Testlar va CI

### Backend

```bash
cd backend
make test       # go test ./...  — bazasiz testlar; bazali testlar o'tkazib yuboriladi
make vet        # go vet ./...
createdb bugun_test
make test-db    # TEST_DATABASE_URL=postgres://localhost:5432/bugun_test?sslmode=disable go test -race -p 1 ./...
```

Nega `-p 1`: barcha paketlar bitta `bugun_test` bazasini ishlatadi va jadvallarni tozalaydi; parallel ishlasa bir-birining ma'lumotini o'chiradi. `-race` poyga holatlarini qidiradi. Model funksiyalari testlari hech qachon haqiqiy modelga murojaat qilmaydi: `internal/aitest` soxta OpenAI-mos server beradi.

### AI o'lchovi

```bash
cd backend
make eval        # go run ./cmd/eval
```

`backend/eval/` dagi belgilangan to'plamlarda (narx xabarlari `parser.json`, katalog so'zlari `match.json`, tahlilchi savollari `routing.json`) qoidali yo'llarni o'lchaydi va natijani `backend/eval/RESULTS.md` ga yozadi. Katalog bazadan o'qiladi (`EVAL_DATABASE_URL`, bo'lmasa `DATABASE_URL`; migratsiya qilingan bo'lishi kerak, hech narsa yozilmaydi). `LLM_*` yoki `EMBEDDINGS_*` berilsa model yo'li ham solishtiriladi. CI eval'ni ishga tushirmaydi.

### Frontend (Playwright)

```bash
cd frontend
npx playwright install chromium       # bir marta
E2E_BASE_URL=http://localhost:3000 npm run test:e2e
```

Testlar ishlab turgan sayt va API'ga (seed bilan) qarshi ishlaydi, ikki qurilmada: desktop va 390 px telefon. Tekshiriladi: bosh sahifa va katalog, mahsulot va tahlil sahifalari, kirmagan foydalanuvchi himoyalangan sahifalardan `/kirish` ga yo'naltirilishi, `/admin` 404, til almashishi, har sahifada nonce'li Content-Security-Policy, 390 px'da gorizontal aylanmaslik, Telegram Mini App skripti CSP ostida ishlashi, limitlar API'dan olinishi va demo belgilari (`frontend/e2e/`).

### CI (`.github/workflows/ci.yml`)

Har push va pull request'da beshta job:

| Job | Nima qiladi |
|---|---|
| Backend | `go vet`, Postgres service bilan `go test -race -p 1 ./...`, `openapi.json` va sqlc kodi yangiligi |
| Frontend | `npm run api:gen` natijasi commit qilinganmi, `tsc --noEmit`, lint, `i18n:check`, `npm run build` |
| E2E | API, `migrate`, `seed` (300 e'lon, 1 yil tarix, rasmsiz) yig'iladi va ishga tushadi, sayt build qilinadi, Playwright; xatoda hisobot va loglar artefakt sifatida |
| Docker | `runtime` va `seed` target'lari yig'iladi; `runtime` ichida `api` va `migrate` bor, `seed` yo'qligi tekshiriladi |
| Secrets | gitleaks butun git tarixini tekshiradi (`.gitleaks.toml`) |

E2E uchun kalitlar (`DATA_ENCRYPTION_KEY`, ichki sir) har ishga tushishda yangidan yaratiladi, repoda kalitga o'xshash qiymat yo'q.

## 6. Build va Docker image'lar

- Backend: `make build` → `backend/bin/api` va `backend/bin/migrate`.
- Frontend: `npm run build`.
- `backend/Dockerfile` — ko'p bosqichli, asosiy image'lar **digest bilan qotirilgan** (`golang:1.26-alpine`, `alpine:3.22`; teg yangilansa digest ham birga yangilanadi). Statik binarlar (CGO o'chiq), `app` (uid 10001) foydalanuvchisi, `APP_ENV=production`, `HTTP_ADDR=:8080`, `MEDIA_DIR=/data/media`.

| Target | Ichida | Qachon |
|---|---|---|
| `runtime` (standart) | `api`, `migrate` | Production. `HEALTHCHECK` har 30 s `/healthz` ni tekshiradi (503 da yiqiladi). `org.opencontainers.image.revision` = git SHA |
| `seed` | `seed` | Faqat so'ralganda, demo ma'lumot uchun; productionga hech qachon chiqmaydi |

## 7. Production server

### Tuzilma

```
/srv/bugun/                 deploy foydalanuvchisiga tegishli, 750
├── app/                    git checkout (DEPLOY_DIR)
├── .env                    server sirlari, 600 (BUGUN_ENV_FILE; shablon: deploy/.env.example)
└── backups/                zaxira nusxalar (BACKUP_DIR)
```

`deploy/docker-compose.yml` (loyiha nomi `bugun`):

- `postgres` — `postgres:16-alpine` (digest), ma'lumot `pgdata` hajmida, `TZ: UTC`, healthcheck `pg_isready`.
- `api` — `bugun-api:${API_IMAGE_TAG}`, `.env` ni oladi, faqat `127.0.0.1:${API_PORT:-8090}` ga ochiladi, rasmlar `media` hajmida, `MIGRATE_ON_START=false`, `TRUSTED_PROXIES=172.16.0.0/12` (nginx konteynerga Docker ko'prigi orqali keladi).
- `migrate`, `seed` — `tools` profilida, faqat qo'lda yoki deploy ichida ishga tushadi.
- Loglar `json-file`, 10 MB × 3 fayl.

Hajmlar compose loyihasiga (`bugun`) tegishli, papkaga emas — checkout'ni ko'chirish ma'lumotga tegmaydi.

### Bir martalik sozlash

Quyidagilar serverda bir marta bajariladi (Ubuntu LTS misolida). Hammasi `deploy/deploy.sh`, `deploy/nginx/bugun.conf` va `deploy/systemd/bugun-backup.service` sarlavhalaridagi izohlarda ham yozilgan.

1. **Docker va deploy foydalanuvchisi.** Skriptlar root talab qilmaydi — `docker` guruhidagi foydalanuvchi yetarli (eslatma: `docker` guruhi hostda amalda root'ga teng huquq beradi).
   ```bash
   sudo adduser --disabled-password --gecos "" deploy
   sudo usermod -aG docker deploy
   sudo install -d -o deploy -g deploy -m 750 /srv/bugun
   sudo -u deploy git clone <repository> /srv/bugun/app
   sudo -u deploy install -m 600 /srv/bugun/app/deploy/.env.example /srv/bugun/.env
   ```
   Keyin `/srv/bugun/.env` ni to'ldiring: kamida `POSTGRES_PASSWORD`, `DATA_ENCRYPTION_KEY`, `WEB_URL`, `API_URL`, `COOKIE_DOMAIN`, `INTERNAL_SECRET`, Telegram kalitlari.

   Eski o'rnatishni (`/root/Bugun`) ko'chirish: `sudo mv /root/Bugun /srv/bugun && sudo chown -R deploy:deploy /srv/bugun`.

2. **TLS sertifikati (Let's Encrypt).** nginx konfiguratsiyasi sertifikat mavjud deb hisoblaydi, shuning uchun avval sertifikat olinadi:
   ```bash
   sudo mkdir -p /var/www/certbot
   sudo certbot certonly --nginx -d api.example.uz --deploy-hook "systemctl reload nginx"
   ```
   Yangilash certbot'ning o'z timeri bilan; `--deploy-hook` yangilangandan keyin nginx'ni qayta yuklaydi.

3. **nginx.** `__SERVER_NAME__` o'rniga API domeni qo'yiladi:
   ```bash
   sudo cp deploy/nginx/bugun-proxy.inc deploy/nginx/bugun-security.inc /etc/nginx/
   sudo sed 's/__SERVER_NAME__/api.example.uz/g' deploy/nginx/bugun.conf > /etc/nginx/sites-available/bugun
   sudo ln -sf /etc/nginx/sites-available/bugun /etc/nginx/sites-enabled/bugun
   sudo nginx -t && sudo systemctl reload nginx
   ```
   Konfiguratsiya: 80-port faqat ACME va HTTPS'ga 301; 443 — TLS 1.2/1.3 (Mozilla intermediate), HSTS, xavfsizlik sarlavhalari, gzip, `limit_req`/`limit_conn`, `client_max_body_size 11m`. `API_PORT` o'zgarsa `upstream bugun_api` ham o'zgartiriladi. Batafsil: [ma'lumotlar va xavfsizlik, 9-bo'lim](10-malumotlar-va-xavfsizlik.md#9-tarmoq-himoyasi-va-limitlar).

4. **Kunlik zaxira (systemd timer).** Har kuni 21:30 UTC (02:30 Toshkent), server o'chiq bo'lsa keyin quvib yetadi:
   ```bash
   sudo cp deploy/systemd/bugun-backup.{service,timer} /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now bugun-backup.timer
   systemctl list-timers bugun-backup.timer
   ```
   Checkout `/srv/bugun/app` da bo'lmasa, service faylidagi `User` va yo'llarni moslang. Cron muqobili `deploy/backup.sh` sarlavhasida.

5. **Offsite nusxa (ixtiyoriy).** `deploy` foydalanuvchisi sifatida `rclone config` bilan remote yarating va `.env` ga `RCLONE_REMOTE` (masalan `b2:bugun-backups/prod`) yozing.

6. **Monitorlar** — 11-bo'lim.

7. **Birinchi deploy:** `sudo -u deploy /srv/bugun/app/deploy/deploy.sh`. Demo ma'lumot kerak bo'lsa — 13-bo'limdagi seed buyrug'i.

## 8. Deploy va orqaga qaytish

O'z kompyuteringizdan:

```bash
deploy/remote-deploy.sh          # BUGUN_SSH_HOST (standart bugun-server), BUGUN_REMOTE_DIR (standart Bugun/app)
```

U ish papkasi toza va `main` da ekanini tekshiradi, `main` ni GitHub'ga push qiladi va SSH orqali serverda `deploy.sh` ni ishga tushiradi. `/srv/bugun` tuzilmasida `BUGUN_REMOTE_DIR=/srv/bugun/app` bering.

Serverda `deploy/deploy.sh` qadamlari:

1. Checkout'da lokal o'zgarish bo'lsa yoki branch `DEPLOY_BRANCH` bo'lmasa — **rad etadi**. `git fetch` va faqat **fast-forward** (`git reset --hard` yo'q).
2. Postgres ishlayotgan bo'lsa — `deploy/backup.sh` (baza + rasmlar).
3. `bugun-api:<git SHA 12 belgi>` image'ini yig'adi (`--pull`, `GIT_SHA` build argumenti).
4. **Migratsiyalar alohida qadam:** `docker compose run --rm migrate up` yangi image bilan. Xato bo'lsa eski API ishlashda davom etadi, checkout oldingi commit'ga qaytariladi, deploy 1 bilan tugaydi.
5. Yangi API'ni ishga tushiradi va `HEALTH_TIMEOUT` (standart 60) soniyagacha `GET /healthz` = 200 ni kutadi.
6. **Sog'lom:** image `bugun-api:latest` deb ham teglanadi, oxirgi `KEEP_IMAGES` (standart 5) SHA teg saqlanadi, qolganlari o'chiriladi.
   **Sog'lom emas:** oxirgi 80 qator log chiqariladi, checkout oldingi commit'ga qaytariladi va **avtomatik ravishda** `bugun-api:latest` (oxirgi sog'lom image) qayta ishga tushiriladi. Deploy baribir xato (1) bilan tugaydi.

**Qo'lda oldingi versiyaga qaytish:** `docker image ls bugun-api` — saqlangan SHA teglar; keyin `API_IMAGE_TAG=<teg> docker compose -p bugun -f deploy/docker-compose.yml --env-file /srv/bugun/.env up -d api`.

**Migratsiya qoidasi.** Orqaga qaytish migratsiyalarni **qaytarmaydi**: eski image yangi sxema bilan ishlaydi. Shuning uchun har relizdagi migratsiya oldingi reliz kodi bilan mos bo'lishi kerak — faqat qo'shish (yangi jadval, `NULL` bo'lishi mumkin yoki standartli ustun, indeks). Ustun yoki jadvalni o'chirish va qayta nomlash ikki relizga bo'linadi: avval kod ishlatmay qo'yadi, keyingi relizda o'chiriladi.

## 9. Frontend (Vercel)

- Sayt Vercel'da, `fra1` mintaqasida (`frontend/vercel.json`), ildiz papkasi `frontend`.
- Vercel muhit o'zgaruvchilari (Production):

| O'zgaruvchi | Qiymat |
|---|---|
| `API_INTERNAL_URL` | API manzili, masalan `https://api.example.uz` |
| `NEXT_PUBLIC_API_URL` | Brauzer uchun API: `/api` (sayt proxy'si) yoki API subdomeni |
| `NEXT_PUBLIC_SITE_URL` | Saytning ochiq manzili (bo'sh bo'lsa Vercel production domeni) |
| `INTERNAL_API_SECRET` | Backenddagi `INTERNAL_SECRET` bilan bir xil |
| `NEXT_PUBLIC_TELEGRAM_BOT_USERNAME` | Bot nomi |
| `NEXT_PUBLIC_SENTRY_*`, `SENTRY_AUTH_TOKEN`, `SENTRY_ORG`, `SENTRY_PROJECT` | Ixtiyoriy, xatolarni kuzatish |

`NEXT_PUBLIC_*` qiymatlari build vaqtida qotadi — o'zgartirilgandan keyin qayta deploy kerak.

- Backend tomonda: `WEB_URL` — sayt manzili, `API_URL` — API manzili, `COOKIE_DOMAIN` — umumiy ota-domen, `INTERNAL_SECRET` — Vercel'dagi `INTERNAL_API_SECRET` bilan bir xil.
- Telegram Mini App uchun @BotFather'da bot Mini App manzili sayt domeni bo'lishi kerak; bot `/elon` bilan `WEB_URL/mahsulotlarim/yangi` ni Mini App sifatida ochadi (faqat `https`).

## 10. Zaxira va tiklash

### Zaxira (`deploy/backup.sh`)

- **Baza:** `pg_dump --format=custom` → `backups/bugun-YYYYMMDD-HHMMSS-db.dump`. Yozilgandan keyin har blok o'qib tekshiriladi (`pg_restore -f /dev/null`) — kesilgan arxiv sezilmay qolmaydi.
- **Rasmlar:** `bugun_media` hajmi read-only vaqtinchalik konteyner orqali → `bugun-…-media.tar.gz`, arxiv tekshiriladi.
- `BACKUP_RETENTION_DAYS` (14) kundan eski lokal nusxalar o'chiriladi; `RCLONE_REMOTE` bo'lsa yangi fayllar offsite nusxalanadi va `RCLONE_RETENTION_DAYS` dan eskilari o'chiriladi.
- Fayllar `umask 077` bilan (faqat egasi o'qiydi). Bir vaqtda bitta zaxira (`flock`).
- Muvaffaqiyatda `BACKUP_HEARTBEAT_URL` ga ping, xatoda `<url>/fail`, skript nol bo'lmagan kod bilan tugaydi.
- Qachon: systemd timer (kuniga bir marta) va har deploy'dan oldin.

### Tiklash (`deploy/restore.sh`)

```bash
deploy/restore.sh --db backups/bugun-20260926-023000-db.dump \
                  --media backups/bugun-20260926-023000-media.tar.gz
```

**Buzuvchi amal:** joriy baza va/yoki rasmlar almashtiriladi, shuning uchun `restore` deb yozib tasdiqlash so'raladi (`--yes` bilan so'ralmaydi). Tartib: API to'xtatiladi → baza qayta yaratiladi va `pg_restore --single-transaction` → rasmlar hajmi tozalanib arxivdan yoziladi (egasi uid 10001) → `migrate up` (eski dump yangi kodga yetkaziladi) → API ishga tushadi. Faqat bittasini (`--db` yoki `--media`) berish mumkin; eski `.sql.gz` dumplar ham qabul qilinadi. Offsite nusxani avval yuklab oling: `rclone copy "$RCLONE_REMOTE/<fayl>" backups/`.

### Tiklash sinovi (2026-09-26)

Production'ga o'xshash baza bilan to'liq aylanish sinab ko'rildi: dump 14 MB, taxminan 4 soniyada olindi; tiklash taxminan 9 soniya davom etdi; 35 ta jadvalning hammasida qatorlar soni asl bazadagi bilan mos keldi (masalan `price_daily` 1 440 544, `listings` 317, `users` 69). Sinovni har katta o'zgarishdan keyin (masalan Postgres versiyasi) takrorlash tavsiya etiladi.

## 11. Monitoring

Ilova ichidagi monitoring (`monitoring` ishi, adminlarga botda ogohlantirish, `/v1/admin/monitoring`) jarayon o'lsa o'zini xabar qila olmaydi, shuning uchun tashqi tekshiruvlar ham kerak:

| Nima | Qayerda | Sozlash |
|---|---|---|
| API va baza tirikmi | UptimeRobot (yoki shunga o'xshash) | `https://<api domeni>/healthz`, har 5 daqiqa; 200 dan boshqa javob — ogohlantirish (baza ishlamasa 503) |
| Rejalashtiruvchi ishlayaptimi | healthchecks.io check | Check URL'ini `HEARTBEAT_URL` ga yozing; davr 5 daqiqa (`HEARTBEAT_INTERVAL`), grace ~10 daqiqa |
| Zaxira bajarildimi | healthchecks.io check | Check URL'ini `BACKUP_HEARTBEAT_URL` ga yozing; davr 1 kun, grace ~2 soat; `/fail` ping darhol ogohlantiradi |
| Ilova xatolari | Sentry (ixtiyoriy) | Backend `SENTRY_DSN`, frontend `NEXT_PUBLIC_SENTRY_DSN`; panic, 5xx va muvaffaqiyatsiz rejali ishlar yuboriladi |

## 12. Admin qo'shish

Eng oddiy yo'l — Telegram id'ni muhitga qo'shish:

1. Foydalanuvchi saytga bir marta Telegram orqali kirsin.
2. Uning Telegram id'sini `ADMIN_TELEGRAM_IDS` ga vergul bilan qo'shing (serverda `/srv/bugun/.env`).
3. API'ni qayta ishga tushiring (`docker compose … up -d api` yoki `deploy/deploy.sh --no-pull`).

Muqobil: bazada `UPDATE users SET role = 'admin' WHERE telegram_id = …;`. Lekin bunday foydalanuvchi profil formasini saqlasa boshqa rol tanlashi kerak bo'ladi va admin huquqini yo'qotadi, shuning uchun birinchi yo'l ishonchliroq ([ma'lumotlar va xavfsizlik](10-malumotlar-va-xavfsizlik.md#11-admin-huquqi)).

`market_admin` va `org_analyst` rollarini admin saytdagi `/admin` sahifasida tasdiqlaydi.

## 13. Foydali buyruqlar

Serverda `C="docker compose -p bugun -f deploy/docker-compose.yml --env-file /srv/bugun/.env"` deb olsak:

| Buyruq | Nima qiladi |
|---|---|
| `make run` / `make build` / `make migrate` | API'ni ishga tushirish / yig'ish / migratsiya |
| `make test`, `make test-db`, `make vet` | Testlar va statik tekshiruv |
| `make sqlc` | `backend/queries/*.sql` dan Go kodini qayta yaratish |
| `make openapi` | `backend/openapi.json` ni yangilash |
| `npm run api:gen` | TypeScript mijozini yangilash |
| `curl -i localhost:8080/healthz` | API va baza holati |
| `psql bugun -c "select job, slot, heartbeat_at, finished_at, error from job_runs order by id desc limit 20"` | Rejali ishlar qachon va qanday bajarilgani |
| `$C logs -f api` | Serverdagi API loglari |
| `$C run --rm migrate status` | Qaysi migratsiyalar qo'llangan |
| `$C --profile tools build seed && $C --profile tools run --rm seed -history 3` | Serverda demo ma'lumot |
| `deploy/backup.sh` | Qo'lda zaxira |
| `deploy/restore.sh --db … --media …` | Zaxiradan tiklash |
| `go run ./cmd/mediasync -dry-run` | Diskdagi rasmlarni S3 ga ko'chirishdan oldin sanash |

## 14. Muammolar va yechimlar

| Belgi | Sabab | Yechim |
|---|---|---|
| `config: required environment variable "DATABASE_URL"` yoki `DATA_ENCRYPTION_KEY must be 32 bytes, base64 encoded` | `.env` yo'q yoki kalit noto'g'ri | `backend/.env` ni to'ldiring; kalitni `openssl rand -base64 32` bilan yarating |
| `config: … must be …` (masalan `JOB_LEASE_TIMEOUT … must be at least twice JOB_HEARTBEAT`) | Sozlama qiymati noto'g'ri | Xabardagi o'zgaruvchini [sozlamalar](13-sozlamalar.md) jadvaliga qarab tuzating |
| Postgres ishga tushmaydi, `postmaster.pid` haqida xabar | Eski `postmaster.pid` qolib ketgan | Postgres jarayoni ishlamayotganiga ishonch hosil qiling (`ps aux \| grep postgres`), keyin `postmaster.pid` ni o'chirib qayta ishga tushiring |
| Lokalda tez-tez 429 `rate_limited` | Frontend `INTERNAL_API_SECRET` siz ishlaydi, server komponentlaridan kelgan so'rovlar bitta IP limitini bo'lishadi | Ikkala tomonda bir xil `INTERNAL_SECRET` / `INTERNAL_API_SECRET`. API logida `via=site` bo'lishi kerak (`site-unverified` — sir mos emas) |
| Productionda hamma bitta IP bo'lib ko'rinadi yoki limit noto'g'ri | nginx manzili `TRUSTED_PROXIES` da yo'q | Compose standarti `172.16.0.0/12`; Docker boshqa pool ishlatsa moslang |
| `next build` yoki `next start`: `… is not set. Production needs it` | Production muhitida manzil berilmagan | `API_INTERNAL_URL` va `NEXT_PUBLIC_SITE_URL` ni o'rnating |
| Kirish `/kirish?xato=sozlanmagan` ga qaytadi | `TELEGRAM_CLIENT_ID` yoki `TELEGRAM_CLIENT_SECRET` bo'sh | @BotFather'dan OIDC ma'lumotlarini qo'ying |
| Kirish `?xato=telegram` | Telegram bilan almashuv xatosi | Callback `API_URL/v1/auth/telegram/callback` BotFather'dagi bilan bir xilligini tekshiring |
| Kirish `?xato=muddat` | Kirish 10 daqiqadan uzoq davom etdi | Qayta kiring |
| Bot javob bermaydi, logda `terminated by other getUpdates request` | Shu token bilan boshqa nusxa ham so'rayapti | Lokalda `BOT_ENABLED=false` yoki alohida test bot tokeni |
| Bot "Juda ko'p xabar yubordingiz" deb javob beradi | Foydalanuvchi daqiqasiga `BOT_MESSAGES_PER_MINUTE` dan ko'p yozdi | Kuting yoki limitni oshiring |
| Brauzerda CORS xatosi | Sayt manbasi `WEB_URL` yoki `CORS_ORIGINS` da yo'q | Manbani qo'shing |
| Kirgandan keyin sayt foydalanuvchini tanimaydi (productionda) | Cookie API domeniga tegishli | `COOKIE_DOMAIN` ni umumiy ota-domenga qo'ying, `WEB_URL` `https://` bilan |
| Tahlil sahifasida yangi narx ko'rinmaydi | `price_daily` har soat, tahlil keshi 10 daqiqa | Kuting yoki API'ni qayta ishga tushiring |
| Kurs yoki ob-havo bo'limi yo'q | `fx_daily`/`weather_daily` hali ishlamagan (birinchi yuklash 3 yil) | `job_runs` va tarmoq ruxsatini tekshiring |
| `deploy.sh`: "migrations failed; the running API was left unchanged" | Migratsiya xatosi | Chiqarilgan xatoni o'qing, tuzatib qayta deploy qiling; ishlab turgan API'ga tegilmagan |
| `deploy.sh`: "… is not healthy" va "rolled back to the previous image" | Yangi versiya `/healthz` = 200 bermadi (`.env` xatosi, baza, panic) | Loglarni o'qing; oldingi image allaqachon ishlayapti |
| `deploy.sh`: "the server checkout has local changes" | Serverda kimdir fayl o'zgartirgan | O'zgarishni tekshiring, keraksiz bo'lsa `git checkout -- .`; server kodni o'zi o'zgartirmasligi kerak |

## Qayerda joylashgan

- `backend/Makefile` — `run`, `build`, `migrate`, `test`, `test-db`, `vet`, `sqlc`, `openapi`
- `backend/.env.example`, `frontend/.env.example`, `deploy/.env.example` — o'zgaruvchilar shablonlari
- `backend/internal/config/` — o'zgaruvchilar, standart qiymatlar va tekshiruv
- `backend/cmd/migrate/`, `backend/cmd/seed/`, `backend/cmd/mediasync/` — yordamchi buyruqlar
- `backend/Dockerfile` — `runtime` va `seed` target'lari
- `deploy/docker-compose.yml`, `deploy/lib.sh`, `deploy/deploy.sh`, `deploy/remote-deploy.sh`, `deploy/backup.sh`, `deploy/restore.sh` — production
- `deploy/nginx/` — `bugun.conf`, `bugun-proxy.inc`, `bugun-security.inc`
- `deploy/systemd/` — `bugun-backup.service`, `bugun-backup.timer`
- `.github/workflows/ci.yml`, `.gitleaks.toml` — CI
- `frontend/package.json`, `frontend/playwright.config.ts`, `frontend/e2e/`, `frontend/vercel.json` — frontend skriptlari, testlar va deploy
