package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message 鍗忚娑堟伅
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// SyncConnection 鍚屾鐨勮繛鎺ラ厤缃?type SyncConnection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password,omitempty"`
	KeyPath   string    `json:"keyPath,omitempty"`
	Source    string    `json:"source,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// SyncData 鍚屾鏁版嵁
type SyncData struct {
	Connections []SyncConnection `json:"connections"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	DeviceID    string           `json:"deviceId"`
}

// CloudClient 浜戠瀹㈡埛绔?type CloudClient struct {
	mu         sync.RWMutex
	serverAddr string
	token      string
	deviceID   string
	conn       *websocket.Conn
	connected  bool
}

// NewCloudClient 鍒涘缓浜戠瀹㈡埛绔?func NewCloudClient(serverAddr, token string) *CloudClient {
	return &CloudClient{
		serverAddr: serverAddr,
		token:      token,
	}
}

// Connect 杩炴帴鍒颁簯绔湇鍔″櫒
func (cc *CloudClient) Connect() error {
	u := url.URL{
		Scheme: "wss",
		Host:   cc.serverAddr,
		Path:   "/ws",
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("WebSocket 杩炴帴澶辫触: %v", err)
	}

	cc.mu.Lock()
	cc.conn = conn
	cc.connected = true
	cc.mu.Unlock()

	return nil
}

// Disconnect 鏂紑杩炴帴
func (cc *CloudClient) Disconnect() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.conn != nil {
		cc.conn.Close()
		cc.conn = nil
	}
	cc.connected = false
}

// IsConnected 妫€鏌ユ槸鍚﹁繛鎺?func (cc *CloudClient) IsConnected() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.connected && cc.conn != nil
}

// send 鍙戦€佹秷鎭苟鎺ユ敹鍝嶅簲
func (cc *CloudClient) send(msg Message) (Message, error) {
	cc.mu.RLock()
	conn := cc.conn
	connected := cc.connected
	cc.mu.RUnlock()

	if conn == nil || !connected {
		return Message{}, fmt.Errorf("鏈繛鎺?)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return Message{}, fmt.Errorf("搴忓垪鍖栧け璐? %v", err)
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		cc.Disconnect()
		return Message{}, fmt.Errorf("鍙戦€佸け璐? %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		cc.Disconnect()
		return Message{}, fmt.Errorf("鎺ユ敹澶辫触: %v", err)
	}

	var resp Message
	if err := json.Unmarshal(respData, &resp); err != nil {
		return Message{}, fmt.Errorf("瑙ｆ瀽澶辫触: %v", err)
	}

	return resp, nil
}

// Register 娉ㄥ唽璁惧
func (cc *CloudClient) Register(deviceName, host, osInfo, version string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"name":      deviceName,
		"host":      host,
		"port":      0,
		"os":        osInfo,
		"version":   version,
		"token":     cc.token,
		"timestamp": time.Now().Format(time.RFC3339),
		"nonce":     generateNonce(),
	})

	resp, err := cc.send(Message{Type: "register", Payload: payload})
	if err != nil {
		return err
	}

	if resp.Type == "error" {
		return fmt.Errorf("娉ㄥ唽澶辫触: %s", string(resp.Payload))
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Payload, &result)
	if id, ok := result["id"].(string); ok {
		cc.deviceID = id
	}

	return nil
}

// Heartbeat 鍙戦€佸績璺?func (cc *CloudClient) Heartbeat() error {
	payload, _ := json.Marshal(map[string]string{"deviceId": cc.deviceID})
	_, err := cc.send(Message{Type: "heartbeat", Payload: payload})
	return err
}

// PullSync 鎷夊彇鍚屾鏁版嵁
func (cc *CloudClient) PullSync() (*SyncData, error) {
	resp, err := cc.send(Message{Type: "sync-pull", Payload: json.RawMessage("{}")})
	if err != nil {
		return nil, err
	}

	if resp.Type == "error" {
		return nil, fmt.Errorf("鎷夊彇澶辫触: %s", string(resp.Payload))
	}

	var data SyncData
	json.Unmarshal(resp.Payload, &data)
	return &data, nil
}

// PushSync 鎺ㄩ€佸悓姝ユ暟鎹?func (cc *CloudClient) PushSync(data SyncData) error {
	data.DeviceID = cc.deviceID
	payload, _ := json.Marshal(data)

	resp, err := cc.send(Message{Type: "sync-push", Payload: payload})
	if err != nil {
		return err
	}

	if resp.Type == "error" {
		return fmt.Errorf("鎺ㄩ€佸け璐? %s", string(resp.Payload))
	}

	return nil
}

func generateNonce() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (8 * (i % 8)))
	}
	return fmt.Sprintf("%x", b)
}

