package main

import (
	"fmt"

	pop3 "github.com/knadh/go-pop3"
)

type MessageData struct {
	ID      int
	Content []byte
}

func connectPOP3S(config Config) (*pop3.Conn, error) {
	// Create POP3 client with TLS enabled
	opt := pop3.Opt{
		Host:       config.Host,
		Port:       config.Port,
		TLSEnabled: true,
	}

	client := pop3.New(opt)

	// Create connection
	conn, err := client.NewConn()
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Authenticate
	err = conn.Auth(config.Username, config.Password)
	if err != nil {
		conn.Quit()
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return conn, nil
}

func fetchAllMessages(conn *pop3.Conn) ([]MessageData, error) {
	// Get message count
	count, _, err := conn.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}

	if count == 0 {
		return []MessageData{}, nil
	}

	messages := make([]MessageData, 0, count)

	// Retrieve each message
	for i := 1; i <= count; i++ {
		buf, err := conn.RetrRaw(i)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve message %d: %w", i, err)
		}

		messages = append(messages, MessageData{
			ID:      i,
			Content: buf.Bytes(),
		})
	}

	return messages, nil
}

func deleteMessages(conn *pop3.Conn, messages []MessageData) error {
	var firstErr error

	for _, msg := range messages {
		err := conn.Dele(msg.ID)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to delete message %d: %w", msg.ID, err)
			}
			fmt.Printf("Warning: failed to delete message %d: %v\n", msg.ID, err)
		}
	}

	return firstErr
}
