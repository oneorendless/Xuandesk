package ssh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ConnectionInfo SSH 杩炴帴淇℃伅銆?// 钀界洏鏃?Password / PrivateKey 濮嬬粓浠ュ姞瀵嗗舰寮忓瓨鍌ㄥ湪瀵瑰簲 Cipher 瀛楁涓€?type ConnectionInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"-"` // 鍐呭瓨鏄庢枃锛屾案涓嶅簭鍒楀寲
	KeyPath    string `json:"keyPath,omitempty"`
	PrivateKey string `json:"-"` // 鍐呭瓨鏄庢枃锛屾案涓嶅簭鍒楀寲

	PasswordCiphertext   string `json:"passwordCiphertext,omitempty"`
	PrivateKeyCiphertext string `json:"privateKeyCiphertext,omitempty"`
	CipherKind           string `json:"cipherKind,omitempty"`

	// 璺虫澘鏈洪厤缃?	JumpHost     string `json:"jumpHost,omitempty"`
	JumpUsername string `json:"jumpUsername,omitempty"`
	JumpPassword string `json:"-"` // 鍐呭瓨鏄庢枃锛屾案涓嶅簭鍒楀寲
	JumpKeyPath  string `json:"jumpKeyPath,omitempty"`

	JumpPasswordCiphertext string `json:"jumpPasswordCiphertext,omitempty"`

	Status  string `json:"status"`
	Saved   bool   `json:"saved"`
	GroupID string `json:"group_id,omitempty"`
}

// SecretCipher 鎶借薄鍔犲瘑鍣?type SecretCipher interface {
	Kind() string
	Encrypt(plain string) (string, error)
	Decrypt(cipher string) (string, error)
}

// StorageManager 杩炴帴瀛樺偍绠＄悊鍣紙缁熶竴鍗曟枃浠?+ 鍏ㄩ噺鍔犲瘑锛夈€?type StorageManager struct {
	mu          sync.RWMutex
	connections map[string]*ConnectionInfo
	dataFile    string
	cipher      SecretCipher
}

// NewStorageManager 鍒涘缓瀛樺偍绠＄悊鍣ㄣ€?// 鏁版嵁鏂囦欢璺緞: {GetDataDir()}/connections.json
func NewStorageManager() *StorageManager {
	dataDir := GetDataDir()
	dataFile := filepath.Join(dataDir, "connections.json")
	cipher := newPlatformCipher()

	sm := &StorageManager{
		connections: make(map[string]*ConnectionInfo),
		dataFile:    dataFile,
		cipher:      cipher,
	}

	sm.loadConnections()
	return sm
}

// loadConnections 浠庢枃浠跺姞杞借繛鎺ワ紙鑷姩瑙ｅ瘑锛夈€?func (sm *StorageManager) loadConnections() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 1. 灏濊瘯浠庝富鏂囦欢鍔犺浇
	data, err := os.ReadFile(sm.dataFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error("璇诲彇杩炴帴鏂囦欢澶辫触", "error", err)
		}
		// 2. 涓绘枃浠朵笉瀛樺湪锛屽皾璇曚粠鏃ф枃浠惰縼绉?		sm.migrateFromLegacy()
		return
	}

	var conns []*ConnectionInfo
	if err := json.Unmarshal(data, &conns); err != nil {
		log.Error("瑙ｆ瀽杩炴帴鏂囦欢澶辫触锛屽皾璇曟棫鏍煎紡杩佺Щ", "error", err)
		sm.migrateFromLegacy()
		return
	}

	// 妫€鏌ユ槸鍚︿负鏃ф牸寮忥紙ID 涓虹┖ = SavedConnection 鏍煎紡锛?	hasValidIDs := false
	for _, c := range conns {
		if c.ID != "" {
			hasValidIDs = true
			break
		}
	}

	if !hasValidIDs && len(conns) > 0 {
		// 鏃ф牸寮忥細SavedConnection 娌℃湁 ID 瀛楁锛岄渶瑕佽縼绉?		log.Info("妫€娴嬪埌鏃ф牸寮忚繛鎺ユ枃浠讹紝鎵ц杩佺Щ", "count", len(conns))
		for _, c := range conns {
			c.ID = generateConnectionID()
			c.Status = "disconnected"
			c.Saved = true
			sm.decryptInPlace(c)
			sm.connections[c.ID] = c
		}
		// 杩佺Щ鍚庡啓鍏ユ柊鏍煎紡
		sm.saveLocked()
		return
	}

	for _, c := range conns {
		c.Status = "disconnected"
		sm.decryptInPlace(c)
		sm.connections[c.ID] = c
	}
	log.Info("鍔犺浇杩炴帴鎴愬姛", "count", len(conns))
}

