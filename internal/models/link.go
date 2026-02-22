package models

import (
	"time"
)

type Link struct {
	ID          int64      `json:"id"`
	ShortCode   string     `json:"short_code"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateLinkInput struct {
	OriginalURL string  `json:"original_url" binding:"required,url"`
	ExpiresIn   *int    `json:"expires_in,omitempty"`
	CustomCode  *string `json:"custom_code,omitempty"`
}

type LinkStats struct {
	ShortCode    string `json:"short_code"`
	Clicks       int64  `json:"clicks"`
	UniqueClicks int64  `json:"unique_clicks"`
}
