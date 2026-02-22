package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/SergeiKhy/url-shortener/internal/repository"
)

// Ошибки сервиса
var (
	ErrInvalidURL  = errors.New("невалидный URL")
	ErrInvalidCode = errors.New("невалидный кастомный код")
	ErrSpamDomain  = errors.New("домен в чёрном списке")
)

// Константы сервиса
const (
	defaultTTL = 24 * time.Hour
	maxTTL     = 30 * 24 * time.Hour
	codeLength = 8
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// Чёрный список доменов (можно вынести в конфиг или БД)
var blacklistedDomains = []string{
	"malware.com",
	"phishing.com",
	"spam.com",
}

// LinkService интерфейс сервиса ссылок
type LinkService interface {
	CreateLink(ctx context.Context, input *models.CreateLinkInput) (*models.Link, error)
	GetLink(ctx context.Context, code string) (*models.Link, error)
	DeleteLink(ctx context.Context, code string) error
}

// linkService реализация сервиса ссылок
type linkService struct {
	linkRepo  repository.LinkRepository
	cacheRepo repository.CacheRepository
}

// NewLinkService создаёт новый экземпляр сервиса
func NewLinkService(linkRepo repository.LinkRepository, cacheRepo repository.CacheRepository) LinkService {
	return &linkService{
		linkRepo:  linkRepo,
		cacheRepo: cacheRepo,
	}
}

// CreateLink создаёт новую короткую ссылку
func (s *linkService) CreateLink(ctx context.Context, input *models.CreateLinkInput) (*models.Link, error) {
	// Валидация URL
	if err := s.validateURL(input.OriginalURL); err != nil {
		return nil, err
	}

	// Проверка на спам-домены
	if err := s.checkSpamDomain(input.OriginalURL); err != nil {
		return nil, err
	}

	// Генерация короткого кода
	shortCode := input.CustomCode
	if shortCode == nil || *shortCode == "" {
		code, err := s.generateShortCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code: %w", err)
		}
		shortCode = &code
	} else {
		// Валидация кастомного кода
		if err := s.validateCustomCode(*shortCode); err != nil {
			return nil, ErrInvalidCode
		}
	}

	// Расчёт TTL
	var expiresAt *time.Time
	if input.ExpiresIn != nil && *input.ExpiresIn > 0 {
		ttl := time.Duration(*input.ExpiresIn) * time.Minute
		if ttl > maxTTL {
			ttl = maxTTL
		}
		t := time.Now().Add(ttl)
		expiresAt = &t
	}

	// Создание ссылки
	link := &models.Link{
		ShortCode:   *shortCode,
		OriginalURL: input.OriginalURL,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
	}

	if err := s.linkRepo.Create(ctx, link); err != nil {
		if errors.Is(err, repository.ErrCodeExists) {
			// Retry с новым кодом
			if input.CustomCode == nil || *input.CustomCode == "" {
				return s.CreateLink(ctx, input)
			}
		}
		return nil, err
	}

	// Кэширование
	ttl := defaultTTL
	if expiresAt != nil {
		ttl = time.Until(*expiresAt)
	}
	if err := s.cacheRepo.Set(ctx, link.ShortCode, link, ttl); err != nil {
		// Логгируем ошибку, но не прерываем создание
		// fmt.Printf("Failed to cache link: %v\n", err)
	}

	return link, nil
}

// GetLink получает ссылку по короткому коду (сначала из кэша, затем из БД)
func (s *linkService) GetLink(ctx context.Context, code string) (*models.Link, error) {
	// Проверка кэша
	link, err := s.cacheRepo.Get(ctx, code)
	if err == nil {
		return link, nil
	}

	// Запрос из БД
	link, err = s.linkRepo.GetByShortCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Кэширование результата
	ttl := defaultTTL
	if link.ExpiresAt != nil {
		ttl = time.Until(*link.ExpiresAt)
	}
	s.cacheRepo.Set(ctx, code, link, ttl)

	return link, nil
}

// DeleteLink удаляет ссылку по короткому коду
func (s *linkService) DeleteLink(ctx context.Context, code string) error {
	// Удаляем кэш
	s.cacheRepo.Delete(ctx, code)

	// Удаляем из БД
	return s.linkRepo.Delete(ctx, code)
}

// generateShortCode генерирует случайный короткий код длиной 8 символов
func (s *linkService) generateShortCode() (string, error) {
	result := make([]byte, codeLength)
	for i := 0; i < codeLength; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}

// validateURL проверяет формат URL с помощью регулярного выражения
func (s *linkService) validateURL(url string) error {
	// Простая валидация через regexp
	pattern := `^https?://[^\s]+$`
	matched, _ := regexp.MatchString(pattern, url)
	if !matched {
		return ErrInvalidURL
	}
	return nil
}

// validateCustomCode проверяет формат кастомного кода (4-12 символов, буквы и цифры)
func (s *linkService) validateCustomCode(code string) error {
	if len(code) < 4 || len(code) > 12 {
		return ErrInvalidCode
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, code)
	if !matched {
		return ErrInvalidCode
	}
	return nil
}

// checkSpamDomain проверяет наличие URL в чёрном списке доменов
func (s *linkService) checkSpamDomain(url string) error {
	for _, domain := range blacklistedDomains {
		if contains(url, domain) {
			return ErrSpamDomain
		}
	}
	return nil
}

// contains проверяет наличие подстроки в строке
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
