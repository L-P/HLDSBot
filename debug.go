package main

import (
	"context"
	"fmt"
	"hldsbot/hlds"
	"os"
)

// /*
func debug(ctx context.Context, _ *hlds.Pool) error {
	path := "/tmp/725199372.download"
	archive, err := hlds.ReadMapArchiveFromFile(path)
	if err != nil {
		return fmt.Errorf("unable to read map archive: %w", err)
	}

	if _, err := os.Stat(hlds.UserContentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(hlds.UserContentDir, 0o755); err != nil {
			return fmt.Errorf("unable to create dir '%s': %w", hlds.UserContentDir, err)
		}
	}
	dstDir, err := os.MkdirTemp(hlds.UserContentDir, "")
	if err != nil {
		return fmt.Errorf("unable to create temp dir: %w", err)
	}

	if _, err := archive.Extract(dstDir); err != nil {
		return fmt.Errorf("unable to extract archive: %w", err)
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("unable to close map archive: %w", err)
	}

	return nil
}

// */

/*
func debug(ctx context.Context, _ *hlds.Pool) error {
	client := twhl.NewClient()
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
