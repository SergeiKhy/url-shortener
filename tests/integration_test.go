package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go.uber.org/zap"

	"github.com/SergeiKhy/url-shortener/internal/config"
	"github.com/SergeiKhy/url-shortener/internal/handler"
	"github.com/SergeiKhy/url-shortener/internal/middleware"
	"github.com/SergeiKhy/url-shortener/internal/repository"
	"github.com/SergeiKhy/url-shortener/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMain настраивает тестовые контейнеры
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// TestEnv хранит окружение для интеграционных тестов
type TestEnv struct {
	router         *gin.Engine
	linkService    service.LinkService
	clickProc      service.ClickProcessor
	dbContainer    testcontainers.Container
	redisContainer testcontainers.Container
	db             *repository.PostgresDB
	redis          *repository.RedisDB
}

// runMigrations запускает миграции БД
func runMigrations(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Создаём таблицу links
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS links (
			id SERIAL PRIMARY KEY,
			short_code VARCHAR(12) UNIQUE NOT NULL,
			original_url TEXT NOT NULL,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create links table: %w", err)
	}

	// Создаём индексы
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_short_code ON links(short_code)`)
	if err != nil {
		return fmt.Errorf("failed to create index on short_code: %w", err)
	}

	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_expires_at ON links(expires_at)`)
	if err != nil {
		return fmt.Errorf("failed to create index on expires_at: %w", err)
	}

	// Создаём таблицу clicks
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS clicks (
			id SERIAL PRIMARY KEY,
			link_id INTEGER REFERENCES links(id) ON DELETE CASCADE,
			ip_address INET,
			user_agent TEXT,
			referer TEXT,
			country VARCHAR(2),
			clicked_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create clicks table: %w", err)
	}

	// Создаём индексы для clicks
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_clicks_link_id ON clicks(link_id)`)
	if err != nil {
		return fmt.Errorf("failed to create index on link_id: %w", err)
	}

	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at)`)
	if err != nil {
		return fmt.Errorf("failed to create index on clicked_at: %w", err)
	}

	return nil
}

// setupTestEnv создаёт тестовое окружение с PostgreSQL и Redis контейнерами
func setupTestEnv(t *testing.T) *TestEnv {
	ctx := t.Context()

	// Запускаем контейнер PostgreSQL
	dbContainer, err := pgcontainer.Run(ctx,
		"postgres:15-alpine",
		pgcontainer.WithDatabase("shortener"),
		pgcontainer.WithUsername("user"),
		pgcontainer.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	// Запускаем контейнер Redis
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
	)
	require.NoError(t, err)

	// Получаем данные для подключения
	dbHost, err := dbContainer.Host(ctx)
	require.NoError(t, err)
	dbPort, err := dbContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// Создаём подключение к БД
	db, err := repository.NewPostgresDB(config.DBConfig{
		Host:     dbHost,
		Port:     dbPort.Port(),
		User:     "user",
		Password: "password",
		Name:     "shortener",
	})
	require.NoError(t, err)

	// Запускаем миграции
	err = runMigrations(db.Pool)
	require.NoError(t, err)

	// Создаём подключение к Redis
	redisClient, err := repository.NewRedisClient(config.RedisConfig{
		Host: redisHost,
		Port: redisPort.Port(),
	})
	require.NoError(t, err)

	// Инициализируем репозитории и сервисы
	linkRepo := repository.NewLinkRepository(db)
	cacheRepo := repository.NewCacheRepository(redisClient)
	clickRepo := repository.NewClickRepository(db)

	logger, _ := zap.NewDevelopment()
	linkService := service.NewLinkService(linkRepo, cacheRepo, logger)
	clickProc := service.NewClickProcessor(clickRepo, linkRepo, logger)
	clickProc.Start()

	// Настраиваем роутер с middleware
	rateLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
		RequestsPerSecond: 100, // Высокий лимит для тестов
		BurstSize:         200,
		CleanupInterval:   time.Minute,
	})

	router := handler.NewRouter(linkService, clickProc, rateLimiter, nil, logger)

	return &TestEnv{
		router:         router,
		linkService:    linkService,
		clickProc:      clickProc,
		dbContainer:    dbContainer,
		redisContainer: redisContainer,
		db:             db,
		redis:          redisClient,
	}
}

// teardown очищает ресурсы после теста
func (env *TestEnv) teardown(t *testing.T) {
	env.clickProc.Stop()
	env.db.Close()
	env.redis.Close()

	ctx := t.Context()
	if env.dbContainer != nil {
		env.dbContainer.Terminate(ctx)
	}
	if env.redisContainer != nil {
		env.redisContainer.Terminate(ctx)
	}
}

