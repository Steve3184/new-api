package service

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const platformTrafficCacheTTL = 10 * time.Second

// PlatformTraffic is how many requests and tokens the whole deployment served
// in the last minute.
type PlatformTraffic struct {
	Rpm int `json:"rpm"`
	Tpm int `json:"tpm"`
}

type platformTrafficCacheItem struct {
	expiresAt time.Time
	logDB     *gorm.DB
	data      *PlatformTraffic
}

var (
	platformTrafficCacheMu sync.Mutex
	platformTrafficCache   platformTrafficCacheItem
	platformTrafficBuilds  singleflight.Group
)

// GetPlatformTraffic returns the request and token rates of the last minute
// across all users. Visitors who are not signed in can ask for it, so the
// answer is cached briefly and concurrent callers share one build: however
// many ask, the log table is read at most once per platformTrafficCacheTTL.
func GetPlatformTraffic() (*PlatformTraffic, error) {
	result, err, _ := platformTrafficBuilds.Do("", func() (any, error) {
		now := time.Now()
		platformTrafficCacheMu.Lock()
		cached := platformTrafficCache
		platformTrafficCacheMu.Unlock()
		if cached.data != nil && cached.logDB == model.LOG_DB && now.Before(cached.expiresAt) {
			return cached.data, nil
		}

		stat, err := model.SumUsedQuota(0, model.LogTypeConsume, now.Add(-time.Minute).Unix(), 0, "", "", "", 0, "", "", "", "")
		if err != nil {
			return nil, err
		}
		data := &PlatformTraffic{Rpm: stat.Rpm, Tpm: stat.Tpm}

		platformTrafficCacheMu.Lock()
		platformTrafficCache = platformTrafficCacheItem{
			expiresAt: now.Add(platformTrafficCacheTTL),
			logDB:     model.LOG_DB,
			data:      data,
		}
		platformTrafficCacheMu.Unlock()
		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*PlatformTraffic), nil
}
