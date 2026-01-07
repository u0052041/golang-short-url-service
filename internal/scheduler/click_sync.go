package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jack/golang-short-url-service/internal/repository"
)

// ClickSyncScheduler handles periodic synchronization of click counts from Redis to PostgreSQL
type ClickSyncScheduler struct {
	postgresRepo *repository.PostgresRepository
	redisRepo    *repository.RedisRepository
	interval     time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewClickSyncScheduler creates a new click sync scheduler
func NewClickSyncScheduler(
	postgresRepo *repository.PostgresRepository,
	redisRepo *repository.RedisRepository,
	interval time.Duration,
) *ClickSyncScheduler {
	return &ClickSyncScheduler{
		postgresRepo: postgresRepo,
		redisRepo:    redisRepo,
		interval:     interval,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the periodic sync process
func (s *ClickSyncScheduler) Start() {
	s.wg.Add(1)
	go s.run()
	log.Printf("Click sync scheduler started (interval: %v)", s.interval)
}

// Stop gracefully stops the scheduler
func (s *ClickSyncScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	log.Println("Click sync scheduler stopped")
}

func (s *ClickSyncScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.syncClickCounts()
		case <-s.stopCh:
			// Perform final sync before stopping
			log.Println("Performing final click count sync before shutdown...")
			s.syncClickCounts()
			return
		}
	}
}

// syncClickCounts syncs all pending click counts from Redis to PostgreSQL
func (s *ClickSyncScheduler) syncClickCounts() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get all click count keys
	keys, err := s.redisRepo.GetAllClickCountKeys(ctx)
	if err != nil {
		log.Printf("Failed to get click count keys: %v", err)
		return
	}

	if len(keys) == 0 {
		return
	}

	log.Printf("Syncing click counts for %d URLs...", len(keys))

	var successCount, failCount int

	for _, key := range keys {
		shortCode := repository.ExtractShortCodeFromKey(key)

		// Atomically get and reset the count
		count, err := s.redisRepo.GetAndResetClickCount(ctx, shortCode)
		if err != nil {
			log.Printf("Failed to get click count for %s: %v", shortCode, err)
			failCount++
			continue
		}

		if count == 0 {
			continue
		}

		// Update database with the accumulated count
		if err := s.postgresRepo.IncrementClickCountBy(ctx, shortCode, count); err != nil {
			// On failure, try to restore the count to Redis
			log.Printf("Failed to sync click count for %s: %v", shortCode, err)
			if restoreErr := s.restoreClickCount(ctx, shortCode, count); restoreErr != nil {
				log.Printf("Failed to restore click count for %s: %v (data loss: %d clicks)", shortCode, restoreErr, count)
			}
			failCount++
			continue
		}

		successCount++
	}

	if successCount > 0 || failCount > 0 {
		log.Printf("Click count sync completed: %d success, %d failed", successCount, failCount)
	}
}

// restoreClickCount restores click count to Redis if database sync fails
func (s *ClickSyncScheduler) restoreClickCount(ctx context.Context, shortCode string, count int64) error {
	return s.redisRepo.IncrementClickCountBy(ctx, shortCode, count)
}

// SyncNow triggers an immediate sync (useful for graceful shutdown)
func (s *ClickSyncScheduler) SyncNow() {
	s.syncClickCounts()
}

