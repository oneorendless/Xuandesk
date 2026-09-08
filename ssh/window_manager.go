package ssh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// WindowPosition 绐楀彛浣嶇疆淇℃伅
type WindowPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WindowManager SSH绐楀彛绠＄悊鍣?type WindowManager struct {
	app            *application.App
	configService  *ConfigService
	windowMutex    sync.RWMutex
	windows        map[string]*application.WebviewWindow // groupID -> Window
	onGroupClose   func(groupID string)                  // 鍒嗙粍鍏抽棴鍥炶皟
	positions      map[string]*WindowPosition            // 绐楀彛浣嶇疆缂撳瓨
	positionsFile  string                                // 浣嶇疆鏂囦欢璺緞
	positionsMutex sync.RWMutex
}

// NewWindowManager 鍒涘缓绐楀彛绠＄悊鍣ㄥ疄渚?func NewWindowManager(app *application.App, onGroupClose func(groupID string)) *WindowManager {
	wm := &WindowManager{
		app:           app,
		windows:       make(map[string]*application.WebviewWindow),
		onGroupClose:  onGroupClose,
		positions:     make(map[string]*WindowPosition),
		positionsFile: getWindowPositionsFile(),
	}

	// 鍔犺浇宸蹭繚瀛樼殑绐楀彛浣嶇疆
	wm.loadPositions()

	return wm
}

// SetConfigService 璁剧疆閰嶇疆鏈嶅姟
func (wm *WindowManager) SetConfigService(cs *ConfigService) {
	wm.configService = cs
}

// SetOnGroupClose 璁剧疆鍒嗙粍鍏抽棴鍥炶皟
func (wm *WindowManager) SetOnGroupClose(callback func(groupID string)) {
	wm.onGroupClose = callback
}

// getWindowPositionsFile 鑾峰彇绐楀彛浣嶇疆鏂囦欢璺緞
func getWindowPositionsFile() string {
	exePath, err := os.Executable()
	if err != nil {
		return "data/config/window_positions.json"
	}
	exeDir := filepath.Dir(exePath)
	dir := filepath.Join(exeDir, "data", "config")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "window_positions.json")
}

// loadPositions 浠庢枃浠跺姞杞界獥鍙ｄ綅缃?func (wm *WindowManager) loadPositions() {
	data, err := os.ReadFile(wm.positionsFile)
	if err != nil {
		return
	}
	var positions map[string]*WindowPosition
	if err := json.Unmarshal(data, &positions); err != nil {
		return
	}
	wm.positions = positions
	fmt.Printf("[WindowManager] 宸插姞杞?%d 涓獥鍙ｄ綅缃褰昞n", len(positions))
}

// savePositions 淇濆瓨绐楀彛浣嶇疆鍒版枃浠?func (wm *WindowManager) savePositions() {
	wm.positionsMutex.RLock()
	data, err := json.MarshalIndent(wm.positions, "", "  ")
	wm.positionsMutex.RUnlock()

	if err != nil {
		return
	}
	os.WriteFile(wm.positionsFile, data, 0644)
}

// isRememberPositionEnabled 妫€鏌ユ槸鍚﹀惎鐢ㄤ綅缃蹇?func (wm *WindowManager) isRememberPositionEnabled() bool {
	if wm.configService == nil {
		return true // 榛樿鍚敤
	}
	cfg := wm.configService.GetConfig()
	return cfg.UI.RememberPosition
}

// SaveMainWindowPositionIfChanged 浠呭湪浣嶇疆鏈夊彉鍖栨椂淇濆瓨涓荤獥鍙ｄ綅缃?func (wm *WindowManager) SaveMainWindowPositionIfChanged(window *application.WebviewWindow) {
	if !wm.isRememberPositionEnabled() {
		return
	}

	x, y := window.Position()
	w, h := window.Size()

	if x < -10000 || y < -10000 || w < 100 || h < 100 {
		return
	}

	wm.positionsMutex.RLock()
	existing := wm.positions["main"]
	wm.positionsMutex.RUnlock()

	if existing != nil && existing.X == x && existing.Y == y && existing.Width == w && existing.Height == h {
		return
	}

	wm.positionsMutex.Lock()
	wm.positions["main"] = &WindowPosition{X: x, Y: y, Width: w, Height: h}
	wm.positionsMutex.Unlock()

	wm.savePositions()
}

