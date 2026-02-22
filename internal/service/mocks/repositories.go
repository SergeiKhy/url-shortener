package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/SergeiKhy/url-shortener/internal/repository"
)

// MockLinkRepository implements repository.LinkRepository for testing
type MockLinkRepository struct {
	mu      sync.RWMutex
	links   map[string]*models.Link
	nextID  int64
}

func NewMockLinkRepository() *MockLinkRepository {
	return &MockLinkRepository{
		links:  make(map[string]*models.Link),
		nextID: 1,
	}
}

func (m *MockLinkRepository) Create(ctx context.Context, link *models.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.links[link.ShortCode]; exists {
		return repository.ErrCodeExists
	}

	link.ID = m.nextID
	m.nextID++
	m.links[link.ShortCode] = link
	return nil
}

func (m *MockLinkRepository) GetByShortCode(ctx context.Context, code string) (*models.Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[code]
	if !exists {
		return nil, repository.ErrLinkNotFound
	}
	return link, nil
}

func (m *MockLinkRepository) Delete(ctx context.Context, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.links[code]; !exists {
		return repository.ErrLinkNotFound
	}
	delete(m.links, code)
	return nil
}

func (m *MockLinkRepository) GetLinkIDByShortCode(ctx context.Context, code string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.links[code]
	if !exists {
		return 0, repository.ErrLinkNotFound
	}
	return link.ID, nil
}

func (m *MockLinkRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links = make(map[string]*models.Link)
	m.nextID = 1
}

// MockCacheRepository implements repository.CacheRepository for testing
type MockCacheRepository struct {
	mu    sync.RWMutex
	cache map[string]*models.Link
}

func NewMockCacheRepository() *MockCacheRepository {
	return &MockCacheRepository{
		cache: make(map[string]*models.Link),
	}
}

func (m *MockCacheRepository) Get(ctx context.Context, key string) (*models.Link, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, exists := m.cache[key]
	if !exists {
		return nil, repository.ErrLinkNotFound
	}
	return link, nil
}

func (m *MockCacheRepository) Set(ctx context.Context, key string, link *models.Link, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[key] = link
	return nil
}

func (m *MockCacheRepository) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, key)
	return nil
}

func (m *MockCacheRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]*models.Link)
}

// MockClickRepository implements repository.ClickRepository for testing
type MockClickRepository struct {
	mu     sync.RWMutex
	clicks map[int64][]*models.Click // link_id -> clicks
}

func NewMockClickRepository() *MockClickRepository {
	return &MockClickRepository{
		clicks: make(map[int64][]*models.Click),
	}
}

func (m *MockClickRepository) RecordClick(ctx context.Context, click *models.Click) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clicks[click.LinkID] = append(m.clicks[click.LinkID], click)
	return nil
}

func (m *MockClickRepository) GetStats(ctx context.Context, shortCode string) (*models.ClickStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find link by short code and count clicks
	var totalClicks int64
	uniqueIPs := make(map[string]bool)

	for _, clicks := range m.clicks {
		for _, click := range clicks {
			if click.ShortCode == shortCode {
				totalClicks++
				uniqueIPs[click.IPAddress] = true
			}
		}
	}

	return &models.ClickStats{
		ShortCode:    shortCode,
		TotalClicks:  totalClicks,
		UniqueClicks: int64(len(uniqueIPs)),
	}, nil
}

func (m *MockClickRepository) GetDailyStats(ctx context.Context, shortCode string, days int) ([]models.DailyClickStats, error) {
	return []models.DailyClickStats{}, nil
}

func (m *MockClickRepository) GetLinkIDByShortCode(ctx context.Context, shortCode string) (int64, error) {
	return 0, nil
}

func (m *MockClickRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clicks = make(map[int64][]*models.Click)
}