// migrateFromLegacy 浠庢棫鐨?cache/permanent 鍙屾枃浠舵牸寮忚縼绉汇€?func (sm *StorageManager) migrateFromLegacy() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)

	// 浼樺厛璇?permanent 鏂囦欢锛堝姞瀵嗙殑锛夛紝鍐嶈 cache 鏂囦欢锛堟槑鏂囩殑锛?	paths := []string{
		filepath.Join(exeDir, "data", "persistent", "connections.json"),
		filepath.Join(exeDir, "data", "cache", "connections.json"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		// 灏濊瘯瑙ｆ瀽涓?[]ConnectionInfo锛堟棫鏍煎紡锛?		var legacy []*ConnectionInfo
		if err := json.Unmarshal(data, &legacy); err != nil || len(legacy) == 0 {
			continue
		}

		// 杩佺Щ锛氳В瀵嗘槑鏂囧瓧娈碉紝鏍囪涓哄凡淇濆瓨
		for _, c := range legacy {
			if c.ID == "" {
				c.ID = generateConnectionID()
			}
			c.Status = "disconnected"
			c.Saved = true
			sm.decryptInPlace(c)
			sm.connections[c.ID] = c
		}

		log.Info("浠庢棫鏍煎紡杩佺Щ杩炴帴", "source", p, "count", len(legacy))
		// 杩佺Щ鍚庣珛鍗冲啓鍏ユ柊鏍煎紡
		sm.saveLocked()
		return
	}
}

// decryptInPlace 瑙ｅ瘑 Enc 瀛楁鍥炲～鍒板唴瀛樻槑鏂囧瓧娈点€?func (sm *StorageManager) decryptInPlace(c *ConnectionInfo) {
	if c.CipherKind == "" {
		return // 鏃ф槑鏂囨暟鎹紝鏃犲姞瀵嗗瓧娈?	}
	if c.PasswordCiphertext != "" {
		if plain, err := sm.cipher.Decrypt(c.PasswordCiphertext); err == nil {
			c.Password = plain
		} else {
			log.Error("瑙ｅ瘑瀵嗙爜澶辫触", "id", c.ID, "error", err)
		}
	}
	if c.PrivateKeyCiphertext != "" {
		if plain, err := sm.cipher.Decrypt(c.PrivateKeyCiphertext); err == nil {
			c.PrivateKey = plain
		} else {
			log.Error("瑙ｅ瘑绉侀挜澶辫触", "id", c.ID, "error", err)
		}
	}
	if c.JumpPasswordCiphertext != "" {
		if plain, err := sm.cipher.Decrypt(c.JumpPasswordCiphertext); err == nil {
			c.JumpPassword = plain
		} else {
			log.Error("瑙ｅ瘑璺虫澘鏈哄瘑鐮佸け璐?, "id", c.ID, "error", err)
		}
	}
}

// encryptForWrite 鍔犲瘑鍐呭瓨鏄庢枃瀛楁鍒?Cipher 瀛楁锛堢敤浜庡啓鐩橈級銆?func (sm *StorageManager) encryptForWrite(c *ConnectionInfo) *ConnectionInfo {
	cp := *c
	cp.Password = ""
	cp.PrivateKey = ""
	cp.JumpPassword = ""

	if c.Password != "" {
		if enc, err := sm.cipher.Encrypt(c.Password); err == nil {
			cp.PasswordCiphertext = enc
			cp.CipherKind = sm.cipher.Kind()
		} else {
			log.Error("鍔犲瘑瀵嗙爜澶辫触", "id", c.ID, "error", err)
		}
	}
	if c.PrivateKey != "" {
		if enc, err := sm.cipher.Encrypt(c.PrivateKey); err == nil {
			cp.PrivateKeyCiphertext = enc
			cp.CipherKind = sm.cipher.Kind()
		} else {
			log.Error("鍔犲瘑绉侀挜澶辫触", "id", c.ID, "error", err)
		}
	}
	if c.JumpPassword != "" {
		if enc, err := sm.cipher.Encrypt(c.JumpPassword); err == nil {
			cp.JumpPasswordCiphertext = enc
			cp.CipherKind = sm.cipher.Kind()
		} else {
			log.Error("鍔犲瘑璺虫澘鏈哄瘑鐮佸け璐?, "id", c.ID, "error", err)
		}
	}
	return &cp
}

// saveLocked 灏嗗綋鍓嶆墍鏈夎繛鎺ュ姞瀵嗗悗鍐欏叆鏂囦欢锛堣皟鐢ㄨ€呴渶鎸佹湁鍐欓攣锛夈€?func (sm *StorageManager) saveLocked() error {
	dir := filepath.Dir(sm.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("鍒涘缓鏁版嵁鐩綍澶辫触: %w", err)
	}

	var list []*ConnectionInfo
	for _, c := range sm.connections {
		if !c.Saved {
			continue // 鏈繚瀛樼殑涓存椂杩炴帴涓嶈惤鐩?		}
		list = append(list, sm.encryptForWrite(c))
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("搴忓垪鍖栬繛鎺ユ暟鎹け璐? %w", err)
	}

	if err := os.WriteFile(sm.dataFile, data, 0644); err != nil {
		return fmt.Errorf("鍐欏叆杩炴帴鏂囦欢澶辫触: %w", err)
	}
	return nil
}

// AddConnection 娣诲姞鏂拌繛鎺ワ紙榛樿鏈繚瀛橈級銆?func (sm *StorageManager) AddConnection(conn *ConnectionInfo) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if conn.ID == "" {
		return fmt.Errorf("杩炴帴 ID 涓嶈兘涓虹┖")
	}

	conn.Status = "disconnected"
	conn.Saved = false
	sm.connections[conn.ID] = conn
	return sm.saveLocked()
}