// SaveMainWindowPosition 淇濆瓨涓荤獥鍙ｄ綅缃?func (wm *WindowManager) SaveMainWindowPosition(window *application.WebviewWindow) {
	if !wm.isRememberPositionEnabled() {
		fmt.Println("[WindowManager] 浣嶇疆璁板繂鏈惎鐢紝璺宠繃淇濆瓨涓荤獥鍙ｄ綅缃?)
		return
	}

	x, y := window.Position()
	w, h := window.Size()

	fmt.Printf("[WindowManager] 涓荤獥鍙ｅ綋鍓嶇姸鎬? (%d,%d %dx%d)\n", x, y, w, h)

	// 鍙繚瀛樻湁鏁堢殑闈炴渶灏忓寲浣嶇疆
	if x < -10000 || y < -10000 || w < 100 || h < 100 {
		fmt.Println("[WindowManager] 涓荤獥鍙ｄ綅缃棤鏁堬紝璺宠繃淇濆瓨")
		return
	}

	wm.positionsMutex.Lock()
	wm.positions["main"] = &WindowPosition{X: x, Y: y, Width: w, Height: h}
	wm.positionsMutex.Unlock()

	wm.savePositions()
	fmt.Printf("[WindowManager] 鉁?宸蹭繚瀛樹富绐楀彛浣嶇疆: (%d,%d %dx%d)\n", x, y, w, h)
}

// RestoreMainWindowPosition 鎭㈠涓荤獥鍙ｄ綅缃紝杩斿洖鏄惁鎭㈠鎴愬姛
func (wm *WindowManager) RestoreMainWindowPosition(window *application.WebviewWindow) bool {
	if !wm.isRememberPositionEnabled() {
		return false
	}

	wm.positionsMutex.RLock()
	pos, exists := wm.positions["main"]
	wm.positionsMutex.RUnlock()

	if !exists || pos == nil {
		return false
	}

	// 楠岃瘉浣嶇疆鏄惁鍦ㄥ睆骞曡寖鍥村唴
	primary := wm.app.Screen.GetPrimary()
	if primary != nil {
		screenW := primary.Size.Width
		screenH := primary.Size.Height
		if pos.X < -100 || pos.Y < -100 || pos.X > screenW-50 || pos.Y > screenH-50 {
			fmt.Printf("[WindowManager] 涓荤獥鍙ｄ綅缃秴鍑哄睆骞曡寖鍥达紝璺宠繃鎭㈠\n")
			return false
		}
	}

	window.SetPosition(pos.X, pos.Y)
	window.SetSize(pos.Width, pos.Height)
	fmt.Printf("[WindowManager] 宸叉仮澶嶄富绐楀彛浣嶇疆: (%d,%d %dx%d)\n", pos.X, pos.Y, pos.Width, pos.Height)
	return true
}

// saveWindowPositionIfChanged 浠呭湪浣嶇疆鏈夊彉鍖栨椂淇濆瓨
func (wm *WindowManager) saveWindowPositionIfChanged(windowID string, window *application.WebviewWindow) {
	if !wm.isRememberPositionEnabled() {
		return
	}

	x, y := window.Position()
	w, h := window.Size()

	// 鏃犳晥浣嶇疆璺宠繃
	if x < -10000 || y < -10000 || w < 100 || h < 100 {
		return
	}

	wm.positionsMutex.RLock()
	existing := wm.positions[windowID]
	wm.positionsMutex.RUnlock()

	// 浣嶇疆娌″彉鍖栵紝璺宠繃
	if existing != nil && existing.X == x && existing.Y == y && existing.Width == w && existing.Height == h {
		return
	}

	wm.positionsMutex.Lock()
	wm.positions[windowID] = &WindowPosition{X: x, Y: y, Width: w, Height: h}
	wm.positionsMutex.Unlock()

	wm.savePositions()
}

