package libbus

import (
	"context"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

type ps struct {
	nc *nats.Conn
}

type natsSubscription struct {
	sub *nats.Subscription
}

type Config struct {
	NATSURL      string
	NATSPassword string
	NATSUser     string
}

func NewPubSub(ctx context.Context, cfg *Config) (Messenger, error) {
	var nc *nats.Conn
	var err error

	natsOpts := []nats.Option{
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Println("NATS connection closed")
			// TODO: Implement reconnection logic or notify the application state
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("NATS disconnected. Will autoreconnect: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS reconnected to %s", nc.ConnectedUrl())
		}),
	}

	if cfg.NATSUser == "" {
		// Connect without authentication
		log.Println("Connecting to NATS without authentication")
		nc, err = nats.Connect(cfg.NATSURL, natsOpts...)
	} else {
		// Connect WITH authentication
		log.Printf("Connecting to NATS with user %s", cfg.NATSUser)
		natsOpts = append(natsOpts, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword))
		nc, err = nats.Connect(cfg.NATSURL, natsOpts...)
	}

	// Check for connection errors
	if err != nil {
		log.Printf("Failed to connect to NATS at %s: %v", cfg.NATSURL, err)
		return nil, fmt.Errorf("failed to connect to NATS: %w", err) // Wrap error for context
	}

	log.Printf("Successfully connected to NATS at %s", nc.ConnectedUrl())
	return &ps{nc: nc}, nil
}

func (p *ps) Publish(ctx context.Context, subject string, data []byte) error {
	if p.nc == nil || p.nc.IsClosed() {
		return ErrConnectionClosed
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		err := p.nc.Publish(subject, data)
		if err != nil {
			return fmt.Errorf("%w: failed to publish to %s", ErrMessagePublish, subject)
		}
		return nil
	}
}

func (p *ps) Stream(ctx context.Context, subject string, ch chan<- []byte) (Subscription, error) {
	return p.stream(ctx, subject, "", ch)
}

func (p *ps) stream(ctx context.Context, subject, queue string, ch chan<- []byte) (Subscription, error) {
	if p.nc == nil || p.nc.IsClosed() {
		return nil, ErrConnectionClosed
	}

	natsChan := make(chan *nats.Msg, 1024)
	var sub *nats.Subscription
	var err error

	if queue == "" {
		sub, err = p.nc.ChanSubscribe(subject, natsChan)
	} else {
		sub, err = p.nc.ChanQueueSubscribe(subject, queue, natsChan)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: unable to subscribe to stream %s", ErrStreamSubscriptionFail, subject)
	}

	go func() {
		defer func() {
			err := sub.Unsubscribe()
			if err != nil {
				log.Printf("error unsubscribing from stream %s: %v", subject, err)
			}
		}()
		defer close(natsChan)

		for {
			select {
			case msg, ok := <-natsChan:
				if !ok {
					return
				}
				select {
				case ch <- msg.Data:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return &natsSubscription{sub: sub}, nil
}

func (p *ps) Close() error {
	if p.nc != nil && !p.nc.IsClosed() {
		p.nc.Close()
	}
	return nil
}

func (s *natsSubscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}
