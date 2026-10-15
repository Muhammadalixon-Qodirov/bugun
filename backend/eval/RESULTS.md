# AI baholash natijalari

2026-09-26 15:54 · `go run ./cmd/eval` (spec §16.3). Qoidaga asoslangan yoʻllar oflayn ishlaydi; model yoʻllari faqat sozlamalar berilganda.

## AI-2: narx ajratish (parser)

72 ta xabar, 73 ta kutilgan narx, 5 ta narxsiz yoki sotuvchidan soʻralishi kerak boʻlgan xabar.

| Koʻrsatkich | Qoidalar | Maqsad |
|---|---|---|
| Narx toʻgʻri ajratildi (±0 soʻm) | 97.3% (71/73) | ≥ 90% |
| Mahsulot katalogga toʻgʻri moslandi | 100.0% (73/73) | ≥ 90% |
| Birlik va hajm toʻgʻri (asosiy birlikka oʻtkazish) | 95.9% (70/73) | ≥ 85% |
| Savdo turi (ulgurji/chakana) | 100.0% (73/73) | — |
| Aniqlik (precision): chiqqan narxlardan toʻgʻrisi | 95.9% (71/74) | — |
| Xabar toʻliq toʻgʻri | 94.4% (68/72) | — |
| Narxsiz xabar toʻgʻri rad etildi | 80.0% (4/5) | — |

<details><summary>Qoidalar xato qilgan xabarlar</summary>

| # | Xabar | Kutilgan | Chiqdi |
|---|---|---|---|
| p51 | ko'k piyoz bog'i 3000 | — | kok-piyoz 3000 chakana |
| p56 | Guruch lazer 1-sort 26 500 | guruch 26500 chakana | guruch 1000 ×26500 chakana |
| p57 | Pomidor (qizil) 1 kg — 9 ming so'm | pomidor 9000 chakana | pomidor 9000 ×1 chakana |
| p69 | Мука 1 мешок 380000 | un 7600 chakana | un 380000 ×50 chakana |

</details>

## AI-4: katalogga moslash

60 ta soʻz (oʻzbek, rus, ingliz, xato yozilgan; 5 tasi katalogda yoʻq). Ishonch chegarasi 0.60; undan past boʻlsa sotuvchi tanlaydi.

| Koʻrsatkich | Lugʻat + trigram (pg_trgm) | Maqsad |
|---|---|---|
| Toʻgʻri qaror (moslandi yoki moslanmadi) | 90.0% (54/60) | ≥ 90% |
| Katalogdagi soʻz avtomatik toʻgʻri moslandi | 89.1% (49/55) | — |
| Toʻgʻri mahsulot top-3 nomzodlar ichida | 92.7% (51/55) | — |
| Notoʻgʻri avtomatik moslash | 0 | 0 |

<details><summary>Xato qarorlar</summary>

| Soʻz | Kutilgan | Qaror | Nomzodlar |
|---|---|---|---|
| bodiring | bodring | — | bodring 0.55 |
| tomatoes | pomidor | — | pomidor 0.50 |
| potato | kartoshka | — |  |
| onion | piyoz | — |  |
| apple | olma | — |  |
| carrot | sabzi | — |  |

</details>

## AI-5: tahlilchi savolni qaysi funksiyaga yoʻnaltiradi

40 ta savol (oʻzbek, rus, ingliz), 8 ta funksiya.

| Koʻrsatkich | Qoidalar |
|---|---|
| Toʻgʻri funksiya | 100.0% (40/40) |

Funksiya boʻyicha (qoidalar): anomalies 4/4, cheapest_offers 6/6, compare_regions 5/5, forecast 5/5, pressure_index 4/4, price_stats 6/6, price_trend 6/6, top_demand 4/4

<details><summary>Xato yoʻnaltirilgan savollar</summary>

| Savol | Kutilgan | Chiqdi |
|---|---|---|

</details>

Raqamlar har doim funksiyadan keladi: model yozgan javobdagi har bir raqam funksiya natijasi bilan solishtiriladi (`internal/numcheck`), mos kelmasa shablon javob qaytadi.

## Oʻlchanmaganlar

- STT (AI-1) soʻz xatosi darajasi (WER) va ovoz → kartochka vaqti: bozorda yozib olingan ovozlar va ularning qoʻlda yozilgan matni kerak (`eval/` ga qoʻshiladi).
- Suratdan oʻqish (AI-3): narx roʻyxati suratlari toʻplami kerak.
