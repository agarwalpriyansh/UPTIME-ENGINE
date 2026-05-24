package database

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"monitor-engine/models"
)

const alertKeyTTL = time.Hour

var (
	alertMemFallback sync.Map // siteID -> models.AlertState
)

func alertFailuresKey(siteID string) string { return "alert:failures:" + siteID }
func alertCooldownKey(siteID string) string { return "alert:cooldown:" + siteID }
func alertIsDownKey(siteID string) string    { return "alert:isdown:" + siteID }

func redisAvailable() bool {
	if RedisClient == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	return RedisClient.Ping(ctx).Err() == nil
}

func saveAlertStateMemory(state models.AlertState) {
	alertMemFallback.Store(state.SiteID, state)
}

func loadAlertStateMemory(siteID string) models.AlertState {
	if v, ok := alertMemFallback.Load(siteID); ok {
		if s, ok := v.(models.AlertState); ok {
			return s
		}
	}
	return models.AlertState{SiteID: siteID}
}

// LoadAlertState reads failure count, cooldown, and is-down flag for a site.
// Falls back to in-memory state when Redis is unavailable.
func LoadAlertState(siteID string) models.AlertState {
	state := models.AlertState{SiteID: siteID}

	if !redisAvailable() {
		log.Printf("[ALERT] Redis unavailable, using memory fallback for %s", siteID)
		return loadAlertStateMemory(siteID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	failures, err := RedisClient.Get(ctx, alertFailuresKey(siteID)).Int()
	if err == nil {
		state.ConsecutiveFailures = failures
	}

	if ts, err := RedisClient.Get(ctx, alertCooldownKey(siteID)).Int64(); err == nil && ts > 0 {
		state.LastAlertSentAt = time.Unix(ts, 0)
	}

	if down, err := RedisClient.Get(ctx, alertIsDownKey(siteID)).Result(); err == nil {
		state.IsDown = down == "1" || down == "true"
	}

	return state
}

// SaveAlertState persists alert tracking fields with a 1-hour TTL on each key.
func SaveAlertState(state models.AlertState) error {
	if !redisAvailable() {
		saveAlertStateMemory(state)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pipe := RedisClient.Pipeline()
	pipe.Set(ctx, alertFailuresKey(state.SiteID), state.ConsecutiveFailures, alertKeyTTL)
	cooldown := int64(0)
	if !state.LastAlertSentAt.IsZero() {
		cooldown = state.LastAlertSentAt.Unix()
	}
	pipe.Set(ctx, alertCooldownKey(state.SiteID), strconv.FormatInt(cooldown, 10), alertKeyTTL)
	downVal := "0"
	if state.IsDown {
		downVal = "1"
	}
	pipe.Set(ctx, alertIsDownKey(state.SiteID), downVal, alertKeyTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[ALERT] Redis save failed for %s: %v (memory fallback)", state.SiteID, err)
		saveAlertStateMemory(state)
		return err
	}
	saveAlertStateMemory(state)
	return nil
}

// SetAlertCooldown records when an alert was last sent for cooldown enforcement.
func SetAlertCooldown(siteID string, at time.Time) {
	state := LoadAlertState(siteID)
	state.LastAlertSentAt = at
	_ = SaveAlertState(state)
}
