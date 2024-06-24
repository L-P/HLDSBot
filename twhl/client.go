package twhl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"
)

var ErrWrongCategory = errors.New("only downloading HLDM maps is supported")

const (
	apiBaseURL = "https://twhl.info/api"

	// The API doesn't give an URL for internal downloads.
	// Instead of building strings from the uploads dir, rely on the download
	// proxy URL.
	downloadURLTemplate = "https://twhl.info/vault/download/%d"
)

// Hardcoding IDs, sue me.
const (
	EngineIDGoldSrc = 1
	GameIDHLDM      = 7
	ItemTypeIDMap   = 1
)

type Client struct {
	http http.Client
}

func NewClient() *Client {
	return &Client{}
}

func apiURL(path string, kvs map[string]string) (string, error) {
	ret, err := url.Parse(apiBaseURL)
	if err != nil {
		return "", fmt.Errorf("unable to parse base URL: %w", err)
	}

	ret.Path += path
	query := ret.Query()
	for k, v := range kvs {
		query.Set(k, v)
	}
	ret.RawQuery = query.Encode()

	return ret.String(), nil
}

// Downloads a Vault item to a temporary file and returns its path.
// The caller is responsible for removing the created file.
func (client *Client) DownloadVaultItem(ctx context.Context, id int) (string, error) {
	log.Info().Int("id", id).Msg("Vault item download requested, querying API.")

	item, err := client.GetVaultItem(ctx, id)
	if err != nil {
		return "", fmt.Errorf("unable to get vault item: %w", err)
	}

	item.ContentText, item.ContentHTML = "", "" // cleaner logs
	log.Debug().Interface("item", item).Msg("")

	if item.EngineID != EngineIDGoldSrc ||
		item.GameID != GameIDHLDM ||
		item.TypeID != ItemTypeIDMap {
		return "", ErrWrongCategory
	}

	downloadURL := fmt.Sprintf(downloadURLTemplate, id)
	log.Info().Int("id", id).Str("url", downloadURL).Msg("Downloading archive.")
	path, err := client.DownloadToFile(ctx, downloadURL)
	if err != nil {
		return "", fmt.Errorf("unable to download file: %w", err)
	}

	return path, nil
}

func (client *Client) DownloadToFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("unable to create request: %w", err)
	}

	res, err := client.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to query download URL: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download endpoint returned HTTP code %d", res.StatusCode)
	}

	f, err := os.CreateTemp("", "*.download")
	if err != nil {
		return "", fmt.Errorf("unable to create temp file: %w", err)
	}

	if _, err := io.Copy(f, res.Body); err != nil {
		return "", fmt.Errorf("unable to copy body to temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("unable to finish writing to temp file: %w", err)
	}

	return f.Name(), nil
}

func (client *Client) GetVaultItem(ctx context.Context, id int) (VaultItem, error) {
	var zero VaultItem
	url, err := apiURL("/vault-items", map[string]string{"id": strconv.Itoa(id)})
	if err != nil {
		return zero, fmt.Errorf("unable to create request URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, fmt.Errorf("unable to create request: %w", err)
	}

	res, err := client.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("unable to query API: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("API returned HTTP code %d", res.StatusCode)
	}

	var (
		dec   = json.NewDecoder(res.Body)
		items []VaultItem
	)
	if err := dec.Decode(&items); err != nil {
		return zero, fmt.Errorf("unable to parse API response: %w", err)
	}

	if len(items) != 1 {
		return zero, fmt.Errorf("API returned wrong number of items: %d", len(items))
	}

	return items[0], nil
}
