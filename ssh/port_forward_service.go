package ssh

import (
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// PortForward 绔彛杞彂瑙勫垯
type PortForward struct {
	ID         string `json:"id"`
	ConnID     string `json:"connId"`
	Type       string `json:"type"`       // "local" / "remote"
	BindAddr   string `json:"bindAddr"`
	BindPort   int    `json:"bindPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
	Status     string `json:"status"` // "running" / "stopped" / "error"
	Error      string `json:"error,omitempty"`
	CreatedAt  int64  `json:"createdAt"`

	// 娴侀噺缁熻
	ActiveConns int64 `json:"activeConns"` // 褰撳墠娲昏穬杩炴帴鏁?	TotalConns  int64 `json:"totalConns"`  // 鍘嗗彶鎬昏繛鎺ユ暟
	BytesSent   int64 `json:"bytesSent"`   // 鍙戦€佸瓧鑺傛暟锛堜笂琛岋級
	BytesRecv   int64 `json:"bytesRecv"`   // 鎺ユ敹瀛楄妭鏁帮紙涓嬭锛?}

// countingConn 鍖呰 net.Conn锛岀粺璁℃祦閲?type countingConn struct {
	net.Conn
	bytesSent *int64
	bytesRecv *int64
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		atomic.AddInt64(c.bytesRecv, int64(n))
	}
	return n, err
}

func (c *countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		atomic.AddInt64(c.bytesSent, int64(n))
	}
	return n, err
}

// forwardState 姣忎釜杞彂鐨勮繍琛屾椂鐘舵€?type forwardState struct {
	mu        sync.Mutex
	activeConns int64
	totalConns  int64
	bytesSent   int64
	bytesRecv   int64
	done        chan struct{}
}

// PortForwardService 绔彛杞彂鏈嶅姟
type PortForwardService struct {
	mu        sync.RWMutex
	forwards  map[string]*PortForward  // id 鈫?forward
	listeners map[string]net.Listener  // id 鈫?listener
	states    map[string]*forwardState // id 鈫?runtime state
	sshSvc    *SSHService
	app       *application.App
}

// NewPortForwardService 鍒涘缓绔彛杞彂鏈嶅姟
func NewPortForwardService(sshSvc *SSHService) *PortForwardService {
	svc := &PortForwardService{
		forwards:  make(map[string]*PortForward),
		listeners: make(map[string]net.Listener),
		states:    make(map[string]*forwardState),
		sshSvc:    sshSvc,
		app:       sshSvc.GetApp(),
	}
	// 鍚姩瀹氭椂缁熻骞挎挱
	go svc.statsBroadcastLoop()
	return svc
}

// statsBroadcastLoop 姣?2 绉掑箍鎾竴娆℃祦閲忕粺璁?func (s *PortForwardService) statsBroadcastLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.broadcastAllStats()
	}
}

// broadcastAllStats 骞挎挱鎵€鏈夋椿璺冭浆鍙戠殑缁熻
func (s *PortForwardService) broadcastAllStats() {
	if s.app == nil {
		return
	}

	s.mu.RLock()
	connIDs := make(map[string]bool)
	for _, fwd := range s.forwards {
		if fwd.Status == "running" {
			connIDs[fwd.ConnID] = true
		}
	}
	s.mu.RUnlock()

	for connID := range connIDs {
		forwards := s.getForwardsWithStats(connID)
		if len(forwards) > 0 {
			s.app.Event.Emit("port-forward:status", map[string]interface{}{
				"connId":   connID,
				"forwards": forwards,
			})
		}
	}
}

// getForwardsWithStats 鑾峰彇杞彂鍒楄〃锛堝惈瀹炴椂缁熻锛屾寜鍒涘缓鏃堕棿鎺掑簭锛?func (s *PortForwardService) getForwardsWithStats(connID string) []PortForward {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []PortForward
	for _, fwd := range s.forwards {
		if fwd.ConnID == connID {
			copy := *fwd
			if st, ok := s.states[fwd.ID]; ok {
				copy.ActiveConns = atomic.LoadInt64(&st.activeConns)
				copy.TotalConns = atomic.LoadInt64(&st.totalConns)
				copy.BytesSent = atomic.LoadInt64(&st.bytesSent)
				copy.BytesRecv = atomic.LoadInt64(&st.bytesRecv)
			}
			result = append(result, copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt < result[j].CreatedAt
	})

	return result
}

// generateForwardID 鐢熸垚杞彂 ID
func generateForwardID() string {
	return fmt.Sprintf("fwd_%d", time.Now().UnixNano())
}

// GetForwards 鑾峰彇鎸囧畾杩炴帴鐨勬墍鏈夎浆鍙戣鍒?func (s *PortForwardService) GetForwards(connID string) []PortForward {
	return s.getForwardsWithStats(connID)
}

// AddLocalForward 娣诲姞鏈湴绔彛杞彂
// bindPort=0 鏃剁敱鎿嶄綔绯荤粺鑷姩鍒嗛厤鍙敤绔彛
func (s *PortForwardService) AddLocalForward(connID, bindAddr string, bindPort int, remoteHost string, remotePort int) (*PortForward, error) {
	if bindPort < 0 || bindPort > 65535 {
		return nil, fmt.Errorf("鏈湴绔彛鏃犳晥: %d", bindPort)
	}
	if remotePort <= 0 || remotePort > 65535 {
		return nil, fmt.Errorf("杩滅▼绔彛鏃犳晥: %d", remotePort)
	}
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}

	// 妫€鏌ョ鍙ｅ啿绐?	s.mu.RLock()
	for _, fwd := range s.forwards {
		if fwd.ConnID == connID && fwd.BindAddr == bindAddr && fwd.BindPort == bindPort && fwd.Status == "running" {
			s.mu.RUnlock()
			return nil, fmt.Errorf("绔彛 %s:%d 宸茶鍗犵敤", bindAddr, bindPort)
		}
	}
	s.mu.RUnlock()

	id := generateForwardID()
	fwd := &PortForward{
		ID:         id,
		ConnID:     connID,
		Type:       "local",
		BindAddr:   bindAddr,
		BindPort:   bindPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Status:     "stopped",
		CreatedAt:  time.Now().UnixMilli(),
	}

	s.mu.Lock()
	s.forwards[id] = fwd
	s.mu.Unlock()

	s.emitStatus(connID)
	return fwd, nil
}

// AddRemoteForward 娣诲姞杩滅▼绔彛杞彂
func (s *PortForwardService) AddRemoteForward(connID, bindAddr string, bindPort int, remoteHost string, remotePort int) (*PortForward, error) {
	if bindPort <= 0 || bindPort > 65535 {
		return nil, fmt.Errorf("杩滅▼鐩戝惉绔彛鏃犳晥: %d", bindPort)
	}
	if remotePort <= 0 || remotePort > 65535 {
		return nil, fmt.Errorf("鏈湴鐩爣绔彛鏃犳晥: %d", remotePort)
	}
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}

	id := generateForwardID()
	fwd := &PortForward{
		ID:         id,
		ConnID:     connID,
		Type:       "remote",
		BindAddr:   bindAddr,
		BindPort:   bindPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Status:     "stopped",
		CreatedAt:  time.Now().UnixMilli(),
	}

	s.mu.Lock()
	s.forwards[id] = fwd
	s.mu.Unlock()

	s.emitStatus(connID)
	return fwd, nil
}

// StartForward 鍚姩杞彂
func (s *PortForwardService) StartForward(forwardID string) error {
	s.mu.RLock()
	fwd, exists := s.forwards[forwardID]
	s.mu.RUnlock()
	if !exists {
		return fmt.Errorf("杞彂瑙勫垯涓嶅瓨鍦? %s", forwardID)
	}

	if fwd.Status == "running" {
		return nil
	}

	client, err := s.sshSvc.GetClient(fwd.ConnID)
	if err != nil {
		s.updateStatus(forwardID, "error", err.Error())
		return err
	}

	if !client.IsConnected() {
		s.updateStatus(forwardID, "error", "SSH杩炴帴宸叉柇寮€")
		return fmt.Errorf("SSH杩炴帴宸叉柇寮€")
	}

	// 鍒涘缓杩愯鏃剁姸鎬?	st := &forwardState{done: make(chan struct{})}
	s.mu.Lock()
	s.states[forwardID] = st
	s.mu.Unlock()

	switch fwd.Type {
	case "local":
		err = s.startLocalForward(fwd, client, st)
	case "remote":
		err = s.startRemoteForward(fwd, client, st)
	default:
		err = fmt.Errorf("鏈煡杞彂绫诲瀷: %s", fwd.Type)
	}

	if err != nil {
		s.mu.Lock()
		delete(s.states, forwardID)
		s.mu.Unlock()
		s.updateStatus(forwardID, "error", err.Error())
		return err
	}

	s.updateStatus(forwardID, "running", "")
	s.emitStatus(fwd.ConnID)
	return nil
}

// startLocalForward 鍚姩鏈湴杞彂
// 娴侀噺鏂瑰悜锛氭湰鍦板鎴风 鈫?鏈湴鐩戝惉绔彛 鈫?SSH闅ч亾 鈫?杩滅▼鏈嶅姟
// bindPort=0 鏃剁敱鎿嶄綔绯荤粺鑷姩鍒嗛厤鍙敤绔彛
func (s *PortForwardService) startLocalForward(fwd *PortForward, client *SSHClient, st *forwardState) error {
	addr := fmt.Sprintf("%s:%d", fwd.BindAddr, fwd.BindPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("鐩戝惉 %s 澶辫触: %v", addr, err)
	}

	// 濡傛灉绔彛鏄?0锛堣嚜鍔ㄥ垎閰嶏級锛屾洿鏂颁负瀹為檯鍒嗛厤鐨勭鍙?	if fwd.BindPort == 0 {
		actualPort := listener.Addr().(*net.TCPAddr).Port
		s.mu.Lock()
		fwd.BindPort = actualPort
		s.mu.Unlock()
		fmt.Printf("[PortForward] 鑷姩鍒嗛厤鏈湴绔彛: %d\n", actualPort)
	}

	s.mu.Lock()
	s.listeners[fwd.ID] = listener
	s.mu.Unlock()

	fmt.Printf("[PortForward] 鏈湴鐩戝惉宸插惎鍔? %s 鈫?%s:%d (SSH闅ч亾)\n",
		listener.Addr().String(), fwd.RemoteHost, fwd.RemotePort)

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return
			}

			remoteAddr := fmt.Sprintf("%s:%d", fwd.RemoteHost, fwd.RemotePort)
			remoteConn, err := client.client.Dial("tcp", remoteAddr)
			if err != nil {
				fmt.Printf("[PortForward] SSH闅ч亾鎷ㄥ彿澶辫触 (%s): %v\n", remoteAddr, err)
				localConn.Close()
				continue
			}

			atomic.AddInt64(&st.activeConns, 1)
			atomic.AddInt64(&st.totalConns, 1)

			wrappedLocal := &countingConn{Conn: localConn, bytesSent: &st.bytesSent, bytesRecv: &st.bytesRecv}
			wrappedRemote := &countingConn{Conn: remoteConn, bytesSent: &st.bytesSent, bytesRecv: &st.bytesRecv}

			go pipeConns(wrappedLocal, wrappedRemote, func() {
				atomic.AddInt64(&st.activeConns, -1)
			})
		}
	}()

	return nil
}

// startRemoteForward 鍚姩杩滅▼杞彂锛堝唴缃戠┛閫忥級
// 娴侀噺鏂瑰悜锛氳繙绋嬪鎴风 鈫?SSH鏈嶅姟鍣ㄧ洃鍚鍙?鈫?SSH闅ч亾 鈫?鏈湴鏈嶅姟
func (s *PortForwardService) startRemoteForward(fwd *PortForward, client *SSHClient, st *forwardState) error {
	// 鍦?SSH 鏈嶅姟鍣ㄤ笂鐩戝惉绔彛锛堜娇鐢?:port 鏍煎紡锛岃 SSH 鏈嶅姟鍣ㄥ喅瀹氱粦瀹氬湴鍧€锛?	listenAddr := fmt.Sprintf("0.0.0.0:%d", fwd.BindPort)
	listener, err := client.client.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("杩滅▼鐩戝惉澶辫触 (%s): %v", listenAddr, err)
	}

	s.mu.Lock()
	s.listeners[fwd.ID] = listener
	s.mu.Unlock()

	fmt.Printf("[PortForward] 杩滅▼鐩戝惉宸插惎鍔? %s 鈫?%s:%d (SSH鏈嶅姟鍣?\n",
		listenAddr, fwd.RemoteHost, fwd.RemotePort)

	go func() {
		for {
			remoteConn, err := listener.Accept()
			if err != nil {
				fmt.Printf("[PortForward] 杩滅▼鐩戝惉 Accept 閫€鍑? %v\n", err)
				return
			}

			// 杩炴帴鍒版湰鍦版湇鍔?			localAddr := fmt.Sprintf("%s:%d", fwd.RemoteHost, fwd.RemotePort)
			localConn, err := net.Dial("tcp", localAddr)
			if err != nil {
				fmt.Printf("[PortForward] 杩炴帴鏈湴鏈嶅姟澶辫触 (%s): %v\n", localAddr, err)
				remoteConn.Close()
				continue
			}

			atomic.AddInt64(&st.activeConns, 1)
			atomic.AddInt64(&st.totalConns, 1)

			fmt.Printf("[PortForward] 杩滅▼杞彂杩炴帴寤虹珛: %s 鈫愨啋 %s\n", listenAddr, localAddr)

			wrappedLocal := &countingConn{Conn: localConn, bytesSent: &st.bytesSent, bytesRecv: &st.bytesRecv}
			wrappedRemote := &countingConn{Conn: remoteConn, bytesSent: &st.bytesSent, bytesRecv: &st.bytesRecv}

			go pipeConns(wrappedRemote, wrappedLocal, func() {
				atomic.AddInt64(&st.activeConns, -1)
			})
		}
	}()

	return nil
}

// pipeConns 鍙屽悜杞彂涓や釜杩炴帴锛屼换涓€鏂瑰悜缁撴潫鏃跺叧闂袱绔?func pipeConns(a, b net.Conn, onDone func()) {
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			a.Close()
			b.Close()
			if onDone != nil {
				onDone()
			}
		})
	}

	go func() {
		defer cleanup()
		io.Copy(a, b)
	}()
	go func() {
		defer cleanup()
		io.Copy(b, a)
	}()
}

// StopForward 鍋滄杞彂
func (s *PortForwardService) StopForward(forwardID string) error {
	s.mu.RLock()
	fwd, exists := s.forwards[forwardID]
	listener, hasListener := s.listeners[forwardID]
	st, hasState := s.states[forwardID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("杞彂瑙勫垯涓嶅瓨鍦? %s", forwardID)
	}

	fmt.Printf("[PortForward] 鍋滄杞彂: %s (%s %s:%d)\n", forwardID, fwd.Type, fwd.BindAddr, fwd.BindPort)

	if hasListener && listener != nil {
		listener.Close()
		fmt.Printf("[PortForward] 鐩戝惉鍣ㄥ凡鍏抽棴: %s\n", forwardID)
	}

	// 鍏抽棴鐘舵€?goroutine
	if hasState && st.done != nil {
		close(st.done)
	}

	// 灏嗘渶缁堢粺璁″啓鍥?fwd
	if hasState {
		s.mu.Lock()
		if f, ok := s.forwards[forwardID]; ok {
			f.ActiveConns = atomic.LoadInt64(&st.activeConns)
			f.TotalConns = atomic.LoadInt64(&st.totalConns)
			f.BytesSent = atomic.LoadInt64(&st.bytesSent)
			f.BytesRecv = atomic.LoadInt64(&st.bytesRecv)
		}
		delete(s.listeners, forwardID)
		delete(s.states, forwardID)
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		delete(s.listeners, forwardID)
		delete(s.states, forwardID)
		s.mu.Unlock()
	}

	s.updateStatus(forwardID, "stopped", "")
	s.emitStatus(fwd.ConnID)
	return nil
}

// RemoveForward 鍒犻櫎杞彂瑙勫垯
func (s *PortForwardService) RemoveForward(forwardID string) error {
	s.mu.RLock()
	fwd, exists := s.forwards[forwardID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("杞彂瑙勫垯涓嶅瓨鍦? %s", forwardID)
	}
	connID := fwd.ConnID
	needStop := fwd.Status == "running"
	s.mu.RUnlock()

	// 鍏堝仠姝㈣浆鍙戯紙鍏抽棴鐩戝惉鍣級
	if needStop {
		s.StopForward(forwardID)
	}

	// 鍒犻櫎瑙勫垯
	s.mu.Lock()
	delete(s.forwards, forwardID)
	delete(s.listeners, forwardID)
	delete(s.states, forwardID)
	s.mu.Unlock()

	s.emitStatus(connID)
	return nil
}

// GetForwardStatus 鑾峰彇鍗曚釜杞彂鐘舵€?func (s *PortForwardService) GetForwardStatus(forwardID string) *PortForward {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.forwards[forwardID]
}

// StopAllByConnID 鍋滄鎸囧畾杩炴帴鐨勬墍鏈夎浆鍙?func (s *PortForwardService) StopAllByConnID(connID string) {
	s.mu.RLock()
	var ids []string
	for id, fwd := range s.forwards {
		if fwd.ConnID == connID && fwd.Status == "running" {
			ids = append(ids, id)
		}
	}
	s.mu.RUnlock()

	for _, id := range ids {
		s.StopForward(id)
	}

	s.mu.Lock()
	for id, fwd := range s.forwards {
		if fwd.ConnID == connID {
			delete(s.forwards, id)
		}
	}
	s.mu.Unlock()
}

// updateStatus 鏇存柊杞彂鐘舵€?func (s *PortForwardService) updateStatus(forwardID, status, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fwd, ok := s.forwards[forwardID]; ok {
		fwd.Status = status
		fwd.Error = errMsg
	}
}

// emitStatus 鍙戦€佺姸鎬佸彉鏇翠簨浠跺埌鍓嶇
func (s *PortForwardService) emitStatus(connID string) {
	if s.app == nil {
		return
	}
	forwards := s.getForwardsWithStats(connID)
	s.app.Event.Emit("port-forward:status", map[string]interface{}{
		"connId":   connID,
		"forwards": forwards,
	})
}

