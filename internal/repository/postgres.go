package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jack/golang-short-url-service/internal/config"
	"github.com/jack/golang-short-url-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrURLNotFound = errors.New("url not found")
	ErrURLExpired  = errors.New("url has expired")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(cfg *config.PostgresConfig) (*PostgresRepository, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxConns)
	poolConfig.MinConns = int32(cfg.MinConns)
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

// CreateURL creates a new short URL and returns the generated ID
func (r *PostgresRepository) CreateURL(ctx context.Context, urlHash, originalURL string, expiresAt *time.Time) (*model.URL, error) {
	query := `
		INSERT INTO urls (short_code, url_hash, original_url, expires_at)
		VALUES ('temp', $1, $2, $3)
		RETURNING id, created_at, updated_at, is_active
	`

	var url model.URL
	url.URLHash = urlHash
	url.OriginalURL = originalURL
	url.ExpiresAt = expiresAt

	err := r.pool.QueryRow(ctx, query, urlHash, originalURL, expiresAt).Scan(
		&url.ID,
		&url.CreatedAt,
		&url.UpdatedAt,
		&url.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create url: %w", err)
	}

	return &url, nil
}

// GetURLByHash retrieves a URL by its hash (for deduplication)
func (r *PostgresRepository) GetURLByHash(ctx context.Context, urlHash string) (*model.URL, error) {
	query := `
		SELECT id, short_code, url_hash, original_url, click_count, created_at, updated_at, expires_at, is_active
		FROM urls
		WHERE url_hash = $1
	`

	var url model.URL
	err := r.pool.QueryRow(ctx, query, urlHash).Scan(
		&url.ID,
		&url.ShortCode,
		&url.URLHash,
		&url.OriginalURL,
		&url.ClickCount,
		&url.CreatedAt,
		&url.UpdatedAt,
		&url.ExpiresAt,
		&url.IsActive,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get url by hash: %w", err)
	}

	return &url, nil
}

// UpdateShortCode updates the short code for a URL
func (r *PostgresRepository) UpdateShortCode(ctx context.Context, id int64, shortCode string) error {
	query := `UPDATE urls SET short_code = $1 WHERE id = $2`
	
	_, err := r.pool.Exec(ctx, query, shortCode, id)
	if err != nil {
		return fmt.Errorf("failed to update short code: %w", err)
	}

	return nil
}

// GetURLByShortCode retrieves a URL by its short code
func (r *PostgresRepository) GetURLByShortCode(ctx context.Context, shortCode string) (*model.URL, error) {
	query := `
		SELECT id, short_code, url_hash, original_url, click_count, created_at, updated_at, expires_at, is_active
		FROM urls
		WHERE short_code = $1
	`

	var url model.URL
	err := r.pool.QueryRow(ctx, query, shortCode).Scan(
		&url.ID,
		&url.ShortCode,
		&url.URLHash,
		&url.OriginalURL,
		&url.ClickCount,
		&url.CreatedAt,
		&url.UpdatedAt,
		&url.ExpiresAt,
		&url.IsActive,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrURLNotFound
		}
		return nil, fmt.Errorf("failed to get url: %w", err)
	}

	return &url, nil
}

// IncrementClickCount increments the click count for a URL by 1
func (r *PostgresRepository) IncrementClickCount(ctx context.Context, id int64) error {
	query := `UPDATE urls SET click_count = click_count + 1 WHERE id = $1`
	
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment click count: %w", err)
	}

	return nil
}

// IncrementClickCountBy increments the click count for a URL by a specified amount (used for batch sync)
func (r *PostgresRepository) IncrementClickCountBy(ctx context.Context, shortCode string, count int64) error {
	query := `UPDATE urls SET click_count = click_count + $1 WHERE short_code = $2`
	
	result, err := r.pool.Exec(ctx, query, count, shortCode)
	if err != nil {
		return fmt.Errorf("failed to increment click count by %d: %w", count, err)
	}

	if result.RowsAffected() == 0 {
		return ErrURLNotFound
	}

	return nil
}

// LogAccess logs an access to a URL
func (r *PostgresRepository) LogAccess(ctx context.Context, log *model.URLAccessLog) error {
	query := `
		INSERT INTO url_access_logs (url_id, ip_address, user_agent, referer)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.pool.Exec(ctx, query, log.URLID, log.IPAddress, log.UserAgent, log.Referer)
	if err != nil {
		return fmt.Errorf("failed to log access: %w", err)
	}

	return nil
}

// GetURLStats retrieves statistics for a URL
func (r *PostgresRepository) GetURLStats(ctx context.Context, shortCode string) (*model.URL, error) {
	return r.GetURLByShortCode(ctx, shortCode)
}

// Health checks the database connection
func (r *PostgresRepository) Health(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

