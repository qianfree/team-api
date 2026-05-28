package billing

import (
	"testing"
	"time"
)

func TestSameDay(t *testing.T) {
	tests := []struct {
		name     string
		a, b     time.Time
		expected bool
	}{
		{
			"same moment",
			time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC),
			true,
		},
		{
			"same day different hour",
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 15, 23, 59, 59, 0, time.UTC),
			true,
		},
		{
			"next day",
			time.Date(2025, 3, 15, 23, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 16, 1, 0, 0, 0, time.UTC),
			false,
		},
		{
			"previous day",
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 14, 23, 0, 0, 0, time.UTC),
			false,
		},
		{
			"year boundary",
			time.Date(2024, 12, 31, 23, 0, 0, 0, time.UTC),
			time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
			false,
		},
		{
			"same day different timezone",
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 15, 8, 0, 0, 0, time.FixedZone("CST", 8*3600)),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameDay(tt.a, tt.b); got != tt.expected {
				t.Errorf("sameDay() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSameWeek(t *testing.T) {
	tests := []struct {
		name     string
		a, b     time.Time
		expected bool
	}{
		{
			"same week Monday and Sunday",
			time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),  // Monday
			time.Date(2025, 3, 16, 23, 0, 0, 0, time.UTC), // Sunday
			true,
		},
		{
			"same week Wednesday and Friday",
			time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC),
			true,
		},
		{
			"across week boundary",
			time.Date(2025, 3, 16, 23, 0, 0, 0, time.UTC), // Sunday
			time.Date(2025, 3, 17, 1, 0, 0, 0, time.UTC),  // Monday
			false,
		},
		{
			"two weeks apart",
			time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 24, 0, 0, 0, 0, time.UTC), // Next next Monday
			false,
		},
		{
			"year boundary week",
			// 2024-12-30 is Monday of ISO week 1 of 2025
			// 2025-01-05 is Sunday of ISO week 1 of 2025
			time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameWeek(tt.a, tt.b); got != tt.expected {
				t.Errorf("sameWeek() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSameMonth(t *testing.T) {
	tests := []struct {
		name     string
		a, b     time.Time
		expected bool
	}{
		{
			"same month first and last day",
			time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 31, 23, 0, 0, 0, time.UTC),
			true,
		},
		{
			"across month boundary",
			time.Date(2025, 3, 31, 23, 0, 0, 0, time.UTC),
			time.Date(2025, 4, 1, 1, 0, 0, 0, time.UTC),
			false,
		},
		{
			"same month different year",
			time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			false,
		},
		{
			"year boundary December to January",
			time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			false,
		},
		{
			"February same year",
			time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameMonth(tt.a, tt.b); got != tt.expected {
				t.Errorf("sameMonth() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNeedsReset(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		quotaType string
		period    string
		resetAt   time.Time
		expected  bool
	}{
		{
			"empty period no reset",
			"periodic", "", now,
			false,
		},
		{
			"day period same day no reset",
			"periodic", "day", now,
			false,
		},
		{
			"day period yesterday needs reset",
			"periodic", "day", now.AddDate(0, 0, -1),
			true,
		},
		{
			"week period same week no reset",
			"periodic", "week", now,
			false,
		},
		{
			"week period last week needs reset",
			"periodic", "week", now.AddDate(0, 0, -7),
			true,
		},
		{
			"month period same month no reset",
			"periodic", "month", now,
			false,
		},
		{
			"month period last month needs reset",
			"periodic", "month", now.AddDate(0, -1, 0),
			true,
		},
		{
			"unknown period no reset",
			"periodic", "quarter", now.AddDate(0, -3, 0),
			false,
		},
		{
			"day period distant past needs reset",
			"periodic", "day", now.AddDate(0, -6, 0),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &memberQuotaInfo{
				QuotaType:   tt.quotaType,
				QuotaPeriod: tt.period,
				QuotaResetAt: func() int64 {
					if tt.resetAt.IsZero() {
						return now.Unix()
					}
					return tt.resetAt.Unix()
				}(),
			}
			if got := needsReset(info); got != tt.expected {
				t.Errorf("needsReset() = %v, want %v (resetAt=%v, now=%v)",
					got, tt.expected, tt.resetAt.Format("2006-01-02"), now.Format("2006-01-02"))
			}
		})
	}
}

func TestNeedsReset_EpochZero(t *testing.T) {
	info := &memberQuotaInfo{
		QuotaType:    "periodic",
		QuotaPeriod:  "day",
		QuotaResetAt: 0, // never set
	}
	// epoch 0 (1970-01-01) is not same day as now → needs reset
	if !needsReset(info) {
		t.Error("needsReset with epoch 0 should return true")
	}
}

func TestMemberQuotaRedisKey(t *testing.T) {
	tests := []struct {
		tenantID int64
		userID   int64
		expected string
	}{
		{1, 100, "member_quota:1:100"},
		{999, 12345, "member_quota:999:12345"},
	}

	for _, tt := range tests {
		got := memberQuotaRedisKey(tt.tenantID, tt.userID)
		if got != tt.expected {
			t.Errorf("memberQuotaRedisKey(%d, %d) = %s, want %s",
				tt.tenantID, tt.userID, got, tt.expected)
		}
	}
}

func TestMemberQuotaConstants(t *testing.T) {
	if MemberQuotaRedisKeyPrefix != "member_quota:" {
		t.Errorf("MemberQuotaRedisKeyPrefix = %q, want %q", MemberQuotaRedisKeyPrefix, "member_quota:")
	}
	if MemberQuotaCacheTTL != 60 {
		t.Errorf("MemberQuotaCacheTTL = %d, want 60", MemberQuotaCacheTTL)
	}
}
