package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"bugun/internal/market"
	"bugun/internal/rfq"
	"bugun/internal/store/db"
)

func (s *Server) registerRFQ(api huma.API) {
	op := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: path, Summary: summary, Tags: []string{"rfq"}, Security: authRequired}
	}
	huma.Register(api, op("createRFQ", http.MethodPost, "/v1/rfqs", "Ask sellers in my viloyat for a price (entrepreneurs with a region, else 403 buyers_only)"), s.createRFQ)
	huma.Register(api, op("listMyRFQs", http.MethodGet, "/v1/me/rfqs", "My requests with offer counts (buyer)"), s.myRFQs)
	huma.Register(api, op("getRFQ", http.MethodGet, "/v1/rfqs/{id}", "One of my requests with its offers, market median and savings (buyer)"), s.getRFQ)
	huma.Register(api, op("cancelRFQ", http.MethodPost, "/v1/rfqs/{id}/cancel", "Close my open request (buyer)"), s.cancelRFQ)
	huma.Register(api, op("rfqInbox", http.MethodGet, "/v1/me/rfq-inbox", "Open requests I can answer, never my own (entrepreneurs with a region, else 403 sellers_only)"), s.rfqInbox)
	huma.Register(api, op("offerRFQ", http.MethodPost, "/v1/rfqs/{id}/offer", "Quote a price, or say I don't have it; not on my own request (entrepreneurs with a region, else 403 sellers_only)"), s.offerRFQ)
	huma.Register(api, op("chooseOffer", http.MethodPost, "/v1/offers/{id}/choose", "Pick an offer; the seller then confirms (buyer)"), s.chooseOffer)
	huma.Register(api, op("answerOffer", http.MethodPost, "/v1/offers/{id}/answer", "Agree to share contacts with the buyer, or refuse (seller)"), s.answerOffer)
	huma.Register(api, op("offerContact", http.MethodGet, "/v1/offers/{id}/contact", "The other side's contact once both agreed"), s.offerContact)
}

// RFQ is a buyer's request as its owner or a seller sees it.
type RFQ struct {
	ID          int64     `json:"id"`
	ProductID   int16     `json:"product_id"`
	ProductName string    `json:"product_name"`
	Quantity    float64   `json:"quantity"`
	Unit        string    `json:"unit"`
	RegionName  string    `json:"region_name"`
	Deadline    time.Time `json:"deadline"`
	MaxPriceSom *int64    `json:"max_price_som,omitempty"`
	Note        string    `json:"note,omitempty"`
	Status      string    `json:"status" enum:"open,closed,cancelled" doc:"closed once the deadline has passed"`
	CreatedAt   time.Time `json:"created_at"`
}

// rfqStatus reports an open RFQ past its deadline as closed.
func rfqStatus(status string, deadline time.Time) string {
	if status == "open" && !deadline.After(time.Now()) {
		return "closed"
	}
	return status
}

