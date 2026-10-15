// Package llm talks to language, speech, vision and embeddings models behind
// OpenAI-compatible APIs (Groq, Gemini, OpenAI and others). Every feature is
// off unless its key, base URL and model are set, and a model never supplies
// a number the app shows: it reads, routes or phrases, and deterministic code
// checks what it returns.
//
// Extract reads prices from posts the rules could not read (ML_REJA.md 8,
// after HUDUD's chat/llm.py); the model only suggests candidates and
// pricing.Verify decides what becomes a price.
package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"bugun/internal/pricing"
)

// systemPrompt tells the model exactly what to return. The rules matter more
// than the wording: numbers as written (pricing.Verify rejects any number not
// in the text), catalog slugs only, and nothing when unsure; the examples
// cover Latin, Cyrillic, Russian words, spelled-out numbers and amounts that
// are not prices.
const systemPrompt = `Sen Oʻzbekiston bozorlaridagi narx xabarlarini oʻqiydigan yordamchisan. Sotuvchi yoki Telegram kanal yozgan matndan har bir mahsulotning narxini ajratib ber.

QOIDALAR
1. "product" maydoniga faqat pastdagi katalogdagi slugni yoz. Sotuvchilar mahsulotni katalogdagidan boshqacha atashi mumkin: boshqa nom, sheva, ruscha, jargon, nav nomi yoki xato yozuv. Maʼnosini aniq bilsang, katalogdagi mos mahsulotni tanla (masalan "морковка" → sabzi, "chereshnya" → gilos, "moloko" → sut). Katalogda umuman yoʻq mahsulotni yoki qaysi mahsulot ekaniga ishonching komil boʻlmaganini tashlab ket.
2. "text": matnda shu mahsulotni bildirgan soʻz yoki soʻzlar, matnda qanday yozilgan boʻlsa shunday (1–3 soʻz, narxsiz). Masalan "bulba", "qizil pomidor", "картошка".
3. "price": narx matnda qanday yozilgan boʻlsa, oʻsha son. Soʻz bilan yozilganini songa aylantir: "250 ming" → 250000, "toʻrt yarim ming" → 4500, "4.5" → 4.5, "12 000" → 12000. Narxni hech qachon oʻzing hisoblama: boʻlma, koʻpaytirma, boshqa birlikka oʻgirma.
4. "unit": narx qaysi birlik uchun ekanini bildirgan soʻz, matnda qanday yozilgan boʻlsa shunday ("kg", "kilosi", "qop", "yashik", "dona", "litr", "tonna"). Birlik yozilmagan boʻlsa "" qoldir.
5. "trade": matnda optom, opt yoki ulgurji deyilgan boʻlsa "ulgurji", aks holda "chakana".
6. Hajm narx emas: "2 tonna bor", "100 qop" kabi sonlarni narx qilib yozma.
7. Bitta mahsulotga bir nechta narx yozilgan boʻlsa (navlar, oʻlchamlar), birinchisini ol.
8. Telefon raqami, sana, manzil va chegirma foizi narx emas.
9. Matnda narx boʻlmasa, "items" boʻsh roʻyxat boʻlsin.

JAVOB: faqat JSON, boshqa hech narsa yozma:
{"items":[{"product":"slug","text":"...","price":0,"unit":"","trade":"chakana"}]}

MISOLLAR
Matn: Bugun chereshnya 28 000 dan, piyoz ham arzonlashdi 3 ming
{"items":[{"product":"gilos","text":"chereshnya","price":28000,"unit":"","trade":"chakana"},{"product":"piyoz","text":"piyoz","price":3000,"unit":"","trade":"chakana"}]}

Matn: Помидор оптом 120 минг яшик, 5 тонна бор. Тел 90 123 45 67
{"items":[{"product":"pomidor","text":"Помидор","price":120000,"unit":"яшик","trade":"ulgurji"}]}

Matn: Assalomu alaykum! Semirenko olma kilosi 12 000, qizil olma 15 ming
{"items":[{"product":"olma","text":"Semirenko olma","price":12000,"unit":"kilosi","trade":"chakana"}]}

Matn: Kokos 35 ming, ertaga yangi mol keladi, 10% chegirma
{"items":[]}

KATALOG (slug: nomi (boshqa nomlari))
`

// Extract asks the model for the prices in a post.
func (c *Client) Extract(ctx context.Context, catalog, post string) ([]pricing.Candidate, error) {
	ctx = WithFeature(ctx, featureOf(ctx, "channel_parse"))
	msg, err := c.Chat(ctx, ChatRequest{
		Temperature: 0, ResponseFormat: JSONObject,
		Messages: []Message{{Role: "system", Content: systemPrompt + catalog}, {Role: "user", Content: "Matn: " + post}},
	})
	if err != nil {
		return nil, err
	}
	return parseItems(msg.Text())
}

// parseItems reads {"items":[...]} from the model's text, tolerating a code
// fence around it.
func parseItems(content string) ([]pricing.Candidate, error) {
	var v struct {
		Items []pricing.Candidate `json:"items"`
	}
	if err := json.Unmarshal([]byte(StripFence(content)), &v); err != nil {
		return nil, fmt.Errorf("llm: answer is not the expected JSON")
	}
	return v.Items, nil
}