// saveWindowPosition 淇濆瓨 SSH 绐楀彛浣嶇疆
func (wm *WindowManager) saveWindowPosition(windowID string) {
	if !wm.isRememberPositionEnabled() {
		fmt.Println("[WindowManager] 浣嶇疆璁板繂鏈惎鐢紝璺宠繃淇濆瓨绐楀彛浣嶇疆")
		return
	}

	wm.windowMutex.RLock()
	var targetWindow *application.WebviewWindow
	for gid, w := range wm.windows {
		if gid == windowID {
			targetWindow = w
			break
		}
	}
	wm.windowMutex.RUnlock()

	if targetWindow == nil {
		fmt.Printf("[WindowManager] 绐楀彛 %s 鏈壘鍒帮紝璺宠繃淇濆瓨\n", windowID)
		return
	}

	x, y := targetWindow.Position()
	w, h := targetWindow.Size()

	fmt.Printf("[WindowManager] 绐楀彛 %s 褰撳墠鐘舵€? (%d,%d %dx%d)\n", windowID, x, y, w, h)

	// 鍙繚瀛樻湁鏁堢殑闈炴渶灏忓寲浣嶇疆
	if x < -10000 || y < -10000 || w < 100 || h < 100 {
		fmt.Printf("[WindowManager] 绐楀彛 %s 浣嶇疆鏃犳晥锛岃烦杩囦繚瀛榎n", windowID)
		return
	}

	wm.positionsMutex.Lock()
	wm.positions[windowID] = &WindowPosition{X: x, Y: y, Width: w, Height: h}
	wm.positionsMutex.Unlock()

	wm.savePositions()
	fmt.Printf("[WindowManager] 鉁?宸蹭繚瀛樼獥鍙ｄ綅缃? %s (%d,%d %dx%d)\n", windowID, x, y, w, h)
}

// restoreWindowPosition 鎭㈠绐楀彛浣嶇疆
func (wm *WindowManager) restoreWindowPosition(windowID string, window *application.WebviewWindow) {
	if !wm.isRememberPositionEnabled() {
		return
	}

	wm.positionsMutex.RLock()
	pos, exists := wm.positions[windowID]
	wm.positionsMutex.RUnlock()

	if !exists || pos == nil {
		return
	}

	// 楠岃瘉浣嶇疆鏄惁鍦ㄥ睆骞曡寖鍥村唴
	primary := wm.app.Screen.GetPrimary()
	if primary != nil {
		screenW := primary.Size.Width
		screenH := primary.Size.Height
		// 浣嶇疆瓒呭嚭灞忓箷鑼冨洿锛屼笉鎭㈠
		if pos.X < -100 || pos.Y < -100 || pos.X > screenW-50 || pos.Y > screenH-50 {
			fmt.Printf("[WindowManager] 绐楀彛浣嶇疆瓒呭嚭灞忓箷鑼冨洿锛岃烦杩囨仮澶? %s\n", windowID)
			return
		}
	}

	window.SetPosition(pos.X, pos.Y)
	window.SetSize(pos.Width, pos.Height)
	fmt.Printf("[WindowManager] 宸叉仮澶嶇獥鍙ｄ綅缃? %s (%d,%d %dx%d)\n", windowID, pos.X, pos.Y, pos.Width, pos.Height)
}