func optPrice(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func numFloat(n pgtype.Numeric) float64 {
	if f := market.Float(n); f != nil {
		return *f
	}
	return 0
}

type RFQBody struct {
	ProductID   int16   `json:"product_id" minimum:"1"`
	Quantity    float64 `json:"quantity" exclusiveMinimum:"0" doc:"In the product's base unit"`
	Days        int     `json:"days,omitempty" minimum:"1" doc:"How long sellers may answer; 1–7 days, 3 by default (current limits: GET /v1/meta/limits)"`
	MaxPriceSom *int64  `json:"max_price_som,omitempty" minimum:"50" doc:"Highest price per unit the buyer would pay"`
	Note        string  `json:"note,omitempty" maxLength:"300"`
}

type CreateRFQInput struct {
	SessionInput
	Body RFQBody
}

type CreateRFQOutput struct {
	Body struct {
		RFQ     RFQ `json:"rfq"`
		Sellers int `json:"sellers" doc:"How many sellers were asked"`
	}
}

func (s *Server) createRFQ(ctx context.Context, in *CreateRFQInput) (*CreateRFQOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	c, err := s.rfq.Create(ctx, u, rfq.Input{
		ProductID: in.Body.ProductID, Quantity: in.Body.Quantity, Days: in.Body.Days,
		MaxPriceSom: in.Body.MaxPriceSom, Note: in.Body.Note,
	})
	if err != nil {
		return nil, rfqError(ctx, err)
	}
	out := &CreateRFQOutput{}
	out.Body.RFQ, err = s.rfqView(ctx, c.RFQ)
	if err != nil {
		return nil, internalError(ctx, "rfq view", err)
	}
	out.Body.Sellers = c.Sellers
	return out, nil
}

func (s *Server) rfqView(ctx context.Context, r db.Rfq) (RFQ, error) {
	p, err := s.q.GetProduct(ctx, r.ProductID)
	if err != nil {
		return RFQ{}, err
	}
	reg, err := s.q.GetRegion(ctx, r.RegionID)
	if err != nil {
		return RFQ{}, err
	}
	return RFQ{
		ID: r.ID, ProductID: r.ProductID, ProductName: p.Name, Quantity: numFloat(r.Quantity), Unit: r.Unit,
		RegionName: reg.Name, Deadline: r.Deadline, MaxPriceSom: optPrice(r.MaxPriceSom), Note: r.Note.String,
		Status: rfqStatus(r.Status, r.Deadline), CreatedAt: r.CreatedAt,
	}, nil
}

type MyRFQ struct {
	RFQ
	Offers    int32 `json:"offers" doc:"Sellers who quoted a price"`
	BestPrice int64 `json:"best_price,omitempty" doc:"Lowest quoted price per unit"`
}

type MyRFQsOutput struct {
	Body struct {
		RFQs []MyRFQ `json:"rfqs"`
	}
}

func (s *Server) myRFQs(ctx context.Context, in *SessionInput) (*MyRFQsOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.BuyerRFQs(ctx, u.ID)
	if err != nil {
		return nil, internalError(ctx, "buyer rfqs", err)
	}
	out := &MyRFQsOutput{}
	out.Body.RFQs = make([]MyRFQ, len(rows))
	for i, r := range rows {
		out.Body.RFQs[i] = MyRFQ{
			RFQ: RFQ{
				ID: r.ID, ProductID: r.ProductID, ProductName: r.ProductName, Quantity: numFloat(r.Quantity), Unit: r.Unit,
				RegionName: r.RegionName, Deadline: r.Deadline, MaxPriceSom: optPrice(r.MaxPriceSom), Note: r.Note.String,
				Status: rfqStatus(r.Status, r.Deadline), CreatedAt: r.CreatedAt,
			},
			Offers: r.Offers, BestPrice: r.BestPrice,
		}
	}
	return out, nil
}

type IDInput struct {
	SessionInput
	ID int64 `path:"id" minimum:"1"`
}

// RFQOffer is one seller's answer as the buyer sees it; contacts come from
// /v1/offers/{id}/contact once both sides agreed.
type RFQOffer struct {
	ID           int64     `json:"id"`
	PriceSom     int64     `json:"price_som"`
	Quantity     *float64  `json:"quantity,omitempty"`
	Status       string    `json:"status" enum:"sent,chosen,confirmed,rejected"`
	SellerName   string    `json:"seller_name"`
	SellerRegion string    `json:"seller_region,omitempty"`
	SellerMarket string    `json:"seller_market,omitempty"`
	DiffPercent  *int      `json:"diff_percent,omitempty" doc:"Price vs. the market median, in percent"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RFQDetailOutput struct {
	Body struct {
		RFQ    RFQ        `json:"rfq"`
		Offers []RFQOffer `json:"offers" doc:"Cheapest first"`
		Median int64      `json:"median,omitempty" doc:"Wholesale market median in the RFQ's viloyat (0 when fewer than 3 prices)"`
		// Savings is spec §10.6: (median − cheapest live offer) × quantity.
		Savings int64 `json:"savings,omitempty" doc:"How much the cheapest offer saves against the market median, in so'm"`
	}
}

func (s *Server) getRFQ(ctx context.Context, in *IDInput) (*RFQDetailOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	r, err := s.rfq.Get(ctx, u, in.ID)
	if err != nil {
		return nil, rfqError(ctx, err)
	}
	out := &RFQDetailOutput{}
	if out.Body.RFQ, err = s.rfqView(ctx, r); err != nil {
		return nil, internalError(ctx, "rfq view", err)
	}
	province, err := market.ProvinceOf(ctx, s.q, r.RegionID)
	if err != nil {
		return nil, internalError(ctx, "rfq province", err)
	}
	st, err := s.q.ProductStats(ctx, db.ProductStatsParams{ProductID: r.ProductID, TradeType: "ulgurji", RegionID: optRegion(province)})
	if err != nil {
		return nil, internalError(ctx, "rfq median", err)
	}
	var median float64
	if st.Offers >= 3 {
		median = st.Median
		out.Body.Median = int64(math.Round(median))
	}
	rows, err := s.q.RFQOffers(ctx, r.ID)
	if err != nil {
		return nil, internalError(ctx, "rfq offers", err)
	}
	out.Body.Offers = make([]RFQOffer, len(rows))
	var cheapest int64
	for i, o := range rows {
		ro := RFQOffer{
			ID: o.ID, PriceSom: o.PriceSom.Int64, Quantity: market.Float(o.Quantity), Status: o.Status,
			SellerName: o.SellerName, SellerRegion: o.SellerRegion.String, SellerMarket: o.SellerMarket, UpdatedAt: o.UpdatedAt,
		}
		if median > 0 {
			d := int(math.Round(100 * (float64(o.PriceSom.Int64) - median) / median))
			ro.DiffPercent = &d
		}
		if o.Status != rfq.StatusRejected && (cheapest == 0 || o.PriceSom.Int64 < cheapest) {
			cheapest = o.PriceSom.Int64
		}
		out.Body.Offers[i] = ro
	}
	out.Body.Savings = rfq.Savings(median, cheapest, numFloat(r.Quantity))
	return out, nil
}

func (s *Server) cancelRFQ(ctx context.Context, in *IDInput) (*struct{}, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if err := s.rfq.Cancel(ctx, u, in.ID); err != nil {
		return nil, rfqError(ctx, err)
	}
	return nil, nil
}

// InboxRFQ is a request a seller can answer, or one where the buyer picked the
// seller's offer (listed first, even after the deadline), with their answer.
type InboxRFQ struct {
	RFQ
	OfferID     int64  `json:"offer_id,omitempty"`
	OfferStatus string `json:"offer_status,omitempty" enum:"sent,chosen,confirmed,rejected,declined" doc:"Empty when not answered yet"`
	OfferPrice  int64  `json:"offer_price,omitempty"`
}

type InboxOutput struct {
	Body struct {
		RFQs []InboxRFQ `json:"rfqs"`
	}
}

func (s *Server) rfqInbox(ctx context.Context, in *SessionInput) (*InboxOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	if !market.CanSell(u) {
		return nil, rfqError(ctx, rfq.ErrNotSeller)
	}
	rows, err := s.q.SellerInbox(ctx, u.ID)
	if err != nil {
		return nil, internalError(ctx, "rfq inbox", err)
	}
	out := &InboxOutput{}
	out.Body.RFQs = make([]InboxRFQ, len(rows))
	for i, r := range rows {
		out.Body.RFQs[i] = InboxRFQ{
			RFQ: RFQ{
				ID: r.ID, ProductID: r.ProductID, ProductName: r.ProductName, Quantity: numFloat(r.Quantity), Unit: r.Unit,
				RegionName: r.RegionName, Deadline: r.Deadline, MaxPriceSom: optPrice(r.MaxPriceSom), Note: r.Note.String,
				Status: rfqStatus(r.Status, r.Deadline), CreatedAt: r.CreatedAt,
			},
			OfferID: r.OfferID.Int64, OfferStatus: r.OfferStatus.String, OfferPrice: r.OfferPrice.Int64,
		}
	}
	return out, nil
}

type OfferInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		PriceSom int64    `json:"price_som,omitempty" doc:"Price per unit; omit or 0 with none=true for \"Menda yo'q\""`
		Quantity *float64 `json:"quantity,omitempty" exclusiveMinimum:"0" doc:"How much the seller can supply, in the base unit"`
		None     bool     `json:"none,omitempty" doc:"The seller does not have it (\"Menda yo'q\")"`
	}
}

// OfferState is an offer's state after an action.
type OfferState struct {
	Body struct {
		ID       int64  `json:"id"`
		Status   string `json:"status" enum:"sent,chosen,confirmed,rejected,declined"`
		PriceSom int64  `json:"price_som,omitempty"`
	}
}

func offerState(o db.Offer) *OfferState {
	out := &OfferState{}
	out.Body.ID, out.Body.Status, out.Body.PriceSom = o.ID, o.Status, o.PriceSom.Int64
	return out
}

func (s *Server) offerRFQ(ctx context.Context, in *OfferInput) (*OfferState, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	price := in.Body.PriceSom
	switch {
	case in.Body.None:
		price = 0
	case price == 0:
		return nil, invalidInput(fieldError("price_som", "required", "Narxni kiriting.", nil))
	}
	o, err := s.rfq.Offer(ctx, u, in.ID, price, in.Body.Quantity)
	if err != nil {
		return nil, rfqError(ctx, err)
	}
	return offerState(o), nil
}

func (s *Server) chooseOffer(ctx context.Context, in *IDInput) (*OfferState, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	o, err := s.rfq.Choose(ctx, u, in.ID)
	if err != nil {
		return nil, rfqError(ctx, err)
	}
	return offerState(o), nil
}

type AnswerInput struct {
	SessionInput
	ID   int64 `path:"id" minimum:"1"`
	Body struct {
		Agree bool `json:"agree" doc:"true shares contacts with the buyer, false refuses"`
	}
}

func (s *Server) answerOffer(ctx context.Context, in *AnswerInput) (*OfferState, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	o, err := s.rfq.Answer(ctx, u, in.ID, in.Body.Agree)
	if err != nil {
		return nil, rfqError(ctx, err)
	}
	return offerState(o), nil
}

type OfferContactOutput struct {
	Body struct {
		Name        string `json:"name"`
		TelegramURL string `json:"telegram_url,omitempty"`
		Phone       string `json:"phone,omitempty"`
	}
}

// offerContact gives each side the other's contact, as that person allows it.
func (s *Server) offerContact(ctx context.Context, in *IDInput) (*OfferContactOutput, error) {
	u, err := s.currentUser(ctx, in.Session)
	if err != nil {
		return nil, err
	}
	p, err := s.rfq.Parties(ctx, u, in.ID)
	if err != nil {
		return nil, rfqError(ctx, err)
	}
	name, username, byTelegram, byPhone, phoneEnc := p.SellerName, p.SellerUsername, p.SellerContactTelegram, p.SellerContactPhone, p.SellerPhoneEnc
	if u.ID == p.SellerID {
		name, username, byTelegram, byPhone, phoneEnc = p.BuyerName, p.BuyerUsername, p.BuyerContactTelegram, p.BuyerContactPhone, p.BuyerPhoneEnc
	}
	out := &OfferContactOutput{}
	out.Body.Name = name
	if byTelegram && username.Valid {
		out.Body.TelegramURL = "https://t.me/" + username.String
	}
	if byPhone && len(phoneEnc) > 0 {
		phone, err := s.sealer.Open(phoneEnc)
		if err != nil {
			return nil, internalError(ctx, "open phone", err)
		}
		out.Body.Phone = phone
	}
	if out.Body.TelegramURL == "" && out.Body.Phone == "" {
		return nil, apiError(http.StatusNotFound, "contact_hidden", "Bu foydalanuvchi hozircha bog'lanish usulini ochmagan.")
	}
	return out, nil
}

func rfqError(ctx context.Context, err error) error {
	var ve *rfq.ValidationError
	switch {
	case errors.As(err, &ve):
		return invalidInput(fieldError(ve.Field, ve.Code, ve.Message, ve.Params))
	case errors.Is(err, rfq.ErrNotBuyer):
		return apiError(http.StatusForbidden, "buyers_only", "So'rov yuborish uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting.")
	case errors.Is(err, rfq.ErrNotSeller):
		return apiError(http.StatusForbidden, "sellers_only", "So'rovga javob berish uchun profilda «Tadbirkor» rolini tanlang va hududingizni kiriting.")
	case errors.Is(err, rfq.ErrTooManyOpen):
		return apiError(http.StatusConflict, "rfq_limit", fmt.Sprintf("Bir vaqtda ko'pi bilan %d ta ochiq so'rov bo'lishi mumkin. Birini yoping yoki muddati tugashini kuting.", rfq.OpenLimit(err)))
	case errors.Is(err, rfq.ErrNotFound):
		return apiError(http.StatusNotFound, "rfq_not_found", "So'rov yoki taklif topilmadi.")
	case errors.Is(err, rfq.ErrClosed):
		return apiError(http.StatusConflict, "rfq_closed", "Bu so'rov yopilgan.")
	case errors.Is(err, rfq.ErrNotEligible):
		return apiError(http.StatusForbidden, "rfq_not_eligible", "Bu so'rovga javob berish uchun viloyatda shu mahsulotdan faol e'loningiz bo'lishi kerak.")
	case errors.Is(err, rfq.ErrOfferLocked):
		return apiError(http.StatusConflict, "offer_locked", "Xaridor taklifingizni tanlagan, endi uni o'zgartirib bo'lmaydi.")
	case errors.Is(err, rfq.ErrBadState):
		return apiError(http.StatusConflict, "offer_state", "Bu amalni taklifning hozirgi holatida bajarib bo'lmaydi.")
	}
	return internalError(ctx, "rfq", err)
}
