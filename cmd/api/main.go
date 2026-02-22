package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/config"
	"github.com/SergeiKhy/url-shortener/internal/handler"
	"github.com/SergeiKhy/url-shortener/internal/middleware"
	"github.com/SergeiKhy/url-shortener/internal/repository"
	"github.com/SergeiKhy/url-shortener/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Загрузка конфига
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация логгера
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Подключение к БД (postgres)
	db, err := repository.NewPostgresDB(cfg.DB)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	logger.Info("Connected to PostgreSQL")

	// Подключение к Redis
	redis, err := repository.NewRedisClient(cfg.Redis)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redis.Close()
	logger.Info("Connected to Redis")

	// Инициализация репозиториев
	linkRepo := repository.NewLinkRepository(db)
	cacheRepo := repository.NewCacheRepository(redis)
	clickRepo := repository.NewClickRepository(db)

	// Инициализация сервиса
	linkService := service.NewLinkService(linkRepo, cacheRepo)

	// Инициализация процессора кликов (Worker Pool)
	clickProcessor := service.NewClickProcessor(clickRepo, linkRepo, logger)
	clickProcessor.Start()
	defer clickProcessor.Stop()

	// Инициализация middleware
	rateLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
		BurstSize:         cfg.RateLimit.BurstSize,
		CleanupInterval:   time.Minute,
	})

	var apiKeyMiddleware gin.HandlerFunc
	if len(cfg.Auth.APIKeys) > 0 {
		apiKeyMiddleware = middleware.RequireAPIKey(cfg.Auth.APIKeys)
		logger.Info("API key authentication enabled", zap.Int("keys_count", len(cfg.Auth.APIKeys)))
	}

	// Настройка роутера
	router := handler.NewRouter(linkService, clickProcessor, rateLimiter, apiKeyMiddleware, logger)

	// Запуск сервера
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запуск в горутине
	go func() {
		logger.Info("Server starting", zap.String("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
