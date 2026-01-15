package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestPOP3Server is a minimal POP3 server for testing
type TestPOP3Server struct {
	listener net.Listener
	messages []string
	username string
	password string
	deleted  map[int]bool
	mu       sync.Mutex
	wg       sync.WaitGroup
	closed   bool
}

// NewTestPOP3Server creates and starts a new test POP3 server
func NewTestPOP3Server(messages []string, username, password string) (*TestPOP3Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &TestPOP3Server{
		listener: listener,
		messages: messages,
		username: username,
		password: password,
		deleted:  make(map[int]bool),
	}

	server.wg.Add(1)
	go server.serve()

	return server, nil
}

func (s *TestPOP3Server) serve() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *TestPOP3Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send greeting
	conn.Write([]byte("+OK POP3 server ready\r\n"))

	buf := make([]byte, 4096)
	authenticated := false

	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		command := strings.TrimSpace(string(buf[:n]))
		parts := strings.SplitN(command, " ", 2)
		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "USER":
			if len(parts) < 2 {
				conn.Write([]byte("-ERR missing username\r\n"))
				continue
			}
			if parts[1] == s.username {
				conn.Write([]byte("+OK\r\n"))
			} else {
				conn.Write([]byte("-ERR invalid username\r\n"))
			}

		case "PASS":
			if len(parts) < 2 {
				conn.Write([]byte("-ERR missing password\r\n"))
				continue
			}
			if parts[1] == s.password {
				authenticated = true
				conn.Write([]byte("+OK logged in\r\n"))
			} else {
				conn.Write([]byte("-ERR invalid password\r\n"))
			}

		case "STAT":
			if !authenticated {
				conn.Write([]byte("-ERR not authenticated\r\n"))
				continue
			}
			s.mu.Lock()
			count := 0
			size := 0
			for i, msg := range s.messages {
				if !s.deleted[i+1] {
					count++
					size += len(msg)
				}
			}
			s.mu.Unlock()
			conn.Write([]byte(fmt.Sprintf("+OK %d %d\r\n", count, size)))

		case "LIST":
			if !authenticated {
				conn.Write([]byte("-ERR not authenticated\r\n"))
				continue
			}
			s.mu.Lock()
			conn.Write([]byte("+OK\r\n"))
			for i, msg := range s.messages {
				if !s.deleted[i+1] {
					conn.Write([]byte(fmt.Sprintf("%d %d\r\n", i+1, len(msg))))
				}
			}
			conn.Write([]byte(".\r\n"))
			s.mu.Unlock()

		case "RETR":
			if !authenticated {
				conn.Write([]byte("-ERR not authenticated\r\n"))
				continue
			}
			if len(parts) < 2 {
				conn.Write([]byte("-ERR missing message number\r\n"))
				continue
			}
			msgNum, err := strconv.Atoi(parts[1])
			if err != nil || msgNum < 1 || msgNum > len(s.messages) {
				conn.Write([]byte("-ERR invalid message number\r\n"))
				continue
			}
			s.mu.Lock()
			if s.deleted[msgNum] {
				s.mu.Unlock()
				conn.Write([]byte("-ERR message deleted\r\n"))
				continue
			}
			msg := s.messages[msgNum-1]
			s.mu.Unlock()
			conn.Write([]byte(fmt.Sprintf("+OK %d octets\r\n", len(msg))))
			conn.Write([]byte(msg))
			if !strings.HasSuffix(msg, "\r\n") {
				conn.Write([]byte("\r\n"))
			}
			conn.Write([]byte(".\r\n"))

		case "DELE":
			if !authenticated {
				conn.Write([]byte("-ERR not authenticated\r\n"))
				continue
			}
			if len(parts) < 2 {
				conn.Write([]byte("-ERR missing message number\r\n"))
				continue
			}
			msgNum, err := strconv.Atoi(parts[1])
			if err != nil || msgNum < 1 || msgNum > len(s.messages) {
				conn.Write([]byte("-ERR invalid message number\r\n"))
				continue
			}
			s.mu.Lock()
			s.deleted[msgNum] = true
			s.mu.Unlock()
			conn.Write([]byte("+OK deleted\r\n"))

		case "QUIT":
			conn.Write([]byte("+OK bye\r\n"))
			return

		case "NOOP":
			conn.Write([]byte("+OK\r\n"))

		case "RSET":
			s.mu.Lock()
			s.deleted = make(map[int]bool)
			s.mu.Unlock()
			conn.Write([]byte("+OK\r\n"))

		default:
			conn.Write([]byte("-ERR unknown command\r\n"))
		}
	}
}

