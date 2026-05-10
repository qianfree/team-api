package oauth

import (
	"sync"
	"time"
)

// OAuthSession 存储进行中的 OAuth 授权流程状态
type OAuthSession struct {
	Platform     string    `json:"platform"`        // "claude" | "openai" | "gemini"
	State        string    `json:"state"`           // CSRF 防护 state 参数
	CodeVerifier string    `json:"code_verifier"`   // PKCE code_verifier
	Scope        string    `json:"scope,omitempty"` // 授权范围
	RedirectURI  string    `json:"redirect_uri,omitempty"`
	Extra        string    `json:"extra,omitempty"` // 平台专属 JSON 数据
	CreatedAt    time.Time `json:"created_at"`
}

const sessionTTL = 30 * time.Minute

// SessionStore 管理内存中的 OAuth 会话
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*OAuthSession
	stopCh   chan struct{}
}

// GlobalSessionStore 全局单例，所有平台共用
var GlobalSessionStore = NewSessionStore()

// NewSessionStore 创建新的 SessionStore 并启动后台清理
func NewSessionStore() *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]*OAuthSession),
		stopCh:   make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// Set 存储一个 OAuth 会话
func (s *SessionStore) Set(sessionID string, session *OAuthSession) {
	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()
}

// Get 获取一个 OAuth 会话
func (s *SessionStore) Get(sessionID string) (*OAuthSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(sess.CreatedAt) > sessionTTL {
		return nil, false
	}
	return sess, true
}

// Delete 删除一个 OAuth 会话
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// Stop 停止后台清理协程
func (s *SessionStore) Stop() {
	close(s.stopCh)
}

func (s *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, sess := range s.sessions {
				if now.Sub(sess.CreatedAt) > sessionTTL {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}