// CreateSSHWindow 鍒涘缓鎴栬仛鐒SH鍒嗙粍绐楀彛
func (wm *WindowManager) CreateSSHWindow(groupID string, groupName string, activeConnID string) error {
	fmt.Printf("[WindowManager] CreateSSHWindow 琚皟鐢? groupID=%s, groupName=%s, activeConn=%s\n", groupID, groupName, activeConnID)

	wm.windowMutex.RLock()
	existingWindow, exists := wm.windows[groupID]
	wm.windowMutex.RUnlock()

	// 濡傛灉绐楀彛宸插瓨鍦紝鑱氱劍鐜版湁绐楀彛锛堜笉鍒涘缓鏂扮獥鍙ｏ級
	if exists && existingWindow != nil {
		fmt.Printf("[WindowManager] 鉁?绐楀彛宸插瓨鍦紝鑱氱劍鐜版湁绐楀彛: %s\n", groupID)
		existingWindow.Focus()
		return nil
	}

	fmt.Printf("[WindowManager] 鍒涘缓鏂扮獥鍙? %s\n", groupID)

	// 鍒涘缓鏂扮獥鍙?	windowTitle := groupName
	if windowTitle == "" {
		windowTitle = "SSH 缁堢"
	}

	// 鏋勫缓 URL锛屼紶閫?groupID 鍜?activeConn 鍙傛暟
	url := "/#/ssh?group=" + groupID
	if activeConnID != "" {
		url += "&activeConn=" + activeConnID
		fmt.Printf("[WindowManager] 绐楀彛URL: %s (鍖呭惈 activeConn)\n", url)
	} else {
		fmt.Printf("[WindowManager] 绐楀彛URL: %s\n", url)
	}

	// 绐楀彛鍚嶇О鏍煎紡锛歴sh-{groupID}锛屼究浜庨€氳繃 GetByName 鏌ユ壘
	windowName := "ssh-" + groupID

	newWindow := wm.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowName,
		Title:            windowTitle,
		URL:              url,
		DisableResize:    false,
		Frameless:        true,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(30, 30, 30),
	})

	// 鏂囦欢鎷栨斁澶勭悊
	newWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		if len(files) > 0 {
			wm.app.Event.Emit("file-manager:files-dropped", map[string]interface{}{
				"files": files,
			})
		}
	})

	// 灏濊瘯鎭㈠绐楀彛浣嶇疆锛屽惁鍒欎娇鐢ㄩ粯璁ゅ昂瀵稿眳涓?	wm.restoreWindowPosition(groupID, newWindow)

	// 濡傛灉娌℃湁鎭㈠鍒颁綅缃紙娌℃湁淇濆瓨杩囷級锛屼娇鐢ㄩ粯璁ゅ昂瀵稿眳涓?	if !wm.isRememberPositionEnabled() || wm.positions[groupID] == nil {
		w, h := wm.calculateWindowSize()
		newWindow.SetSize(w, h)
		newWindow.Center()
		fmt.Printf("[WindowManager] 浣跨敤榛樿绐楀彛澶у皬: %dx%d\n", w, h)
	}

	// 淇濆瓨绐楀彛寮曠敤
	wm.windowMutex.Lock()
	wm.windows[groupID] = newWindow
	wm.windowMutex.Unlock()

	// 鏄剧ず骞惰仛鐒︾獥鍙?	newWindow.Show()
	newWindow.Focus()
	fmt.Printf("[WindowManager] 绐楀彛寮曠敤宸蹭繚瀛樺苟鑱氱劍: %s\n", groupID)

	// 瀹氭椂淇濆瓨绐楀彛浣嶇疆锛堟瘡3绉掓鏌ヤ竴娆★紝鏈夊彉鍖栨墠鍐欑洏锛?	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			wm.windowMutex.RLock()
			win, exists := wm.windows[groupID]
			wm.windowMutex.RUnlock()
			if !exists {
				return // 绐楀彛宸插叧闂紝鍋滄瀹氭椂鍣?			}
			wm.saveWindowPositionIfChanged(groupID, win)
		}
	}()

	// 鐩戝惉绐楀彛鍏抽棴浜嬩欢锛屼繚瀛樻渶缁堜綅缃苟閿€姣佸垎缁?	newWindow.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		fmt.Printf("[WindowManager] 馃棏锔?绐楀彛鍏抽棴浜嬩欢瑙﹀彂: %s\n", groupID)

		// 鍏抽棴鍓嶆渶鍚庝繚瀛樹竴娆?		wm.saveWindowPosition(groupID)

		// 娓呯悊绐楀彛寮曠敤
		wm.windowMutex.Lock()
		delete(wm.windows, groupID)
		wm.windowMutex.Unlock()

		// 璋冪敤鍥炶皟鍑芥暟锛岄攢姣佸垎缁?		if wm.onGroupClose != nil {
			fmt.Printf("[WindowManager] 馃攧 璋冪敤鍒嗙粍閿€姣佸洖璋僜n")
			wm.onGroupClose(groupID)
		}
	})

	return nil
}

