package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig конфигурация rate limiter
type RateLimiterConfig struct {
	RequestsPerSecond float64       // Количество запросов в секунду
	BurstSize         int           // Максимальный размер burst
	CleanupInterval   time.Duration // Интервал очистки неактивных посетителей
}

// DefaultRateLimiterConfig конфигурация по умолчанию
var DefaultRateLimiterConfig = RateLimiterConfig{
	RequestsPerSecond: 10, // 10 запросов в секунду
	BurstSize:         20, // Burst до 20 запросов
	CleanupInterval:   time.Minute,
}

// visitor представляет rate limiter для одного клиента
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter middleware для ограничения запросов с использованием алгоритма Token Bucket
type RateLimiter struct {
	config   RateLimiterConfig
	visitors map[string]*visitor // IP -> visitor
	mu       sync.RWMutex
}

// NewRateLimiter создаёт новый rate limiter middleware
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:   config,
		visitors: make(map[string]*visitor),
	}

	// Запускаем горутину для периодической очистки
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop периодически удаляет неактивных посетителей
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup удаляет посетителей, которые не были активны долгое время
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > rl.config.CleanupInterval*3 {
			delete(rl.visitors, ip)
		}
	}
}

// getLimiter возвращает или создаёт rate limiter для данного IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if v, exists := rl.visitors[ip]; exists {
		v.lastSeen = time.Now()
		return v.limiter
	}

	// Создаём новый limiter с заданными параметрами
	limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize)
	rl.visitors[ip] = &visitor{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

// Middleware возвращает Gin middleware handler для rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Слишком много запросов, попробуйте позже",
				"retry_after": int(rl.config.CleanupInterval / time.Second),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// MiddlewareWithKey возвращает rate limiter с кастомным ключом (например, API ключ)
func (rl *RateLimiter) MiddlewareWithKey(getKey func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := getKey(c)
		if key == "" {
			key = c.ClientIP()
		}
		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Слишком много запросов, попробуйте позже",
				"retry_after": int(rl.config.CleanupInterval / time.Second),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
