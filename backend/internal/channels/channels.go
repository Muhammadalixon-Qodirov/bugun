// Package channels reads prices from Telegram channels and groups whose owner
// added the bot (KANAL_ULASH.md). Only chats the owner connected are read.
package channels

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"bugun/internal/analytics"
	"bugun/internal/market"
	"bugun/internal/pricing"
	"bugun/internal/roles"
	"bugun/internal/store/db"
)

const (
	// Validity is how long a channel price counts; a deleted post is never
	// reported by Telegram, so prices must expire on their own.
	Validity = 7 * 24 * time.Hour
	// maxPostLen keeps huge posts out of the parser.
	maxPostLen = 4000
	// farFactor: a price this many times off the viloyat median is held.
	farFactor = 2.0
)

// Chat kinds.
const (
	KindChannel    = "channel"
	KindGroup      = "group"
	KindSupergroup = "supergroup"
)

var ErrNotFound = errors.New("channel not found")

type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	prices *pricing.Service
}

// modelConfidence marks prices a language model read.
const modelConfidence = 0.5

func NewService(pool *pgxpool.Pool, prices *pricing.Service) *Service {
	return &Service{pool: pool, q: db.New(pool), prices: prices}
}

// Chat is the Telegram chat the bot was added to.
type Chat struct {
	ID       int64
	Kind     string
	Title    string
	Username string
}

// Connected records the bot joining a chat, added by the Telegram user
// addedBy (their profile when they have one). The channel is active when
// the owner's viloyat is known, else pending until they pick one.
func (s *Service) Connected(ctx context.Context, c Chat, addedBy int64, owner *db.User) (db.TelegramChannel, error) {
	p := db.UpsertTelegramChannelParams{ChatID: c.ID, Kind: c.Kind, Title: c.Title, AddedByTg: addedBy, Status: "pending",
		Username: pgtype.Text{String: c.Username, Valid: c.Username != ""}}
	if owner != nil {
		p.OwnerID = pgtype.UUID{Bytes: owner.ID, Valid: true}
		if owner.RegionID.Valid {
			prov, err := market.ProvinceOf(ctx, s.q, owner.RegionID.Int16)
			if err != nil {
				return db.TelegramChannel{}, err
			}
			p.ProvinceID, p.Status = pgtype.Int2{Int16: prov, Valid: true}, "active"
		}
	}
	return s.q.UpsertTelegramChannel(ctx, p)
}

// Removed records the bot leaving or being removed from a chat.
func (s *Service) Removed(ctx context.Context, chatID int64) error {
	return s.q.DisableTelegramChannel(ctx, chatID)
}

// Post is a channel post or group message.
type Post struct {
	ChatID    int64
	MessageID int64
	Text      string
	Edited    bool
	SenderTg  int64 // group messages: who wrote it; 0 in channels
}

// Ingested is what a post gave.
type Ingested struct {
	Channel   db.TelegramChannel
	Taken     []pricing.Item
	Held      int // far from the market, kept out of the median
	Problems  []pricing.Problem
	PostsSeen int32
	Listings  int  // owner's listings updated
	ByModel   bool // the rules found nothing; a language model read the prices
}