// calculateWindowSize 鏍规嵁灞忓箷澶у皬璁＄畻鍚堥€傜殑绐楀彛灏哄
func (wm *WindowManager) calculateWindowSize() (int, int) {
	// 妗ｄ綅瀹氫箟
	type size struct {
		width  int
		height int
	}
	sizes := []size{
		{1920, 1080},
		{1600, 1000},
		{1400, 900},
		{1200, 800},
	}

	// 鑾峰彇涓诲睆骞?	primary := wm.app.Screen.GetPrimary()
	if primary == nil {
		return 1400, 900
	}

	screenW := primary.Size.Width
	screenH := primary.Size.Height

	// 閫夋嫨涓嶈秴杩囧睆骞?85% 鐨勬渶澶ф。浣?	maxW := int(float64(screenW) * 0.85)
	maxH := int(float64(screenH) * 0.85)

	fmt.Printf("[WindowManager] 灞忓箷澶у皬: %dx%d, 鏈€澶х獥鍙? %dx%d\n", screenW, screenH, maxW, maxH)

	for _, s := range sizes {
		if s.width <= maxW && s.height <= maxH {
			return s.width, s.height
		}
	}

	return 1200, 800
}

// CloseWindow 鍏抽棴鍒嗙粍绐楀彛骞舵竻鐞?func (wm *WindowManager) CloseWindow(groupID string) {
	wm.windowMutex.Lock()
	defer wm.windowMutex.Unlock()

	if window, exists := wm.windows[groupID]; exists {
		// 淇濆瓨绐楀彛浣嶇疆
		wm.saveWindowPosition(groupID)

		window.Close()
		delete(wm.windows, groupID)
		fmt.Printf("[WindowManager] 绐楀彛宸插叧闂苟娓呯悊: %s\n", groupID)
	}
}

// CleanupWindow 娓呯悊绐楀彛寮曠敤锛堜笉鍏抽棴绐楀彛锛屼粎娓呯悊寮曠敤锛?// 鐢ㄤ簬鍓嶇閫氱煡鍚庣绐楀彛宸查€氳繃绯荤粺鏂瑰紡鍏抽棴鐨勬儏鍐?func (wm *WindowManager) CleanupWindow(groupID string) {
	wm.windowMutex.Lock()
	defer wm.windowMutex.Unlock()

	if _, exists := wm.windows[groupID]; exists {
		delete(wm.windows, groupID)
		fmt.Printf("[WindowManager] 绐楀彛寮曠敤宸叉竻鐞? %s\n", groupID)
	}
}

// GetWindowCount 鑾峰彇褰撳墠鎵撳紑鐨勭獥鍙ｆ暟閲?func (wm *WindowManager) GetWindowCount() int {
	wm.windowMutex.RLock()
	defer wm.windowMutex.RUnlock()
	return len(wm.windows)
}

// HasWindow 妫€鏌ユ寚瀹氬垎缁勭殑绐楀彛鏄惁瀛樺湪
func (wm *WindowManager) HasWindow(groupID string) bool {
	wm.windowMutex.RLock()
	defer wm.windowMutex.RUnlock()
	_, exists := wm.windows[groupID]
	return exists
}

// HideAllWindows 闅愯棌鎵€鏈夌獥鍙?func (wm *WindowManager) HideAllWindows() {
	wm.windowMutex.RLock()
	defer wm.windowMutex.RUnlock()
	for _, window := range wm.windows {
		window.Hide()
	}
	fmt.Println("[WindowManager] 宸查殣钘忔墍鏈?SSH 绐楀彛")
}

// ShowAllWindows 鏄剧ず鎵€鏈夌獥鍙?func (wm *WindowManager) ShowAllWindows() {
	wm.windowMutex.RLock()
	defer wm.windowMutex.RUnlock()
	for _, window := range wm.windows {
		window.Show()
		window.Focus()
	}
	fmt.Println("[WindowManager] 宸叉樉绀烘墍鏈?SSH 绐楀彛")
}

// ClearPositions 娓呴櫎鎵€鏈夌獥鍙ｄ綅缃蹇?func (wm *WindowManager) ClearPositions() {
	wm.positionsMutex.Lock()
	wm.positions = make(map[string]*WindowPosition)
	wm.positionsMutex.Unlock()

	// 鍒犻櫎浣嶇疆鏂囦欢
	os.Remove(wm.positionsFile)
	fmt.Println("[WindowManager] 宸叉竻闄ゆ墍鏈夌獥鍙ｄ綅缃蹇?)
}

