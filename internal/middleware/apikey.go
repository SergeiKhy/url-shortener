package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyConfig конфигурация для API key аутентификации
type APIKeyConfig struct {
	// ValidKeys карта валидных API ключей к их описаниям
	ValidKeys map[string]string
	// HeaderName имя заголовка для API ключа (по умолчанию: X-API-Key)
	HeaderName string
	// Optional если true, запросы без API ключа будут обработаны (но без повышенных привилегий)
	Optional bool
}

// DefaultAPIKeyConfig конфигурация по умолчанию
var DefaultAPIKeyConfig = APIKeyConfig{
	HeaderName: "X-API-Key",
	Optional:   false,
}

// APIKey middleware для аутентификации по API ключу
type APIKey struct {
	config APIKeyConfig
}

// NewAPIKey создаёт новый API key middleware
func NewAPIKey(config APIKeyConfig) *APIKey {
	if config.HeaderName == "" {
		config.HeaderName = DefaultAPIKeyConfig.HeaderName
	}
	return &APIKey{config: config}
}

// Middleware возвращает Gin middleware handler для API key аутентификации
func (ak *APIKey) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(ak.config.HeaderName)

		// Также проверяем query параметр как запасной вариант
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		// Также проверяем заголовок Authorization с Bearer схемой
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			if ak.config.Optional {
				c.Set("api_key_validated", false)
				c.Next()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "missing_api_key",
				"message": "Требуется API ключ. Передайте его через заголовок X-API-Key, query параметр api_key или Authorization: Bearer",
			})
			c.Abort()
			return
		}

		// Валидация API ключа с использованием constant-time comparison
		valid := false
		var keyName string
		for validKey, name := range ak.config.ValidKeys {
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
				valid = true
				keyName = name
				break
			}
		}

		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_api_key",
				"message": "Невалидный API ключ",
			})
			c.Abort()
			return
		}

		// Устанавливаем значения в контекст для последующих handlers
		c.Set("api_key_validated", true)
		c.Set("api_key_name", keyName)
		c.Set("api_key", apiKey)

		c.Next()
	}
}

// RequireAPIKey хелпер для создания middleware, требующего API ключ для определённых роутов
func RequireAPIKey(validKeys map[string]string) gin.HandlerFunc {
	ak := NewAPIKey(APIKeyConfig{
		ValidKeys:  validKeys,
		HeaderName: "X-API-Key",
		Optional:   false,
	})
	return ak.Middleware()
}

// OptionalAPIKey хелпер для создания middleware, который опционально принимает API ключ
func OptionalAPIKey(validKeys map[string]string) gin.HandlerFunc {
	ak := NewAPIKey(APIKeyConfig{
		ValidKeys:  validKeys,
		HeaderName: "X-API-Key",
		Optional:   true,
	})
	return ak.Middleware()
}

// GetAPIKeyFromContext извлекает API ключ из контекста
func GetAPIKeyFromContext(c *gin.Context) (string, bool) {
	key, exists := c.Get("api_key")
	if !exists {
		return "", false
	}
	return key.(string), true
}

// IsAPIKeyValidated проверяет, был ли API ключ успешно валидирован
func IsAPIKeyValidated(c *gin.Context) bool {
	validated, exists := c.Get("api_key_validated")
	if !exists {
		return false
	}
	return validated.(bool)
}
