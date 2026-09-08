package ssh

import (
	"sync"
	"time"
)

// MonitorSnapshot 鐩戞帶蹇収
type MonitorSnapshot struct {
	Timestamp   int64   `json:"timestamp"`
	CPUPercent  float64 `json:"cpuPercent"`
	MemPercent  float64 `json:"memPercent"`
	MemUsed     uint64  `json:"memUsed"`
	MemTotal    uint64  `json:"memTotal"`
	DiskPercent float64 `json:"diskPercent"`
	DiskUsed    uint64  `json:"diskUsed"`
	DiskTotal   uint64  `json:"diskTotal"`
	NetRx       uint64  `json:"netRx"`
	NetTx       uint64  `json:"netTx"`
}

// MonitorHistory 鐩戞帶鍘嗗彶鏁版嵁
type MonitorHistory struct {
	mu       sync.RWMutex
	data     map[string][]MonitorSnapshot // connID 鈫?snapshots
	maxAge   time.Duration
	maxItems int
}

// NewMonitorHistory 鍒涘缓鐩戞帶鍘嗗彶
func NewMonitorHistory() *MonitorHistory {
	mh := &MonitorHistory{
		data:     make(map[string][]MonitorSnapshot),
		maxAge:   1 * time.Hour,
		maxItems: 360, // 姣?10 绉掍竴涓偣锛? 灏忔椂 = 360 涓偣
	}

	// 鍚姩娓呯悊瀹氭椂鍣?	go mh.cleanupLoop()

	return mh
}

// Add 娣诲姞鐩戞帶蹇収
func (mh *MonitorHistory) Add(connID string, snapshot MonitorSnapshot) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	snapshots := mh.data[connID]
	snapshots = append(snapshots, snapshot)

	// 闄愬埗鏁伴噺
	if len(snapshots) > mh.maxItems {
		snapshots = snapshots[len(snapshots)-mh.maxItems:]
	}

	mh.data[connID] = snapshots
}

// Get 鑾峰彇鏈€杩?N 涓揩鐓?func (mh *MonitorHistory) Get(connID string, count int) []MonitorSnapshot {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	snapshots := mh.data[connID]
	if count <= 0 || count > len(snapshots) {
		count = len(snapshots)
	}

	// 杩斿洖鏈€杩?N 涓?	start := len(snapshots) - count
	if start < 0 {
		start = 0
	}

	result := make([]MonitorSnapshot, count)
	copy(result, snapshots[start:])
	return result
}

// GetSince 鑾峰彇鎸囧畾鏃堕棿浠ユ潵鐨勫揩鐓?func (mh *MonitorHistory) GetSince(connID string, since int64) []MonitorSnapshot {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	snapshots := mh.data[connID]
	var result []MonitorSnapshot
	for _, s := range snapshots {
		if s.Timestamp >= since {
			result = append(result, s)
		}
	}
	return result
}

// Clear 娓呴櫎鎸囧畾杩炴帴鐨勫巻鍙叉暟鎹?func (mh *MonitorHistory) Clear(connID string) {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	delete(mh.data, connID)
}

// cleanupLoop 瀹氭湡娓呯悊杩囨湡鏁版嵁
func (mh *MonitorHistory) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mh.cleanup()
	}
}

// cleanup 娓呯悊杩囨湡鏁版嵁
func (mh *MonitorHistory) cleanup() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	cutoff := time.Now().Add(-mh.maxAge).UnixMilli()
	for connID, snapshots := range mh.data {
		valid := 0
		for _, s := range snapshots {
			if s.Timestamp >= cutoff {
				snapshots[valid] = s
				valid++
			}
		}
		if valid == 0 {
			delete(mh.data, connID)
		} else {
			mh.data[connID] = snapshots[:valid]
		}
	}
}

