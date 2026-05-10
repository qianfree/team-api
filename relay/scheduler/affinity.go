package scheduler

import (
	"sync"
	"time"
)

const (
	// AffinityTTL 亲和性缓存默认 TTL（秒）
	AffinityTTL = 1800
)

// affinityKey 亲和性缓存键
type affinityKey struct {
	TenantID  int64
	UserID    int64
	ModelName string
}

// AffinityStore 亲和性内存缓存
// 用户+模型 → 渠道ID 映射，避免频繁切换渠道导致上下文丢失
type AffinityStore struct {
	mu      sync.RWMutex
	entries map[affinityKey]*affinityEntry
}

type affinityEntry struct {
	ChannelID int64
	HitCount  int
	ExpiresAt time.Time
}

// NewAffinityStore 创建亲和性缓存
func NewAffinityStore() *AffinityStore {
	return &AffinityStore{
		entries: make(map[affinityKey]*affinityEntry),
	}
}

// Get 获取亲和性渠道
func (s *AffinityStore) Get(tenantID, userID int64, modelName string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := affinityKey{TenantID: tenantID, UserID: userID, ModelName: modelName}
	entry, ok := s.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return 0, false
	}
	return entry.ChannelID, true
}

// Set 设置亲和性渠道
func (s *AffinityStore) Set(tenantID, userID int64, modelName string, channelID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := affinityKey{TenantID: tenantID, UserID: userID, ModelName: modelName}
	if existing, ok := s.entries[key]; ok {
		existing.ChannelID = channelID
		existing.HitCount++
		existing.ExpiresAt = time.Now().Add(AffinityTTL * time.Second)
	} else {
		s.entries[key] = &affinityEntry{
			ChannelID: channelID,
			HitCount:  1,
			ExpiresAt: time.Now().Add(AffinityTTL * time.Second),
		}
	}
}

// Delete 删除亲和性记录
func (s *AffinityStore) Delete(tenantID, userID int64, modelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, affinityKey{TenantID: tenantID, UserID: userID, ModelName: modelName})
}

// DeleteByChannel 删除某渠道的所有亲和性记录
func (s *AffinityStore) DeleteByChannel(channelID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.entries {
		if entry.ChannelID == channelID {
			delete(s.entries, key)
		}
	}
}

// CleanExpired 清理过期记录
func (s *AffinityStore) CleanExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	count := 0
	for key, entry := range s.entries {
		if now.After(entry.ExpiresAt) {
			delete(s.entries, key)
			count++
		}
	}
	return count
}

// Size 返回缓存条目数
func (s *AffinityStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// 全局亲和性缓存实例
var globalAffinity = NewAffinityStore()

// GetGlobalAffinity 获取全局亲和性缓存
func GetGlobalAffinity() *AffinityStore {
	return globalAffinity
}
