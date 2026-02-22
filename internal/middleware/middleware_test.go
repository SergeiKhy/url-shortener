package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestRateLimiter_Middleware проверяет работу rate limiter middleware
func TestRateLimiter_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Создаём rate limiter с лимитом 5 запросов в секунду и burst 5
	rl := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		RequestsPerSecond: 5,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
	})

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Первые 5 запросов должны пройти (в пределах burst лимита)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Следующие запросы должны быть ограничены
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// TestRateLimiter_MiddlewareWithKey проверяет rate limiting с кастомным ключом
func TestRateLimiter_MiddlewareWithKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Создаём rate limiter с лимитом 2 запроса в секунду
	rl := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		RequestsPerSecond: 2,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
	})

	// Функция получения ключа из заголовка
	keyGetter := func(c *gin.Context) string {
		return c.GetHeader("X-User-ID")
	}

	router := gin.New()
	router.Use(rl.MiddlewareWithKey(keyGetter))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Пользователь 1 - первые 2 запроса успешны
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", "user1")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Пользователь 1 - третий запрос должен быть ограничен
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Пользователь 2 - запрос успешен (другой ключ)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user2")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAPIKey_Middleware проверяет аутентификацию по API ключу
func TestAPIKey_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Настраиваем валидные API ключи
	validKeys := map[string]string{
		"test-key-1": "Test Key 1",
		"test-key-2": "Test Key 2",
	}

	ak := middleware.NewAPIKey(middleware.APIKeyConfig{
		ValidKeys:  validKeys,
		HeaderName: "X-API-Key",
		Optional:   false,
	})

	router := gin.New()
	router.Use(ak.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Запрос без API ключа должен быть отклонён
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Запрос с невалидным API ключом должен быть отклонён
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Запрос с валидным API ключом должен пройти
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAPIKey_Middleware_Optional проверяет опциональную аутентификацию
func TestAPIKey_Middleware_Optional(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validKeys := map[string]string{
		"test-key-1": "Test Key 1",
	}

	ak := middleware.NewAPIKey(middleware.APIKeyConfig{
		ValidKeys:  validKeys,
		HeaderName: "X-API-Key",
		Optional:   true,
	})

	router := gin.New()
	router.Use(ak.Middleware())
	router.GET("/test", func(c *gin.Context) {
		validated, _ := c.Get("api_key_validated")
		c.JSON(http.StatusOK, gin.H{"validated": validated})
	})

	// Запрос без API ключа должен пройти, но не быть валидированным
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"validated":false`)

	// Запрос с валидным API ключом должен пройти и быть валидированным
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "test-key-1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"validated":true`)
}

// TestAPIKey_Middleware_QueryParam проверяет передачу API ключа через query параметр
func TestAPIKey_Middleware_QueryParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validKeys := map[string]string{
		"test-key-1": "Test Key 1",
	}

	ak := middleware.NewAPIKey(middleware.APIKeyConfig{
		ValidKeys:  validKeys,
		HeaderName: "X-API-Key",
		Optional:   false,
	})

	router := gin.New()
	router.Use(ak.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Запрос с API ключом в query параметре должен пройти
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?api_key=test-key-1", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAPIKey_Middleware_BearerToken проверяет передачу API ключа через Bearer токен
func TestAPIKey_Middleware_BearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validKeys := map[string]string{
		"test-key-1": "Test Key 1",
	}

	ak := middleware.NewAPIKey(middleware.APIKeyConfig{
		ValidKeys:  validKeys,
		HeaderName: "X-API-Key",
		Optional:   false,
	})

	router := gin.New()
	router.Use(ak.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Запрос с Bearer токеном должен пройти
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-key-1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
