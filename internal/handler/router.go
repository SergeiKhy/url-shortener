package handler

import (
	"github.com/SergeiKhy/url-shortener/internal/middleware"
	"github.com/SergeiKhy/url-shortener/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(
	linkService service.LinkService,
	clickProcessor service.ClickProcessor,
	rateLimiter *middleware.RateLimiter,
	apiKeyMiddleware gin.HandlerFunc,
	logger *zap.Logger,
) *gin.Engine {
	router := gin.Default()

	// Middleware для логгирования
	router.Use(func(c *gin.Context) {
		logger.Info("Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
		)
		c.Next()
	})

	// Rate limiting для всех запросов
	router.Use(rateLimiter.Middleware())

	// Инициализация обработчика ссылок
	linkHandler := NewLinkHandler(linkService, clickProcessor, logger)

	// API v.1
	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", HealthCheck)

		// Применяем API Key middleware только к защищенным эндпоинтам
		if apiKeyMiddleware != nil {
			v1.Use(apiKeyMiddleware)
		}

		v1.POST("/links", linkHandler.CreateLink)
		v1.DELETE("/links/:code", linkHandler.DeleteLink)
		v1.GET("/links/:code/stats", linkHandler.GetStats)
		v1.GET("/links/:code/stats/daily", linkHandler.GetDailyStats)
	}

	// Редирект (корневой путь) - без API key проверки
	router.GET("/:code", linkHandler.Redirect)

	// Swagger документация (без аутентификации)
	AddSwaggerRoutes(router)

	return router
}
