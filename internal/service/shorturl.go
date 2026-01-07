package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jack/golang-short-url-service/internal/config"
	"github.com/jack/golang-short-url-service/internal/model"
	"github.com/jack/golang-short-url-service/internal/repository"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type ShortURLService struct {
	postgresRepo *repository.PostgresRepository
	redisRepo    *repository.RedisRepository
	cfg          *config.Config
}

func NewShortURLService(
	postgresRepo *repository.PostgresRepository,
	redisRepo *repository.RedisRepository,
	cfg *config.Config,
) *ShortURLService {
	return &ShortURLService{
		postgresRepo: postgresRepo,
		redisRepo:    redisRepo,
		cfg:          cfg,
	}
}

func (s *ShortURLService) CreateShortURL(ctx context.Context, req *model.CreateURLRequest) (*model.CreateURLResponse, error) {
	urlHash := hashURL(req.URL)

	existing, err := s.postgresRepo.GetURLByHash(ctx, urlHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing url: %w", err)
	}

	if existing != nil && existing.IsValid() {
		response := &model.CreateURLResponse{
			ShortCode:   existing.ShortCode,
			ShortURL:    s.cfg.App.BaseURL + "/" + existing.ShortCode,
			OriginalURL: existing.OriginalURL,
		}
		if existing.ExpiresAt != nil {
			response.ExpiresAt = existing.ExpiresAt.Format(time.RFC3339)
		}
		return response, nil
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err != nil {
			return nil, fmt.Errorf("invalid expires_in format: %w", err)
		}
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	url, err := s.postgresRepo.CreateURL(ctx, urlHash, req.URL, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create url: %w", err)
	}

	shortCode := encodeBase62(url.ID)

	for len(shortCode) < s.cfg.URL.ShortCodeLength {
		shortCode = "0" + shortCode
	}

	if err := s.postgresRepo.UpdateShortCode(ctx, url.ID, shortCode); err != nil {
		return nil, fmt.Errorf("failed to update short code: %w", err)
	}

	url.ShortCode = shortCode

	if err := s.redisRepo.SetURL(ctx, url); err != nil {
		log.Printf("cache set url failed: shortCode=%s err=%v", shortCode, err)
	}

	response := &model.CreateURLResponse{
		ShortCode:   shortCode,
		ShortURL:    s.cfg.App.BaseURL + "/" + shortCode,
		OriginalURL: req.URL,
	}

	if expiresAt != nil {
		response.ExpiresAt = expiresAt.Format(time.RFC3339)
	}

	return response, nil
}

func hashURL(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}

func (s *ShortURLService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	url, err := s.redisRepo.GetURL(ctx, shortCode)
	if err != nil {
		log.Printf("cache get url failed: shortCode=%s err=%v", shortCode, err)
	}

	if url != nil {
		if !url.IsValid() {
			return "", repository.ErrURLExpired
		}

		// 點擊計數用 Redis 累積，交給 scheduler 批次回寫 PostgreSQL（減少寫入壓力）。
		s.incrementClickCount(shortCode)

		return url.OriginalURL, nil
	}

	url, err = s.postgresRepo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		return "", err
	}

	if !url.IsValid() {
		return "", repository.ErrURLExpired
	}

	if err := s.redisRepo.SetURL(ctx, url); err != nil {
		log.Printf("cache set url failed: shortCode=%s err=%v", shortCode, err)
	}

	s.incrementClickCount(shortCode)

	return url.OriginalURL, nil
}

func (s *ShortURLService) GetURLStats(ctx context.Context, shortCode string) (*model.URLStatsResponse, error) {
	url, err := s.postgresRepo.GetURLStats(ctx, shortCode)
	if err != nil {
		return nil, err
	}

	// Stats 需要合併「DB 已同步」+「Redis 尚未同步」的點擊數，才能接近即時。
	pendingClicks, err := s.redisRepo.GetClickCount(ctx, shortCode)
	if err != nil {
		log.Printf("cache get pending clicks failed: shortCode=%s err=%v", shortCode, err)
	}

	response := &model.URLStatsResponse{
		ShortCode:   url.ShortCode,
		OriginalURL: url.OriginalURL,
		ClickCount:  url.ClickCount + pendingClicks,
		CreatedAt:   url.CreatedAt,
		IsActive:    url.IsActive,
	}

	if url.ExpiresAt != nil {
		response.ExpiresAt = url.ExpiresAt.Format(time.RFC3339)
	}

	return response, nil
}

func (s *ShortURLService) LogAccess(ctx context.Context, urlID int64, ip, userAgent, referer string) {
	accessLog := &model.URLAccessLog{
		URLID:     urlID,
		IPAddress: ip,
		UserAgent: userAgent,
		Referer:   referer,
	}

	if err := s.postgresRepo.LogAccess(ctx, accessLog); err != nil {
		log.Printf("db log access failed: urlID=%d err=%v", urlID, err)
	}
}

func (s *ShortURLService) incrementClickCount(shortCode string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := s.redisRepo.IncrementClickCount(ctx, shortCode); err != nil {
		log.Printf("cache incr click failed: shortCode=%s err=%v", shortCode, err)
	}
}

func encodeBase62(num int64) string {
	if num == 0 {
		return string(base62Chars[0])
	}

	var result strings.Builder
	for num > 0 {
		result.WriteByte(base62Chars[num%62])
		num /= 62
	}

	runes := []rune(result.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

func decodeBase62(s string) int64 {
	var num int64
	for _, c := range s {
		num *= 62
		if c >= '0' && c <= '9' {
			num += int64(c - '0')
		} else if c >= 'A' && c <= 'Z' {
			num += int64(c - 'A' + 10)
		} else if c >= 'a' && c <= 'z' {
			num += int64(c - 'a' + 36)
		}
	}
	return num
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	// Go 的 time.ParseDuration 不支援 "7d"（只支援 h/m/s...），這裡額外補 day。
	if strings.HasSuffix(s, "d") {
		days := s[:len(s)-1]
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
