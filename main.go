package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	MboxPath string
	DryRun   bool
	Read     bool
}

type ConfigFile struct {
	Password string `toml:"password"`
}

func loadConfigFile(path string) (ConfigFile, error) {
	var cfg ConfigFile
	_, err := toml.DecodeFile(path, &cfg)
	return cfg, err
}

func parseFlags() Config {
	var config Config
	var configPath string

	// Construct default config path: $HOME/.config/pop3-downloader-config.toml
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not determine home directory: %v\n", err)
		homeDir = "."
	}
	defaultConfigPath := filepath.Join(homeDir, ".config", "pop3-downloader-config.toml")

	flag.StringVar(&config.Host, "host", "", "POP3S server hostname (required)")
	flag.IntVar(&config.Port, "port", 995, "POP3S server port")
	flag.StringVar(&config.Username, "username", "", "Username for authentication (required)")
	flag.StringVar(&configPath, "config", defaultConfigPath, "Path to config file containing password")
	flag.StringVar(&config.MboxPath, "mbox", "./messages.mbox", "Path to output mbox file")
	flag.BoolVar(&config.DryRun, "dryrun", false, "Download messages without deleting from server")
	flag.BoolVar(&config.Read, "read", false, "Open mbox in mutt after download (requires mutt in PATH)")

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

	// Load password from config file
	configFile, err := loadConfigFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config file '%s': %v\n", configPath, err)
		os.Exit(1)
	}
	config.Password = configFile.Password

	if config.Password == "" {
		fmt.Fprintf(os.Stderr, "Error: password not found in config file\n")
		os.Exit(1)
	}

	return config
}

func openMutt(mboxPath string) error {
	muttPath, err := exec.LookPath("mutt")
	if err != nil {
		return fmt.Errorf("mutt not found in PATH: %w", err)
	}

	cmd := exec.Command(muttPath, "-R", "-f", mboxPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
		if config.Read {
			fmt.Println("Opening mbox in mutt...")
			if err := openMutt(config.MboxPath); err != nil {
				return fmt.Errorf("failed to open mutt: %w", err)
			}
		}
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

	// 5. Open mutt if requested
	if config.Read {
		fmt.Println("Opening mbox in mutt...")
		if err := openMutt(config.MboxPath); err != nil {
			return fmt.Errorf("failed to open mutt: %w", err)
		}
	}

	return nil
}

func main() {
	config := parseFlags()

	if err := run(config); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
