package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/emersion/go-mbox"
)

func writeMbox(messages []MessageData, path string) error {
	// Open or create mbox file with appropriate permissions
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open mbox file: %w", err)
	}
	defer file.Close()

	// Create mbox writer
	mboxWriter := mbox.NewWriter(file)

	// Write each message
	for _, msg := range messages {
		err := appendMessage(mboxWriter, msg)
		if err != nil {
			return fmt.Errorf("failed to write message %d: %w", msg.ID, err)
		}
	}

	// Sync to disk to ensure data is written
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file to disk: %w", err)
	}

	return nil
}

func appendMessage(mboxWriter *mbox.Writer, msg MessageData) error {
	// Create a message writer
	// The mbox.Writer handles the "From " separator line automatically
	// We just need to provide a time for the From line
	messageWriter, err := mboxWriter.CreateMessage("MAILER-DAEMON", time.Now())
	if err != nil {
		return fmt.Errorf("failed to create message writer: %w", err)
	}

	// Write the message content
	reader := bytes.NewReader(msg.Content)
	_, err = io.Copy(messageWriter, reader)
	if err != nil {
		return fmt.Errorf("failed to write message content: %w", err)
	}

	return nil
}
