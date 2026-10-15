package main

// seedProduct describes how demo listings for a catalog product look.
//
// Price is a typical wholesale price per base unit in late September 2026.
// Sources: Food City and Chorsu wholesale review (Ministry of Economy and
// Finance, 25.09.2026), Namangan wholesale Telegram channels (flour, grain,
// meat, 19–25.09.2026), narxtahlil.uz retail for Namangan (16.09.2026) and
// OLX asking prices (2026). Herbs, some fruit, services and a few processed
// foods have no 2026 quote and are estimates.
type seedProduct struct {
	Price     float64
	Query     string
	Varieties []variety
	// Markup is the retail premium over wholesale; 0 uses the category default.
	Markup float64
	// OffSeason products are rare in late September: few sellers, cold storage
	// or imports, so they get fewer listings.
	OffSeason bool
}

// variety is a sort with its price relative to the product's Price. Spreads
// stay within about 30% because the review check compares all varieties of a
// product together.
type variety struct {
	Name   string
	Factor float64
}

func v(name string, factor float64) variety { return variety{name, factor} }

// categoryMarkup is the usual retail premium: small for staples and meat
// (about 10–20% in Namangan), larger for cheap vegetables and fruit.
var categoryMarkup = map[string]float64{
	"sabzavot":   0.35,
	"meva":       0.30,
	"gosht":      0.15,
	"don":        0.15,
	"oziq_ovqat": 0.12,
	"qurilish":   0.10,
	"xizmat":     0.10,
}

