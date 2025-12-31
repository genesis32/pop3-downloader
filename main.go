package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	MboxPath string
	DryRun   bool
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.Host, "host", "", "POP3S server hostname (required)")
	flag.IntVar(&config.Port, "port", 995, "POP3S server port")
	flag.StringVar(&config.Username, "username", "", "Username for authentication (required)")
	flag.StringVar(&config.Password, "password", "", "Password for authentication (required)")
	flag.StringVar(&config.MboxPath, "mbox", "./messages.mbox", "Path to output mbox file")
	flag.BoolVar(&config.DryRun, "dryrun", false, "Download messages without deleting from server")

	flag.Parse()

	// Validate required fields
	if config.Host == "" {
		fmt.Fprintf(os.Stderr, "Error: -host is required\n")
		flag.Usage()
		os.Exit(1)
	}
	if config.Username == "" {
		fmt.Fprintf(os.Stderr, "Error: -username is required\n")
		flag.Usage()
		os.Exit(1)
	}
	if config.Password == "" {
		fmt.Fprintf(os.Stderr, "Error: -password is required\n")
		flag.Usage()
		os.Exit(1)
	}

	return config
}

func run(config Config) error {
	// 1. Connect to POP3S server
	fmt.Printf("Connecting to %s:%d...\n", config.Host, config.Port)
	conn, err := connectPOP3S(config)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Quit()
	fmt.Println("Connected and authenticated successfully")

	// 2. Fetch all messages
	fmt.Println("Fetching messages...")
	messages, err := fetchAllMessages(conn)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	if len(messages) == 0 {
		fmt.Println("No messages to download")
		return nil
	}

	fmt.Printf("Retrieved %d message(s)\n", len(messages))

	// 3. Write to mbox (CRITICAL: before deletion)
	fmt.Printf("Writing messages to %s...\n", config.MboxPath)
	err = writeMbox(messages, config.MboxPath)
	if err != nil {
		return fmt.Errorf("mbox write failed: %w", err)
	}
	fmt.Println("Messages written successfully")

	// 4. Delete from server (only after successful write, skip if dry-run)
	if config.DryRun {
		fmt.Println("Dry-run mode: Skipping deletion from server")
		fmt.Printf("\nSuccessfully downloaded %d message(s) to %s (dry-run, messages not deleted)\n", len(messages), config.MboxPath)
	} else {
		fmt.Println("Deleting messages from server...")
		err = deleteMessages(conn, messages)
		if err != nil {
			return fmt.Errorf("deletion failed: %w", err)
		}
		fmt.Println("Messages deleted from server")
		fmt.Printf("\nSuccessfully downloaded %d message(s) to %s\n", len(messages), config.MboxPath)
	}

	return nil
}

func main() {
	config := parseFlags()

	if err := run(config); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
