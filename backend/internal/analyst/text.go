package analyst

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"bugun/internal/pricing"
)

// Templated sentences for answers written from tool data without a model,
// in the three languages of the site.

// Langs are the answer languages; anything else is answered in Uzbek.
var Langs = []string{"uz", "ru", "en"}

func langOf(l string) string {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "ru":
		return "ru"
	case "en":
		return "en"
	}
	return "uz"
}

var phrases = map[string][3]string{
	"country":          {"butun respublika", "вся страна", "the whole country"},
	"stats":            {"%s (%s, %s): oʻrtacha narx %s soʻm/%s, oddiy oraliq %s – %s, eng arzon – eng qimmat %s – %s; %d ta narx (%s – %s).", "%s (%s, %s): медиана %s сум/%s, обычный диапазон %s – %s, мин – макс %s – %s; %d цен (%s – %s).", "%s (%s, %s): median %s so'm/%s, usual range %s – %s, min – max %s – %s; %d prices (%s – %s)."},
	"stats_range":      {"%s (%s, %s), %s – %s: oʻrtacha narx %s soʻm/%s, eng arzon – eng qimmat %s – %s; %d ta narx.", "%s (%s, %s), %s – %s: медиана %s сум/%s, мин – макс %s – %s; %d цен.", "%s (%s, %s), %s – %s: median %s so'm/%s, min – max %s – %s; %d prices."},
	"widened":          {"%s boʻyicha narxlar kam, kengroq hudud olindi.", "По %s цен мало, взят более широкий регион.", "Few prices in %s, a wider area was used."},
	"insufficient":     {"%s boʻyicha maʼlumot yetarli emas (%d ta narx, kamida 3 ta kerak). Taxmin qilmaymiz.", "По %s данных недостаточно (%d цен, нужно не меньше 3). Мы не делаем предположений.", "Not enough data for %s (%d prices, at least 3 are needed). We do not guess."},
	"no_product":       {"Savolda mahsulot tanilmadi. Masalan: «Samarqandda piyoz qancha?»", "В вопросе не распознан товар. Например: «Сколько стоит лук в Самарканде?»", "No product was recognised in the question. For example: \"How much is onion in Samarkand?\""},
	"trend":            {"%s (%s, %s), oxirgi %d hafta: %s soʻmdan %s soʻmgacha (%s).", "%s (%s, %s), последние %d недель: с %s до %s сум (%s).", "%s (%s, %s), last %d weeks: from %s to %s so'm (%s)."},
	"offers_none":      {"%s (%s, %s): hozir mos faol taklif yoʻq.", "%s (%s, %s): подходящих активных предложений сейчас нет.", "%s (%s, %s): no matching live offers now."},
	"offers":           {"%s (%s, %s), eng arzon takliflar:", "%s (%s, %s), самые дешёвые предложения:", "%s (%s, %s), cheapest offers:"},
	"offer":            {"%s soʻm/%s — %s (%s)", "%s сум/%s — %s (%s)", "%s so'm/%s — %s (%s)"},
	"areas_none":       {"%s (%s): hududlarni solishtirish uchun narxlar hali kam.", "%s (%s): для сравнения регионов пока мало цен.", "%s (%s): too few prices yet to compare areas."},
	"areas":            {"%s (%s), hududlar boʻyicha oʻrtacha narx (arzonidan):", "%s (%s), медиана по регионам (от дешёвых):", "%s (%s), median by area (cheapest first):"},
	"area":             {"%s — %s soʻm (%d ta narx)", "%s — %s сум (%d цен)", "%s — %s so'm (%d prices)"},
	"pressure_none":    {"%s (%s, %s): talab va taklif indeksi uchun hali kamida ikki hafta maʼlumot kerak. Hozirgi hafta: talab %s, sotuvchilar %d.", "%s (%s, %s): для индекса спроса и предложения нужно ещё хотя бы две недели данных. Эта неделя: спрос %s, продавцов %d.", "%s (%s, %s): the demand/supply index needs at least two weeks of data. This week: demand %s, sellers %d."},
	"pressure":         {"%s (%s, %s): talab/taklif indeksi %s (%s). Talab: %s (odatda %s), sotuvchilar: %d (odatda %s).", "%s (%s, %s): индекс спроса/предложения %s (%s). Спрос: %s (обычно %s), продавцов: %d (обычно %s).", "%s (%s, %s): demand/supply index %s (%s). Demand: %s (usually %s), sellers: %d (usually %s)."},
	"zone_surplus":     {"taklif koʻp — xarid uchun qulay", "предложение выше обычного — удобно покупать", "supply is high — a good time to buy"},
	"zone_balanced":    {"muvozanat", "баланс", "balanced"},
	"zone_shortage":    {"talab yuqori — narx oshishi mumkin", "спрос выше обычного — цена может вырасти", "demand is high — prices may rise"},
	"forecast_none":    {"%s (%s, %s): prognoz uchun kamida bir yillik tarix kerak.", "%s (%s, %s): для прогноза нужна история хотя бы за год.", "%s (%s, %s): a forecast needs about a year of history."},
	"forecast":         {"%s (%s, %s), keyingi haftalar prognozi (usul: %s):", "%s (%s, %s), прогноз на следующие недели (метод: %s):", "%s (%s, %s), forecast for the next weeks (method: %s):"},
	"forecast_week":    {"%s haftasi: %s soʻm (%s – %s)", "неделя с %s: %s сум (%s – %s)", "week of %s: %s so'm (%s – %s)"},
	"anomalies_none":   {"%s (%s, %s): oxirgi 90 kunda shubhali kun topilmadi.", "%s (%s, %s): за последние 90 дней подозрительных дней нет.", "%s (%s, %s): no suspicious days in the last 90 days."},
	"anomalies":        {"%s (%s, %s), oxirgi 90 kundagi shubhali kunlar:", "%s (%s, %s), подозрительные дни за последние 90 дней:", "%s (%s, %s), suspicious days in the last 90 days:"},
	"anomaly":          {"%s, %s: %s soʻm (kutilgan %s)", "%s, %s: %s сум (ожидалось %s)", "%s, %s: %s so'm (expected %s)"},
	"demand_none":      {"%s: bu hafta talab maʼlumoti hali kam.", "%s: данных о спросе на этой неделе пока мало.", "%s: too little demand data this week."},
	"demand":           {"%s: bu hafta eng koʻp soʻralgan mahsulotlar:", "%s: самые востребованные товары этой недели:", "%s: most wanted products this week:"},
	"demand_item":      {"%s — %s ball", "%s — %s баллов", "%s — score %s"},
	"demo":             {"Diqqat: maʼlumotning %s qismi demo.", "Внимание: %s данных — демо.", "Note: %s of the data is demo."},
	"found_products":   {"Katalogda mos mahsulotlar: %s.", "Подходящие товары в каталоге: %s.", "Matching catalog products: %s."},
	"advice":           {"%s (%s): %s.", "%s (%s): %s.", "%s (%s): %s."},
	"adv_buy_now":      {"xaridorga — hozir olish maʼqul", "покупателю — лучше купить сейчас", "for buyers — better to buy now"},
	"adv_wait":         {"xaridorga — kutish maʼqul", "покупателю — лучше подождать", "for buyers — better to wait"},
	"adv_neutral":      {"aniq signal yoʻq", "явного сигнала нет", "no clear signal"},
	"season_none":      {"%s (%s): mavsumni aniqlash uchun kamida ikki yillik tarix kerak.", "%s (%s): для сезонности нужна история хотя бы за два года.", "%s (%s): seasonality needs about two years of history."},
	"season":           {"%s (%s): odatda %s oyida eng arzon (yillik darajadan %s past), %s oyida eng qimmat (%s yuqori).", "%s (%s): обычно дешевле всего в %s (на %s ниже годового уровня), дороже всего в %s (на %s выше).", "%s (%s): usually cheapest in %s (%s below the yearly level), dearest in %s (%s above)."},
	"weekdays_none":    {"%s (%s): hafta kunlari orasida sezilarli farq yoʻq.", "%s (%s): заметной разницы по дням недели нет.", "%s (%s): no real difference between weekdays."},
	"weekdays":         {"%s (%s): odatda %s kuni eng arzon, %s kuni eng qimmat (farq %s).", "%s (%s): обычно дешевле всего в %s, дороже всего в %s (разница %s).", "%s (%s): usually cheapest on %s, dearest on %s (difference %s)."},
	"reasons_head":     {"%s (%s), narx oʻzgarishi sabablari:", "%s (%s), причины изменения цены:", "%s (%s), why the price moved:"},
	"seller_reason":    {"sotuvchilar aytgan sabab: %s (%d marta)", "причина от продавцов: %s (%d раз)", "reason sellers gave: %s (%d times)"},
	"reasons_none":     {"Oxirgi davrda keskin oʻzgarish ham, aytilgan sabab ham yoʻq.", "Резких изменений и названных причин за период нет.", "No sharp changes or stated reasons in the period."},
	"districts_none":   {"%s (%s): tumanlar va bozorlar boʻyicha narxlar hali kam.", "%s (%s): по районам и базарам пока мало цен.", "%s (%s): too few prices by tuman and bazaar yet."},
	"districts":        {"%s (%s), tumanlar va bozorlar boʻyicha:", "%s (%s), по районам и базарам:", "%s (%s), by tuman and bazaar:"},
	"markup_none":      {"%s (%s): ulgurji va chakana narxni solishtirish uchun maʼlumot kam.", "%s (%s): мало данных для сравнения опта и розницы.", "%s (%s): too little data to compare wholesale and retail."},
	"markup":           {"%s (%s): chakana narx ulgurjidan %s qimmat.", "%s (%s): розница дороже опта на %s.", "%s (%s): retail is %s dearer than wholesale."},
	"arbitrage_none":   {"%s: yoʻl xarajatidan keyin foyda beradigan viloyatlar juftligi hozir yoʻq.", "%s: пар регионов с выгодой после дороги сейчас нет.", "%s: no viloyat pair pays after the road now."},
	"arbitrage":        {"%s, yoʻl xarajatidan keyin foydali yoʻnalishlar:", "%s, выгодные направления после дороги:", "%s, routes that pay after the road:"},
	"arbitrage_pair":   {"%s (%s soʻm) → %s (%s soʻm), sof foyda %s soʻm", "%s (%s сум) → %s (%s сум), чистая выгода %s сум", "%s (%s so'm) → %s (%s so'm), net %s so'm"},
	"region":           {"%s: %d sotuvchi, %d eʼlon, %d bozor.", "%s: %d продавцов, %d объявлений, %d базаров.", "%s: %d sellers, %d listings, %d bazaars."},
	"region_short":     {"Talab yuqori: %s.", "Спрос выше обычного: %s.", "Demand is high: %s."},
	"region_surplus":   {"Taklif koʻp: %s.", "Предложение выше обычного: %s.", "Supply is high: %s."},
	"movers_none":      {"Bu hafta oʻzgarishlar uchun maʼlumot kam.", "Для изменений этой недели мало данных.", "Too little data for this week's changes."},
	"movers_up":        {"Bu hafta qimmatlaganlar: %s.", "Подорожали за неделю: %s.", "Rose this week: %s."},
	"movers_down":      {"Arzonlaganlar: %s.", "Подешевели: %s.", "Fell: %s."},
	"compare_head":     {"Solishtirish (%s):", "Сравнение (%s):", "Comparison (%s):"},
	"som":              {"soʻm", "сум", "so'm"},
	"basket_head":      {"Savat narxi (%s, hozirgi oʻrtacha narxlar):", "Стоимость корзины (%s, текущие медианы):", "Basket cost (%s, live medians):"},
	"basket_line":      {"%s: %s %s × %s = %s soʻm", "%s: %s %s × %s = %s сум", "%s: %s %s × %s = %s so'm"},
	"basket_total":     {"Jami: %s soʻm.", "Итого: %s сум.", "Total: %s so'm."},
	"basket_missing":   {"Narxi topilmadi, jamiga kirmadi: %s.", "Цена не найдена, в итог не вошло: %s.", "No price, left out of the total: %s."},
	"fx_none":          {"Kurs maʼlumoti hali yuklanmagan.", "Данные о курсе ещё не загружены.", "Exchange rates are not loaded yet."},
	"fx":               {"Markaziy bank kursi: 1 %s = %s soʻm (%s), 30 kunda %s.", "Курс ЦБ: 1 %s = %s сум (%s), за 30 дней %s.", "Central Bank rate: 1 %s = %s so'm (%s), %s over 30 days."},
	"weather_none":     {"Ob-havo maʼlumoti yoʻq.", "Данных о погоде нет.", "No weather data."},
	"weather":          {"%s, %s: %s…%s °C.", "%s, %s: %s…%s °C.", "%s, %s: %s…%s °C."},
	"weather_extremes": {"Oxirgi 2 haftada sovuq yoki jazirama kunlar: %d ta.", "За 2 недели дней с заморозком или жарой: %d.", "Frost or heat days in the last 2 weeks: %d."},
	"events_none":      {"Yaqin davrda qayd etilgan voqea yoki bayram yoʻq.", "Событий и праздников за период нет.", "No events or holidays in the period."},
	"events_head":      {"Voqealar va bayramlar:", "События и праздники:", "Events and holidays:"},
	"seller_none":      {"«%s» nomli sotuvchi topilmadi.", "Продавец «%s» не найден.", "No seller named \"%s\"."},
	"seller":           {"%s (%s): %d ta faol eʼlon.", "%s (%s): %d активных объявлений.", "%s (%s): %d live listings."},
	"act_request":      {"Soʻrov yuborish: %s, %s %s", "Отправить запрос: %s, %s %s", "Send a request: %s, %s %s"},
	"act_watch":        {"%s narxi tushsa xabar berish", "Сообщить, когда %s подешевеет", "Tell me when %s gets cheaper"},
	"act_confirm":      {"Tasdiqlash uchun tugmani bosing.", "Нажмите кнопку, чтобы подтвердить.", "Press the button to confirm."},
	"m_jan":            {"yanvar", "январь", "January"}, "m_feb": {"fevral", "февраль", "February"}, "m_mar": {"mart", "март", "March"},
	"m_apr": {"aprel", "апрель", "April"}, "m_may": {"may", "май", "May"}, "m_jun": {"iyun", "июнь", "June"},
	"m_jul": {"iyul", "июль", "July"}, "m_aug": {"avgust", "август", "August"}, "m_sep": {"sentabr", "сентябрь", "September"},
	"m_oct": {"oktabr", "октябрь", "October"}, "m_nov": {"noyabr", "ноябрь", "November"}, "m_dec": {"dekabr", "декабрь", "December"},
	"d_mon": {"dushanba", "понедельник", "Monday"}, "d_tue": {"seshanba", "вторник", "Tuesday"}, "d_wed": {"chorshanba", "среда", "Wednesday"},
	"d_thu": {"payshanba", "четверг", "Thursday"}, "d_fri": {"juma", "пятница", "Friday"}, "d_sat": {"shanba", "суббота", "Saturday"},
	"d_sun":            {"yakshanba", "воскресенье", "Sunday"},
	"cannot_off_topic": {"Men bozor narxlari, mahsulotlar va viloyatlar boʻyicha yordam beraman. Bu savolga javob bera olmayman.", "Я помогаю с рыночными ценами, товарами и регионами. На этот вопрос ответить не могу.", "I help with market prices, products and regions. I cannot answer this question."},
	"cannot_no_data":   {"Bunga hozircha maʼlumot yoʻq. Savolingizni adminlarga yetkazdim.", "Данных по этому пока нет. Я передал вопрос администраторам.", "There is no data for this yet. I passed the question to the admins."},
	"cannot_no_tool":   {"Bunday savolga hozircha javob bera olmayman. Savolingizni adminlarga yetkazdim, ular bunday imkoniyat qoʻshishi mumkin.", "На такой вопрос пока ответить не могу. Я передал его администраторам.", "I cannot answer this kind of question yet. I passed it to the admins."},
	"clarify_generic":  {"Aniqroq yozing: qaysi mahsulot va qaysi viloyat haqida?", "Уточните: какой товар и какой регион?", "Could you say which product and which region?"},
	"busy":             {"Hozir savollar koʻp, birozdan keyin qayta urinib koʻring.", "Сейчас много вопросов, попробуйте чуть позже.", "Many questions right now, please try again shortly."},
	"up":               {"oshmoqda", "растёт", "rising"},
	"down":             {"tushmoqda", "снижается", "falling"},
	"flat":             {"barqaror", "стабильно", "flat"},
}

func tr(lang, key string, args ...any) string {
	p, ok := phrases[key]
	if !ok {
		return key
	}
	i := 0
	switch langOf(lang) {
	case "ru":
		i = 1
	case "en":
		i = 2
	}
	if len(args) == 0 {
		return p[i]
	}
	return fmt.Sprintf(p[i], args...)
}

// som formats an amount as "4 500".
func som(v float64) string { return pricing.FormatSom(int64(math.Round(v))) }

// pct formats a share (0.123) as "12%".
func pct(share float64) string { return strconv.Itoa(int(math.Round(share*100))) + "%" }

func trades(lang, trade string) string {
	switch langOf(lang) {
	case "ru":
		if trade == "chakana" {
			return "розница"
		}
		return "опт"
	case "en":
		if trade == "chakana" {
			return "retail"
		}
		return "wholesale"
	}
	return trade
}

func place(lang, name string) string {
	if name == "" {
		return tr(lang, "country")
	}
	return name
}
