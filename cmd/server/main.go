package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jack/golang-short-url-service/internal/config"
	"github.com/jack/golang-short-url-service/internal/handler"
	"github.com/jack/golang-short-url-service/internal/middleware"
	"github.com/jack/golang-short-url-service/internal/repository"
	"github.com/jack/golang-short-url-service/internal/scheduler"
	"github.com/jack/golang-short-url-service/internal/service"
)

const (
	ClickSyncInterval = 1 * time.Hour
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	postgresRepo, err := repository.NewPostgresRepository(&cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer postgresRepo.Close()
	log.Println("Connected to PostgreSQL")

	redisRepo, err := repository.NewRedisRepository(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisRepo.Close()
	log.Println("Connected to Redis")

	clickSyncScheduler := scheduler.NewClickSyncScheduler(postgresRepo, redisRepo, ClickSyncInterval)
	clickSyncScheduler.Start()
	defer clickSyncScheduler.Stop()

	shortURLService := service.NewShortURLService(postgresRepo, redisRepo, cfg)

	h := handler.NewHandler(shortURLService)

	// 一般 API 限流（使用配置文件設定）
	rateLimiter := middleware.NewRateLimiter(redisRepo.Client(), &cfg.RateLimit)

	// 創建短網址的嚴格限流（10次/分鐘）
	strictRateLimitConfig := &config.RateLimitConfig{
		Requests: 10,
		Duration: time.Minute,
	}
	strictRateLimiter := middleware.NewRateLimiter(redisRepo.Client(), strictRateLimitConfig)

	router := gin.New()

	// 依需求：避免 panic 時回傳 HTTP 500；錯誤細節寫入 log，對外回固定格式。
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Printf("panic recovered: path=%s err=%v", c.Request.URL.Path, recovered)
		c.AbortWithStatusJSON(http.StatusOK, gin.H{
			"error":   "internal_error",
			"message": "Internal server error",
		})
	}))
	router.Use(gin.Logger())

	// 若服務部署在 Nginx/Proxy 後面，需設定可信任來源，否則 ClientIP() 可能被偽造。
	router.SetTrustedProxies([]string{"127.0.0.1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})

	// Swagger UI - API 文檔界面
	SetupSwagger(router, &cfg.Auth)

	// 健康檢查
	router.GET("/health", h.Health)
	router.GET("/health/detailed", h.HealthDetailed)

	api := router.Group("/api/v1")
	{
		// 創建短網址 - 嚴格限流（10次/分鐘）
		api.POST("/shorten", strictRateLimiter.Middleware(), h.CreateShortURL)
		// 統計查詢 - 一般限流
		api.GET("/stats/:code", rateLimiter.Middleware(), h.GetStats)
	}

	// 重定向 - 一般限流
	router.GET("/:code", rateLimiter.Middleware(), h.Redirect)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
