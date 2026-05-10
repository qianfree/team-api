package monitor

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
)

// RedisPoolMetrics holds Redis connection and memory statistics.
type RedisPoolMetrics struct {
	ConnectedClients  int     `json:"connected_clients"`
	UsedMemoryMB      float64 `json:"used_memory_mb"`
	MaxMemoryMB       float64 `json:"max_memory_mb"`
	UsedMemoryPercent float64 `json:"used_memory_percent"`
	TotalCommands     int64   `json:"total_commands"`
	InstantaneousOps  int     `json:"instantaneous_ops"`
	KeyspaceHits      int64   `json:"keyspace_hits"`
	KeyspaceMisses    int64   `json:"keyspace_misses"`
	HitRate           float64 `json:"hit_rate"`
}

// GetRedisPoolMetrics returns current Redis statistics.
func GetRedisPoolMetrics(ctx context.Context) (*RedisPoolMetrics, error) {
	m := &RedisPoolMetrics{}

	// Get INFO all at once
	val, err := g.Redis().Do(ctx, "INFO", "all")
	if err != nil {
		g.Log().Warningf(ctx, "redis INFO failed: %v", err)
		return m, err
	}

	info := val.String()
	lines := strings.Split(info, "\n")
	kv := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Clients
	if v, ok := kv["connected_clients"]; ok {
		m.ConnectedClients = gconv.Int(v)
	}

	// Memory
	if v, ok := kv["used_memory"]; ok {
		bytes := gconv.Int64(v)
		m.UsedMemoryMB = float64(bytes) / 1024 / 1024
	}
	if v, ok := kv["maxmemory"]; ok {
		bytes := gconv.Int64(v)
		m.MaxMemoryMB = float64(bytes) / 1024 / 1024
	}
	if v, ok := kv["used_memory_percent"]; ok {
		m.UsedMemoryPercent = gconv.Float64(strings.TrimSuffix(v, "%"))
	} else if m.MaxMemoryMB > 0 {
		m.UsedMemoryPercent = (m.UsedMemoryMB / m.MaxMemoryMB) * 100
	}

	// Stats
	if v, ok := kv["total_commands_processed"]; ok {
		m.TotalCommands = gconv.Int64(v)
	}
	if v, ok := kv["instantaneous_ops_per_sec"]; ok {
		m.InstantaneousOps = gconv.Int(v)
	}

	// Keyspace
	if v, ok := kv["keyspace_hits"]; ok {
		m.KeyspaceHits = gconv.Int64(v)
	}
	if v, ok := kv["keyspace_misses"]; ok {
		m.KeyspaceMisses = gconv.Int64(v)
	}
	total := m.KeyspaceHits + m.KeyspaceMisses
	if total > 0 {
		m.HitRate = float64(m.KeyspaceHits) / float64(total) * 100
	}

	return m, nil
}

// GetRedisUsedMemoryMB returns current Redis used memory in MB.
func GetRedisUsedMemoryMB(ctx context.Context) (float64, error) {
	val, err := g.Redis().Do(ctx, "INFO", "memory")
	if err != nil {
		return 0, err
	}
	info := val.String()
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "used_memory:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				bytes := gconv.Int64(strings.TrimSpace(parts[1]))
				return float64(bytes) / 1024 / 1024, nil
			}
		}
	}
	return 0, nil
}
