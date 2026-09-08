package ssh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEntry 瀹¤鏃ュ織鏉＄洰
type AuditEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	ConnID    string `json:"connId"`
	Host      string `json:"host"`
	Username  string `json:"username"`
	Action    string `json:"action"`    // command, upload, download, delete, rename, connect, disconnect
	Detail    string `json:"detail"`    // 鍏蜂綋鍐呭
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// AuditLogger 鎿嶄綔瀹¤鏃ュ織
type AuditLogger struct {
	mu       sync.Mutex
	baseDir  string
	buf      []AuditEntry
	flushAt  int
}

// NewAuditLogger 鍒涘缓瀹¤鏃ュ織鍣?func NewAuditLogger() *AuditLogger {
	baseDir := filepath.Join(GetDataDir(), "audit")
	os.MkdirAll(baseDir, 0755)
	return &AuditLogger{
		baseDir: baseDir,
		flushAt: 10,
	}
}

// Log 璁板綍涓€鏉″璁℃棩蹇?func (al *AuditLogger) Log(connID, host, username, action, detail string, success bool, errMsg string) {
	entry := AuditEntry{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		ConnID:    connID,
		Host:      host,
		Username:  username,
		Action:    action,
		Detail:    detail,
		Success:   success,
		Error:     errMsg,
	}

	al.mu.Lock()
	al.buf = append(al.buf, entry)
	if len(al.buf) >= al.flushAt {
		al.flush()
	}
	al.mu.Unlock()
}

// GetLogs 鑾峰彇瀹¤鏃ュ織锛堟渶杩?N 鏉★級
func (al *AuditLogger) GetLogs(limit int) []AuditEntry {
	al.mu.Lock()
	defer al.mu.Unlock()

	// 鍏堝埛鏂扮紦鍐插尯
	al.flush()

	// 璇诲彇浠婂ぉ鐨勬棩蹇楁枃浠?	logFile := al.getLogFile(time.Now())
	data, err := os.ReadFile(logFile)
	if err != nil {
		return []AuditEntry{}
	}

	var entries []AuditEntry
	json.Unmarshal(data, &entries)

	// 杩斿洖鏈€杩?N 鏉?	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries
}

// GetLogsByDate 鎸夋棩鏈熻幏鍙栧璁℃棩蹇?func (al *AuditLogger) GetLogsByDate(date string) []AuditEntry {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.flush()

	logFile := filepath.Join(al.baseDir, date+".json")
	data, err := os.ReadFile(logFile)
	if err != nil {
		return []AuditEntry{}
	}

	var entries []AuditEntry
	json.Unmarshal(data, &entries)
	return entries
}

// ExportLogs 瀵煎嚭瀹¤鏃ュ織涓?JSON 瀛楃涓?func (al *AuditLogger) ExportLogs(startDate, endDate string) (string, error) {
	var allEntries []AuditEntry

	// 閬嶅巻鏃ユ湡鑼冨洿鍐呯殑鏃ュ織鏂囦欢
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return "", fmt.Errorf("鏃犳晥鐨勫紑濮嬫棩鏈? %v", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", fmt.Errorf("鏃犳晥鐨勭粨鏉熸棩鏈? %v", err)
	}

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		entries := al.GetLogsByDate(d.Format("2006-01-02"))
		allEntries = append(allEntries, entries...)
	}

	data, err := json.MarshalIndent(allEntries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("搴忓垪鍖栧け璐? %v", err)
	}

	return string(data), nil
}

// flush 灏嗙紦鍐插啓鍏ユ枃浠?func (al *AuditLogger) flush() {
	if len(al.buf) == 0 {
		return
	}

	logFile := al.getLogFile(time.Now())

	// 璇诲彇宸叉湁鏁版嵁
	var existing []AuditEntry
	if data, err := os.ReadFile(logFile); err == nil {
		json.Unmarshal(data, &existing)
	}

	// 鍚堝苟
	existing = append(existing, al.buf...)

	// 鍐欏叆
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		al.buf = nil
		return
	}

	os.WriteFile(logFile, data, 0644)
	al.buf = nil
}

// getLogFile 鑾峰彇鏃ュ織鏂囦欢璺緞
func (al *AuditLogger) getLogFile(t time.Time) string {
	return filepath.Join(al.baseDir, t.Format("2006-01-02")+".json")
}