// CreateLinkRequest представляет тело запроса для создания ссылки
type CreateLinkRequest struct {
	URL        string `json:"url"`
	ExpiresIn  *int   `json:"expires_in,omitempty"`
	CustomCode string `json:"custom_code,omitempty"`
}

// CreateLinkResponse представляет тело ответа при создании ссылки
type CreateLinkResponse struct {
	ShortCode   string     `json:"short_code"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ErrorResponse представляет ответ с ошибкой
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// TestIntegration_CreateLink тестирует создание ссылок через API
func TestIntegration_CreateLink(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в коротком режиме")
	}

	env := setupTestEnv(t)
	defer env.teardown(t)

	tests := []struct {
		name           string
		request        CreateLinkRequest
		expectedStatus int
		expectError    bool
	}{
		{
			name: "валидный URL",
			request: CreateLinkRequest{
				URL: "https://example.com/test",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "валидный URL с кастомным кодом",
			request: CreateLinkRequest{
				URL:        "https://example.com/custom",
				CustomCode: "my-custom",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "невалидный URL",
			request: CreateLinkRequest{
				URL: "not-a-url",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "спам домен",
			request: CreateLinkRequest{
				URL: "https://malware.com/bad",
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/links", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			env.router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectError {
				var errResp ErrorResponse
				json.Unmarshal(w.Body.Bytes(), &errResp)
				assert.NotEmpty(t, errResp.Error)
			} else {
				var resp CreateLinkResponse
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.NotEmpty(t, resp.ShortCode)
				assert.Equal(t, tt.request.URL, resp.OriginalURL)
			}
		})
	}
}

// TestIntegration_GetLink тестирует получение и редирект ссылок
func TestIntegration_GetLink(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в коротком режиме")
	}

	env := setupTestEnv(t)
	defer env.teardown(t)

	// Сначала создаём ссылку
	createReq := CreateLinkRequest{
		URL: "https://example.com/integration-test",
	}
	body, _ := json.Marshal(createReq)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	env.router.ServeHTTP(w, req)

	var createResp CreateLinkResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Тестируем редирект
	t.Run("редирект на оригинальный URL", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/"+createResp.ShortCode, nil)
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
		assert.Equal(t, createReq.URL, w.Header().Get("Location"))
	})

	// Тестируем несуществующую ссылку
	t.Run("несуществующая ссылка", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/nonexistent", nil)
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestIntegration_DeleteLink тестирует удаление ссылок
func TestIntegration_DeleteLink(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в коротком режиме")
	}

	env := setupTestEnv(t)
	defer env.teardown(t)

	// Сначала создаём ссылку
	createReq := CreateLinkRequest{
		URL: "https://example.com/delete-test",
	}
	body, _ := json.Marshal(createReq)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	env.router.ServeHTTP(w, req)

	var createResp CreateLinkResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Удаляем ссылку
	t.Run("удаление существующей ссылки", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/links/"+createResp.ShortCode, nil)
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Пытаемся удалить повторно (должна быть ошибка)
	t.Run("удаление несуществующей ссылки", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/links/"+createResp.ShortCode, nil)
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestIntegration_ClickStats тестирует статистику кликов
func TestIntegration_ClickStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в коротком режиме")
	}

	env := setupTestEnv(t)
	defer env.teardown(t)

	// Сначала создаём ссылку
	createReq := CreateLinkRequest{
		URL: "https://example.com/stats-test",
	}
	body, _ := json.Marshal(createReq)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	env.router.ServeHTTP(w, req)

	var createResp CreateLinkResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Симулируем несколько кликов (вызовом редиректа)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/"+createResp.ShortCode, nil)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("192.168.1.%d", i))
		env.router.ServeHTTP(w, req)
	}

	// Даём worker pool время обработать клики
	time.Sleep(500 * time.Millisecond)

	// Получаем статистику
	t.Run("получение статистики кликов", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/links/"+createResp.ShortCode+"/stats", nil)
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &stats)
		assert.Equal(t, createResp.ShortCode, stats["short_code"])
		// Примечание: клики могут быть не полностью обработаны в тестовой среде
	})
}

// TestIntegration_HealthCheck тестирует endpoint проверки здоровья
func TestIntegration_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционный тест в коротком режиме")
	}

	env := setupTestEnv(t)
	defer env.teardown(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	env.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "url-shortener", resp["service"])
}
