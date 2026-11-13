// storage-init creates the private poster bucket during local setup.
package main

import (
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/poponyas/TicketRush/internal/core/storage/s3"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return err
	}
	cfg, err := s3.NewConfig()
	if err != nil {
		return err
	}
	store, err := s3.New(cfg)
	if err != nil {
		return err
	}
	return store.EnsureBucket(context.Background())
}
