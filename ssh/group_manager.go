package ssh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// GroupManager 鍒嗙粍绠＄悊鍣?type GroupManager struct {
	mu       sync.RWMutex
	groups   map[string]*SSHGroup
	nextID   int
	dataFile string
}

// SSHGroup SSH鍒嗙粍
type SSHGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	ParentID  string   `json:"parent_id,omitempty"` // 鐖跺垎缁処D锛岀┖=椤剁骇鍒嗙粍
	Color     string   `json:"color,omitempty"`     // 鍒嗙粍棰滆壊鏍囪
	Order     int      `json:"order"`               // 鎺掑簭搴忓彿
	ConnIDs   []string `json:"conn_ids"`
	WindowID  string   `json:"window_id"`
	IsDefault bool     `json:"is_default"`
}

// NewGroupManager 鍒涘缓鍒嗙粍绠＄悊鍣?func NewGroupManager() *GroupManager {
	dataFile := filepath.Join(GetDataDir(), "groups.json")

	gm := &GroupManager{
		groups:   make(map[string]*SSHGroup),
		nextID:   2,
		dataFile: dataFile,
	}

	// 浠庢枃浠跺姞杞藉垎缁?	if err := gm.loadGroups(); err != nil {
		log.Error("鍔犺浇鍒嗙粍澶辫触锛屼娇鐢ㄩ粯璁ゅ垎缁?, "error", err)
	}

	// 纭繚榛樿鍒嗙粍瀛樺湪
	if _, exists := gm.groups["group-1"]; !exists {
		gm.groups["group-1"] = &SSHGroup{
			ID:        "group-1",
			Name:      "榛樿鍒嗙粍",
			ConnIDs:   make([]string, 0),
			IsDefault: true,
			Order:     0,
		}
	}

	return gm
}

// loadGroups 浠庢枃浠跺姞杞藉垎缁?func (gm *GroupManager) loadGroups() error {
	data, err := os.ReadFile(gm.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var groups []*SSHGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return err
	}

	for _, g := range groups {
		if g.ConnIDs == nil {
			g.ConnIDs = make([]string, 0)
		}
		gm.groups[g.ID] = g
	}

	return nil
}

// saveGroups 淇濆瓨鍒嗙粍鍒版枃浠?func (gm *GroupManager) saveGroups() error {
	dir := filepath.Dir(gm.dataFile)
	os.MkdirAll(dir, 0755)

	var groups []*SSHGroup
	for _, g := range gm.groups {
		groups = append(groups, g)
	}

	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(gm.dataFile, data, 0644)
}

// CreateGroup 鍒涘缓鏂板垎缁?func (gm *GroupManager) CreateGroup(name string) *SSHGroup {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	
	groupID := fmt.Sprintf("group-%d", gm.nextID)
	gm.nextID++
	
	return gm.createGroupInternal(groupID, name, false)
}

// CreateGroupWithName 浣跨敤鎸囧畾ID鍜屽悕绉板垱寤哄垎缁勶紙鐢ㄤ簬閲嶆柊鍒涘缓宸插垹闄ょ殑鍒嗙粍锛?func (gm *GroupManager) CreateGroupWithName(groupID, name string) *SSHGroup {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	
	// 妫€鏌ユ槸鍚﹀凡瀛樺湪
	if _, exists := gm.groups[groupID]; exists {
		log.Info("鍒嗙粍宸插瓨鍦紝鏃犻渶閲嶆柊鍒涘缓", "id", groupID)
		return gm.groups[groupID]
	}
	
	isDefault := (groupID == "group-1")
	return gm.createGroupInternal(groupID, name, isDefault)
}

// createGroupInternal 鍐呴儴鏂规硶锛氬垱寤哄垎缁勶紙闇€瑕佸凡鎸佹湁閿侊級
func (gm *GroupManager) createGroupInternal(groupID, name string, isDefault bool) *SSHGroup {
	// 璁＄畻鎺掑簭搴忓彿锛堣拷鍔犲埌鏈熬锛?	maxOrder := 0
	for _, g := range gm.groups {
		if g.Order > maxOrder {
			maxOrder = g.Order
		}
	}

	group := &SSHGroup{
		ID:        groupID,
		Name:      name,
		ConnIDs:   make([]string, 0),
		IsDefault: isDefault,
		Order:     maxOrder + 1,
	}

	gm.groups[groupID] = group
	log.Info("鍒涘缓鍒嗙粍", "id", groupID, "name", name, "default", isDefault)

	// 鏇存柊nextID锛堝鏋滄槸鑷姩鐢熸垚鐨処D锛?	if !isDefault && len(groupID) > 6 {
		var num int
		fmt.Sscanf(groupID, "group-%d", &num)
		if num >= gm.nextID {
			gm.nextID = num + 1
		}
	}

	gm.saveGroups()
	return group
}

// GetGroup 鑾峰彇鍒嗙粍
func (gm *GroupManager) GetGroup(groupID string) *SSHGroup {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	
	return gm.groups[groupID]
}

// GetAllGroups 鑾峰彇鎵€鏈夊垎缁?func (gm *GroupManager) GetAllGroups() []*SSHGroup {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	
	var groups []*SSHGroup
	for _, group := range gm.groups {
		groups = append(groups, group)
	}
	
	return groups
}

