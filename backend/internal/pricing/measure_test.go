package pricing

import (
	"context"
	"testing"
)

// Every attempt is logged with what the seller did; the stats count them
// and list the words nobody knew.
func TestParseLogAndStats(t *testing.T) {
	svc, u := testService(t)
	ctx := context.Background()
	pool := svc.pool
	if _, err := pool.Exec(ctx, `DELETE FROM parse_log`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM parse_log`) })

	ok, err := svc.CreateDraft(ctx, u, "sabzi 3100", ViaBot)
	if err != nil || ok.ID == 0 {
		t.Fatalf("draft %+v %v", ok, err)
	}
	if _, err := svc.Confirm(ctx, u, ok.ID); err != nil {
		t.Fatal(err)
	}
	bad, err := svc.CreateDraft(ctx, u, "sabzi 3200, 5 tonna", ViaWeb)
	if err != nil || bad.ID == 0 {
		t.Fatal(err)
	}
	if err := svc.Discard(ctx, u.ID, bad.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Discard(ctx, u.ID, bad.ID); err != nil {
		t.Fatalf("discarding twice: %v", err)
	}
	if _, err := svc.CreateDraft(ctx, u, "ananas 25000 zo'r", ViaWeb); err != nil { // nothing understood
		t.Fatal(err)
	}
	st, err := svc.Stats(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if st.Messages != 3 || st.Bot != 1 || st.Full != 2 || st.Failed != 1 || st.Confirmed != 1 || st.Discarded != 1 {
		t.Fatalf("stats %+v", st)
	}
	if st.FirstTry == nil || *st.FirstTry != 0.5 {
		t.Errorf("first try %v", st.FirstTry)
	}
	if len(st.Unknown) == 0 || st.Unknown[0].Word != "ananas" {
		t.Errorf("unknown words %+v", st.Unknown)
	}
	for _, w := range st.Unknown {
		if w.Word == "sabzi" || w.Word == "tonna" {
			t.Errorf("known word listed: %s", w.Word)
		}
	}
}

// A name the model read and the seller confirmed is read by the rules next
// time, without asking the model.
func TestLearnedNames(t *testing.T) {
	svc, u := testService(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = svc.pool.Exec(context.Background(), `DELETE FROM learned_aliases WHERE user_id = $1`, u.ID)
		_, _ = svc.pool.Exec(context.Background(), `DELETE FROM parse_log WHERE user_id = $1`, u.ID)
	})
	var potato int16
	if err := svc.pool.QueryRow(ctx, `SELECT id FROM products WHERE slug = 'kartoshka'`).Scan(&potato); err != nil {
		t.Fatal(err)
	}
	model := &fakeExtractor{cands: []Candidate{{Product: "kartoshka", Text: "bulba", Price: 4500, Trade: "chakana"}}}
	svc.SetExtractor(model)
	d, err := svc.CreateDraft(ctx, u, "bulba 4500", ViaBot)
	if err != nil || d.ID == 0 || len(d.Items) != 1 || d.Items[0].ProductID != potato {
		t.Fatalf("draft %+v %v", d, err)
	}
	if _, err := svc.Confirm(ctx, u, d.ID); err != nil {
		t.Fatal(err)
	}
	model.calls = 0
	d, err = svc.CreateDraft(ctx, u, "bulba 4600", ViaBot)
	if err != nil || len(d.Items) != 1 || d.Items[0].ProductID != potato || d.Items[0].PriceSom != 4600 || model.calls != 0 {
		t.Fatalf("second draft %+v %v, model calls %d", d, err, model.calls)
	}
	_ = svc.Discard(ctx, u.ID, d.ID)
}
