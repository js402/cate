package libbus

import (
	"context"
	"errors"
)

var (
	ErrConnectionClosed       = errors.New("connection closed")
	ErrStreamSubscriptionFail = errors.New("stream subscription failed")
	ErrMessagePublish         = errors.New("message publishing failed")
)

// Real-time event notifications (e.g., job state updates to a UI)
// Triggering ephemeral tasks (e.g., quick, non-persistent jobs)
// Distributing lightweight messages between services
type Messenger interface {
	// Publish sends a message on the given subject.
	Publish(ctx context.Context, subject string, data []byte) error

	// Stream streams messages (using channels) from the given subject.
	Stream(ctx context.Context, subject string, ch chan<- []byte) (Subscription, error)

	// Close cleans up any underlying resources.
	Close() error
}

type Subscription interface {
	Unsubscribe() error
}
