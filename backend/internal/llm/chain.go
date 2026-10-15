package llm

import (
	"context"
	"errors"
	"net"
)

// Chatter answers chat completions: a Client, or a Chain of them.
type Chatter interface {
	Chat(ctx context.Context, req ChatRequest) (Message, error)
}

// Chain tries its clients in order: when one is rate limited (by the
// service or its own hourly limit), times out or fails on the service side,
// the next one answers. A free tier limits tokens per minute for each model
// separately, so a chain of models on one key multiplies what gets through.
// A request the service rejects as such (400, 401) is not retried.
type Chain []*Client

// NewChain keeps the non-nil clients; it returns nil when none is left.
func NewChain(clients ...*Client) Chain {
	var c Chain
	for _, cl := range clients {
		if cl != nil {
			c = append(c, cl)
		}
	}
	return c
}

func (c Chain) Chat(ctx context.Context, req ChatRequest) (Message, error) {
	err := ErrLimit
	for _, cl := range c {
		var msg Message
		if msg, err = cl.Chat(ctx, req); err == nil {
			return msg, nil
		}
		if !retryable(err) || ctx.Err() != nil {
			return Message{}, err
		}
	}
	return Message{}, err
}

func retryable(err error) bool {
	var se *StatusError
	var ne net.Error
	switch {
	case errors.Is(err, ErrLimit), errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return true
	case errors.As(err, &se):
		return se.Code == 429 || se.Code >= 500
	}
	return errors.Is(err, errBadAnswer)
}
