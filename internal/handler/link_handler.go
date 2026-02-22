package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/SergeiKhy/url-shortener/internal/models"
	"github.com/SergeiKhy/url-shortener/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type LinkHandler struct {
	service         service.LinkService
	clickProcessor  service.ClickProcessor
	logger          *zap.Logger
}

func NewLinkHandler(service service.LinkService, clickProcessor service.ClickProcessor, logger *zap.Logger) *LinkHandler {
	return &LinkHandler{
		service:        service,
		clickProcessor: clickProcessor,
		logger:         logger,
	}
}

type CreateLinkRequest struct {
	URL         string `json:"url" binding:"required,url"`
	ExpiresIn   *int   `json:"expires_in,omitempty"`
	CustomCode  string `json:"custom_code,omitempty"`
}

type CreateLinkResponse struct {
	ShortCode   string     `json:"short_code"`
	ShortURL    string     `json:"short_url"`
	OriginalURL string     `json:"original_url"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// CreateLink godoc
// @Summary Create a short link
// @Description Create a new shortened URL
// @Tags links
// @Accept json
// @Produce json
// @Param request body CreateLinkRequest true "Link creation request"
// @Success 201 {object} CreateLinkResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/links [post]
func (h *LinkHandler) CreateLink(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	input := &models.CreateLinkInput{
		OriginalURL: req.URL,
		ExpiresIn:   req.ExpiresIn,
	}

	if req.CustomCode != "" {
		input.CustomCode = &req.CustomCode
	}

	link, err := h.service.CreateLink(c.Request.Context(), input)
	if err != nil {
		h.logger.Error("Failed to create link", zap.Error(err))

		switch err {
		case service.ErrInvalidURL:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_url",
				Message: "Invalid URL format",
			})
		case service.ErrInvalidCode:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_code",
				Message: "Custom code must be 4-12 alphanumeric characters",
			})
		case service.ErrSpamDomain:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "spam_domain",
				Message: "Domain is blacklisted",
			})
		default:
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to create link",
			})
		}
		return
	}

	response := CreateLinkResponse{
		ShortCode:   link.ShortCode,
		ShortURL:    "http://localhost:8080/" + link.ShortCode,
		OriginalURL: link.OriginalURL,
		ExpiresAt:   link.ExpiresAt,
		CreatedAt:   link.CreatedAt,
	}

	c.JSON(http.StatusCreated, response)
}

// Redirect godoc
// @Summary Redirect to original URL
// @Description Redirect to the original URL by short code
// @Tags links
// @Produce json
// @Param code path string true "Short code"
// @Success 307 {object} nil
// @Failure 404 {object} ErrorResponse
// @Router /{code} [get]
func (h *LinkHandler) Redirect(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_code",
			Message: "Short code is required",
		})
		return
	}

	link, err := h.service.GetLink(c.Request.Context(), code)
	if err != nil {
		h.logger.Warn("Link not found", zap.String("code", code), zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "not_found",
			Message: "Link not found or expired",
		})
		return
	}

	// Асинхронная запись статистики
	clickEvent := &models.ClickEvent{
		ShortCode: code,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Referer:   c.Request.Referer(),
		Country:   "", // Can be populated via GeoIP lookup
	}
	if err := h.clickProcessor.RecordClick(c.Request.Context(), clickEvent); err != nil {
		h.logger.Debug("Failed to record click (non-blocking)", zap.Error(err))
	}

	c.Redirect(http.StatusTemporaryRedirect, link.OriginalURL)
}

// DeleteLink godoc
// @Summary Delete a short link
// @Description Delete a shortened URL by short code
// @Tags links
// @Produce json
// @Param code path string true "Short code"
// @Success 200 {object} map[string]string
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/links/{code} [delete]
func (h *LinkHandler) DeleteLink(c *gin.Context) {
	code := c.Param("code")

	err := h.service.DeleteLink(c.Request.Context(), code)
	if err != nil {
		h.logger.Warn("Failed to delete link", zap.String("code", code), zap.Error(err))
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Link not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Link deleted successfully"})
}

// GetStats godoc
// @Summary Get click statistics for a short link
// @Description Get total and unique click counts for a shortened URL
// @Tags links
// @Produce json
// @Param code path string true "Short code"
// @Success 200 {object} models.ClickStats
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/links/{code}/stats [get]
func (h *LinkHandler) GetStats(c *gin.Context) {
	code := c.Param("code")

	stats, err := h.clickProcessor.GetStats(c.Request.Context(), code)
	if err != nil {
		h.logger.Warn("Failed to get stats", zap.String("code", code), zap.Error(err))
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Link not found",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetDailyStats godoc
// @Summary Get daily click statistics
// @Description Get daily click counts for a shortened URL
// @Tags links
// @Produce json
// @Param code path string true "Short code"
// @Param days query int false "Number of days" default(7)
// @Success 200 {array} models.DailyClickStats
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/links/{code}/stats/daily [get]
func (h *LinkHandler) GetDailyStats(c *gin.Context) {
	code := c.Param("code")
	days := 7
	if d := c.Query("days"); d != "" {
		if _, err := fmt.Sscanf(d, "%d", &days); err != nil || days < 1 || days > 90 {
			days = 7
		}
	}

	stats, err := h.clickProcessor.GetDailyStats(c.Request.Context(), code, days)
	if err != nil {
		h.logger.Warn("Failed to get daily stats", zap.String("code", code), zap.Error(err))
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Link not found",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}
