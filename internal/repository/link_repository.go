package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

var (
	ErrLinkNotFound = errors.New("link not found")
	ErrCodeExists   = errors.New("short code already exists")
)

type LinkRepository interface {
	Create(ctx context.Context, link *models.Link) error
	GetByShortCode(ctx context.Context, code string) (*models.Link, error)
	Delete(ctx context.Context, code string) error
	GetLinkIDByShortCode(ctx context.Context, code string) (int64, error)
}

type linkRepository struct {
	db *PostgresDB
}

func NewLinkRepository(db *PostgresDB) LinkRepository {
	return &linkRepository{db: db}
}

func (r *linkRepository) Create(ctx context.Context, link *models.Link) error {
	query := `
		INSERT INTO links (short_code, original_url, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	err := r.db.Pool.QueryRow(
		ctx,
		query,
		link.ShortCode,
		link.OriginalURL,
		link.ExpiresAt,
		link.CreatedAt,
	).Scan(&link.ID, &link.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return ErrCodeExists
		}
		return fmt.Errorf("failed to create link: %w", err)
	}

	return nil
}

func (r *linkRepository) GetByShortCode(ctx context.Context, code string) (*models.Link, error) {
	query := `
		SELECT id, short_code, original_url, expires_at, created_at
		FROM links
		WHERE short_code = $1 AND (expires_at IS NULL OR expires_at > NOW())
	`

	link := &models.Link{}
	err := r.db.Pool.QueryRow(ctx, query, code).Scan(
		&link.ID,
		&link.ShortCode,
		&link.OriginalURL,
		&link.ExpiresAt,
		&link.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLinkNotFound
		}
		return nil, fmt.Errorf("failed to get link: %w", err)
	}

	return link, nil
}

func (r *linkRepository) Delete(ctx context.Context, code string) error {
	query := `DELETE FROM links WHERE short_code = $1`

	result, err := r.db.Pool.Exec(ctx, query, code)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrLinkNotFound
	}

	return nil
}

func (r *linkRepository) GetLinkIDByShortCode(ctx context.Context, code string) (int64, error) {
	query := `SELECT id FROM links WHERE short_code = $1`

	var linkID int64
	err := r.db.Pool.QueryRow(ctx, query, code).Scan(&linkID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrLinkNotFound
		}
		return 0, fmt.Errorf("failed to get link ID: %w", err)
	}

	return linkID, nil
}

// Проверка на уникальность
func isUniqueViolation(err error) bool {
	// Для pgx v5 проверяем код ошибки
	return err != nil && (err.Error() == "ErrCodeExists" ||
		(err.Error() != "" && contains(err.Error(), "unique")))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
