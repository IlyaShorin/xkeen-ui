package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"xkeen-ui/internal/auth"
	"xkeen-ui/internal/commands"
	"xkeen-ui/internal/config"
	"xkeen-ui/internal/files"
	"xkeen-ui/internal/logview"
	"xkeen-ui/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "hash-password":
		if err := runHashPassword(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := flags.String("config", "/opt/etc/xkeen-ui/config.yaml", "path to config file")
	flags.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.BackupDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	logger := log.New(os.Stdout, "xkeen-ui ", log.LstdFlags|log.LUTC)
	fileService := files.NewService(cfg.XrayConfigDir, cfg.BackupDir)
	commandService := commands.NewService(cfg, 15*time.Second, logger)
	logService := logview.NewService(cfg.LogFiles, 200, 256*1024)
	sessionManager, err := auth.NewSessionManager(cfg.Username+"\n"+cfg.PasswordHash, 12*time.Hour)
	if err != nil {
		return err
	}

	server, err := web.NewServer(cfg, sessionManager, fileService, commandService, logService, logger)
	if err != nil {
		return err
	}

	networkAuthorizer, err := auth.NewNetworkAuthorizer(cfg.AllowCIDRs)
	if err != nil {
		return err
	}

	handler := networkAuthorizer.Middleware(server.Handler())

	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("listening on %s using %s", cfg.Listen, filepath.Clean(*configPath))
	return httpServer.ListenAndServe()
}

func runHashPassword(args []string) error {
	flags := flag.NewFlagSet("hash-password", flag.ExitOnError)
	password := flags.String("password", "", "password to hash")
	flags.Parse(args)

	if *password == "" {
		return fmt.Errorf("password is required")
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		return err
	}

	fmt.Println(hash)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  xkeen-ui serve -config /opt/etc/xkeen-ui/config.yaml")
	fmt.Fprintln(os.Stderr, "  xkeen-ui hash-password -password 'change-me'")
}
