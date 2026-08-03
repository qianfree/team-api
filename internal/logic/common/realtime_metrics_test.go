package common

import "testing"

// 滑动窗口聚合的纯函数单测：桶从旧到新排列，最旧桶按 (bucketSeconds-elapsed)/bucketSeconds 加权。
func TestSlidingWindowSum(t *testing.T) {
	tests := []struct {
		name    string
		buckets []int64
		elapsed int64
		want    int64
	}{
		{
			name:    "空桶返回0",
			buckets: nil,
			elapsed: 5,
			want:    0,
		},
		{
			name:    "全零桶",
			buckets: []int64{0, 0, 0, 0, 0, 0, 0},
			elapsed: 3,
			want:    0,
		},
		{
			// elapsed=0：当前桶刚开始，最旧桶完整落在窗口内（权重 1），7 桶全额相加
			name:    "桶起点最旧桶全额计入",
			buckets: []int64{10, 10, 10, 10, 10, 10, 10},
			elapsed: 0,
			want:    70,
		},
		{
			// elapsed=5：最旧桶权重 (10-5)/10 = 0.5 → 10*0.5 + 60 = 65
			name:    "桶中点最旧桶半权重",
			buckets: []int64{10, 10, 10, 10, 10, 10, 10},
			elapsed: 5,
			want:    65,
		},
		{
			// elapsed=9：最旧桶权重 0.1 → 10*0.1 + 60 = 61
			name:    "桶末尾最旧桶接近零权重",
			buckets: []int64{10, 10, 10, 10, 10, 10, 10},
			elapsed: 9,
			want:    61,
		},
		{
			// 只有最旧桶有值：elapsed=5 → 100*0.5 = 50
			name:    "仅最旧桶有计数",
			buckets: []int64{100, 0, 0, 0, 0, 0, 0},
			elapsed: 5,
			want:    50,
		},
		{
			// 只有当前桶有值：不加权，全额计入
			name:    "仅当前桶有计数",
			buckets: []int64{0, 0, 0, 0, 0, 0, 100},
			elapsed: 5,
			want:    100,
		},
		{
			// 加权结果四舍五入：7*0.3 = 2.1 → 2 + 5 = 7
			name:    "加权结果四舍五入",
			buckets: []int64{7, 5},
			elapsed: 7,
			want:    7,
		},
		{
			// 异常 elapsed 超过桶宽：权重钳制为 0，最旧桶不计入
			name:    "elapsed超界权重钳制为零",
			buckets: []int64{100, 10},
			elapsed: 15,
			want:    10,
		},
		{
			// 异常负 elapsed：权重钳制为 1，最旧桶全额计入
			name:    "负elapsed权重钳制为一",
			buckets: []int64{100, 10},
			elapsed: -3,
			want:    110,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slidingWindowSum(tt.buckets, tt.elapsed); got != tt.want {
				t.Errorf("slidingWindowSum(%v, %d) = %d, want %d", tt.buckets, tt.elapsed, got, tt.want)
			}
		})
	}
}

// 桶 key 生成格式校验
func TestRtMetricsBucketKey(t *testing.T) {
	if got := rtMetricsBucketKey("g", 1700000000); got != "rt_metrics:g:1700000000" {
		t.Errorf("global bucket key = %q", got)
	}
	if got := rtMetricsBucketKey(rtMetricsTenantScope(42), 1700000000); got != "rt_metrics:t:42:1700000000" {
		t.Errorf("tenant bucket key = %q", got)
	}
}
