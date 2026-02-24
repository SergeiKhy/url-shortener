package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/SergeiKhy/url-shortener/internal/service"
	"github.com/SergeiKhy/url-shortener/internal/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestService создаёт тестовое окружение с моковыми репозиториями
func setupTestService() (service.LinkService, *mocks.MockLinkRepository, *mocks.MockCacheRepository) {
	linkRepo := mocks.NewMockLinkRepository()
	cacheRepo := mocks.NewMockCacheRepository()
	logger, _ := zap.NewDevelopment()
	linkService := service.NewLinkService(linkRepo, cacheRepo, logger)
	return linkService, linkRepo, cacheRepo
}

// TestLinkService_CreateLink_Success проверяет успешное создание ссылки
func TestLinkService_CreateLink_Success(t *testing.T) {
	linkService, _, _ := setupTestService()

	input := &models.CreateLinkInput{
		OriginalURL: "https://example.com/test",
	}

	ctx := context.Background()
	link, err := linkService.CreateLink(ctx, input)

	require.NoError(t, err)
	assert.NotEmpty(t, link.ShortCode)
	assert.Equal(t, input.OriginalURL, link.OriginalURL)
	assert.NotNil(t, link.CreatedAt)
}

// TestLinkService_CreateLink_WithCustomCode проверяет создание ссылки с кастомным кодом
func TestLinkService_CreateLink_WithCustomCode(t *testing.T) {
	linkService, _, _ := setupTestService()

	customCode := "my-custom"
	input := &models.CreateLinkInput{
		OriginalURL: "https://example.com/test",
		CustomCode:  &customCode,
	}

	ctx := context.Background()
	link, err := linkService.CreateLink(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, customCode, link.ShortCode)
}

// TestLinkService_CreateLink_WithExpiration проверяет создание ссылки с временем жизни
func TestLinkService_CreateLink_WithExpiration(t *testing.T) {
	linkService, _, _ := setupTestService()

	expiresIn := 60 // 60 минут
	input := &models.CreateLinkInput{
		OriginalURL: "https://example.com/test",
		ExpiresIn:   &expiresIn,
	}

	ctx := context.Background()
	link, err := linkService.CreateLink(ctx, input)

	require.NoError(t, err)
	assert.NotNil(t, link.ExpiresAt)
	assert.True(t, link.ExpiresAt.After(time.Now()))
}

// TestLinkService_CreateLink_InvalidURL проверяет отклонение невалидного URL
func TestLinkService_CreateLink_InvalidURL(t *testing.T) {
	linkService, _, _ := setupTestService()

	input := &models.CreateLinkInput{
		OriginalURL: "not-a-valid-url",
	}

	ctx := context.Background()
	link, err := linkService.CreateLink(ctx, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, service.ErrInvalidURL)
	assert.Nil(t, link)
}

// TestLinkService_CreateLink_SpamDomain проверяет блокировку спам-доменов
func TestLinkService_CreateLink_SpamDomain(t *testing.T) {
	linkService, _, _ := setupTestService()

	input := &models.CreateLinkInput{
		OriginalURL: "https://malware.com/bad-link",
	}

	ctx := context.Background()
	link, err := linkService.CreateLink(ctx, input)

	assert.Error(t, err)
	assert.ErrorIs(t, err, service.ErrSpamDomain)
	assert.Nil(t, link)
}

// TestLinkService_CreateLink_InvalidCustomCode проверяет валидацию кастомного кода
func TestLinkService_CreateLink_InvalidCustomCode(t *testing.T) {
	linkService, _, _ := setupTestService()

	// Невалидные коды: слишком короткий, слишком длинный, с недопустимыми символами
	invalidCodes := []string{"ab", "toolongcustomcode123", "invalid@code"}

	for _, code := range invalidCodes {
		customCode := code
		input := &models.CreateLinkInput{
			OriginalURL: "https://example.com/test",
			CustomCode:  &customCode,
		}

		ctx := context.Background()
		link, err := linkService.CreateLink(ctx, input)

		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidCode)
		assert.Nil(t, link)
	}
}

// TestLinkService_GetLink_FromCache проверяет получение ссылки из кэша
func TestLinkService_GetLink_FromCache(t *testing.T) {
	linkService, _, cacheRepo := setupTestService()

	// Сначала создаём ссылку
	input := &models.CreateLinkInput{
		OriginalURL: "https://example.com/test",
	}
	ctx := context.Background()
	createdLink, err := linkService.CreateLink(ctx, input)
	require.NoError(t, err)

	// Проверяем, что ссылка попала в кэш
	cachedLink, err := cacheRepo.Get(ctx, createdLink.ShortCode)
	require.NoError(t, err)
	assert.Equal(t, createdLink.ShortCode, cachedLink.ShortCode)

	// Получаем ссылку (должна вернуться из кэша)
	retrievedLink, err := linkService.GetLink(ctx, createdLink.ShortCode)
	require.NoError(t, err)
	assert.Equal(t, createdLink.ShortCode, retrievedLink.ShortCode)
}

// TestLinkService_GetLink_NotFound проверяет обработку несуществующей ссылки
func TestLinkService_GetLink_NotFound(t *testing.T) {
	linkService, _, _ := setupTestService()

	ctx := context.Background()
	link, err := linkService.GetLink(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, link)
}

