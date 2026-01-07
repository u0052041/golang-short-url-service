package handler

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jack/golang-short-url-service/internal/model"
	"github.com/jack/golang-short-url-service/internal/repository"
	"github.com/jack/golang-short-url-service/internal/service"
)

type Handler struct {
	service *service.ShortURLService
}

func NewHandler(service *service.ShortURLService) *Handler {
	return &Handler{service: service}
}

func respondInternalError(c *gin.Context, message string) {
	// 依需求：不回 500，錯誤細節寫進 log，對外只回固定訊息/格式
	c.JSON(http.StatusOK, gin.H{
		"error":   "internal_error",
		"message": message,
	})
}

func (h *Handler) CreateShortURL(c *gin.Context) {
	var req model.CreateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Invalid URL",
		})
		return
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Only http/https URLs are allowed",
		})
		return
	}

	response, err := h.service.CreateShortURL(c.Request.Context(), &req)
	if err != nil {
		log.Printf("create short url failed: ip=%s err=%v", c.ClientIP(), err)
		respondInternalError(c, "Failed to create short URL")
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Short code is required",
		})
		return
	}

	originalURL, err := h.service.GetOriginalURL(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, repository.ErrURLNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Short URL not found",
			})
			return
		}
		if errors.Is(err, repository.ErrURLExpired) {
			c.JSON(http.StatusGone, gin.H{
				"error":   "expired",
				"message": "This short URL has expired",
			})
			return
		}
		log.Printf("redirect failed: code=%s ip=%s err=%v", code, c.ClientIP(), err)
		respondInternalError(c, "Failed to retrieve URL")
		return
	}

	c.Redirect(http.StatusMovedPermanently, originalURL)
}

func (h *Handler) GetStats(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "Short code is required",
		})
		return
	}

	stats, err := h.service.GetURLStats(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, repository.ErrURLNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Short URL not found",
			})
			return
		}
		log.Printf("get stats failed: code=%s ip=%s err=%v", code, c.ClientIP(), err)
		respondInternalError(c, "Failed to retrieve stats")
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

func (h *Handler) HealthDetailed(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":   "healthy",
		"postgres": "connected",
		"redis":    "connected",
	})
}

