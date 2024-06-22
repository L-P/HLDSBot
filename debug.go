package main

import (
	"context"
	"fmt"
	"hldsbot/hlds"
	"hldsbot/twhl"
	"os"

	"github.com/rs/zerolog/log"
)

// /*
func debug(ctx context.Context, _ *hlds.Pool) error {
	client, err := twhl.NewClient(os.Getenv("TWHL_API_KEY"))
	if err != nil {
		return fmt.Errorf("unable to create TWHL client: %w", err)
	}

	path, err := client.DownloadVaultItem(ctx, 6910)
	if err != nil {
		return fmt.Errorf("unable to download TWHL vault item: %w", err)
	}

	log.Debug().Str("path", path).Msg("downloaded item")

	return nil
}

// */

/*
func debug(ctx context.Context, pool *hlds.Pool) error {
	cfg, err := hlds.NewServerConfig(
		1*time.Hour,
		"",
		2,
		[]string{"crossfire"},
		map[string]string{
			"rcon_password": "f9990989b25847399ae5327627ca82c0",
			"password":      "13eac3f58cb14d249a6033583772c6da",
		},
	)

	if err != nil {
		return fmt.Errorf("unable to create server config: %w", err)
	}

	if _, err := pool.AddServer(ctx, cfg); err != nil {
		return fmt.Errorf("unable to add server: %w", err)
	}

	return nil
}

// */