// TestLinkService_DeleteLink_Success проверяет успешное удаление ссылки
func TestLinkService_DeleteLink_Success(t *testing.T) {
	linkService, linkRepo, cacheRepo := setupTestService()

	// Создаём ссылку
	input := &models.CreateLinkInput{
		OriginalURL: "https://example.com/test",
	}
	ctx := context.Background()
	createdLink, err := linkService.CreateLink(ctx, input)
	require.NoError(t, err)

	// Удаляем ссылку
	err = linkService.DeleteLink(ctx, createdLink.ShortCode)
	require.NoError(t, err)

	// Проверяем, что ссылка удалена из кэша
	_, err = cacheRepo.Get(ctx, createdLink.ShortCode)
	assert.Error(t, err)

	// Проверяем, что ссылка удалена из БД
	_, err = linkRepo.GetByShortCode(ctx, createdLink.ShortCode)
	assert.Error(t, err)
}

// TestLinkService_DeleteLink_NotFound проверяет удаление несуществующей ссылки
func TestLinkService_DeleteLink_NotFound(t *testing.T) {
	linkService, _, _ := setupTestService()

	ctx := context.Background()
	err := linkService.DeleteLink(ctx, "nonexistent")

	assert.Error(t, err)
}

// TestLinkService_ValidateURL проверяет валидацию URL
func TestLinkService_ValidateURL(t *testing.T) {
	// Тестовые данные для валидных URL
	validURLs := []string{
		"https://example.com",
		"http://example.com/path",
		"https://sub.example.com/path?query=value",
	}

	// Тестовые данные для невалидных URL
	invalidURLs := []string{
		"not-a-url",
		"ftp://example.com",
		"",
		"example.com",
	}

	// Проверяем, что валидные URL принимаются
	for _, url := range validURLs {
		linkService, _, _ := setupTestService()
		input := &models.CreateLinkInput{
			OriginalURL: url,
		}
		ctx := context.Background()
		link, err := linkService.CreateLink(ctx, input)
		assert.NoError(t, err, "URL должен быть валидным: %s", url)
		assert.NotNil(t, link)
	}

	// Проверяем, что невалидные URL отклоняются
	for _, url := range invalidURLs {
		linkService, _, _ := setupTestService()
		input := &models.CreateLinkInput{
			OriginalURL: url,
		}
		ctx := context.Background()
		link, err := linkService.CreateLink(ctx, input)
		assert.Error(t, err, "URL должен быть невалидным: %s", url)
		assert.Nil(t, link)
	}
}

// TestLinkService_CheckSpamDomain проверяет блокировку спам-доменов
func TestLinkService_CheckSpamDomain(t *testing.T) {
	// Список спам-доменов для проверки
	spamURLs := []string{
		"https://malware.com/bad",
		"https://phishing.com/steal",
		"https://spam.com/junk",
	}

	// Список чистых доменов для проверки
	cleanURLs := []string{
		"https://example.com",
		"https://google.com",
		"https://github.com/user/repo",
	}

	// Проверяем, что спам-домены блокируются
	for _, url := range spamURLs {
		linkService, _, _ := setupTestService()
		input := &models.CreateLinkInput{
			OriginalURL: url,
		}
		ctx := context.Background()
		link, err := linkService.CreateLink(ctx, input)
		assert.Error(t, err, "URL должен быть заблокирован как спам: %s", url)
		assert.Nil(t, link)
	}

	// Проверяем, что чистые URL принимаются
	for _, url := range cleanURLs {
		linkService, _, _ := setupTestService()
		input := &models.CreateLinkInput{
			OriginalURL: url,
		}
		ctx := context.Background()
		link, err := linkService.CreateLink(ctx, input)
		assert.NoError(t, err, "URL должен быть чистым: %s", url)
		assert.NotNil(t, link)
	}
}

// TestLinkService_GenerateShortCode проверяет генерацию уникальных коротких кодов
func TestLinkService_GenerateShortCode(t *testing.T) {
	linkService, _, _ := setupTestService()

	// Генерируем множество кодов и проверяем их уникальность и длину
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		input := &models.CreateLinkInput{
			OriginalURL: "https://example.com/test" + string(rune('a'+i)),
		}
		ctx := context.Background()
		link, err := linkService.CreateLink(ctx, input)
		require.NoError(t, err)
		assert.Len(t, link.ShortCode, 8, "Длина короткого кода должна быть 8 символов")
		assert.NotContains(t, codes, link.ShortCode, "Короткие коды должны быть уникальными")
		codes[link.ShortCode] = true
	}
}

// TestLinkService_ConcurrentAccess проверяет потокобезопасность при одновременном доступе
func TestLinkService_ConcurrentAccess(t *testing.T) {
	linkService, _, _ := setupTestService()

	ctx := context.Background()
	done := make(chan bool, 10)

	// Создаём ссылки параллельно в 10 горутинах
	for i := 0; i < 10; i++ {
		go func(id int) {
			input := &models.CreateLinkInput{
				OriginalURL: "https://example.com/test" + fmt.Sprintf("%d", id),
			}
			link, err := linkService.CreateLink(ctx, input)
			assert.NoError(t, err)
			assert.NotNil(t, link)
			done <- true
		}(i)
	}

	// Ждём завершения всех горутин
	for i := 0; i < 10; i++ {
		<-done
	}
}
