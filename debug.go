package main

import (
	"context"
	"fmt"
	"hldsbot/hlds"
	"time"
)

// /*
func debug(ctx context.Context, pool *hlds.Pool) error {
	/*
		addonsDir, mapName, err := fetchAndExtractVaultMap(ctx, 6847)
		// 6910
		if err != nil {
			return fmt.Errorf("unable to fetch and extract vault item: %w", err)
		}
	*/

	cfg, err := hlds.NewServerConfig(
		1*time.Hour,
		"", /* addonsDir */
		2,
		[]string{"crossfire" /*mapName*/},
		map[string]string{
			"rcon_password": generatePassword(32),
			"sv_password":   generatePassword(8),
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create server config: %w", err)
	}

	if _, err := pool.AddServer(ctx, cfg); err != nil {
		return fmt.Errorf("unable to add server: %w", err)
	}

	<-ctx.Done()

	return nil
}
