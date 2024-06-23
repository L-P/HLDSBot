package twhl

import (
	"context"
	"fmt"
	"hldsbot/hlds"
	"os"

	"github.com/rs/zerolog/log"
)

// Returns the path to the extracted data ready to be mounted as valve_addon,
// and name of the map found in the archive.
func FetchAndExtractVaultMap(ctx context.Context, itemID int) (string, string, error) {
	client := NewClient()
	archivePath, err := client.DownloadVaultItem(ctx, itemID)
	if err != nil {
		return "", "", fmt.Errorf("unable to download vault item #%d: %w", itemID, err)
	}

	defer func() {
		if err := os.Remove(archivePath); err != nil {
			log.Error().Err(err).Msg("unable to remove downloaded archive")
		}
	}()

	archive, err := hlds.ReadMapArchiveFromFile(archivePath)
	if err != nil {
		return "", "", fmt.Errorf("unable to read map archive: %w", err)
	}

	if _, err := os.Stat(hlds.UserContentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(hlds.UserContentDir, 0o755); err != nil {
			return "", "", fmt.Errorf("unable to create dir '%s': %w", hlds.UserContentDir, err)
		}
	}
	dstDir, err := os.MkdirTemp(hlds.UserContentDir, "")
	if err != nil {
		return "", "", fmt.Errorf("unable to create temp dir: %w", err)
	}

	if _, err := archive.Extract(dstDir); err != nil {
		return "", "", fmt.Errorf("unable to extract archive: %w", err)
	}

	var mapName = archive.MapName()
	if err := archive.Close(); err != nil {
		return "", "", fmt.Errorf("unable to close map archive: %w", err)
	}

	return dstDir, mapName, nil
}
