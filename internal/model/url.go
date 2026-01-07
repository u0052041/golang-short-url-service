package model

import (
	"time"
)

// URL represents a short URL mapping
type URL struct {
	ID          int64      `json:"id"`
	ShortCode   string     `json:"short_code"`
	URLHash     string     `json:"url_hash"` // SHA256 hash for deduplication
	OriginalURL string     `json:"original_url"`
	ClickCount  int64      `json:"click_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `json:"is_active"`
}

// URLAccessLog represents an access log entry
type URLAccessLog struct {
	ID         int64     `json:"id"`
	URLID      int64     `json:"url_id"`
	AccessedAt time.Time `json:"accessed_at"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	Referer    string    `json:"referer"`
}

// CreateURLRequest represents the request body for creating a short URL
type CreateURLRequest struct {
	URL       string `json:"url" binding:"required,url"`
	ExpiresIn string `json:"expires_in,omitempty"` // e.g., "24h", "7d"
}

// CreateURLResponse represents the response after creating a short URL
type CreateURLResponse struct {
	ShortCode   string `json:"short_code"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

// URLStatsResponse represents URL statistics
type URLStatsResponse struct {
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	ClickCount  int64     `json:"click_count"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   string    `json:"expires_at,omitempty"`
	IsActive    bool      `json:"is_active"`
}

// IsExpired checks if the URL has expired
func (u *URL) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*u.ExpiresAt)
}

// IsValid checks if the URL is valid for redirection
func (u *URL) IsValid() bool {
	return u.IsActive && !u.IsExpired()
}
