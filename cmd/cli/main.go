package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"osto-cli-auth/internal/cli"
	"osto-cli-auth/internal/config"
	"osto-cli-auth/internal/session"
	"osto-cli-auth/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	db, err := store.OpenDB(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database initialization error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Handle graceful shutdown on interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		_ = db.Close()
		os.Exit(0)
	}()

	userStore := store.NewSQLiteUserStore(db)
	sessionStore := session.NewStore()

	repl := cli.NewREPL(cfg, userStore, sessionStore)
	if err := repl.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
}
