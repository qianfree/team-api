package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// excludeCacheFields are request body fields that should be excluded from cache hash
// because they vary between requests but don't affect the model's output.
var excludeCacheFields = map[string]bool{
	"stream":                true,
	"seed":                  true,
	"user":                  true,
	"request_id":            true,
	"top_logprobs":          true,
	"logprobs":              true,
	"n":                     true,
	"frequency_penalty":     false, // include - affects output
	"presence_penalty":      false, // include - affects output
	"temperature":           false, // include - affects output
	"max_tokens":            false, // include - affects output
	"max_completion_tokens": false,
}

// ComputeCacheHash produces a deterministic SHA-256 hash from a normalized request body.
// It removes non-deterministic fields (stream, seed, user, request_id, top_logprobs, logprobs, n)
// and sorts JSON keys for consistent hashing.
func ComputeCacheHash(body []byte, modelName string) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		// Fallback: hash raw body + model name
		h := sha256.New()
		h.Write([]byte(modelName))
		h.Write(body)
		return hex.EncodeToString(h.Sum(nil))
	}

	// Remove excluded fields
	for field := range excludeCacheFields {
		delete(raw, field)
	}

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Rebuild JSON with sorted keys
	var buf strings.Builder
	for _, k := range keys {
		if buf.Len() > 0 {
			buf.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(k)
		buf.Write(keyBytes)
		buf.WriteByte(':')
		buf.Write(raw[k])
	}

	h := sha256.New()
	h.Write([]byte(modelName))
	h.Write([]byte(buf.String()))
	return hex.EncodeToString(h.Sum(nil))
}