var seedProducts = map[string]seedProduct{
	// sabzavot — Food City 25.09.2026 unless noted
	"kartoshka":        {Price: 3300, Query: "potatoes", Varieties: []variety{v("qizil", 1), v("oq", 0.95), v("Sante", 1.1), v("Arizona", 1.1)}},
	"piyoz":            {Price: 1900, Query: "onions", Varieties: []variety{v("oq", 0.95), v("qizil", 1.1), v("sariq", 1)}, Markup: 0.5},
	"sabzi":            {Price: 3500, Query: "carrots", Varieties: []variety{v("sariq", 1.05), v("qizil", 0.95)}, Markup: 0.5},
	"pomidor":          {Price: 6000, Query: "tomatoes", Varieties: []variety{v("pushti", 1.15), v("qizil", 1), v("cherri", 1.3)}, Markup: 0.2},
	"bodring":          {Price: 5000, Query: "cucumbers", Markup: 0.4},
	"karam":            {Price: 2500, Query: "cabbage", Markup: 0.5},
	"sarimsoq":         {Price: 8000, Query: "garlic", Markup: 0.6},
	"bulgor":           {Price: 5000, Query: "bell peppers", Varieties: []variety{v("qizil", 1.1), v("sariq", 1.15), v("yashil", 0.9)}},
	"lavlagi":          {Price: 3500, Query: "beetroot"},
	"turp":             {Price: 2500, Query: "radish"},
	"sholgom":          {Price: 2500, Query: "turnips"}, // estimate
	"baqlajon":         {Price: 4500, Query: "eggplant", Markup: 0.2},
	"kabachok":         {Price: 3000, Query: "zucchini"},                                                                 // estimate
	"qovoq":            {Price: 2000, Query: "pumpkin"},                                                                  // estimate
	"gulkaram":         {Price: 6000, Query: "cauliflower"},                                                              // estimate
	"brokkoli":         {Price: 12000, Query: "broccoli"},                                                                // estimate
	"kok-piyoz":        {Price: 8000, Query: "spring onions"},                                                            // estimate
	"shivit":           {Price: 10000, Query: "dill herb"},                                                               // estimate
	"petrushka":        {Price: 10000, Query: "parsley"},                                                                 // estimate
	"kashnich":         {Price: 8000, Query: "coriander leaves"},                                                         // estimate
	"rayhon":           {Price: 10000, Query: "basil"},                                                                   // estimate
	"achchiq-qalampir": {Price: 8000, Query: "chili peppers"},                                                            // estimate
	"makkajoxori":      {Price: 3000, Query: "corn cobs"},                                                                // estimate
	"ismaloq":          {Price: 8000, Query: "spinach"},                                                                  // estimate
	"salat-bargi":      {Price: 10000, Query: "lettuce"},                                                                 // estimate
	"zanjabil":         {Price: 35000, Query: "ginger root"},                                                             // estimate, imported
	"qizil-karam":      {Price: 4000, Query: "red cabbage"},                                                              // estimate
	"qoziqorin":        {Price: 30000, Query: "mushrooms", Varieties: []variety{v("shampinon", 1), v("veshenka", 0.85)}}, // estimate
	// meva — Food City 25.09.2026 unless noted
	"olma":     {Price: 7500, Query: "apples", Varieties: []variety{v("Gala", 1.1), v("Golden", 1), v("Semerenko", 0.9), v("Fuji", 1.2)}},
	"uzum":     {Price: 9000, Query: "grapes", Varieties: []variety{v("Kishmish", 1.1), v("Husayni", 1.2), v("Toifi", 0.9)}},
	"anor":     {Price: 15000, Query: "pomegranates", Varieties: []variety{v("Qozoqi", 1.1), v("Achchiq-dona", 0.9)}}, // estimate
	"limon":    {Price: 30000, Query: "lemons", Markup: 0.5},
	"nok":      {Price: 14000, Query: "pears"},
	"orik":     {Price: 30000, Query: "apricots", OffSeason: true},      // cold storage
	"shaftoli": {Price: 12000, Query: "peaches"},                        // late varieties, estimate
	"olcha":    {Price: 35000, Query: "sour cherries", OffSeason: true}, // cold storage
	"gilos":    {Price: 60000, Query: "cherries", OffSeason: true},      // imported
	"qulupnay": {Price: 60000, Query: "strawberries", OffSeason: true},  // greenhouse or imported
	"tarvuz":   {Price: 2200, Query: "watermelon"},
	"qovun":    {Price: 3500, Query: "melon", Varieties: []variety{v("Gulobi", 1.1), v("Ich qizil", 1), v("Mirzachul", 1.15)}},
	"xurmo":    {Price: 12000, Query: "persimmon"}, // estimate
	"behi":     {Price: 12000, Query: "quince fruit"},
	"anjir":    {Price: 18000, Query: "figs"}, // estimate
	"banan":    {Price: 14000, Query: "bananas"},
	"apelsin":  {Price: 14000, Query: "oranges"},
	"mandarin": {Price: 16000, Query: "mandarins"},
	"kivi":     {Price: 20000, Query: "kiwi fruit"},
	"olxori":   {Price: 10000, Query: "plums"},          // estimate
	"malina":   {Price: 45000, Query: "raspberries"},    // estimate
	"yongoq":   {Price: 85000, Query: "walnuts"},        // kernel, estimate
	"bodom":    {Price: 78000, Query: "almonds"},        // in shell, OLX Samarqand 24.09.2026
	"pista":    {Price: 170000, Query: "pistachios"},    // estimate
	"mayiz":    {Price: 40000, Query: "raisins"},        // estimate
	"turshak":  {Price: 35000, Query: "dried apricots"}, // quritilgan_mevalar 09.09.2026
	// gosht — Chorsu 25.09.2026 and Namangan channels (Aug–Sep 2026)
	"mol-goshti":   {Price: 100000, Query: "beef meat"},
	"qoy-goshti":   {Price: 112000, Query: "lamb meat"},
	"tovuq":        {Price: 29000, Query: "whole chicken meat", Markup: 0.1},
	"qiyma":        {Price: 95000, Query: "minced meat"}, // estimate
	"jigar":        {Price: 60000, Query: "liver meat"},  // estimate
	"kurka":        {Price: 65000, Query: "turkey meat"}, // estimate
	"tovuq-filesi": {Price: 37000, Query: "chicken breast"},
	"baliq":        {Price: 27000, Query: "carp fish"},
	"bedana":       {Price: 17000, Query: "quail"},             // per bird, OLX Aug 2026
	"ot-goshti":    {Price: 120000, Query: "raw meat butcher"}, // estimate
	"qazi":         {Price: 200000, Query: "sausage"},          // estimate
	// don — Namangan channels 20–25.09.2026 and Food City
	"guruch":      {Price: 16000, Query: "rice grains", Varieties: []variety{v("Lazer", 0.95), v("Alanga", 1), v("Devzira", 1.3)}},
	"un":          {Price: 6600, Query: "flour sack", Varieties: []variety{v("oliy nav", 1.13), v("1-nav", 0.88)}},
	"bugdoy":      {Price: 3550, Query: "wheat grain"},
	"nohat":       {Price: 18000, Query: "chickpeas"},
	"mosh":        {Price: 16500, Query: "mung beans"},
	"arpa":        {Price: 3800, Query: "barley grain"},
	"joxori-doni": {Price: 3550, Query: "corn kernels"},
	"loviya":      {Price: 16000, Query: "beans"},
	"yasmiq":      {Price: 16000, Query: "lentils"},   // estimate
	"grechka":     {Price: 17000, Query: "buckwheat"}, // estimate
	"suli":        {Price: 9000, Query: "oats"},       // estimate
	"kunjut":      {Price: 26000, Query: "sesame seeds"},
	"makaron":     {Price: 10000, Query: "pasta"}, // estimate
	// oziq_ovqat — narxtahlil Namangan, OLX 2026; dairy and tea partly estimates
	"tuxum":           {Price: 1250, Query: "eggs", Markup: 0.08},
	"paxta-yogi":      {Price: 17000, Query: "cooking oil bottle"},
	"kungaboqar-yogi": {Price: 19500, Query: "sunflower oil"},
	"shakar":          {Price: 11500, Query: "sugar"},
	"tuz":             {Price: 2500, Query: "salt"}, // estimate
	"sut":             {Price: 6000, Query: "milk", Markup: 0.2},
	"qatiq":           {Price: 8000, Query: "yogurt"}, // estimate
	"suzma":           {Price: 20000, Query: "strained yogurt"},
	"pishloq":         {Price: 70000, Query: "cheese"}, // estimate
	"sariyog":         {Price: 55000, Query: "butter"},
	"qaymoq":          {Price: 50000, Query: "cream dairy"}, // estimate
	"asal":            {Price: 100000, Query: "honey jar"},
	"choy":            {Price: 60000, Query: "green tea leaves"}, // estimate
	"non":             {Price: 3000, Query: "flatbread"},         // estimate
	"tvorog":          {Price: 35000, Query: "cottage cheese"},   // estimate
	"tomat-pastasi":   {Price: 15000, Query: "tomato paste"},
	"zira":            {Price: 90000, Query: "cumin seeds"},
	// qurilish — OLX 2026 and Namangan channels
	"sement":      {Price: 650, Query: "cement bags"}, // 30–40 thousand per 50 kg bag
	"gisht":       {Price: 850, Query: "bricks"},
	"armatura":    {Price: 8300, Query: "rebar steel"},        // 7.0–8.7 million per tonne
	"qum":         {Price: 85000, Query: "construction sand"}, // estimate
	"shagal":      {Price: 100000, Query: "gravel"},           // estimate
	"taxta":       {Price: 3800000, Query: "lumber planks"},   // estimate
	"gipsokarton": {Price: 10000, Query: "drywall"},
	"kafel":       {Price: 60000, Query: "ceramic tiles"},
	"boyoq":       {Price: 15000, Query: "paint cans"}, // estimate
	"penoblok":    {Price: 13000, Query: "concrete blocks"},
	"shifer":      {Price: 51000, Query: "roof sheets"},
	// xizmat — OLX 2026; delivery and packing are estimates
	"yuk-tashish":   {Price: 150000, Query: "cargo truck"},
	"ombor-ijarasi": {Price: 30000, Query: "warehouse"},
	"qadoqlash":     {Price: 50000, Query: "packaging boxes"},
}

