package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-mbox"
)

// extractMessageID extracts the Message-ID header from an email message
func extractMessageID(content []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()

		// Stop at empty line (end of headers)
		if line == "" {
			break
		}

		// Look for Message-ID header (case-insensitive)
		if strings.HasPrefix(strings.ToLower(line), "message-id:") {
			// Extract the value after "Message-ID:"
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

// getExistingMessageIDs reads the mbox file and returns a set of all Message-IDs
func getExistingMessageIDs(path string) (map[string]bool, error) {
	messageIDs := make(map[string]bool)

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist, return empty set
		return messageIDs, nil
	}

	// Open mbox file for reading
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open mbox file for reading: %w", err)
	}
	defer file.Close()

	// Create mbox reader
	mboxReader := mbox.NewReader(file)

	// Read each message
	for {
		msg, err := mboxReader.NextMessage()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read message from mbox: %w", err)
		}

		// Read message content
		content, err := io.ReadAll(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to read message content: %w", err)
		}

		// Extract and store Message-ID
		msgID := extractMessageID(content)
		if msgID != "" {
			messageIDs[msgID] = true
		}
	}

	return messageIDs, nil
}

func writeMbox(messages []MessageData, path string) error {
	// Get existing Message-IDs to check for duplicates
	existingIDs, err := getExistingMessageIDs(path)
	if err != nil {
		return fmt.Errorf("failed to read existing messages: %w", err)
	}

	// Filter out duplicate messages
	newMessages := make([]MessageData, 0, len(messages))
	duplicateCount := 0

	for _, msg := range messages {
		msgID := extractMessageID(msg.Content)

		// Skip messages without Message-ID or that already exist
		if msgID == "" {
			// Messages without Message-ID are always written (can't determine duplicates)
			newMessages = append(newMessages, msg)
		} else if existingIDs[msgID] {
			// Duplicate found, skip it
			duplicateCount++
			fmt.Printf("Skipping duplicate message (Message-ID: %s)\n", msgID)
		} else {
			// New message, add it
			newMessages = append(newMessages, msg)
		}
	}

	// Report duplicate statistics
	if duplicateCount > 0 {
		fmt.Printf("Found %d duplicate message(s), skipping...\n", duplicateCount)
	}

	// If all messages are duplicates, nothing to write
	if len(newMessages) == 0 {
		fmt.Println("All messages are duplicates, nothing to write")
		return nil
	}

	// Open or create mbox file with appropriate permissions
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open mbox file: %w", err)
	}
	defer file.Close()

	// Create mbox writer
	mboxWriter := mbox.NewWriter(file)

	// Write each new message
	for _, msg := range newMessages {
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

	fmt.Printf("Wrote %d new message(s) to mbox\n", len(newMessages))

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
