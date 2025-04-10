package libbus

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
)

func SetupNatsInstance(ctx context.Context) (string, testcontainers.Container, func(), error) {
	cleanup := func() {}
	natsContainer, err := nats.Run(ctx, "nats:2.10")
	if err != nil {
		return "", nil, cleanup, err
	}
	cleanup = func() {
		if err := testcontainers.TerminateContainer(natsContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	}
	cons, err := natsContainer.ConnectionString(ctx)
	if err != nil {
		return "", nil, cleanup, err
	}
	return cons, natsContainer, cleanup, nil
}

// NewTestPubSub starts a NATS container using SetupNatsInstance,
// creates a new PubSub instance, and returns it along with a cleanup function.
func NewTestPubSub(t *testing.T) (Messenger, func()) {
	ctx := context.Background()
	cons, container, cleanup, err := SetupNatsInstance(ctx)
	require.NoError(t, err)
	// Optionally log container status if needed.
	log.Printf("NATS container running: %v", container)

	cfg := &Config{
		NATSURL: cons,
	}
	ps, err := NewPubSub(ctx, cfg)
	require.NoError(t, err)

	// Return a cleanup function that closes PubSub and terminates the container.
	return ps, func() {
		_ = ps.Close()
		cleanup()
	}
}