// Addr returns the server address
func (s *TestPOP3Server) Addr() string {
	return s.listener.Addr().String()
}

// Port returns the server port
func (s *TestPOP3Server) Port() int {
	addr := s.listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// Close shuts down the server
func (s *TestPOP3Server) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.listener.Close()
}

// Deleted returns which message IDs were marked for deletion
func (s *TestPOP3Server) Deleted() map[int]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[int]bool)
	for k, v := range s.deleted {
		result[k] = v
	}
	return result
}

// loadTestEmail loads a test email from the testdata directory
func loadTestEmail(t *testing.T, filename string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("failed to load test email %s: %v", filename, err)
	}
	return string(content)
}

func TestFetchAllMessages_Success(t *testing.T) {
	email1 := loadTestEmail(t, "sample_email_1.txt")
	email2 := loadTestEmail(t, "sample_email_2.txt")

	server, err := NewTestPOP3Server([]string{email1, email2}, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Close()

	// Wait for server to be ready
	time.Sleep(10 * time.Millisecond)

	conn, err := connectPOP3("127.0.0.1", server.Port(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Quit()

	messages, err := fetchAllMessages(conn)
	if err != nil {
		t.Fatalf("fetchAllMessages failed: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	// Verify message IDs
	if messages[0].ID != 1 || messages[1].ID != 2 {
		t.Errorf("unexpected message IDs: %d, %d", messages[0].ID, messages[1].ID)
	}

	// Verify content contains expected data
	if !strings.Contains(string(messages[0].Content), "Test Email 1") {
		t.Errorf("message 1 content mismatch")
	}
	if !strings.Contains(string(messages[1].Content), "Test Email 2") {
		t.Errorf("message 2 content mismatch")
	}
}

func TestFetchAllMessages_Empty(t *testing.T) {
	server, err := NewTestPOP3Server([]string{}, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := connectPOP3("127.0.0.1", server.Port(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Quit()

	messages, err := fetchAllMessages(conn)
	if err != nil {
		t.Fatalf("fetchAllMessages failed: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestDeleteMessages_Success(t *testing.T) {
	email1 := loadTestEmail(t, "sample_email_1.txt")
	email2 := loadTestEmail(t, "sample_email_2.txt")

	server, err := NewTestPOP3Server([]string{email1, email2}, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := connectPOP3("127.0.0.1", server.Port(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Quit()

	messages := []MessageData{
		{ID: 1, Content: []byte(email1)},
		{ID: 2, Content: []byte(email2)},
	}

	err = deleteMessages(conn, messages)
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}

	deleted := server.Deleted()
	if !deleted[1] || !deleted[2] {
		t.Errorf("messages not marked as deleted: %v", deleted)
	}
}

func TestConnectPOP3_AuthFailure(t *testing.T) {
	server, err := NewTestPOP3Server([]string{}, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	_, err = connectPOP3("127.0.0.1", server.Port(), "testuser", "wrongpass")
	if err == nil {
		t.Fatal("expected authentication error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConnectPOP3_Success(t *testing.T) {
	server, err := NewTestPOP3Server([]string{}, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	conn, err := connectPOP3("127.0.0.1", server.Port(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("connectPOP3 failed: %v", err)
	}
	defer conn.Quit()
}

func TestFetchAndDelete_Integration(t *testing.T) {
	email1 := loadTestEmail(t, "sample_email_1.txt")
	email2 := loadTestEmail(t, "sample_email_2.txt")
	emailNoMsgID := loadTestEmail(t, "sample_email_no_msgid.txt")

	server, err := NewTestPOP3Server([]string{email1, email2, emailNoMsgID}, "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Close()

	time.Sleep(10 * time.Millisecond)

	// Connect and fetch
	conn, err := connectPOP3("127.0.0.1", server.Port(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	messages, err := fetchAllMessages(conn)
	if err != nil {
		t.Fatalf("fetchAllMessages failed: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}

	// Delete messages
	err = deleteMessages(conn, messages)
	if err != nil {
		t.Fatalf("deleteMessages failed: %v", err)
	}

	conn.Quit()

	// Verify all messages were deleted
	deleted := server.Deleted()
	for i := 1; i <= 3; i++ {
		if !deleted[i] {
			t.Errorf("message %d was not deleted", i)
		}
	}
}
