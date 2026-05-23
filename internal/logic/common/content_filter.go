package common

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudflare/ahocorasick"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
)

// FilterResult holds the result of a content filter check.
type FilterResult struct {
	Matched      bool
	MatchedWords []string
	FilteredText string
}

// filterSnapshot is an immutable snapshot of the filter engine state.
// Stored in atomic.Value for lock-free reads in the hot path.
type filterSnapshot struct {
	mode            string
	repl            string
	matcher         *ahocorasick.Matcher
	literalPatterns []string
	wildcardInners  []string // precomputed inner literals from wildcard patterns
}

// ContentFilterEngine manages the Aho-Corasick automaton for multi-pattern matching.
type ContentFilterEngine struct {
	snapshot atomic.Value // stores *filterSnapshot
	mu       sync.Mutex   // protects Rebuild
}

var (
	contentFilterEngine     *ContentFilterEngine
	contentFilterEngineOnce sync.Once
)

// ContentFilter returns the global ContentFilterEngine singleton.
func ContentFilter() *ContentFilterEngine {
	contentFilterEngineOnce.Do(func() {
		eng := &ContentFilterEngine{}
		eng.snapshot.Store(&filterSnapshot{mode: "off", repl: "***"})
		contentFilterEngine = eng
	})
	return contentFilterEngine
}

// InitContentFilter initializes the content filter engine and starts the
// Redis Pub/Sub subscriber for auto-rebuild on settings changes.
func InitContentFilter(ctx context.Context) {
	engine := ContentFilter()
	engine.Rebuild(ctx)
	go engine.startSubscriber(ctx)
	g.Log().Info(ctx, "content filter engine initialized")
}

// Rebuild reads the current configuration and rebuilds the Aho-Corasick matcher.
func (e *ContentFilterEngine) Rebuild(ctx context.Context) {
	cfg := Config()

	mode := cfg.GetString(ctx, "content_filter_mode")
	repl := cfg.GetString(ctx, "content_filter_replacement")
	if repl == "" {
		repl = "***"
	}

	var words []string
	if err := cfg.GetOptionJSON(ctx, "content_filter_words", &words); err != nil {
		g.Log().Warningf(ctx, "[ContentFilter] failed to parse content_filter_words: %v", err)
		words = nil
	}

	var literalPatterns []string
	var wildcardInners []string
	for _, w := range words {
		if w == "" {
			continue
		}
		if strings.Contains(w, "*") {
			inner := strings.TrimLeft(strings.TrimRight(w, "*"), "*")
			if inner != "" {
				wildcardInners = append(wildcardInners, inner)
			}
			continue
		}
		literalPatterns = append(literalPatterns, w)
	}

	var matcher *ahocorasick.Matcher
	if len(literalPatterns) > 0 {
		matcher = ahocorasick.NewStringMatcher(literalPatterns)
	}

	e.mu.Lock()
	e.snapshot.Store(&filterSnapshot{
		mode:            mode,
		repl:            repl,
		matcher:         matcher,
		literalPatterns: literalPatterns,
		wildcardInners:  wildcardInners,
	})
	e.mu.Unlock()

	g.Log().Infof(ctx, "[ContentFilter] rebuilt: mode=%s, words=%d, literals=%d, wildcards=%d", mode, len(words), len(literalPatterns), len(wildcardInners))
}

// Check scans the given body for sensitive words and returns a FilterResult.
// Accepts []byte to avoid string conversion on the hot path.
func (e *ContentFilterEngine) Check(body []byte) *FilterResult {
	snap := e.snapshot.Load().(*filterSnapshot)

	if snap.mode == "off" || len(body) == 0 {
		return &FilterResult{FilteredText: string(body)}
	}

	matchedSet := make(map[string]bool)

	// Phase 1: Aho-Corasick multi-pattern matching for literal words.
	if snap.matcher != nil {
		hits := snap.matcher.MatchThreadSafe(body)
		for _, idx := range hits {
			if idx >= 0 && idx < len(snap.literalPatterns) {
				matchedSet[snap.literalPatterns[idx]] = true
			}
		}
	}

	// Phase 2: Wildcard pattern matching via bytes.Contains.
	// For non-replace modes, skip if already matched; early-exit on first hit.
	if snap.mode != "replace" {
		if len(matchedSet) > 0 {
			// Already have a match, no need for wildcard phase
		} else {
			for _, inner := range snap.wildcardInners {
				if bytes.Contains(body, []byte(inner)) {
					matchedSet[inner] = true
					break // one hit is enough for log/block
				}
			}
		}
	} else {
		// Replace mode: must find ALL matches for complete replacement
		for _, inner := range snap.wildcardInners {
			if bytes.Contains(body, []byte(inner)) {
				matchedSet[inner] = true
			}
		}
	}

	result := &FilterResult{FilteredText: string(body)}
	if len(matchedSet) == 0 {
		return result
	}

	result.Matched = true
	for w := range matchedSet {
		result.MatchedWords = append(result.MatchedWords, w)
	}

	// Build filtered text (only needed for replace mode, but compute for completeness)
	filtered := string(body)
	for w := range matchedSet {
		filtered = strings.ReplaceAll(filtered, w, snap.repl)
	}
	result.FilteredText = filtered

	return result
}

// GetMode returns the current filter mode (lock-free via atomic).
func (e *ContentFilterEngine) GetMode() string {
	return e.snapshot.Load().(*filterSnapshot).mode
}

// startSubscriber subscribes to Redis Pub/Sub "settings:changed" channel
// and rebuilds the matcher when content filter settings change.
func (e *ContentFilterEngine) startSubscriber(ctx context.Context) {
	for {
		conn, _, err := g.Redis().Subscribe(ctx, "settings:changed")
		if err != nil {
			g.Log().Errorf(ctx, "[ContentFilter] subscriber connect failed: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		g.Log().Info(ctx, "[ContentFilter] Pub/Sub subscriber started")

		for {
			v, err := conn.Receive(ctx)
			if err != nil {
				g.Log().Warningf(ctx, "[ContentFilter] subscriber recv error: %v", err)
				time.Sleep(5 * time.Second)
				break // reconnect
			}

			msg, ok := v.Val().(*gredis.Message)
			if !ok {
				continue // skip Subscription/Pong etc.
			}

			key := msg.Payload
			switch key {
			case "content_filter_words", "content_filter_mode", "content_filter_replacement":
				e.Rebuild(ctx)
				g.Log().Infof(ctx, "[ContentFilter] rebuilt via Pub/Sub for key: %s", key)
			}
		}

		conn.Close(ctx)
	}
}
