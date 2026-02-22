package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

type ClickRepository interface {
	RecordClick(ctx context.Context, click *models.Click) error
	GetStats(ctx context.Context, shortCode string) (*models.ClickStats, error)
	GetDailyStats(ctx context.Context, shortCode string, days int) ([]models.DailyClickStats, error)
	GetLinkIDByShortCode(ctx context.Context, shortCode string) (int64, error)
}

type clickRepository struct {
	db *PostgresDB
}

func NewClickRepository(db *PostgresDB) ClickRepository {
	return &clickRepository{db: db}
}

func (r *clickRepository) RecordClick(ctx context.Context, click *models.Click) error {
	query := `
		INSERT INTO clicks (link_id, ip_address, user_agent, referer, country, clicked_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		click.LinkID,
		click.IPAddress,
		click.UserAgent,
		click.Referer,
		click.Country,
		click.ClickedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record click: %w", err)
	}

	return nil
}

func (r *clickRepository) GetStats(ctx context.Context, shortCode string) (*models.ClickStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_clicks,
			COUNT(DISTINCT ip_address) as unique_clicks
		FROM clicks c
		JOIN links l ON c.link_id = l.id
		WHERE l.short_code = $1
	`

	stats := &models.ClickStats{
		ShortCode: shortCode,
	}

	err := r.db.Pool.QueryRow(ctx, query, shortCode).Scan(
		&stats.TotalClicks,
		&stats.UniqueClicks,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get click stats: %w", err)
	}

	return stats, nil
}

func (r *clickRepository) GetDailyStats(ctx context.Context, shortCode string, days int) ([]models.DailyClickStats, error) {
	query := `
		SELECT 
			DATE(c.clicked_at) as date,
			COUNT(*) as clicks
		FROM clicks c
		JOIN links l ON c.link_id = l.id
		WHERE l.short_code = $1 
			AND c.clicked_at >= NOW() - INTERVAL '1 day' * $2
		GROUP BY DATE(c.clicked_at)
		ORDER BY date DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, shortCode, days)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []models.DailyClickStats{}, nil
		}
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer rows.Close()

	var stats []models.DailyClickStats
	for rows.Next() {
		var dailyStat models.DailyClickStats
		if err := rows.Scan(&dailyStat.Date, &dailyStat.Clicks); err != nil {
			return nil, fmt.Errorf("failed to scan daily stat: %w", err)
		}
		stats = append(stats, dailyStat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily stats: %w", err)
	}

	return stats, nil
}

func (r *clickRepository) GetLinkIDByShortCode(ctx context.Context, shortCode string) (int64, error) {
	query := `SELECT id FROM links WHERE short_code = $1`

	var linkID int64
	err := r.db.Pool.QueryRow(ctx, query, shortCode).Scan(&linkID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, ErrLinkNotFound
		}
		return 0, fmt.Errorf("failed to get link ID: %w", err)
	}

	return linkID, nil
}

// RecordClickWithRetry records a click with retry logic
func (r *clickRepository) RecordClickWithRetry(ctx context.Context, click *models.Click, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := r.RecordClick(ctx, click); err == nil {
			return nil
		} else {
			lastErr = err
		}
		// Exponential backoff
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}
