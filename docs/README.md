# Bugun — texnik hujjatlar

Bu papkada Bugun platformasi qanday ishlashi to'liq yozilgan: loyiha g'oyasi, arxitektura, narx va tahlil metodologiyasi (formulalar, chegaralar, algoritmlar), savdo va talab mexanizmlari, Telegram bot, veb-sayt, API, ma'lumotlar xavfsizligi, ishga tushirish va sozlamalar. Hakamlar uchun qisqa kirish — ildizdagi [README](../README.md). Barcha raqamlar (vaznlar, chegaralar, vaqtlar, limitlar) kodning o'zidan olingan. `spec.md` bilan farq bo'lgan joylarda kod asos qilib olingan va farq alohida qayd etilgan.

## Hujjatlar

| # | Fayl | Nima haqida |
|---|---|---|
| 1 | [Loyiha haqida](01-loyiha-haqida.md) | Muammo, yechim, rollar, asosiy oqimlar, tamoyillar, funksiyalar xaritasi |
| 2 | [Arxitektura](02-arxitektura.md) | Komponentlar, so'rov oqimi, Go paketlari, rejali vazifalar va lease'lar, tashqi manbalar, vaqt zonasi, model funksiyalari |
| 3 | [Narx metodologiyasi](03-narx-metodologiyasi.md) | Narx manbalari va vaznlari, vaznli median, hudud kengayishi, anomaliya tekshiruvi, bozordagi o'rin |
| 4 | [Tahlil metodologiyasi](04-tahlil-metodologiyasi.md) | Kunlik tarix, trend, mavsum, prognoz, maslahat, bosim indeksi, arbitraj, viloyat sahifasi, issiqlik xaritasi, hisobot |
| 5 | [Savdo va talab](05-savdo-va-talab.md) | RFQ, savdolar, reyting, kuzatuv, talab signallari, e'lon voronkasi |
| 6 | [Avtomatik narx](06-avtomatik-narx.md) | Avtomatik narx algoritmi qadamma-qadam va misol bilan |
| 7 | [Telegram bot](07-telegram-bot.md) | Buyruqlar (`/elon` Mini App), narx parseri, ovoz va rasm, `/savol`, kanallar, limit, rejali xabarlar |
| 8 | [Veb-sayt](08-veb-sayt.md) | Sahifalar, navigatsiya, tillar, mavzu, grafiklar, Mini App, CSP va SEO |
| 9 | [API](09-api.md) | Qoidalar, 82 ta endpoint katalogi, xato kodlari |
| 10 | [Ma'lumotlar va xavfsizlik](10-malumotlar-va-xavfsizlik.md) | Jadvallar, ER diagramma, shaxsiy ma'lumotlar, sessiyalar, shifrlash, rasmlarga kirish, himoya qatlamlari, Sentry, zaxira, migratsiyalar mosligi |
| 11 | [Ishga tushirish](11-ishga-tushirish.md) | Lokal sozlash, testlar va CI, Docker, serverni sozlash, deploy va orqaga qaytish, zaxira va tiklash, monitoring, muammolar |
| 12 | [ML usullari](12-ml-usullari.md) | Robust statistika, prognoz usullari musobaqasi, Shapley, kunlik anomaliyalar, Isolation Forest, K-Means, Local Moran I, monitoring, ixtiyoriy til modellari (tahlilchi, ovoz, rasm, moslash, moderatsiya) va ularning o'lchovi |
| 13 | [Sozlamalar](13-sozlamalar.md) | Barcha muhit o'zgaruvchilari, standart qiymatlar va ma'nosi |

## Kim nimani o'qisin

- **Hakamlar va birinchi tanishuv:** 1 → 3 → 4 (bosh qismlari) → 8.
- **Metodologiya (nazariy tomon):** 3 → 4 → 12 → 6 → 5.
- **Yangi backend dasturchi:** 2 → 10 → 9 → 11 → 13, keyin kerakli metodologiya fayli.
- **Serverni boshqaruvchi:** 11 → 13 → 10.
- **Yangi frontend dasturchi:** 8 → 9 → 11.
- **Bot bilan ishlovchi:** 7 → 5.

## Asosiy atamalar

| Atama | Ma'nosi |
|---|---|
| Kuzatuv (`price_observations`) | Bitta narx yozuvi: mahsulot, narx, savdo turi, hudud, manba, vaqt |
| Vaznli median | Narxlar tartiblanib, vaznlar yig'indisi yarmiga yetgan narx. Vazn manbaga ishonch va narx yangiligidan kelib chiqadi |
| P25 / P75 | Narxlarning pastki va yuqori choragi; "yarmi shu oraliqda" |
| Ulgurji / chakana | `ulgurji` / `chakana` — narxlar hech qachon aralashtirilmaydi |
| Hudud kengayishi | Tumanda narx kam bo'lsa viloyat, u ham kam bo'lsa respublika bo'yicha hisoblash |
| Tekshiruv (`review`) | Keskin farq qilgan narx; admin tasdiqlaguncha medianga kirmaydi |
| Bozordagi o'rin | Sotuvchi narxining viloyatdagi boshqa narxlar orasidagi tartibi (1 = eng arzon) |
| RFQ | Xaridorning narx so'rovi; mos sotuvchilar taklif beradi |
| Bosim indeksi | Talab o'sishining taklif o'sishiga nisbati; zonalar: taklif ko'p, muvozanat, talab yuqori |
| `demo_share` | Hisobdagi demo (namuna) ma'lumot ulushi; 0 dan katta bo'lsa saytda "Demo ma'lumot" belgisi chiqadi |
| Toshkent kuni | Barcha kunlik hisoblar `Asia/Tashkent` bo'yicha; bazada vaqt UTC da |

## Ma'lum cheklovlar va ochiq masalalar

Hujjatlarni yozishda kodda quyidagilar aniqlandi. Batafsili tegishli fayllarning oxirgi bo'limlarida.

**Narx va tahlil**
- Tarixiy tahlillarning ko'pi hozircha sintetik demo tarix ustida (`demo_share` bilan belgilangan).
- UzEx konnektori faqat 3 mahsulotni (bug'doy, armatura, sement) qamraydi va standart o'chiq; narxtahlil konnektori yo'q. Shu sababli "rasmiy narxdan uzoq" tekshiruvi ko'p mahsulotda ishlamaydi ([02](02-arxitektura.md#8-tashqi-malumot-manbalari)).
- Tekshiruvga tushgan savdo narxini admin tasdiqlash yo'li yo'q ([03](03-narx-metodologiyasi.md)).
- Bir sotuvchining o'zining boshqa narxlari (boshqa nav, RFQ taklifi) bozordagi o'rinda raqib sifatida sanaladi ([03](03-narx-metodologiyasi.md)).
- `price_daily` har soatda faqat kecha va bugun uchun qayta quriladi. Keyinroq rad etilgan eski narx tarixda qoladi ([04](04-tahlil-metodologiyasi.md)).
- "14 kunda 5 ta narx" qoidalari aslida "narx-kun"larni sanaydi ([04](04-tahlil-metodologiyasi.md)).
- Bosim indeksida talab ulgurji va chakana uchun birga, taklif esa alohida hisoblanadi ([04](04-tahlil-metodologiyasi.md)).

**Savdo, kuzatuv va bot**
- Savdoning `disputed` holati hech qayerda o'rnatilmaydi ([05](05-savdo-va-talab.md)).
- RFQ muddati o'tgach ham taklifni tanlash va unga rozilik berish mumkin. Faqat bekor qilish buni to'xtatadi ([05](05-savdo-va-talab.md)).
- RFQ dagi maksimal narx sotuvchiga ko'rsatiladi, lekin qattiq cheklov sifatida tekshirilmaydi ([05](05-savdo-va-talab.md)).
- Botda narx "optom/ulgurji" so'zisiz yozilsa chakana deb olinadi ([07](07-telegram-bot.md)).
- Bot javoblari faqat o'zbek tilida.

**Tizim**
- Bitta VPS: API, bot va rejalashtiruvchi bitta jarayonda, Postgres replikasiz; sayt Vercel `fra1` da ([10](10-malumotlar-va-xavfsizlik.md#13-malum-kamchiliklar)).
- Model funksiyalari kalit talab qiladi va standart o'chiq; o'chiq bo'lsa saytdagi ovoz va rasm tugmalari ko'rinmaydi ([08](08-veb-sayt.md)).

## Tegishli fayllar

- [`../spec.md`](../spec.md) — texnik topshiriq va qarorlar tarixi.
- [`../HOLAT.md`](../HOLAT.md) — bajarilgan ishlar holati.
- [`../README.md`](../README.md) — loyiha haqida qisqacha, demo yo'li, ishga tushirish.
- [`../backend/openapi.json`](../backend/openapi.json) — API ning mashina o'qiydigan tavsifi.
