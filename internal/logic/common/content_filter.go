package common

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/ahocorasick"
	"github.com/gogf/gf/v2/frame/g"
)

// FilterResult holds the result of a content filter check.
type FilterResult struct {
	Matched      bool
	MatchedWords []string
	FilteredText string
}

// ContentFilterEngine manages the Aho-Corasick automaton for multi-pattern matching.
// It is safe for concurrent use via sync.RWMutex.
type ContentFilterEngine struct {
	mu              sync.RWMutex
	matcher         *ahocorasick.Matcher
	literalPatterns []string // patterns fed into the AC matcher (no wildcards)
	words           []string // original word list (including wildcard patterns)
	mode            string   // off/log/replace/block
	repl            string   // replacement string for "replace" mode
}

var (
	contentFilterEngine     *ContentFilterEngine
	contentFilterEngineOnce sync.Once
)

// ContentFilter returns the global ContentFilterEngine singleton.
func ContentFilter() *ContentFilterEngine {
	contentFilterEngineOnce.Do(func() {
		contentFilterEngine = &ContentFilterEngine{
			mode: "off",
			repl: "***",
		}
	})
	return contentFilterEngine
}

// InitContentFilter initializes the content filter engine and starts the
// Redis Pub/Sub subscriber for auto-rebuild on settings changes.
// Call this once at startup.
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

	var rawWords []string
	if err := cfg.GetOptionJSON(ctx, "content_filter_words", &rawWords); err != nil {
		g.Log().Warningf(ctx, "[ContentFilter] failed to parse content_filter_words: %v", err)
		rawWords = nil
	}

	// Wildcard patterns like "*赌博*" cannot be matched directly by AC,
	// so we separate them and handle via strings.Contains in Check().
	var literalPatterns []string
	for _, w := range rawWords {
		if w == "" {
			continue
		}
		if strings.Contains(w, "*") {
			continue
		}
		literalPatterns = append(literalPatterns, w)
	}

	var matcher *ahocorasick.Matcher
	if len(literalPatterns) > 0 {
		matcher = ahocorasick.NewStringMatcher(literalPatterns)
	}

	e.mu.Lock()
	e.matcher = matcher
	e.literalPatterns = literalPatterns
	e.words = rawWords
	e.mode = mode
	e.repl = repl
	e.mu.Unlock()

	g.Log().Infof(ctx, "[ContentFilter] rebuilt: mode=%s, words=%d, literals=%d", mode, len(rawWords), len(literalPatterns))
}

// Check scans the given text for sensitive words and returns a FilterResult.
func (e *ContentFilterEngine) Check(text string) *FilterResult {
	e.mu.RLock()
	mode := e.mode
	matcher := e.matcher
	literalPatterns := e.literalPatterns
	words := e.words
	repl := e.repl
	e.mu.RUnlock()

	if mode == "off" || text == "" {
		return &FilterResult{
			Matched:      false,
			MatchedWords: nil,
			FilteredText: text,
		}
	}

	result := &FilterResult{
		Matched:      false,
		MatchedWords: nil,
		FilteredText: text,
	}

	matchedSet := make(map[string]bool)

	// Phase 1: Aho-Corasick multi-pattern matching for literal words.
	// MatchThreadSafe returns indices into the original dictionary (literalPatterns).
	if matcher != nil {
		hits := matcher.MatchThreadSafe([]byte(text))
		for _, idx := range hits {
			if idx >= 0 && idx < len(literalPatterns) {
				matchedSet[literalPatterns[idx]] = true
			}
		}
	}

	// Phase 2: Wildcard pattern matching via strings.Contains.
	// Patterns like "*赌博*" mean "contains 赌博".
	for _, w := range words {
		if !strings.Contains(w, "*") {
			continue
		}
		// Strip leading/trailing wildcards to get the inner literal.
		inner := strings.TrimLeft(strings.TrimRight(w, "*"), "*")
		if inner != "" && strings.Contains(text, inner) {
			matchedSet[inner] = true
		}
	}

	if len(matchedSet) == 0 {
		return result
	}

	result.Matched = true
	for w := range matchedSet {
		result.MatchedWords = append(result.MatchedWords, w)
	}

	// Build filtered text by replacing all matched words.
	filtered := text
	for w := range matchedSet {
		filtered = strings.ReplaceAll(filtered, w, repl)
	}
	result.FilteredText = filtered

	return result
}

// GetMode returns the current filter mode thread-safely.
func (e *ContentFilterEngine) GetMode() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mode
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
			msg, err := conn.ReceiveMessage(ctx)
			if err != nil {
				g.Log().Warningf(ctx, "[ContentFilter] subscriber recv error: %v", err)
				time.Sleep(5 * time.Second)
				break // reconnect
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
