package main

import (
	"context"
	"fmt"
	"hldsbot/hlds"
	"time"
)

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
