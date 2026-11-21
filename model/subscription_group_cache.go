package model

import (
	"sync"
	"time"
)

// subscriptionBindingCache keeps the set of bound subscription groups. Every
// billed request asks whether its group is bound, yet the set changes only when
// an administrator edits a plan. It lives for the plan cache TTL and is dropped
// on every plan write; other nodes drop theirs through the plan sync topic.
var subscriptionBindingCache struct {
	sync.RWMutex
	names      []string
	set        map[string]struct{}
	expiresAt  time.Time
	generation uint64
}

func loadSubscriptionBindings() ([]string, map[string]struct{}, error) {
	subscriptionBindingCache.RLock()
	if subscriptionBindingCache.set != nil && time.Now().Before(subscriptionBindingCache.expiresAt) {
		names, set := subscriptionBindingCache.names, subscriptionBindingCache.set
		subscriptionBindingCache.RUnlock()
		return names, set, nil
	}
	generation := subscriptionBindingCache.generation
	subscriptionBindingCache.RUnlock()

	names, err := subscriptionGroupNamesTx(DB)
	if err != nil {
		return nil, nil, err
	}
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	subscriptionBindingCache.Lock()
	// A plan write that landed during the read must not be overwritten by the older snapshot.
	if subscriptionBindingCache.generation == generation {
		subscriptionBindingCache.names, subscriptionBindingCache.set = names, set
		subscriptionBindingCache.expiresAt = time.Now().Add(subscriptionPlanCacheTTL())
	}
	subscriptionBindingCache.Unlock()
	return names, set, nil
}

func invalidateSubscriptionBindingCache() {
	subscriptionBindingCache.Lock()
	subscriptionBindingCache.names = nil
	subscriptionBindingCache.set = nil
	subscriptionBindingCache.generation++
	subscriptionBindingCache.Unlock()
}
