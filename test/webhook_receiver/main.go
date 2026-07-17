package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebhookEvent 接收到的 webhook 事件格式
type WebhookEvent struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	TenantID  int64                  `json:"tenant_id"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// 全局变量：webhook 密钥，用于验签
var secretKey string

func main() {
	// 从命令行参数或环境变量获取密钥
	secretKey = os.Getenv("WEBHOOK_SECRET")
	if len(os.Args) > 1 {
		secretKey = os.Args[1]
	}

	if secretKey == "" {
		log.Println("⚠️  未提供 webhook 密钥，签名验证将跳过")
		log.Println("   用法: go run main.go <secret_key>")
		log.Println("   或设置环境变量: WEBHOOK_SECRET=whk_xxx go run main.go")
		log.Println()
	} else {
		log.Printf("🔑 已加载密钥: %s...%s\n", secretKey[:8], secretKey[len(secretKey)-4:])
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/", handleNotFound)

	addr := ":" + port
	log.Printf("🚀 Webhook 接收服务启动在 http://localhost%s/webhook\n", addr)
	log.Printf("   健康检查: http://localhost%s/health\n", addr)
	log.Printf("   等待接收 webhook 事件...\n")
	log.Println(strings.Repeat("=", 70))

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   "not_found",
		"message": "请使用 POST /webhook 接收事件",
	})
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	now := time.Now()
	receiveTime := now.Format("2006-01-02 15:04:05.000")

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ [%s] 读取请求体失败: %v\n", receiveTime, err)
		http.Error(w, `{"error":"read_body_failed"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 提取关键 Header
	signature := r.Header.Get("X-Webhook-Signature")
	timestamp := r.Header.Get("X-Webhook-Timestamp")
	eventType := r.Header.Get("X-Webhook-Event")
	eventID := r.Header.Get("X-Webhook-ID")

	log.Printf("📩 [%s] 收到 Webhook 事件", receiveTime)
	log.Println(strings.Repeat("-", 50))
	log.Printf("   Event ID:    %s", eventID)
	log.Printf("   Event Type:  %s", eventType)
	log.Printf("   Timestamp:   %s", timestamp)

	// 签名验证
	if secretKey != "" {
		expectedSig := computeSignature(secretKey, timestamp, body)
		if hmac.Equal([]byte(signature), []byte(expectedSig)) {
			log.Printf("   签名验证:    ✅ 通过")
		} else {
			log.Printf("   签名验证:    ❌ 失败!")
			log.Printf("   期望签名:    %s", expectedSig)
			log.Printf("   实际签名:    %s", signature)
		}
	} else {
		log.Printf("   签名验证:    ⏭️  跳过（未配置密钥）")
	}

	// 解析 Payload
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("   JSON 解析:   ❌ 失败: %v", err)
		log.Printf("   原始 Body:   %s", string(body))
	} else {
		log.Printf("   Tenant ID:   %d", event.TenantID)
		log.Printf("   Event Time:  %s", event.Timestamp)

		// 格式化输出 Data
		if event.Data != nil {
			dataJSON, _ := json.MarshalIndent(event.Data, "               ", "  ")
			log.Printf("   Data:")
			log.Printf("               %s", string(dataJSON))
		}
	}

	// 原始 Body（美化的 JSON）
	var prettyBody map[string]interface{}
	if err := json.Unmarshal(body, &prettyBody); err == nil {
		beautified, _ := json.MarshalIndent(prettyBody, "   ", "  ")
		log.Printf("   完整 Payload:")
		log.Printf("   %s", string(beautified))
	} else {
		log.Printf("   原始 Body: %s", string(body))
	}

	log.Println(strings.Repeat("-", 50))
	log.Printf("   ✅ 已处理 (%d bytes)", len(body))
	log.Println(strings.Repeat("=", 70))

	// 返回 200 OK
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "received",
		"event_id": eventID,
	})
}

// computeSignature 计算 HMAC-SHA256 签名，与系统实现一致
// 签名内容: timestamp + "." + body
func computeSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}
