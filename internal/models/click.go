package models

import (
	"time"
)

type Click struct {
	ID        int64      `json:"id"`
	LinkID    int64      `json:"link_id"`
	ShortCode string     `json:"short_code"`
	IPAddress string     `json:"ip_address"`
	UserAgent string     `json:"user_agent"`
	Referer   string     `json:"referer"`
	Country   string     `json:"country"`
	ClickedAt time.Time  `json:"clicked_at"`
}

type ClickEvent struct {
	ShortCode string
	IPAddress string
	UserAgent string
	Referer   string
	Country   string
}

type ClickStats struct {
	ShortCode    string `json:"short_code"`
	TotalClicks  int64  `json:"total_clicks"`
	UniqueClicks int64  `json:"unique_clicks"`
}

type DailyClickStats struct {
	Date  string `json:"date"`
	Clicks int64  `json:"clicks"`
}
