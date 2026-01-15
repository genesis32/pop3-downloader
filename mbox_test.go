package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMessageID(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "standard message-id",
			content: `From: test@example.com
To: recipient@example.com
Message-ID: <abc123@example.com>
Subject: Test

Body`,
			expected: "<abc123@example.com>",
		},
		{
			name: "lowercase message-id header",
			content: `From: test@example.com
message-id: <lower123@example.com>
Subject: Test

Body`,
			expected: "<lower123@example.com>",
		},
		{
			name: "mixed case message-id header",
			content: `From: test@example.com
Message-Id: <mixed123@example.com>
Subject: Test

Body`,
			expected: "<mixed123@example.com>",
		},
		{
			name: "no message-id",
			content: `From: test@example.com
Subject: Test

Body`,
			expected: "",
		},
		{
			name: "message-id with extra whitespace",
			content: `From: test@example.com
Message-ID:   <spaced123@example.com>  
Subject: Test

Body`,
			expected: "<spaced123@example.com>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMessageID([]byte(tt.content))
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWriteMbox_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	email1 := loadTestEmailContent(t, "sample_email_1.txt")
	email2 := loadTestEmailContent(t, "sample_email_2.txt")

	messages := []MessageData{
		{ID: 1, Content: email1},
		{ID: 2, Content: email2},
	}

	err := writeMbox(messages, mboxPath)
	if err != nil {
		t.Fatalf("writeMbox failed: %v", err)
	}

	// Verify file exists and contains expected data
	content, err := os.ReadFile(mboxPath)
	if err != nil {
		t.Fatalf("failed to read mbox file: %v", err)
	}

	if !strings.Contains(string(content), "Test Email 1") {
		t.Error("mbox missing content from email 1")
	}
	if !strings.Contains(string(content), "Test Email 2") {
		t.Error("mbox missing content from email 2")
	}
	if !strings.Contains(string(content), "From MAILER-DAEMON") {
		t.Error("mbox missing From separator line")
	}
}

func TestWriteMbox_AppendToExisting(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	email1 := loadTestEmailContent(t, "sample_email_1.txt")
	email2 := loadTestEmailContent(t, "sample_email_2.txt")

	// Write first message
	messages1 := []MessageData{{ID: 1, Content: email1}}
	err := writeMbox(messages1, mboxPath)
	if err != nil {
		t.Fatalf("first writeMbox failed: %v", err)
	}

	// Append second message
	messages2 := []MessageData{{ID: 2, Content: email2}}
	err = writeMbox(messages2, mboxPath)
	if err != nil {
		t.Fatalf("second writeMbox failed: %v", err)
	}

	// Verify both messages are present
	content, err := os.ReadFile(mboxPath)
	if err != nil {
		t.Fatalf("failed to read mbox file: %v", err)
	}

	if !strings.Contains(string(content), "Test Email 1") {
		t.Error("mbox missing content from email 1")
	}
	if !strings.Contains(string(content), "Test Email 2") {
		t.Error("mbox missing content from email 2")
	}
}

func TestWriteMbox_DeduplicatesMessages(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	email1 := loadTestEmailContent(t, "sample_email_1.txt")

	// Write message first time
	messages1 := []MessageData{{ID: 1, Content: email1}}
	err := writeMbox(messages1, mboxPath)
	if err != nil {
		t.Fatalf("first writeMbox failed: %v", err)
	}

	firstSize := getFileSize(t, mboxPath)

	// Try to write same message again (should be skipped as duplicate)
	messages2 := []MessageData{{ID: 1, Content: email1}}
	err = writeMbox(messages2, mboxPath)
	if err != nil {
		t.Fatalf("second writeMbox failed: %v", err)
	}

	secondSize := getFileSize(t, mboxPath)

	// File size should not have changed (duplicate was skipped)
	if secondSize != firstSize {
		t.Errorf("file size changed after duplicate write: %d -> %d", firstSize, secondSize)
	}
}

func TestWriteMbox_NoMessageID(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	emailNoMsgID := loadTestEmailContent(t, "sample_email_no_msgid.txt")

	// Write message without Message-ID
	messages := []MessageData{{ID: 1, Content: emailNoMsgID}}
	err := writeMbox(messages, mboxPath)
	if err != nil {
		t.Fatalf("writeMbox failed: %v", err)
	}

	// Verify message was written
	content, err := os.ReadFile(mboxPath)
	if err != nil {
		t.Fatalf("failed to read mbox file: %v", err)
	}

	if !strings.Contains(string(content), "Email Without Message-ID") {
		t.Error("mbox missing content from email without message-id")
	}

	firstSize := getFileSize(t, mboxPath)

	// Write same message again (should be written since no Message-ID for dedup)
	err = writeMbox(messages, mboxPath)
	if err != nil {
		t.Fatalf("second writeMbox failed: %v", err)
	}

	secondSize := getFileSize(t, mboxPath)

	// File size should have increased (no Message-ID to detect duplicate)
	if secondSize <= firstSize {
		t.Errorf("file size should have increased for message without Message-ID: %d -> %d", firstSize, secondSize)
	}
}

func TestGetExistingMessageIDs(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	email1 := loadTestEmailContent(t, "sample_email_1.txt")
	email2 := loadTestEmailContent(t, "sample_email_2.txt")

	messages := []MessageData{
		{ID: 1, Content: email1},
		{ID: 2, Content: email2},
	}

	err := writeMbox(messages, mboxPath)
	if err != nil {
		t.Fatalf("writeMbox failed: %v", err)
	}

	// Get existing message IDs
	existingIDs, err := getExistingMessageIDs(mboxPath)
	if err != nil {
		t.Fatalf("getExistingMessageIDs failed: %v", err)
	}

	if len(existingIDs) != 2 {
		t.Errorf("expected 2 message IDs, got %d", len(existingIDs))
	}

	if !existingIDs["<test-message-001@example.com>"] {
		t.Error("missing message ID 1")
	}
	if !existingIDs["<test-message-002@example.com>"] {
		t.Error("missing message ID 2")
	}
}

func TestGetExistingMessageIDs_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "nonexistent.mbox")

	existingIDs, err := getExistingMessageIDs(mboxPath)
	if err != nil {
		t.Fatalf("getExistingMessageIDs failed: %v", err)
	}

	if len(existingIDs) != 0 {
		t.Errorf("expected 0 message IDs for nonexistent file, got %d", len(existingIDs))
	}
}

func TestWriteMbox_EmptyMessages(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	messages := []MessageData{}

	err := writeMbox(messages, mboxPath)
	if err != nil {
		t.Fatalf("writeMbox failed: %v", err)
	}

	// File should not exist (no messages to write)
	if _, err := os.Stat(mboxPath); !os.IsNotExist(err) {
		t.Error("mbox file should not exist when no messages to write")
	}
}

func TestWriteMbox_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	mboxPath := filepath.Join(tmpDir, "test.mbox")

	email1 := loadTestEmailContent(t, "sample_email_1.txt")
	messages := []MessageData{{ID: 1, Content: email1}}

	err := writeMbox(messages, mboxPath)
	if err != nil {
		t.Fatalf("writeMbox failed: %v", err)
	}

	// Check file permissions (should be 0600)
	info, err := os.Stat(mboxPath)
	if err != nil {
		t.Fatalf("failed to stat mbox file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}

// Helper functions

func loadTestEmailContent(t *testing.T, filename string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("failed to load test email %s: %v", filename, err)
	}
	return content
}

func getFileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	return info.Size()
}
