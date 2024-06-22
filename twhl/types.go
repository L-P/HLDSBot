package twhl

import "time"

type VaultItem struct {
	ID                 int       `json:"id"`
	UserID             int       `json:"user_id"`
	EngineID           int       `json:"engine_id"`
	GameID             int       `json:"game_id"`
	CategoryID         int       `json:"category_id"`
	TypeID             int       `json:"type_id"`
	LicenseID          int       `json:"license_id"`
	Name               string    `json:"name"`
	ContentText        string    `json:"content_text"`
	ContentHTML        string    `json:"content_html"`
	IsHostedExternally int       `json:"is_hosted_externally"` // should have been a bool
	FileLocation       string    `json:"file_location"`
	FileSize           int       `json:"file_size"`
	FlagRatings        int       `json:"flag_ratings"`
	StatViews          int       `json:"stat_views"`
	StatDownloads      int       `json:"stat_downloads"`
	StatRatings        int       `json:"stat_ratings"`
	StatComments       int       `json:"stat_comments"`
	StatAverageRating  string    `json:"stat_average_rating"` // float encoded as string
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
