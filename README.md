# URL Shortener Service

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Coverage](https://img.shields.io/badge/coverage-80%25-brightgreen.svg)]()

Высокопроизводительный сервис сокращения URL на Go с отслеживанием кликов, ограничением запросов, аутентификацией по API ключам и подробной аналитикой.

## 🚀 Возможности

- **Сокращение URL** — создание коротких ссылок из длинных URL
- **Кастомные коды** — возможность указания своего короткого кода
- **Истечение ссылок** — установка времени жизни для ссылок
- **Отслеживание кликов** — асинхронная статистика кликов с использованием Worker Pool
- **Аналитика** — общие клики, уникальные клики, дневная статистика
- **Rate Limiting** — ограничение запросов по алгоритму Token Bucket
- **API Key аутентификация** — защита API с помощью ключей
- **Кэширование** — Redis для быстрых запросов
- **Graceful Shutdown** — корректное завершение работы
- **Swagger документация** — интерактивная документация API

## 📋 Требования

- Go 1.21+
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+

## 🏗️ Архитектура

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Клиент    │────▶│  Gin Router  │────▶│  Handlers   │
└─────────────┘     └──────────────┘     └─────────────┘
                          │                    │
                    ┌─────▼─────┐      ┌───────▼────────┐
                    │   Rate    │      │    Services    │
                    │  Limiter  │      │  (Бизнес)      │
                    └───────────┘      └────────────────┘
                          │                    │
                    ┌─────▼─────┐      ┌───────▼────────┐
                    │   API     │      │  Repositories  │
                    │   Key     │      │   (Данные)     │
                    └───────────┘      └────────────────┘
                                              │
                              ┌───────────────┼───────────────┐
                              │               │               │
                        ┌─────▼─────┐  ┌──────▼──────┐  ┌────▼────┐
                        │ PostgreSQL│  │    Redis    │  │ Workers │
                        │   (БД)    │  │   (Кэш)     │  │(Клики)  │
                        └───────────┘  └─────────────┘  └─────────┘
```

## 🛠️ Технологический стек

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.21+ |
| Web фреймворк | Gin |
| База данных | PostgreSQL 15 |
| Кэш | Redis 7 |
| Миграции | golang-migrate |
| Логгер | Zap |
| Конфигурация | Viper |
| Тестирование | testify, testcontainers |

## 📦 Установка

### 1. Клонирование репозитория

```bash
git clone https://github.com/SergeiKhy/url-shortener.git
cd url-shortener
```

### 2. Настройка окружения

```bash
cp .env.example .env
# Отредактируйте .env под ваши настройки
```

### 3. Запуск инфраструктуры через Docker

```bash
make docker-up
```

### 4. Запуск миграций БД

```bash
# С помощью утилиты migrate
migrate -path migration -database "postgres://user:password@localhost:5432/shortener?sslmode=disable" up

# Или через Docker
docker-compose exec api ./migrate -path /app/migration -database "postgres://user:password@postgres:5432/shortener?sslmode=disable" up
```

### 5. Запуск приложения

```bash
make run
```

Или сборка и запуск:

```bash
make build
./bin/api
```

## 📖 Документация API

### Интерактивная документация

После запуска сервера посетите:
- **Swagger UI**: http://localhost:8080/docs
- **Swagger JSON**: http://localhost:8080/docs/swagger.json

### API Endpoints

#### Проверка здоровья

```http
GET /api/v1/health
```

Ответ:
```json
{
  "status": "ok",
  "service": "url-shortener"
}
```

#### Создание короткой ссылки

```http
POST /api/v1/links
Content-Type: application/json
X-API-Key: your-api-key (если включено)

{
  "url": "https://example.com/very/long/url",
  "expires_in": 60,        // опционально, минуты
  "custom_code": "my-code" // опционально, 4-12 символов
}
```

Ответ (201 Created):
```json
{
  "short_code": "abc123xyz",
  "short_url": "http://localhost:8080/abc123xyz",
  "original_url": "https://example.com/very/long/url",
  "expires_at": "2024-01-15T12:00:00Z",
  "created_at": "2024-01-15T11:00:00Z"
}
```

#### Редирект на оригинальный URL

```http
GET /:code
```

Ответ: 307 Temporary Redirect

#### Удаление ссылки

```http
DELETE /api/v1/links/:code
```

Ответ (200 OK):
```json
{
  "message": "Ссылка успешно удалена"
}
```

#### Получение статистики кликов

```http
GET /api/v1/links/:code/stats
```

Ответ:
```json
{
  "short_code": "abc123xyz",
  "total_clicks": 150,
  "unique_clicks": 75
}
```

#### Получение дневной статистики

```http
GET /api/v1/links/:code/stats/daily?days=7
```

Ответ:
```json
[
  {"date": "2024-01-15", "clicks": 25},
  {"date": "2024-01-14", "clicks": 30}
]
```

## 🔐 Аутентификация

### Настройка API ключей

API ключи настраиваются через переменную окружения `API_KEYS`:

```bash
# Формат: key1:name1,key2:name2
API_KEYS=secret-key-1:Production,secret-key-2:Development
```

### Способы передачи API ключа

API ключ можно передать тремя способами:

1. **Заголовок** (рекомендуется):
   ```
   X-API-Key: your-api-key
   ```

2. **Query параметр**:
   ```
   GET /api/v1/links?api_key=your-api-key
   ```

3. **Bearer токен**:
   ```
   Authorization: Bearer your-api-key
   ```

## ⚡ Rate Limiting

Rate limiting включён по умолчанию со следующими настройками:

| Настройка | По умолчанию | Описание |
|-----------|--------------|----------|
| `RATE_LIMIT_RPS` | 10 | Запросов в секунду |
| `RATE_LIMIT_BURST` | 20 | Максимальный размер burst |

Настройка в `.env`:
```bash
RATE_LIMIT_RPS=10
RATE_LIMIT_BURST=20
```

При превышении лимита:
```json
{
  "error": "rate_limit_exceeded",
  "message": "Слишком много запросов, попробуйте позже",
  "retry_after": 60
}
```

## 📊 Статистика кликов (Worker Pool)

Сервис использует паттерн Worker Pool для асинхронного отслеживания кликов:

- **3 воркера** обрабатывают события кликов параллельно
- **Буфер канала** на 1000 событий
- **Retry логика** с экспоненциальной задержкой
- **Non-blocking** — клики не замедляют редиректы

### Архитектура Worker Pool

```
Запрос редиректа
       │
       ▼
┌─────────────┐
│   Handler   │
└─────────────┘
       │
       ▼
┌─────────────┐     ┌──────────┐     ┌──────────┐
│   Channel   │────▶│ Worker 1 │     │ Worker 2 │
│  (buffer)   │     └──────────┘     └──────────┘
└─────────────┘           │                │
       │                  ▼                ▼
       │           ┌─────────────────────────┐
       │           │    PostgreSQL (clicks)  │
       │           └─────────────────────────┘
       ▼
  Редирект (мгновенно)
```

## 🧪 Тестирование

### Запуск юнит-тестов

```bash
make test
```

### Запуск тестов с покрытием

```bash
go test ./... -v -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Запуск интеграционных тестов

```bash
# Требуется Docker для testcontainers
go test ./tests/... -v
```

### Запуск конкретного теста

```bash
go test ./internal/service/... -run TestLinkService_CreateLink -v
```

## 📁 Структура проекта

```
url-shortener/
├── cmd/
│   └── api/
│       └── main.go              # Точка входа приложения
├── internal/
│   ├── config/
│   │   └── config.go            # Управление конфигурацией
│   ├── handler/
│   │   ├── router.go            # Настройка HTTP роутера
│   │   ├── link_handler.go      # Обработчики ссылок
│   │   ├── health.go            # Health check handler
│   │   └── swagger.go           # Swagger документация
│   ├── middleware/
│   │   ├── ratelimit.go         # Rate limiting middleware
│   │   └── apikey.go            # API key аутентификация
│   ├── models/
│   │   ├── link.go              # Модели ссылок
│   │   └── click.go             # Модели кликов
│   ├── repository/
│   │   ├── repository.go        # PostgreSQL подключение
│   │   ├── redis.go             # Redis подключение
│   │   ├── link_repository.go   # Доступ к данным ссылок
│   │   ├── cache_repository.go  # Доступ к кэшу
│   │   └── click_repository.go  # Доступ к данным кликов
│   └── service/
│       ├── link_service.go      # Бизнес-логика ссылок
│       ├── click_processor.go   # Worker pool кликов
│       └── mocks/               # Мокы для тестов
├── migration/
│   └── 000001_init.sql          # Миграции БД
├── tests/
│   └── integration_test.go      # Интеграционные тесты
├── docs/
│   ├── swagger.json             # Swagger спецификация
│   └── swagger-ui.html          # Swagger UI
├── .env.example                 # Шаблон окружения
├── docker-compose.yml           # Docker сервисы
├── Dockerfile                   # Контейнер приложения
├── Makefile                     # Команды сборки
└── go.mod                       # Go модуль
```

## 🔧 Конфигурация

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `APP_PORT` | 8080 | Порт сервера |
| `DB_HOST` | localhost | Хост PostgreSQL |
| `DB_PORT` | 5432 | Порт PostgreSQL |
| `DB_USER` | user | Пользователь БД |
| `DB_PASSWORD` | password | Пароль БД |
| `DB_NAME` | shortener | Имя БД |
| `REDIS_HOST` | localhost | Хост Redis |
| `REDIS_PORT` | 6379 | Порт Redis |
| `LOG_LEVEL` | debug | Уровень логирования |
| `RATE_LIMIT_RPS` | 10 | Лимит запросов/секунду |
| `RATE_LIMIT_BURST` | 20 | Размер burst лимита |
| `API_KEYS` | - | API ключи (key:name,key:name) |

## 🐳 Docker

### Сборка и запуск

```bash
# Сборка образа
docker build -t url-shortener .

# Запуск контейнера
docker run -p 8080:8080 --env-file .env url-shortener
```

### Docker Compose

```bash
# Запуск всех сервисов
docker-compose up -d

# Просмотр логов
docker-compose logs -f

# Остановка всех сервисов
docker-compose down
```

## 📈 Мониторинг

### Health Check

```bash
curl http://localhost:8080/api/v1/health
```

### Статистика Click Processor

Click Processor предоставляет статистику канала для мониторинга:

```go
stats := clickProcessor.GetChannelStats()
// stats.BufferSize - Общая ёмкость канала
// stats.BufferUsed - Текущий размер очереди
// stats.WorkerCount - Активные воркеры
```

## 🔒 Безопасность

1. **API ключи** — используйте сложные сгенерированные ключи
2. **Rate Limiting** — защита от DDoS и злоупотреблений
3. **Валидация входных данных** — проверка формата URL и кастомных кодов
4. **Защита от спама** — чёрный список известных вредоносных доменов
5. **SQL Injection** — параметризованные запросы через pgx

## 🤝 Вклад в проект

1. Форкните репозиторий
2. Создайте ветку (`git checkout -b feature/amazing-feature`)
3. Закоммитьте изменения (`git commit -m 'Добавить amazing feature'`)
4. Отправьте в ветку (`git push origin feature/amazing-feature`)
5. Откройте Pull Request

## 📄 Лицензия

Этот проект лицензирован под MIT License — подробности в файле [LICENSE](LICENSE).

## 🙏 Благодарности

- [Gin Web Framework](https://github.com/gin-gonic/gin)
- [pgx - PostgreSQL Driver](https://github.com/jackc/pgx)
- [Go Redis](https://github.com/redis/go-redis)
- [Zap Logger](https://github.com/uber-go/zap)
- [Viper Configuration](https://github.com/spf13/viper)

---

**Сделано с ❤️ SergeiKhy**
