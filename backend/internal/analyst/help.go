package analyst

// Short guides for platform_help, in the site's three languages.

var helpTopics = []string{"post_price", "auto_pricing", "requests", "alerts", "channels", "account", "commands"}

var helpTexts = map[string][3]string{
	"post_price": {
		"Narx joylash: saytda «Narx yuborish» yoki botda /narx. Oddiy matn yozing, masalan «pomidor 6000, kartoshka qopi 250 ming». Tizim kartochka qaytaradi, tekshirib «Toʻgʻri» ni bosasiz. Ovozli xabar va narxlar roʻyxati surati ham qabul qilinadi. Narx 7 kun amal qiladi.",
		"Как разместить цену: «Отправить цену» на сайте или /narx в боте. Напишите обычным текстом, например «помидор 6000, картошка мешок 250 тысяч». Система покажет карточку, проверьте и нажмите «Верно». Можно голосом или фото прайса. Цена действует 7 дней.",
		"Posting a price: \"Send price\" on the site or /narx in the bot. Write plain text, e.g. \"pomidor 6000, kartoshka qopi 250 ming\". You get a card to check and confirm. Voice messages and photos of price lists work too. A price stays live for 7 days."},
	"auto_pricing": {
		"Avtomatik narx: eʼlonda oʻz oraligʻingizni (eng past va eng yuqori) va maqsadni (tez sotish, muvozanat, foyda) belgilaysiz. Tizim kuniga uch marta viloyat bozoriga qarab narxni oraliq ichida yangilaydi, bir kunda 5% dan koʻp oʻzgartirmaydi. Har oʻzgarishni botda qaytarish mumkin.",
		"Автоцена: задайте свой диапазон и цель (быстро продать, баланс, прибыль). Три раза в день система подстраивает цену под рынок области внутри диапазона, не больше 5% в день. Любое изменение можно отменить в боте.",
		"Automatic pricing: set your range and goal (sell fast, balanced, profit). Three times a day the price follows the viloyat market inside your range, at most 5% a day. Any change can be undone in the bot."},
	"requests": {
		"Soʻrov (RFQ): xaridor mahsulot, miqdor va hududni yozadi, mos sotuvchilar narx taklif qiladi, xaridor eng yaxshisini tanlaydi. Saytda «Soʻrovlar» yoki botda /sorov.",
		"Запрос (RFQ): покупатель указывает товар, количество и регион, продавцы присылают цены, покупатель выбирает лучшее. «Запросы» на сайте или /sorov в боте.",
		"Requests (RFQ): a buyer gives the product, amount and place, matching sellers send prices, the buyer picks the best. \"Requests\" on the site or /sorov in the bot."},
	"alerts": {
		"Narx kuzatuvi: mahsulot sahifasida «Narx tushsa xabar ber» yoki botda /kuzatuv. Arzonroq narx chiqsa yoki median 5% tushsa, ertalab va kechqurun botda xabar keladi.",
		"Отслеживание цены: «Сообщить о снижении» на странице товара или /kuzatuv в боте. Когда появится цена дешевле или медиана упадёт на 5%, придёт сообщение утром и вечером.",
		"Price alerts: \"Tell me when it drops\" on a product page or /kuzatuv in the bot. When a cheaper price appears or the median falls 5%, the bot messages you morning and evening."},
	"channels": {
		"Telegram kanal yoki guruh: botni kanalingizga admin qilib yoki savdo guruhingizga qoʻshing. U yerga yozilgan narxlar platformaga oʻzi tushadi. Boshqaruv: botda /kanallar. Guruhda bot hech qachon javob yozmaydi.",
		"Telegram-канал или группа: добавьте бота админом в канал или в торговую группу — цены оттуда попадут на платформу сами. Управление: /kanallar в боте. В группе бот никогда не пишет.",
		"Telegram channel or group: add the bot as an admin to your channel or to your trade group, and prices posted there reach the platform by themselves. Manage them with /kanallar. The bot never writes in groups."},
	"account": {
		"Hisob: saytga Telegram orqali kirasiz, bot avtomatik ulanadi. Rol («Tadbirkor» — ham sotadi, ham soʻrov yuboradi; yoki «Bozor maʼmuriyati»), viloyat va bozor profil sahifasida oʻzgartiriladi. Botni uzish: /uzish.",
		"Аккаунт: вход на сайт через Telegram, бот подключается сам. Роль («Предприниматель» — и продаёт, и отправляет запросы; или «Администрация базара»), область и базар меняются в профиле. Отключить бота: /uzish.",
		"Account: sign in with Telegram and the bot links itself. Role (\"Entrepreneur\" both sells and sends requests, or \"Market administration\"), viloyat and bazaar are set on the profile page. To unlink the bot: /uzish."},
	"commands": {
		"Bot buyruqlari: /narx — narx yuborish, /mening — narxlarim, /sorov — soʻrov, /savol — savol berish, /kuzatuv — kuzatuvlar, /kanallar — kanallar, /profil, /yordam.",
		"Команды бота: /narx — цена, /mening — мои цены, /sorov — запрос, /savol — вопрос, /kuzatuv — отслеживание, /kanallar — каналы, /profil, /yordam.",
		"Bot commands: /narx send a price, /mening my prices, /sorov a request, /savol ask, /kuzatuv alerts, /kanallar channels, /profil, /yordam."},
}

func platformHelp(topic, lang string) result {
	t, ok := helpTexts[topic]
	if !ok {
		return failed(ToolPlatformHelp, "bad_arguments")
	}
	i := map[string]int{"uz": 0, "ru": 1, "en": 2}[langOf(lang)]
	data := map[string]any{"topic": topic, "text": t[i], "enough": true}
	return simple(ToolPlatformHelp, map[string]any{"topic": topic}, data, true, func(lang string) []string {
		return []string{t[map[string]int{"uz": 0, "ru": 1, "en": 2}[langOf(lang)]]}
	})
}