// UpdateConnection 鏇存柊杩炴帴淇℃伅銆?func (sm *StorageManager) UpdateConnection(conn *ConnectionInfo) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	existing, ok := sm.connections[conn.ID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", conn.ID)
	}

	conn.Status = existing.Status
	conn.Saved = existing.Saved
	sm.connections[conn.ID] = conn
	return sm.saveLocked()
}

// SyncImportConnection 浜戠鍚屾瀵煎叆锛堟寜 host:port:username 鍘婚噸锛夈€?func (sm *StorageManager) SyncImportConnection(conn *ConnectionInfo) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, existing := range sm.connections {
		if existing.Host == conn.Host && existing.Port == conn.Port && existing.Username == conn.Username {
			existing.Name = conn.Name
			if conn.Password != "" {
				existing.Password = conn.Password
			}
			if conn.PrivateKey != "" {
				existing.PrivateKey = conn.PrivateKey
			}
			existing.Saved = true
			return sm.saveLocked()
		}
	}

	conn.ID = generateConnectionID()
	conn.Status = "disconnected"
	conn.Saved = true
	sm.connections[conn.ID] = conn
	return sm.saveLocked()
}

// DeleteConnection 鍒犻櫎杩炴帴銆?func (sm *StorageManager) DeleteConnection(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.connections[id]; !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", id)
	}
	delete(sm.connections, id)
	return sm.saveLocked()
}

// DeleteFromCache 浠庣紦瀛樹腑绉婚櫎锛堟湭淇濆瓨鐨勮繛鎺ヤ粠鍐呭瓨鍒犻櫎锛屽凡淇濆瓨鐨勪粎閲嶇疆鐘舵€侊級銆?func (sm *StorageManager) DeleteFromCache(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	conn, ok := sm.connections[id]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", id)
	}

	if conn.Saved {
		conn.Status = "disconnected"
	} else {
		delete(sm.connections, id)
	}
	return sm.saveLocked()
}

// GetConnection 鑾峰彇鍗曚釜杩炴帴锛堣繑鍥炲壇鏈級銆?func (sm *StorageManager) GetConnection(id string) (*ConnectionInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	conn, ok := sm.connections[id]
	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", id)
	}
	cp := *conn
	return &cp, nil
}

// GetAllConnections 鑾峰彇鎵€鏈夎繛鎺ワ紙杩斿洖鍓湰鍒楄〃锛夈€?func (sm *StorageManager) GetAllConnections() []*ConnectionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*ConnectionInfo, 0, len(sm.connections))
	for _, c := range sm.connections {
		cp := *c
		result = append(result, &cp)
	}
	return result
}

// UpdateConnectionStatus 鏇存柊杩炴帴鐘舵€併€?func (sm *StorageManager) UpdateConnectionStatus(id string, status string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if c, ok := sm.connections[id]; ok {
		c.Status = status
	}
}

// MarkAsSaved 鏍囪涓哄凡淇濆瓨骞跺啓鐩樸€?func (sm *StorageManager) MarkAsSaved(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	c, ok := sm.connections[id]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", id)
	}
	c.Saved = true
	return sm.saveLocked()
}

// MarkAsUnsaved 鏍囪涓烘湭淇濆瓨銆?func (sm *StorageManager) MarkAsUnsaved(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if c, ok := sm.connections[id]; ok {
		c.Saved = false
	}
}

// SaveToPermanent 淇濇寔鍚戝悗鍏煎锛堝疄闄呭凡缁熶竴涓?saveLocked锛夈€?func (sm *StorageManager) SaveToPermanent() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.saveLocked()
}

// LoadFromPermanent 淇濇寔鍚戝悗鍏煎锛堝疄闄呭凡缁熶竴涓?loadConnections锛夈€?func (sm *StorageManager) LoadFromPermanent() error {
	return nil
}

