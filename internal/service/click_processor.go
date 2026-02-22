package service

import (
	"context"
	"sync"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/SergeiKhy/url-shortener/internal/repository"
	"go.uber.org/zap"
)

// Константы worker pool
const (
	defaultWorkerCount   = 3  // Количество воркеров
	defaultChannelBuffer = 1000 // Размер буфера канала
	maxRetries           = 3  // Максимальное количество попыток записи
)

// ClickProcessor интерфейс для асинхронного отслеживания кликов
type ClickProcessor interface {
	Start()
	Stop()
	RecordClick(ctx context.Context, event *models.ClickEvent) error
	GetStats(ctx context.Context, shortCode string) (*models.ClickStats, error)
	GetDailyStats(ctx context.Context, shortCode string, days int) ([]models.DailyClickStats, error)
}

// clickProcessor реализация процессора кликов с использованием Worker Pool
type clickProcessor struct {
	clickRepo    repository.ClickRepository
	linkRepo     repository.LinkRepository
	logger       *zap.Logger
	clickChannel chan *models.ClickEvent // Канал для событий кликов
	workerCount  int                     // Количество воркеров
	wg           sync.WaitGroup          // WaitGroup для ожидания завершения воркеров
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewClickProcessor создаёт новый экземпляр процессора кликов
func NewClickProcessor(
	clickRepo repository.ClickRepository,
	linkRepo repository.LinkRepository,
	logger *zap.Logger,
) ClickProcessor {
	return &clickProcessor{
		clickRepo:    clickRepo,
		linkRepo:     linkRepo,
		logger:       logger,
		clickChannel: make(chan *models.ClickEvent, defaultChannelBuffer),
		workerCount:  defaultWorkerCount,
	}
}

// Start запускает worker pool
func (p *clickProcessor) Start() {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	p.logger.Info("Запуск воркеров процессора кликов", zap.Int("count", p.workerCount))

	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop корректно останавливает worker pool
func (p *clickProcessor) Stop() {
	p.logger.Info("Остановка процессора кликов...")
	p.cancel()
	p.wg.Wait()
	p.logger.Info("Процессор кликов остановлен")
}

// worker обрабатывает события кликов из канала
func (p *clickProcessor) worker(id int) {
	defer p.wg.Done()

	p.logger.Debug("Воркер кликов запущен", zap.Int("id", id))

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Воркер кликов остановлен", zap.Int("id", id))
			return

		case event, ok := <-p.clickChannel:
			if !ok {
				return
			}
			p.processClick(event)
		}
	}
}

// processClick обрабатывает одно событие клика с retry логикой
func (p *clickProcessor) processClick(event *models.ClickEvent) {
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	// Получаем ID ссылки по короткому коду
	linkID, err := p.linkRepo.GetLinkIDByShortCode(ctx, event.ShortCode)
	if err != nil {
		p.logger.Warn("Не удалось получить ID ссылки для клика",
			zap.String("short_code", event.ShortCode),
			zap.Error(err),
		)
		return
	}

	click := &models.Click{
		LinkID:    linkID,
		ShortCode: event.ShortCode,
		IPAddress: event.IPAddress,
		UserAgent: event.UserAgent,
		Referer:   event.Referer,
		Country:   event.Country,
		ClickedAt: time.Now(),
	}

	// Retry логика для записи в БД
	for i := 0; i < maxRetries; i++ {
		if err := p.clickRepo.RecordClick(ctx, click); err == nil {
			return
		}
		// Логгируем попытку retry
		if i < maxRetries-1 {
			p.logger.Debug("Повторная попытка записи клика",
				zap.String("short_code", event.ShortCode),
				zap.Int("attempt", i+1),
				zap.Error(err),
			)
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}

	p.logger.Error("Не удалось записать клик после всех попыток",
		zap.String("short_code", event.ShortCode),
		zap.Error(err),
	)
}

// RecordClick отправляет событие клика в worker pool (неблокирующая операция)
func (p *clickProcessor) RecordClick(ctx context.Context, event *models.ClickEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.clickChannel <- event:
		return nil
	default:
		// Канал заполнен, логируем предупреждение, но не блокируем запрос
		p.logger.Warn("Буфер канала кликов заполнен, событие потеряно",
			zap.String("short_code", event.ShortCode),
		)
		return nil // Не прерываем запрос, просто теряем статистику
	}
}

// GetStats получает статистику кликов для короткого кода
func (p *clickProcessor) GetStats(ctx context.Context, shortCode string) (*models.ClickStats, error) {
	return p.clickRepo.GetStats(ctx, shortCode)
}

// GetDailyStats получает дневную статистику кликов
func (p *clickProcessor) GetDailyStats(ctx context.Context, shortCode string, days int) ([]models.DailyClickStats, error) {
	return p.clickRepo.GetDailyStats(ctx, shortCode, days)
}

// GetChannelStats возвращает статистику канала для мониторинга
func (p *clickProcessor) GetChannelStats() ChannelStats {
	return ChannelStats{
		BufferSize:  cap(p.clickChannel),
		BufferUsed:  len(p.clickChannel),
		WorkerCount: p.workerCount,
	}
}

// ChannelStats статистика канала worker pool
type ChannelStats struct {
	BufferSize  int `json:"buffer_size"`  // Общая ёмкость канала
	BufferUsed  int `json:"buffer_used"`  // Текущее использование
	WorkerCount int `json:"worker_count"` // Количество воркеров
}
