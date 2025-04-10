/*
Package libbus provides an interface for core publish-subscribe messaging.

Basic Usage:

	// Configuration (replace with your actual values)
	cfg := &libbus.Config{
		NATSURL: nats.DefaultURL, // "nats://127.0.0.1:4222"
		// NATSUser: "user",
		// NATSPassword: "password",
	}

	// Create a new Messenger instance
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	messenger, err := libbus.NewPubSub(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer messenger.Close() // Ensure connection is closed

	// --- Publish ---
	go func() {
		time.Sleep(100 * time.Millisecond)
		log.Println("Publishing message...")
		err := messenger.Publish(context.Background(), "updates.topic", []byte("hello world"))
		if err != nil {
			log.Printf("Publish failed: %v", err)
		}
	}()


	// --- Stream ---
	msgChan := make(chan []byte, 64)
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	sub, err := messenger.Stream(streamCtx, "updates.topic", msgChan)
	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}
	// defer sub.Unsubscribe() // Unsubscribe is handled internally when context is cancelled

	log.Println("Listening for messages...")
	select {
	case msgData := <-msgChan:
		log.Printf("Received message: %s", string(msgData))
		// Typically you'd loop here or handle messages in a goroutine
	case <-time.After(1 * time.Second): // Timeout example
		log.Println("Timeout waiting for message")
	}

	// To stop the stream, cancel its context
	// streamCancel()
*/
package libbus