var namePrefixes = []string{
	"Barakali", "Oltin", "Yashil", "Toza", "Sharq", "Vodiy", "Bog'bon", "Hosil", "Ravon", "Omad",
	"Zamin", "Nurli", "Saxovat", "Mehnat", "Dehqon", "Tabiiy", "Sifat", "Ishonch", "Yangi", "Katta",
}

var nameSuffixes = []string{
	"Savdo", "Agro", "Fermer", "Bozor", "Ulgurji", "Baza", "Market", "Hosil", "Trade", "Mahsulot",
}

var descriptions = map[string][]string{
	"sabzavot":   {"Yangi uzilgan, saralangan.", "O'z dalamizdan, kimyoviy o'g'itsiz.", "Qopda yoki yashikda beramiz. Yetkazib berish kelishiladi.", "Omborda saqlangan, sifati kafolatlanadi."},
	"meva":       {"Shirin, saralangan. Yashikda.", "O'z bog'imizdan. Katta hajmga chegirma.", "Eksport sifatida, qadoqlangan.", "Yangi hosil. Oshxona va do'konlar uchun."},
	"gosht":      {"Har kuni yangi so'yilgan. Veterinariya hujjati bor.", "Restoran va oshxonalarga muntazam yetkazamiz.", "Muzlatilmagan, sovutgichda saqlanadi."},
	"don":        {"Qopda, tozalangan.", "Omborimizdan, katta hajmga chegirma.", "Sifat sertifikati bor."},
	"oziq_ovqat": {"Ishlab chiqaruvchidan to'g'ridan-to'g'ri.", "Do'kon va oshxonalar uchun ulgurji.", "Muddati yangi, hujjatlari bor."},
	"qurilish":   {"Omborda bor, tez yetkazamiz.", "Zavoddan to'g'ridan-to'g'ri narx.", "Katta hajmga alohida narx."},
	"xizmat":     {"Shahar ichida va viloyatlararo.", "Tajribali jamoa, shartnoma asosida.", "Oldindan buyurtma qabul qilinadi."},
}
