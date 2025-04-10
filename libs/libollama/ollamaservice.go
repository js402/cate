package libollama

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func SetupLocalInstance(ctx context.Context) (string, testcontainers.Container, func(), error) {
	cleanup := func() {}
	exposedPort := "11434/tcp"

	// Mount the unique volume
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:           "ollama/ollama:latest",
			ExposedPorts:    []string{exposedPort},
			WaitingFor:      wait.ForHTTP("/").WithStartupTimeout(10 * time.Second),
			AlwaysPullImage: false,
		},
		Started: false,
	})
	if err != nil {
		return "", nil, cleanup, err
	}
	cleanup = func() {
		timeout := time.Second
		container.Stop(ctx, &timeout)
	}
	err = container.Start(ctx)
	if err != nil {
		return "", nil, cleanup, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return "", nil, cleanup, err
	}

	mappedPort, err := container.MappedPort(ctx, "11434")
	if err != nil {
		return "", nil, cleanup, err
	}

	uri := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	u, err := url.Parse(uri)
	if err != nil {
		return "", nil, cleanup, err
	}

	client := api.NewClient(u, http.DefaultClient)

	const maxRetries = 5
	const retryInterval = 1 * time.Second
	var heartbeatErr error
	for attempt := range maxRetries {
		heartbeatErr = client.Heartbeat(ctx)
		if heartbeatErr == nil {
			break
		}
		if attempt < maxRetries-1 {
			time.Sleep(retryInterval)
		}
	}
	if heartbeatErr != nil {
		return "", nil, cleanup, heartbeatErr
	}

	return uri, container, cleanup, nil
}