// AddConnectionToGroup 灏嗚繛鎺ユ坊鍔犲埌鍒嗙粍
func (gm *GroupManager) AddConnectionToGroup(groupID, connID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupID]
	if !exists {
		return fmt.Errorf("鍒嗙粍涓嶅瓨鍦? %s", groupID)
	}

	group.ConnIDs = append(group.ConnIDs, connID)
	log.Info("娣诲姞杩炴帴鍒板垎缁?, "group", groupID, "conn", connID)
	gm.saveGroups()
	return nil
}

// RemoveConnectionFromGroup 浠庡垎缁勪腑绉婚櫎杩炴帴
func (gm *GroupManager) RemoveConnectionFromGroup(groupID, connID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupID]
	if !exists {
		return fmt.Errorf("鍒嗙粍涓嶅瓨鍦? %s", groupID)
	}

	// 绉婚櫎connID
	for i, id := range group.ConnIDs {
		if id == connID {
			group.ConnIDs = append(group.ConnIDs[:i], group.ConnIDs[i+1:]...)
			break
		}
	}

	log.Info("浠庡垎缁勭Щ闄よ繛鎺?, "group", groupID, "conn", connID, "remaining", len(group.ConnIDs))

	// 濡傛灉娌℃湁杩炴帴浜嗭紝鍒犻櫎鍒嗙粍锛堝寘鎷粯璁ゅ垎缁勶級
	if len(group.ConnIDs) == 0 {
		delete(gm.groups, groupID)
		log.Info("鍒犻櫎绌哄垎缁?, "id", groupID)
	}

	gm.saveGroups()
	return nil
}

// GetGroupByConnID 鏍规嵁杩炴帴ID鏌ユ壘鍒嗙粍
func (gm *GroupManager) GetGroupByConnID(connID string) *SSHGroup {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	
	for _, group := range gm.groups {
		for _, id := range group.ConnIDs {
			if id == connID {
				return group
			}
		}
	}
	
	return nil
}

// GetDefaultGroup 鑾峰彇榛樿鍒嗙粍
func (gm *GroupManager) GetDefaultGroup() *SSHGroup {
	return gm.GetGroup("group-1")
}

// ClearGroup 娓呯┖鍒嗙粍涓殑鎵€鏈夎繛鎺ワ紙浣嗕笉鍒犻櫎鍒嗙粍锛?func (gm *GroupManager) ClearGroup(groupID string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupID]
	if !exists {
		log.Warn("鍒嗙粍涓嶅瓨鍦紝鏃犳硶娓呯┖", "id", groupID)
		return
	}

	count := len(group.ConnIDs)
	group.ConnIDs = make([]string, 0)
	log.Info("宸叉竻绌哄垎缁?, "id", groupID, "removed", count)
	gm.saveGroups()
}

// DeleteGroup 鍒犻櫎鍒嗙粍锛堝寘鎷墍鏈夎繛鎺ワ級
func (gm *GroupManager) DeleteGroup(groupID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupID]
	if !exists {
		return fmt.Errorf("鍒嗙粍涓嶅瓨鍦? %s", groupID)
	}

	// 涓嶅厑璁稿垹闄ら粯璁ゅ垎缁?	if group.IsDefault {
		log.Warn("灏濊瘯鍒犻櫎榛樿鍒嗙粍锛屾敼涓烘竻绌鸿繛鎺?)
		group.ConnIDs = make([]string, 0)
		gm.saveGroups()
		return nil
	}

	delete(gm.groups, groupID)
	log.Info("宸插垹闄ゅ垎缁?, "id", groupID, "connCount", len(group.ConnIDs))
	gm.saveGroups()
	return nil
}

// UpdateGroup 鏇存柊鍒嗙粍灞炴€э紙鍚嶇О銆侀鑹层€佺埗鍒嗙粍锛?func (gm *GroupManager) UpdateGroup(groupID, name, color, parentID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	group, exists := gm.groups[groupID]
	if !exists {
		return fmt.Errorf("鍒嗙粍涓嶅瓨鍦? %s", groupID)
	}

	if name != "" {
		group.Name = name
	}
	if color != "" {
		group.Color = color
	}
	if parentID != "" && parentID != groupID {
		group.ParentID = parentID
	}

	log.Info("鏇存柊鍒嗙粍", "id", groupID, "name", group.Name, "color", group.Color)
	gm.saveGroups()
	return nil
}

// ReorderGroups 鏇存柊鍒嗙粍鎺掑簭
func (gm *GroupManager) ReorderGroups(groupIDs []string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	for i, id := range groupIDs {
		if group, exists := gm.groups[id]; exists {
			group.Order = i
		}
	}

	gm.saveGroups()
	return nil
}

// GetChildGroups 鑾峰彇鎸囧畾鍒嗙粍鐨勫瓙鍒嗙粍
func (gm *GroupManager) GetChildGroups(parentID string) []*SSHGroup {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var children []*SSHGroup
	for _, g := range gm.groups {
		if g.ParentID == parentID {
			children = append(children, g)
		}
	}
	return children
}