// Ingest reads prices from a post of a connected, active chat. It returns
// ok = false for a chat that is not connected or not active. An edited post
// replaces the prices its earlier version gave.
func (s *Service) Ingest(ctx context.Context, p Post) (Ingested, bool, error) {
	ch, err := s.q.GetTelegramChannel(ctx, p.ChatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ingested{}, false, nil
	}
	if err != nil {
		return Ingested{}, false, err
	}
	if ch.Status != "active" {
		return Ingested{}, false, nil
	}
	text := strings.TrimSpace(p.Text)
	if r := []rune(text); len(r) > maxPostLen {
		text = string(r[:maxPostLen])
	}
	und := s.prices.UnderstandChannel(ctx, text)
	res := und.Result
	out := Ingested{Channel: ch, Problems: res.Problems, ByModel: und.ByModel}
	confidence := pgtype.Float4{}
	if out.ByModel {
		confidence = pgtype.Float4{Float32: modelConfidence, Valid: true}
	}

	var seller *db.User
	switch {
	case ch.Kind == KindChannel && ch.OwnerID.Valid:
		if u, err := s.q.GetUser(ctx, ch.OwnerID.Bytes); err == nil {
			seller = &u
		}
	case ch.Kind != KindChannel && p.SenderTg != 0:
		if u, err := s.q.GetUserByTelegramID(ctx, p.SenderTg); err == nil && u.BotEnabled {
			seller = &u
		}
	}

	err = s.inTx(ctx, func(q *db.Queries) error {
		chat, msg := pgtype.Int8{Int64: p.ChatID, Valid: true}, pgtype.Int8{Int64: p.MessageID, Valid: true}
		if p.Edited {
			if err := q.ExpireTelegramPost(ctx, db.ExpireTelegramPostParams{ChatID: chat, MessageID: msg}); err != nil {
				return err
			}
		}
		source := pgtype.Text{}
		if ch.Username.Valid && ch.Kind != KindGroup {
			source = pgtype.Text{String: fmt.Sprintf("https://t.me/%s/%d", ch.Username.String, p.MessageID), Valid: true}
		}
		sellerID := pgtype.UUID{}
		if seller != nil && roles.IsEntrepreneur(seller.Role.String) {
			sellerID = pgtype.UUID{Bytes: seller.ID, Valid: true}
		}
		for _, it := range res.Items {
			status, err := screen(ctx, q, it, ch.ProvinceID.Int16)
			if err != nil {
				return err
			}
			if status == "review" {
				out.Held++
			}
			if err := q.InsertTelegramObservation(ctx, db.InsertTelegramObservationParams{
				ProductID: it.ProductID, PriceSom: it.PriceSom, Unit: it.Unit, Quantity: numeric(it.Quantity),
				TradeType: it.TradeType, RegionID: ch.ProvinceID.Int16, SourceUrl: source, SellerID: sellerID,
				ValidUntil: time.Now().Add(Validity), RawText: pgtype.Text{String: it.Raw, Valid: it.Raw != ""},
				Status: status, ChatID: chat, MessageID: msg, AiConfidence: confidence,
			}); err != nil {
				return err
			}
			out.Taken = append(out.Taken, it)
		}
		// The owner's own channel may keep their listings up to date.
		if ch.UpdateListings && ch.Kind == KindChannel && seller != nil && market.CanSell(*seller) {
			for _, it := range res.Items {
				if _, err := market.UpsertTx(ctx, q, *seller, market.Input{
					ProductID: it.ProductID, PriceSom: it.PriceSom, Stock: it.Quantity, TradeType: it.TradeType, RawText: text,
				}); err != nil {
					return err
				}
				out.Listings++
			}
		}
		seen, err := q.ChannelPostSeen(ctx, db.ChannelPostSeenParams{ID: ch.ID, Prices: int32(len(out.Taken))})
		out.PostsSeen = seen
		return err
	})
	return out, true, err
}

// screen holds a price far from the viloyat's live median, like a typo.
func screen(ctx context.Context, q *db.Queries, it pricing.Item, province int16) (string, error) {
	m, err := q.ProvinceLiveMedian(ctx, db.ProvinceLiveMedianParams{ProductID: it.ProductID, TradeType: it.TradeType, ProvinceID: province})
	if err != nil {
		return "", err
	}
	if m.Offers >= analytics.MinObservations && m.Median > 0 {
		if r := float64(it.PriceSom) / m.Median; r > farFactor || r < 1/farFactor {
			return "review", nil
		}
	}
	return "active", nil
}

// Mine lists the chats a person connected (by profile or Telegram account).
func (s *Service) Mine(ctx context.Context, u db.User) ([]db.ListMyChannelsRow, error) {
	return s.q.ListMyChannels(ctx, db.ListMyChannelsParams{OwnerID: pgtype.UUID{Bytes: u.ID, Valid: true}, TelegramID: u.TelegramID})
}

// SetProvince sets a chat's viloyat and activates it for its adder.
func (s *Service) SetProvince(ctx context.Context, u db.User, id int64, province int16) (db.TelegramChannel, error) {
	c, err := s.q.SetChannelProvince(ctx, db.SetChannelProvinceParams{
		OwnerID: pgtype.UUID{Bytes: u.ID, Valid: true}, ProvinceID: pgtype.Int2{Int16: province, Valid: true}, ID: id, TelegramID: u.TelegramID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// SetUpdateListings turns on or off updating the owner's listings from posts.
func (s *Service) SetUpdateListings(ctx context.Context, u db.User, id int64, on bool) (db.TelegramChannel, error) {
	c, err := s.q.SetChannelUpdateListings(ctx, db.SetChannelUpdateListingsParams{UpdateListings: on, ID: id, OwnerID: pgtype.UUID{Bytes: u.ID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// Disconnect stops reading a chat and returns its Telegram id, so the bot can leave it.
func (s *Service) Disconnect(ctx context.Context, u db.User, id int64) (int64, error) {
	chat, err := s.q.DisconnectChannel(ctx, db.DisconnectChannelParams{ID: id, OwnerID: pgtype.UUID{Bytes: u.ID, Valid: true}, TelegramID: u.TelegramID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return chat, err
}

func (s *Service) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func numeric(v *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if v != nil {
		_ = n.Scan(strconv.FormatFloat(*v, 'f', -1, 64))
	}
	return n
}
