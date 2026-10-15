# Telegram bot

Bot saytdagi profilning tezkor "pulti". Sotuvchi unga oddiy matn bilan narx yuboradi, xaridor so'rov ochadi, har kim narx haqida savol beradi. Bot rejalashtirilgan eslatmalar va xabarlarni ham yuboradi. Botda alohida ro'yxatdan o'tish yo'q: foydalanuvchi saytga Telegram orqali kirganda saqlangan Telegram ID orqali taniladi. Bu hujjat buyruqlarni, matndan narx ajratish qoidalarini (parser), ovozli xabar va rasmdan narxni, `/savol` savol-javobini, kanal va guruhlarni ulashni, limitlarni hamda bot yuboradigan barcha xabarlarni tushuntiradi. Barcha raqamlar `backend/` kodidan olingan.

## Mundarija

1. [Bot va sayt profili](#1-bot-va-sayt-profili)
2. [Buyruqlar](#2-buyruqlar)
3. [Oddiy matn qanday tushuniladi](#3-oddiy-matn-qanday-tushuniladi)
4. [Narx parseri](#4-narx-parseri)
5. [Qoralama va tasdiqlash kartasi](#5-qoralama-va-tasdiqlash-kartasi)
6. [/savol: savol-javob](#6-savol-savol-javob)
7. [RFQ, savdo va reyting botda](#7-rfq-savdo-va-reyting-botda)
8. [Kanallar va guruhlar](#8-kanallar-va-guruhlar)
9. [Rejalashtirilgan xabarlar](#9-rejalashtirilgan-xabarlar)
10. [Bot yuboradigan sayt havolalari](#10-bot-yuboradigan-sayt-havolalari)
11. [E'tibor talab qiladigan joylar](#11-etibor-talab-qiladigan-joylar)
12. [Qayerda joylashgan](#qayerda-joylashgan)

---

## 1. Bot va sayt profili

1. Foydalanuvchi saytga **Telegram orqali kiradi** (OpenID Connect, `backend/internal/auth/telegram.go`). So'raladigan ruxsatlar: `openid`, `profile`, `phone` va `telegram:bot_access`. Oxirgisi botga foydalanuvchiga yozish huquqini beradi, shuning uchun alohida bog'lash qadami kerak emas.
2. Kirishda `users.telegram_id` saqlanadi (`UpsertTelegramUser`).
3. Bot har xabarda yozuvchini `GetUserByTelegramID` bilan topadi (`linkedUser`). Profil topilmasa yoki `bot_enabled = false` bo'lsa, bot "Saytga kirish" tugmasini yuboradi va boshqa hech narsa qilmaydi.
4. `/uzish` `bot_enabled` ni `false` qiladi. Shundan keyin bot bu odamga hech narsa yubormaydi (`sendTo` va barcha joblar `bot_enabled` ni tekshiradi). Qayta ulash saytdagi profil sahifasida (`PUT /v1/me/bot`).

Rol muhim:

| Rol | Botda nima qila oladi |
|---|---|
| `seller` (hududi bor, `market.CanSell`) | narx yuborish, `/mening`, RFQ ga javob, "Sotildi", kanal ulash |
| `buyer` (hududi bor, `rfq.CanBuy`) | `/sorov`, taklif tanlash, savdo savollariga javob, baho |
| rol tanlanmagan | `/start` "Yakunlash" tugmasi bilan saytdagi `/boshlash` sahifasiga yuboradi |

Bot holatni xotirada saqlamaydi. Har tugmaning `callback_data` si kerakli ID'larni o'zida olib yuradi (masalan, `r:new:<mahsulot>:<miqdor>`), matnli javoblar esa botning "So'rov #N", "Savdo #N", "Sotildi #N", "Baho #N" xabarlariga **reply** qilib yoziladi. Shuning uchun server qayta ishga tushganda hech narsa yo'qolmaydi.

---

## 2. Buyruqlar

Buyruqlar ro'yxati (`commands`) bot ishga tushganda Telegram menyusiga yoziladi (`SetMyCommands`).

| Buyruq | Kim uchun | Nima qiladi |
|---|---|---|
| `/start` | hamma | salomlashadi va holatga qarab yo'naltiradi: bog'lanmagan → "Saytga kirish"; bot uzilgan → "Profilni ochish"; rol yo'q → "Yakunlash"; aks holda rolga mos maslahat va "Kabinetni ochish". Menyuda yo'q |
| `/yordam` | hamma | buyruqlar ro'yxati |
| `/narx` | sotuvchi | narxni qanday yozish kerakligi va namuna |
| `/mening` | sotuvchi | e'lonlari (ko'pi bilan 15 qator): narx, savdo turi, zaxira, bozordagi o'rni, "tekshiruvda" yoki "muddati o'tgan" belgisi. Faol narx bozordan **≥ 15%** qimmat bo'lsa ogohlantirish. Faol e'lonlar uchun "sotildi" tugmalari (ko'pi bilan 8) va "Mahsulotlarim" havolasi |
| `/sorov` | xaridor | `/sorov kartoshka 200 kg` — so'rov kartasini tayyorlaydi (7-bo'lim) |
| `/savol` | bog'langan har kim | narx haqidagi savolga javob (6-bo'lim) |
| `/kuzatuv` | bog'langan har kim | kuzatuvlar ro'yxati va o'chirish tugmalari |
| `/kanallar` | bog'langan har kim | ulangan kanal va guruhlar, sozlash tugmalari (8-bo'lim) |
| `/elon` | sotuvchi | "➕ Yangi e'lon" tugmasi saytdagi `/mahsulotlarim/yangi` formasini **Telegram Mini App** sifatida ochadi: rasmlar, narx, miqdor Telegram'dan chiqmasdan. Mini App faqat `https` sayt uchun; lokal `http` saytda oddiy havola beriladi. Sotuvchi bo'lmaganga "Profilni ochish" |
| `/profil` | bog'langan har kim | saytdagi profil havolasi |
| `/uzish` | bog'langan har kim | botni profildan uzadi |

`/mening` dagi o'rin satri: "12 sotuvchi ichida 9-o'rindasiz · o'rtacha 4 200 so'm (+7%)". Bu yerda "o'rtacha" deb viloyatdagi boshqa sotuvchilar narxlarining **medianasi** ataladi (qarang [narx metodologiyasi](03-narx-metodologiyasi.md)).

---

## 3. Oddiy matn qanday tushuniladi

Buyruq bo'lmagan xabar `fallback` ga tushadi. Tekshiruvlar tartibi muhim:

1. "So'rov #N" ga reply → sotuvchining RFQ narxi.
2. "Savdo #N" ga reply → savdo miqdori va narxi.
3. "Sotildi #N" ga reply → "Sotildi" miqdori va narxi.
4. "Baho #N" ga reply → baho izohi.
5. Matn savolga o'xshasa (`looksLikeQuestion`) → `/savol`.
6. Yozuvchi sotuvchi bo'lsa → narx qoralamasi (4–5-bo'limlar).
7. Yozuvchi xaridor bo'lsa → so'rov kartasi (`rfqDraft`).
8. Ovozli xabar yoki audio → matnga o'giriladi va xuddi shu matn yozilgandek 1–7-qadamlardan o'tadi (3a-bo'lim).
9. Rasm → izohida narx bo'lsa izoh matn sifatida o'qiladi; izohsiz (yoki izoh `narx`) bo'lsa sotuvchi uchun narxlar ro'yxati rasmidan qoralama (3a-bo'lim).
10. Boshqa matnsiz xabar (stiker va h.k.) → "Hozircha faqat matnli xabarlarni qabul qilaman".
11. Qolgani → "Bu buyruqni tushunmadim ... /yordam".

**Tezlik limiti.** Har bir Telegram foydalanuvchisi daqiqasiga `BOT_MESSAGES_PER_MINUTE` (standart 20) ta xabar yoki tugma bosishi mumkin (token bucket, bir daqiqada teng to'ladi, `throttle.go`). Oshsa bot bir marta "Juda ko'p xabar yubordingiz. Iltimos, bir daqiqa kutib, qayta urinib ko'ring." deydi (tugmada — ogohlantirish oynasi), keyingi ortiqcha xabarlar jimgina tashlanadi. Bu bitta foydalanuvchi bazani va Telegram'ning umumiy limitini band qilib qo'ymasligi uchun.

**Noma'lum mahsulot so'zi.** Parser mahsulotni tanimasa va katalogga moslash (`internal/catalogmatch`: lug'at, trigram, ixtiyoriy embeddings) ishonchli javob bermasa, bot "🤔 «so'z» — qaysi mahsulot?" deb 4 tagacha nomzod tugmasini beradi; tanlangani bilan xabar qayta o'qiladi.

### 3a. Ovoz va rasm (ixtiyoriy)

Ikkalasi ham standart o'chiq va model kalitini talab qiladi ([sozlamalar, 9-bo'lim](13-sozlamalar.md#9-backend-ixtiyoriy-model-funksiyalari)). O'chiq bo'lsa bot xushmuomala javob beradi va matn bilan yozishni so'raydi.

| Xabar | Nima bo'ladi |
|---|---|
| Ovozli xabar (`STT_*`) | `STT_MAX_SECONDS` (60) dan uzun bo'lsa rad etiladi. Yozuv yuklanib, formati birinchi baytlardan aniqlanadi (`internal/speech`), STT modeliga yuboriladi. Bot tanilgan matnni "🎤 …" deb qaytaradi va uni oddiy matn kabi ishlaydi: sotuvchiga narx kartochkasi, savolga javob. Hech narsa tanilmasa yoki soatlik chegara (`STT_MAX_PER_HOUR`) oshsa — matn bilan yozishni so'raydi |
| Narxlar ro'yxati rasmi (`VISION_MODEL`) | Faqat sotuvchi uchun. Eng katta o'lcham yuklanadi (`MEDIA_MAX_UPLOAD_BYTES` gacha). Model rasm matnini o'qiydi; avval qoidali parser shu matnni o'qiydi, model taklif qilgan har narx kanal postidagidek tekshiriladi (`pricing.Verify`: mahsulot katalogda, birlik ma'lum, raqam rasm matnida aynan yozilgan). Natija "📷 Rasmdan o'qildi" bilan odatiy tasdiq kartochkasi — "To'g'ri" bosilmaguncha hech narsa saqlanmaydi |

Ovozli yozuv ham, rasm ham saqlanmaydi; `ai_log` ga faqat hajm, vaqt va narx yoziladi.

`looksLikeQuestion` qoidasi:

- matnda `?` bo'lsa — savol;
- aks holda matnda raqam bo'lsa — savol **emas** ("kartoshka narxi 4500" narx hisoblanadi);
- aks holda `qancha`, `narxi`, `arzon`, `qaysi`, `kimda`, `qayerda`, `сколько`, `почем` so'zlaridan biri bo'lsa — savol.

Shaxsiy chatdagi tahrirlangan xabarlar qayta o'qilmaydi.

---

## 4. Narx parseri

Parser (`backend/internal/pricing/parser.go`) qoidalarga asoslangan va deterministik: bir xil matn doim bir xil natija beradi, barcha holatlar testlangan (`parser_test.go`). Sayt, bot va kanallar bitta `pricing.Service` dan foydalanadi, shuning uchun bir xil matn hamma joyda bir xil narxga aylanadi. Mahsulotlar katalogi (nomlar, sinonimlar, qadoq o'lchamlari) server ishga tushganda bazadan bir marta yuklanadi (`ListParserProducts`).

### 4.1 Normallashtirish (`Normalize`)

1. Hammasi kichik harfga o'tadi.
2. Kirill harflari lotinga o'giriladi: `ш→sh`, `ч→ch`, `ғ→g`, `ў→o`, `қ→q`, `ҳ→h`, `ё→yo`, `ю→yu`, `я→ya`, `ц→s`, `щ→sh`, `ы→i`, `ь` tashlanadi.
3. Apostrof turlari (`'`, `‘`, `’`, `` ` ``, `ʻ`, `ʼ`, `´`) olib tashlanadi.

Natijada `Ўрик`, `o‘rik` va `o'rik` hammasi `orik` bo'ladi, `MOL GO'SHTI` esa `mol goshti`.

### 4.2 Tokenlarga ajratish

| Token | Misol | Izoh |
|---|---|---|
| so'z | `kartoshka`, `kilosi` | harflar va `-` |
| raqam | `4500`, `4,5`, `4.5` | nuqta/vergul faqat raqamlar orasida |
| `/` | `120000/yashik` | narx birligi |
| qattiq chegara | yangi qator yoki `;` | mahsulotlarni ajratadi |
| vergul | `,` | raqamlarni ajratadi, mahsulotni emas |

Bo'sh joy bilan yozilgan minglar birlashtiriladi: `4 500 → 4500`, `1 200 000 → 1200000` (`mergeThousands`: keyingi guruh aynan 3 raqamli bo'lsa).

### 4.3 Mahsulotni topish va bo'laklash

- Har mahsulotning nomi va sinonimlari normallashtiriladi va **eng uzunidan** boshlab solishtiriladi, shuning uchun `mol goshti` `mol` dan oldin topiladi.
- Oxirgi so'zga qo'shimcha qo'shilishi mumkin: sinonim so'zi kamida 4 harfli bo'lsa, ortiga 4 tagacha harf qo'shilgani ham mos keladi (`kartoshkani`, `piyozdan`).
- Xabar har mahsulot tilga olingan joyda bo'laklarga bo'linadi. Har mahsulot keyingi mahsulot yoki qattiq chegaragacha bo'lgan so'zlarga egalik qiladi. Shuning uchun bitta xabarda bir nechta mahsulot bo'lishi mumkin.
- Birinchi mahsulotdan oldingi so'zlar ("**2 tonna** kartoshka 4500") birinchi mahsulotga qo'shiladi.

### 4.4 Sonlar

`findNumbers` raqamlarni va so'z bilan yozilgan sonlarni qiymatga aylantiradi:

| Yozilishi | Qiymat |
|---|---|
| `bir … toqqiz` | 1…9 |
| `on, yigirma, ottiz, qirq, ellik, oltmish, yetmish, sakson, toqson` | 10…90 |
| `yuz` | ×100 |
| `yarim` | +0.5 |
| `ming` | ×1 000 |
| `mln`, `million`, `mlyon` | ×1 000 000 |

Misollar: `to'rt ming besh yuz = 4 500`, `to'rt yarim ming = 4 500`, `4,5 ming = 4 500`, `115 ming = 115 000`. `ming`/`mln` bor son "ko'paytirilgan" (`multiplied`) deb belgilanadi.

### 4.5 Qaysi son narx, qaysi biri miqdor

Birinchi o'tish — aniq belgilari bor sonlar:

1. Sondan **oldin** egalik shaklidagi birlik bo'lsa (`kilosi`, `kgsi`, `tonnasi`, `qopi`, `yashigi`, `karobkasi`, `lotogi`, `litri`, `donasi` …) — bu shu birlik uchun **narx**.
2. Sondan keyin valyuta so'zi (`som`, `sum`, `somdan`, `sumdan`, `dan`, `somga`, `sumga`) bo'lsa yoki son `ming`/`mln` bilan bo'lsa — **narx**. Undan keyin `/birlik` yoki egalik birligi bo'lsa, narx o'sha birlik uchun.
3. Sondan keyin `/birlik` bo'lsa — shu birlik uchun **narx**.
4. Sondan keyin birlik so'zi bo'lsa (`kg`, `tonna`, `qop` …) — **miqdor**.

Ikkinchi o'tish: qolgan "yalang'och" sonlar bo'sh o'rinni to'ldiradi, **avval narx, keyin miqdor**.

Maxsus holat: bo'lakda bitta son bo'lsa va u birlik bilan yozilgan bo'lsa ("kartoshka 4500 kg"), u shu birlik uchun **narx** hisoblanadi.

Birlik so'zlari:

| Birlik | Yozilishlari |
|---|---|
| `kg` | `kg`, `kilo`, `kila`, `kilogramm`, `kilogram`, `kgdan` |
| `tonna` | `tonna`, `t`, `tn`, `tonnadan` |
| `gramm` | `gramm`, `gr` |
| `qop` | `qop`, `meshok`, `qopdan` |
| `yashik` | `yashik`, `yashikdan` |
| `karobka`, `lotok` | `karobka`, `lotok` |
| `litr` | `litr`, `l`, `litrdan` |
| `dona` | `dona`, `ta`, `donadan` |

### 4.6 "4.5" — minglarda o'qish

Sotuvchilar ko'pincha "4.5" deb 4 500 so'mni nazarda tutadi. Qoida:

```
agar narx < 1000  VA  ming/mln yo'q  VA  (kasr bilan yozilgan YOKI narx < 100):
    narx = narx × 1000
    izoh: «4.5» 4 500 so'm deb tushunildi   (hint: read_as_thousands)
```

Natijalar: `4.5 → 4 500`, `45 → 45 000`, lekin `450` o'zgarmaydi (kasr emas va 100 dan kichik emas). Bu talqin kartada ogohlantirish bilan ko'rsatiladi, sotuvchi uni tasdiqlashi kerak.

### 4.7 Birliklarni asosiy birlikka o'tkazish (`toBase`)

Narx va miqdor mahsulotning asosiy birligida (`base_unit`: `kg`, `litr`, `dona` …) saqlanadi.

| Yozilgan birlik | Qoida |
|---|---|
| bo'sh yoki asosiy birlik | o'zgarmaydi |
| `tonna` (asosiy `kg` bo'lsa) | × 1 000 |
| `gramm` (asosiy `kg` bo'lsa) | ÷ 1 000 |
| qadoq (`qop`, `yashik`, `karobka`, `lotok`) | katalogdagi o'lcham bo'yicha (`unit_conversions` jadvali), masalan 1 qop kartoshka = 50 kg, 1 qop piyoz = 40 kg, 1 yashik pomidor = 20 kg, 1 yashik olma = 18 kg |
| boshqa | xato `unknown_unit` |

Narx `narx_asosiy = round(yozilgan_narx / birlikdagi_asosiy_miqdor)`, miqdor esa `miqdor × birlikdagi_asosiy_miqdor` (3 kasrgacha yaxlitlanadi). Asosiy birlikdan farqli birlik ishlatilgan bo'lsa, izoh qo'shiladi: "1 qop = 50 kg bo'yicha hisoblandi" (hint: `converted`).

### 4.8 Chegaralar

| Tekshiruv | Chegara | Xato kodi |
|---|---|---|
| Narx (asosiy birlikda) | 50 – 50 000 000 so'm | `bad_price` |
| Miqdor (asosiy birlikda) | `0 < q ≤ 10 000 000` | `bad_quantity` |
| Botdagi xabar uzunligi | 2 000 belgi (`maxMessageLen`) | bot: "Xabar juda uzun" |

### 4.9 Ulgurji yoki chakana

Savdo turi **butun xabar** bo'yicha aniqlanadi: xabarda `optom`, `opt`, `ulgurji`, `ulgurjiga`, `optomga`, `optovoy` so'zlaridan biri bo'lsa, barcha mahsulotlar `ulgurji`, aks holda `chakana`.

### 4.10 Muammolar va izohlar (kodlar)

Parser natijasi `items` (tanilgan narxlar) va `problems` (tanilmagan bo'laklar). Har biri o'zbekcha matn bilan birga **kod** ham beradi, shunda sayt uni boshqa tilga tarjima qila oladi.

| Kod | Turi | Qachon | Qo'shimcha maydonlar |
|---|---|---|---|
| `no_price` | problem | mahsulot bor, narx topilmadi | `product`, `product_slug` |
| `unknown_unit` | problem | mahsulot uchun bu qadoqning o'lchami yo'q ("piyoz yashigi") | `unit`, `base_unit` |
| `bad_price` | problem | narx 50 – 50 000 000 oralig'idan tashqarida | `price` |
| `bad_quantity` | problem | miqdor noto'g'ri | — |
| `no_product` | problem | faqat `ParseDemand` (so'rov va savol): mahsulot tanilmadi | — |
| `read_as_thousands` | hint | "4.5" → 4 500 | `written`, `price` |
| `converted` | hint | birlik o'tkazildi | `unit`, `per_base`, `base_unit` |

Hech bir mahsulot tanilmagan xabar ("salom, qalaysiz?") bo'sh natija beradi, xato emas.

### 4.11 Ishlangan misollar

Katalog o'lchamlari yuqoridagidek (kartoshka qop = 50 kg va hokazo).

| Xabar | Natija |
|---|---|
| `kartoshka 4500, 2 tonna` | Kartoshka 4 500 so'm/kg, 2 000 kg, chakana |
| `kartoshka, kilosi to'rt ming besh yuz, ikki tonna bor` | Kartoshka 4 500 so'm/kg, 2 000 kg |
| `piyoz 4,5 ming so'm` | Piyoz 4 500 so'm/kg |
| `piyoz 4.5` | Piyoz 4 500 so'm/kg, izoh «4.5» 4 500 so'm deb tushunildi |
| `kartoshka 4 500 so'm` | Kartoshka 4 500 so'm/kg |
| `kartoshka qopi 200 ming` | 200 000 / 50 = **4 000 so'm/kg**, izoh "1 qop = 50 kg" |
| `kartoshka 4500 so'mdan 10 qop` | 4 500 so'm/kg, miqdor 10 × 50 = **500 kg** |
| `pomidor 120000/yashik` | 120 000 / 20 = **6 000 so'm/kg**, izoh "1 yashik = 20 kg" |
| `kartoshka tonnasi 4 mln` | 4 000 000 / 1 000 = **4 000 so'm/kg** |
| `optom kartoshka 4200 5 tonna` | 4 200 so'm/kg, 5 000 kg, **ulgurji** |
| `Картошка 4500 сўм` | Kartoshka 4 500 so'm/kg |
| `kartoshkani 4300 dan beraman` | Kartoshka 4 300 so'm/kg |
| `mol go'shti 115 ming` | Mol go'shti 115 000 so'm/kg |
| `kartoshka 4500 kg` | 4 500 so'm/kg (birlikli yagona son — narx) |
| `2 tonna kartoshka 4500` | 4 500 so'm/kg, 2 000 kg |
| `kartoshka 4500 piyoz 3800 olma 12000` | uchta narx: 4 500, 3 800, 12 000 |
| `kartoshka 4500` + yangi qator + `piyoz 3800 1 tonna` | ikki narx, piyoz uchun 1 000 kg |
| `kartoshka bor` | `no_price` |
| `piyoz yashigi 100 ming` | `unknown_unit` (piyozda faqat `qop` bor) |
| `kartoshka 900000000` | `bad_price`, `price = 900000000` |

Qadam-baqadam: `kartoshka qopi 200 ming, optom`

1. Normallashtirish: `kartoshka qopi 200 ming, optom`.
2. Tokenlar: `kartoshka` `qopi` `200` `ming` `,` `optom`. `optom` bor, demak **ulgurji**.
3. Mahsulot: `kartoshka`. Uning bo'lagi: `qopi 200 ming , optom`.
4. Sonlar: `200 ming = 200 000` (multiplied).
5. Sondan oldin `qopi` → narx birligi `qop`.
6. `200 000 < 1000` emas, minglarda o'qish yo'q.
7. `toBase(qop) = 50` → `200 000 / 50 = 4 000` so'm/kg. Izoh: "1 qop = 50 kg bo'yicha hisoblandi".
8. 4 000 oraliqda (50 – 50 000 000). Natija: **Kartoshka 4 000 so'm/kg, ulgurji**.

### 4.12 So'rov matni (`ParseDemand`)

Xaridor matni ("kartoshka 200 kg", "200 kg kartoshka kerak", "картошка 500 кг") uchun alohida o'qish bor. Unda narx yo'q: xabardagi **birinchi mahsulot** va uning **birinchi soni** miqdor sifatida olinadi, keyin asosiy birlikka o'tkaziladi. Mahsulot topilmasa `no_product`, noma'lum qadoq bo'lsa `unknown_unit` ("piyoz 3 yashik"). Bu funksiya `/sorov` va `/savol` da ishlatiladi.

---

## 5. Qoralama va tasdiqlash kartasi

Qoida: **tasdiqlanmagan narx hech qachon saqlanmaydi** (`price_observations` ga yozilmaydi). Sabab: parser talqin qiladi ("4.5" → 4 500, qop → kg) va talqin xato bo'lishi mumkin. Noto'g'ri narx bozor medianini buzadi, shuning uchun uni sotuvchi o'zi ko'rib tasdiqlashi shart.

Jarayon:

1. Sotuvchi matn yozadi. `pricing.CreateDraft` uni parser orqali o'tkazadi.
2. Hech narsa tanilmasa — xatolar ("❗ …") va namuna yuboriladi, qoralama yaratilmaydi.
3. Tanilgan bo'lsa — `price_drafts` jadvaliga qoralama yoziladi (xom matn va JSON bandlar), muddati **1 soat** (`draftTTL`).
4. Bot karta yuboradi:

   ```
   Tekshiring:

   1. Kartoshka — 4 000 so'm/kg · 2 000 kg · ulgurji
      ⚠️ 1 qop = 50 kg bo'yicha hisoblandi
   ❗ Piyoz: narx topilmadi. Masalan: «piyoz 4500».

   Hammasi to'g'rimi? Tasdiqlanmagan narx saqlanmaydi.
   [✅ To'g'ri] [✏️ Tuzatish]
   ```

5. **"To'g'ri"** (`p:ok:<id>`) — `pricing.Confirm` bitta tranzaksiyada qoralamani o'chiradi (`ConsumePriceDraft`, qayta bosish ishlamaydi) va har band uchun sotuvchining shu mahsulot va savdo turidagi e'lonini yaratadi yoki yangilaydi (`market.UpsertTx`). Miqdor e'lonning zaxirasiga (`stock`) aylanadi.
6. **"Tuzatish"** (`p:fix:<id>`) — qoralama o'chiriladi, sotuvchi tuzatilgan matnni qayta yuboradi.
7. Qoralama eskirgan yoki allaqachon saqlangan bo'lsa: "Bu kartochka eskirgan yoki allaqachon saqlangan".

Saqlangandan keyin bot har narx uchun bozordagi o'rnini ko'rsatadi ("12 sotuvchi ichida 9-o'rindasiz …"). Narx bozordan juda farq qilsa: "Narx odatdagidan juda farq qiladi, tekshiruvdan so'ng chiqadi" (`review`). Narx qachongacha amal qilishi va "Kabinetni ochish" havolasi ham beriladi.

**"Nega o'zgardi?" savoli.** Faol narx oldingi qabul qilingan narxdan **≥ 10%** farq qilsa (`reasonThreshold`), bot sababini so'raydi va tayyor javob tugmalarini beradi (ko'tarilish va tushish uchun alohida ro'yxat, `insight.UpReasons` / `DownReasons`). Javob `RecordPriceReason` bilan yoziladi, sotuvchi ismi ko'rsatilmaydi.

**Misol.** Oldingi narx 4 000, yangisi 4 600: `4 600 / 4 000 − 1 = +15%`, ya'ni ≥ 10%. Bot so'raydi: "Kartoshka narxini 4 000 dan 4 600 so'mga (+15%) o'zgartirdingiz. Nega?"

---

## 6. /savol: savol-javob

`/savol` tahlilchi (`backend/internal/analyst`) orqali javob beradi. U ikki rejimda ishlaydi, javobdagi raqamlar ikkalasida ham faqat deterministik funksiyalardan:

- **Qoidalar (standart).** Savol qoidalar bilan tushuniladi (`backend/internal/ask/ask.go`, 6.1–6.2), shuning uchun har javobni tekshirib, takrorlab bo'ladi. Oddiy narx savollariga odatiy kartochka qaytadi.
- **Model (`ANALYST_ENABLED=true` va `LLM_*`).** Til modeli 8 ta qat'iy funksiyadan tanlaydi (narx statistikasi, haftalik trend, eng arzon takliflar, bosim indeksi, viloyatlar solishtiruvi, prognoz, anomaliyalar, top talab), ko'pi bilan `ANALYST_MAX_TOOL_ROUNDS` raund, va javob matnini yozadi. Matndagi har bir son funksiya natijalari bilan solishtiriladi (`internal/numcheck`); mos kelmagan son bo'lsa matn shablon javob bilan almashtiriladi. Model xato qilsa, kechiksa yoki foydalanuvchi daqiqasiga `ANALYST_PER_MINUTE` (6) dan ko'p so'rasa — qoidali javob. Javob ostida: kuzatuvlar soni, sana oralig'i, "manba: Bugun ma'lumotlari" va demo ulushi (0 dan katta bo'lsa).

Saytdagi ko'rinishi — `POST /v1/ask` ([API](09-api.md)).

### 6.1 Savolni tushunish

| Qism | Qanday topiladi |
|---|---|
| Mahsulot | `ParseDemand` (katalog va sinonimlar). Topilmasa: "Savolda mahsulot tanilmadi" |
| Joy | hudud nomlari "viloyati/shahri/tumani/respublikasi" so'zlarisiz, so'z boshida mos kelishi bilan (`namanganda`, `chortoqdagi`). Eng uzun nom yutadi. Teng nomlarda savolda aytilgan tur (`tuman`, `shahr`, `viloyat`), keyin viloyat ustun. Oddiy "Toshkent" shahar deb olinadi. 3 harfdan qisqa nomlar e'tiborga olinmaydi |
| Savdo turi | standart `ulgurji`, savolda `chakana` bo'lsa `chakana` |
| Niyat (intent) | pastdagi jadval |

| Intent | Qoida | Misol |
|---|---|---|
| `areas` | `qaysi` + (`tuman`/`viloyat`/`hudud`/`joy`/`shahar`) | "Qaysi tumanda pomidor arzon?" |
| `cheapest` | `arzon`, `kimda`, `sotuvchi`, `qayerdan`, `qayerda` | "Chortoqda eng arzon kartoshka kimda?" |
| `price` | boshqa hollarda | "Namanganda piyoz qancha?" |

### 6.2 Javob

1. **Statistika** (`stats`): hudud kengayib boradi — aytilgan joy → uning viloyati → butun respublika — toki kamida **3** ta narx topilguncha. Kengaytirilgan bo'lsa: "… bo'yicha narxlar hali kam, kengroq hudud olindi". 3 tadan kam bo'lsa: "narxlar hali kam".
2. Javobda median ("O'rtacha"), oddiy oraliq (P25 – P75), eng arzon – eng qimmat, narxlar soni va sana oralig'i beriladi.
3. `cheapest`: aytilgan hududdagi eng arzon **3** ta e'lon (sotuvchi nomi, bozor, hudud).
4. `areas`: joy aytilgan bo'lsa shu viloyat tumanlari medianlari, aytilmasa viloyatlar medianlari (faqat ≥ 3 narxi borlari), arzonidan boshlab.
5. Tugmalar: "Kuzatish" (kuzatuv qo'shadi, qarang [savdo va talab](05-savdo-va-talab.md#7-kuzatuvlar-watches)) va saytdagi mahsulot sahifasiga "Batafsil".

Har savol `question_log` ga yoziladi (foydalanuvchi, matn, mahsulot, hudud). Mahsulot tanilgan bo'lsa, `question` turidagi talab signali ham yoziladi (bir foydalanuvchi, mahsulot va kun uchun bir marta).

**Misol.** "Namanganda piyoz qancha?" → mahsulot Piyoz, joy Namangan viloyati, intent `price`, savdo turi `ulgurji`. Viloyatda 8 ta narx bo'lsa, javob: "O'rtacha: 3 800 so'm/kg · Oddiy oraliq: 3 600 – 4 000 · Eng arzon – eng qimmat: 3 400 – 4 500 · 8 ta narx · 20-sentabr – 26-sentabr".

---

## 7. RFQ, savdo va reyting botda

Batafsil qoidalar [savdo va talab](05-savdo-va-talab.md) hujjatida. Botdagi ko'rinishi:

| Oqim | Botda |
|---|---|
| Xaridor so'rovi | `/sorov kartoshka 200 kg` yoki oddiy matn → karta "So'rov: Kartoshka — 200 kg, muddat 3 kun. Yuborilsinmi?" → `r:new` so'rovni ochadi va nechta sotuvchiga ketganini aytadi. Miqdor yozilmasa: "Qancha kartoshka kerak?" |
| Sotuvchiga yangi so'rov | "Yangi so'rov #N" + "Narx taklif qilish" (`r:quote`) va "Menda yo'q" (`r:none`) |
| Taklif | sotuvchi "So'rov #N" xabariga narx bilan reply qiladi (`parseSom`: `3200`, `3 200`, `3.200`, `3,2 ming`) |
| Xaridorga taklif | narx va sotuvchi ismi, "Tanlash" (`o:pick`) va "Barcha takliflar" havolasi |
| Sotuvchiga tanlov | "Kontakt almashishga rozimisiz?" → `o:yes` / `o:no` |
| Rozilikdan keyin | ikkala tomonga qarshi tomonning Telegram username'i (ruxsat bo'lsa); telefon faqat saytda |
| Savdo savoli | "Kelishdingizmi?" → `d:yes` / `d:talk` / `d:no`, keyin "Savdo #N" ga "200 5800" kabi reply |
| Kechki savol | "Bugun raqamingizni ochishdi" → `s:sold:<e'lon>` / `s:none` |
| "Sotildi" | `/mening` → `x:sold:<e'lon>` → "Sotildi #N" ga reply "200 5800" |
| Baho | 1–5 yulduz (`k:<savdo>:<yulduz>`), keyin "Baho #N" ga izoh |

"200 5800" formati (`parseDealDetails`): avval miqdor (kasr bo'lishi mumkin: `2,5`), ixtiyoriy birlik so'zi, keyin ixtiyoriy narx. Birlik yozilsa, u e'lon birligiga mos bo'lishi kerak (`kg` uchun `kilo` ham qabul qilinadi).

---

## 8. Kanallar va guruhlar

### 8.1 Ulash

Sotuvchi botni o'z Telegram kanaliga admin qilib (hech qanday huquqsiz ham bo'ladi) yoki savdo guruhiga qo'shadi.

1. Telegram `my_chat_member` hodisasini yuboradi. Bot chatni `telegram_channels` ga yozadi (`channels.Connected`), qo'shgan odamning Telegram ID'si bilan.
2. Qo'shgan odamning profili bor va hududi ma'lum bo'lsa — chat darhol `active`, viloyati egasining viloyati. Aks holda `pending`: egasi `/kanallar` → "Viloyat" orqali tanlaydi. Bazada cheklov bor: `active` chatda egasi va viloyati bo'lishi shart.
3. Bot egasiga shaxsiy xabar yozadi. Guruh uchun: "Bot guruhda hech narsa yozmaydi, faqat narxlarni o'qiydi". Bot admin bo'lmasa, Telegram unga faqat botni tilga olgan xabarlarni ko'rsatadi, shuning uchun botni admin qilish tavsiya etiladi.
4. Bot chiqarilsa yoki bloklansa — chat `disabled`.

`/kanallar` tugmalari: "Viloyat" (`c:prov`, `c:setp`), faqat kanallar uchun "E'lonlarni yangilash" yoqish/o'chirish (`c:upd`) va "Uzish" (`c:off`, bot chatdan chiqib ketadi).

### 8.2 Postdan narx olish (`channels.Ingest`)

1. Faqat `active` chatlar o'qiladi. Post matni yoki rasm izohi olinadi, **4 000** belgigacha kesiladi.
2. Matn o'sha parserdan o'tadi (4-bo'lim). Qoralama va tasdiqlash yo'q: bu sotuvchi kanalga allaqachon e'lon qilgan ochiq narx.
3. Har tanilgan narx `price_observations` ga yoziladi:
   - `source = 'telegram_channel'`, vazn **0.6** (`price_weight`);
   - `region_id` — chatning viloyati;
   - `valid_until = now + 7 kun` (`Validity`): Telegram post o'chirilganini xabar qilmaydi, shuning uchun narx o'zi eskiradi;
   - ochiq kanal yoki superguruhda `source_url = https://t.me/<username>/<message_id>`;
   - `seller_id`: kanalda egasi, guruhda xabar yozgan odam (profili bog'langan va boti yoqilgan bo'lsa), faqat roli `seller` bo'lsa.
4. **Uzoq narxlar tekshiruvga tushadi** (`screen`): viloyatda shu mahsulot va savdo turi bo'yicha kamida 3 ta jonli narx bo'lsa va `narx / median > 2` yoki `< 0.5` bo'lsa, `status = 'review'`, medianga kirmaydi.
5. **Tahrirlangan post** eski narxlarini almashtiradi: avval shu `(chat_id, message_id)` ning barcha narxlari `expired` qilinadi, keyin yangi matn o'qiladi.
6. "E'lonlarni yangilash" yoqilgan **kanal** bo'lsa va egasi sotuvchi bo'lsa, postdagi narxlar egasining saytdagi e'lonlarini ham yangilaydi (`market.UpsertTx`).
7. Chat hisoblagichlari (`posts_seen`, `prices_taken`) oshiriladi.

**Bot guruh yoki kanalda hech qachon javob yozmaydi.** `chatGuard` shaxsiy bo'lmagan barcha xabarlarni oddiy handlerlarga yetkazmaydi, guruhdagi tugma bosishlarga ham javob bermaydi. Egasi faqat **birinchi 3 ta** post haqida shaxsiy hisobot oladi (`reportPosts`): nechta narx olindi, nechtasi tekshiruvda, qaysi qator tushunilmadi, nechta e'lon yangilandi.

**Misol.** Namangan viloyatiga ulangan kanal posti: `Piyoz optom 3800, kartoshka optom 9500`. Viloyatda kartoshka ulgurji medianasi 4 300 (5 ta narx). Piyoz 3 800 → `active`. Kartoshka `9 500 / 4 300 = 2.2 > 2` → `review`. Egasi hisobot oladi: "2 ta narx olindi … 1 tasi bozor narxidan juda farq qiladi, tekshiruvdan so'ng hisobga olinadi". Keyin post "kartoshka optom 4500" deb tahrirlansa, ikkala eski narx `expired` bo'ladi va 4 500 `active` sifatida yoziladi.

---

## 9. Rejalashtirilgan xabarlar

Barcha jadvallar Toshkent vaqtida (`backend/internal/jobs/jobs.go`). Har ish o'z vaqt slotini `job_runs` jadvalida band qiladi: ikki server bo'lsa ham slot bir marta ishlaydi, server qayta ishga tushsa ham "oyna" ichida o'tkazib yuborilgan slot bajariladi. Ommaviy yuborishda xabarlar orasida 40 ms pauza bor (`sendPause`, Telegram cheklovi taxminan 30 xabar/soniya).

| Xabar | Vaqt (oyna) | Kimga | Mazmuni |
|---|---|---|---|
| Haftalik eslatma (`weekly_reminder`) | Dushanba 09:00 (6 soat) | e'loni oxirgi 14 kunda amal qilgan, boti yoqilgan sotuvchilar | faol e'lonlar (10 qatorgacha) va "amal qiladi …gacha" yoki "muddati o'tgan". "Ha, o'zgarmadi" (`m:ok`) — hammasi yana **7 kunga** uzaytiriladi (`RenewAll`). "O'zgardi" (`m:edit`) — yangi narxni qanday yozish |
| "Nega o'zgardi?" | narx tasdiqlangan zahoti | sotuvchi | ≥ 10% o'zgarishda sabab tugmalari (5-bo'lim) |
| Avtomatik narx (`auto_pricing`) | 07:30, 12:00, 16:30 (3 soat) | narxi avtomatik o'zgargan sotuvchilar | bitta xabarda barcha o'zgarishlar: eski → yangi narx, viloyat medianasi, sabablar. Har o'zgarish uchun "Qaytarish" (`a:undo`): eski narx qaytadi va **24 soat** avtomatik o'zgarmaydi. Batafsil: [avtomatik narx](06-avtomatik-narx.md) |
| Kuzatuv xabari (`watch_alerts`) | 08:00 va 17:00 (2 soat) | kuzatuvchilar | arzonroq yangi narx va/yoki median ≥ 5% tushgani, kuniga ko'pi bilan bir marta |
| Xaridorga savdo savoli (`deal_prompts`) | har 15 daqiqa, faqat 09:00–20:00 | kontakt ochgan xaridor, 24 soatdan keyin | "Kelishdingizmi?", kuniga ko'pi bilan 3 ta |
| Sotuvchiga kechki savol (`seller_deal_digest`) | 19:00 (1 soat) | bugun kontakti ochilgan sotuvchilar | "Sotuv bo'ldimi?" |
| Baho so'rash (`rating_prompts`) | 11:00 (3 soat) | tasdiqlangan savdo xaridori, 3–30 kun o'tgach | 1–5 yulduz |
| RFQ xabarlari | darhol (fon rejimida) | sotuvchilar va xaridor | yangi so'rov, taklif, tanlov, rozilik yoki rad |
| Kanal hisoboti | darhol | kanal egasi | faqat birinchi 3 ta post uchun |

---

## 10. Bot yuboradigan sayt havolalari

Barcha havolalar `WEB_URL` sozlamasiga qo'shiladi.

| Havola | Qayerda |
|---|---|
| `/kirish` | bog'lanmagan foydalanuvchiga "Saytga kirish" |
| `/profil` | `/start`, `/profil`, bot uzilgan holat |
| `/boshlash` | rol tanlanmagan foydalanuvchiga "Yakunlash" |
| `/kabinet` | narx saqlangandan keyin, RFQ roziligidan keyin sotuvchiga |
| `/mahsulotlarim` | `/mening`, haftalik eslatma javoblari |
| `/sorovlar` | so'rov ochilgandan keyin |
| `/sorovlar/{id}` | "Barcha takliflar", "Telefon va batafsil", rad etilganda "Takliflar" |
| `/mahsulot/{slug}` | `/savol` javobidagi "Batafsil" |
| `/mahsulot/{slug}?region={viloyat_id}` | kuzatuv xabaridagi "Ko'rish" |

---

## 11. E'tibor talab qiladigan joylar

- **Standart savdo turi chakana.** Parser xabarda `optom`/`ulgurji` so'zi bo'lmasa `chakana` qo'yadi (botda ham, kanallarda ham). `/savol`, RFQ va kuzatuvlar esa standart bo'yicha `ulgurji` bilan ishlaydi. Sotuvchi "optom" yozishni unutsa, narxi ulgurji medianiga kirmaydi.
- **Qoidali `/savol` javobi pastida "manba: sotuvchilar" yoziladi**, holbuki median `telegram_channel`, `rfq_offer` va `deal` narxlarini ham o'z ichiga oladi (model rejimidagi javobda "manba: Bugun ma'lumotlari").
- **Bot javoblari faqat o'zbek tilida.**
- **Respublika bo'yicha kuzatuv havolasi** `?region=0` bilan yuboriladi.
- **"450" kabi narxlar** minglarda o'qilmaydi (4.6-bo'lim). Shunday narx 50 so'mdan yuqori bo'lgani uchun saqlanishi mumkin. Uni ushlashning yagona yo'li — medianga nisbatan tekshiruv (`review`).

---

## Qayerda joylashgan

- `backend/internal/bot/bot.go` — buyruqlar, `/start`, `/yordam`, `/profil`, `/uzish`, `fallback`, `linkedUser`
- `backend/internal/bot/miniapp.go` — `/elon`, Mini App tugmasi
- `backend/internal/bot/throttle.go` — foydalanuvchi limiti
- `backend/internal/bot/ai.go` — ovoz, rasm, mahsulot tanlash tugmalari, tahlilchi javobi
- `backend/internal/analyst/`, `backend/internal/speech/`, `backend/internal/photoprice/`, `backend/internal/catalogmatch/` — model funksiyalari
- `backend/internal/bot/prices.go` — `/narx`, `submitPrice`, `draftCard`, `priceCallback`, `positionLine`
- `backend/internal/bot/mine.go` — `/mening`
- `backend/internal/bot/savol.go`, `backend/internal/ask/ask.go` — `/savol`, `looksLikeQuestion`, intentlar, hudud topish
- `backend/internal/bot/rfq.go` — `/sorov`, RFQ tugmalari, `parseSom`, RFQ bildirishnomalari (`rfq.Notifier`)
- `backend/internal/bot/deals.go` — savdo savollari, kechki savol, "Sotildi"
- `backend/internal/bot/rating.go`, `backend/internal/bot/watch.go` — baho, `/kuzatuv`, kuzatuv xabarlari
- `backend/internal/bot/reminder.go` — haftalik eslatma
- `backend/internal/bot/reasons.go` — "Nega o'zgardi?"
- `backend/internal/bot/autoprice.go` — avtomatik narx xabari va "Qaytarish"
- `backend/internal/bot/channels.go`, `backend/internal/channels/channels.go` — kanal va guruhlar, `chatGuard`, `Ingest`, `screen`
- `backend/internal/pricing/normalize.go`, `backend/internal/pricing/parser.go`, `backend/internal/pricing/parser_test.go` — parser
- `backend/internal/pricing/service.go` — `CreateDraft`, `Confirm`, `Discard`
- `backend/internal/auth/telegram.go` — Telegram orqali kirish
- `backend/internal/jobs/jobs.go`, `backend/cmd/api/main.go` — rejalashtirilgan ishlar
- `backend/queries/channels.sql`, `backend/queries/jobs.sql`, `backend/queries/users.sql`
- `backend/migrations/00005_price_drafts.sql`, `00030_price_reasons.sql`, `00031_telegram_channels.sql`
