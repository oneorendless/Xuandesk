package ssh

import (
	"fmt"
	"sync"
	"time"

	"github.com/xuandesk/xuandesk/ssh/cloud/client"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// CloudService 浜戠鏈嶅姟锛圵ails 鏆撮湶缁欏墠绔級
type CloudService struct {
	mu      sync.RWMutex
	app     *application.App
	client  *client.CloudClient
	config  *ConfigService
	running bool
	stopCh  chan struct{}
}

// NewCloudService 鍒涘缓浜戠鏈嶅姟
func NewCloudService(configService *ConfigService) *CloudService {
	return &CloudService{
		config: configService,
	}
}

// SetApp 璁剧疆搴旂敤瀹炰緥
func (cs *CloudService) SetApp(app *application.App) {
	cs.app = app
}

// Connect 杩炴帴鍒颁簯绔紙涓嶈繑鍥?error锛岄伩鍏?Wails 鎵撳嵃 ERR 鏃ュ織锛?func (cs *CloudService) Connect(serverAddr, token string) bool {
	fmt.Printf("[CloudService] 杩炴帴浜戠: %s\n", serverAddr)

	// 鏂紑鏃ц繛鎺?	cs.Disconnect()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	c := client.NewCloudClient(serverAddr, token)
	fmt.Println("[CloudService] 姝ｅ湪寤虹珛 TLS 杩炴帴...")
	if err := c.Connect(); err != nil {
		fmt.Printf("[CloudService] 鉁?杩炴帴澶辫触: %v\n", err)
		cs.emitStatus(false, err.Error())
		return false
	}
	fmt.Println("[CloudService] 鉁?TLS 杩炴帴鎴愬姛")

	// 娉ㄥ唽璁惧
	fmt.Println("[CloudService] 姝ｅ湪娉ㄥ唽璁惧...")
	if err := c.Register("鍚疭SH瀹㈡埛绔?, "localhost", "windows", "0.3.0"); err != nil {
		fmt.Printf("[CloudService] 鉁?娉ㄥ唽澶辫触: %v\n", err)
		c.Disconnect()
		cs.emitStatus(false, err.Error())
		return false
	}
	fmt.Println("[CloudService] 鉁?璁惧娉ㄥ唽鎴愬姛")

	cs.client = c
	cs.running = true

	// 鍚姩蹇冭烦
	cs.stopCh = make(chan struct{})
	go cs.heartbeatLoop()

	cs.emitStatus(true, "")
	fmt.Println("[CloudService] 鉁?宸茶繛鎺ュ埌浜戠")
	return true
}

// emitStatus 鍙戦€佽繛鎺ョ姸鎬佷簨浠剁粰鍓嶇
func (cs *CloudService) emitStatus(connected bool, errMsg string) {
	if cs.app != nil {
		cs.app.Event.Emit("cloud:status", map[string]interface{}{
			"connected": connected,
			"error":     errMsg,
		})
	}
}

// Disconnect 鏂紑杩炴帴
func (cs *CloudService) Disconnect() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.stopCh != nil {
		close(cs.stopCh)
		cs.stopCh = nil
	}
	if cs.client != nil {
		cs.client.Disconnect()
		cs.client = nil
		cs.running = false
		fmt.Println("[CloudService] 宸叉柇寮€浜戠杩炴帴")

		// 閫氱煡鍓嶇
		if cs.app != nil {
			cs.app.Event.Emit("cloud:status", map[string]interface{}{
				"connected": false,
			})
		}
	}
}

// IsConnected 妫€鏌ユ槸鍚﹁繛鎺?func (cs *CloudService) IsConnected() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	connected := cs.client != nil && cs.client.IsConnected()
	return connected
}

// PullSync 鎷夊彇鍚屾鏁版嵁
func (cs *CloudService) PullSync() ([]client.SyncConnection, error) {
	cs.mu.RLock()
	c := cs.client
	cs.mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return nil, fmt.Errorf("鏈繛鎺ュ埌浜戠")
	}

	fmt.Println("[CloudService] 鎷夊彇鍚屾鏁版嵁...")
	data, err := c.PullSync()
	if err != nil {
		fmt.Printf("[CloudService] 鉁?鎷夊彇澶辫触: %v\n", err)
		return nil, err
	}

	if data == nil {
		return []client.SyncConnection{}, nil
	}
	fmt.Printf("[CloudService] 鉁?鎷夊彇鎴愬姛: %d 涓繛鎺n", len(data.Connections))
	return data.Connections, nil
}

// PushSync 鎺ㄩ€佸悓姝ユ暟鎹?func (cs *CloudService) PushSync(connections []client.SyncConnection) error {
	cs.mu.RLock()
	c := cs.client
	cs.mu.RUnlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("鏈繛鎺ュ埌浜戠")
	}

	fmt.Printf("[CloudService] 鎺ㄩ€佸悓姝ユ暟鎹? %d 涓繛鎺n", len(connections))
	err := c.PushSync(client.SyncData{
		Connections: connections,
		UpdatedAt:   time.Now(),
	})
	if err != nil {
		fmt.Printf("[CloudService] 鉁?鎺ㄩ€佸け璐? %v\n", err)
		return err
	}
	fmt.Println("[CloudService] 鉁?鎺ㄩ€佹垚鍔?)
	return nil
}

// heartbeatLoop 蹇冭烦寰幆
func (cs *CloudService) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cs.stopCh:
			return
		case <-ticker.C:
			cs.mu.RLock()
			c := cs.client
			cs.mu.RUnlock()

			if c != nil && c.IsConnected() {
				if err := c.Heartbeat(); err != nil {
					fmt.Printf("[CloudService] 蹇冭烦澶辫触: %v\n", err)
				}
			}
		}
	}
}

