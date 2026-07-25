package relay

import "testing"

func TestAffinityIdentity(t *testing.T) {
	key, seed := affinityIdentity(1, 2, 3, "gpt-4o")
	sameKey, sameSeed := affinityIdentity(1, 2, 3, "gpt-4o")
	if key != sameKey || seed != sameSeed {
		t.Fatal("affinity identity is not deterministic")
	}

	tests := []struct {
		name      string
		tenantID  int64
		userID    int64
		apiKeyID  int64
		modelName string
	}{
		{name: "tenant", tenantID: 9, userID: 2, apiKeyID: 3, modelName: "gpt-4o"},
		{name: "user", tenantID: 1, userID: 9, apiKeyID: 3, modelName: "gpt-4o"},
		{name: "api key", tenantID: 1, userID: 2, apiKeyID: 9, modelName: "gpt-4o"},
		{name: "model", tenantID: 1, userID: 2, apiKeyID: 3, modelName: "claude-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otherKey, _ := affinityIdentity(tt.tenantID, tt.userID, tt.apiKeyID, tt.modelName)
			if otherKey == key {
				t.Fatal("distinct affinity identity produced the same key")
			}
		})
	}
}
