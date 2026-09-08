package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

// HostKeyCallback SSH 涓绘満瀵嗛挜鏍￠獙绛栫暐
type HostKeyCallback string

const (
	HostKeyCallbackStrict HostKeyCallback = "strict"
	HostKeyCallbackTOFU   HostKeyCallback = "tofu"
	HostKeyCallbackOff    HostKeyCallback = "off"
)

// SSHConfig SSH 杩炴帴閰嶇疆
type SSHConfig struct {
	Name           string          `json:"name"`
	Host           string          `json:"host"`
	Port           int             `json:"port"`
	Username       string          `json:"username"`
	Password       string          `json:"password,omitempty"`
	KeyPath        string          `json:"keyPath,omitempty"`
	PrivateKey     string          `json:"privateKey,omitempty"`
	Timeout        int             `json:"timeout,omitempty"`
	HostKeyPolicy  HostKeyCallback `json:"hostKeyPolicy,omitempty"`
	KnownHostsFile string          `json:"knownHostsFile,omitempty"`
	JumpHost       string          `json:"jumpHost,omitempty"`       // 璺虫澘鏈?host:port
	JumpUsername   string          `json:"jumpUsername,omitempty"`   // 璺虫澘鏈虹敤鎴峰悕
	JumpPassword   string          `json:"jumpPassword,omitempty"`   // 璺虫澘鏈哄瘑鐮?	JumpKeyPath    string          `json:"jumpKeyPath,omitempty"`    // 璺虫澘鏈哄瘑閽ヨ矾寰?}

// shellSession 鍗曚釜 shell 浼氳瘽
type shellSession struct {
	session  *ssh.Session
	stdin    io.WriteCloser
	stdout   io.Reader
	readBuf  []byte
	bufMutex sync.Mutex
	closing  atomic.Int32
}

// SSHClient SSH 瀹㈡埛绔?type SSHClient struct {
	config     *SSHConfig
	client     *ssh.Client
	sftpClient *sftp.Client
	isConnected atomic.Int32 // 0=鏂紑, 1=宸茶繛鎺?	closing     atomic.Int32 // 0=姝ｅ父, 1=姝ｅ湪鍏抽棴

	sessions map[string]*shellSession
	sessMu   sync.RWMutex

	lastPingTime time.Time
	latency      int64

	searchCancelMap map[string]bool
	searchMutex     sync.RWMutex

	onDisconnect func(connID string)
	connID       string
}

// CommandResult 鍛戒护鎵ц缁撴灉
type CommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Success  bool   `json:"success"`
}

// PortForwardConfig 绔彛杞彂閰嶇疆
type PortForwardConfig struct {
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
}

// NewSSHClient 鍒涘缓鏂扮殑 SSH 瀹㈡埛绔?func NewSSHClient(config *SSHConfig) *SSHClient {
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	return &SSHClient{
		config:          config,
		sessions:        make(map[string]*shellSession),
		searchCancelMap: make(map[string]bool),
	}
}

// SetDisconnectCallback 璁剧疆鏂嚎鍥炶皟
func (s *SSHClient) SetDisconnectCallback(connID string, callback func(string)) {
	s.connID = connID
	s.onDisconnect = callback
}

// handleDisconnect 澶勭悊鏂嚎
func (s *SSHClient) handleDisconnect() {
	if s.closing.Load() == 1 || s.isConnected.Load() == 0 {
		return
	}
	s.isConnected.Store(0)
	if s.onDisconnect != nil {
		go s.onDisconnect(s.connID)
	}
}

// keepaliveLoop 瀹氭湡鍙戦€?SSH keepalive 妫€娴嬭繛鎺ュ瓨娲?func (s *SSHClient) keepaliveLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	consecutiveFailures := 0
	const maxFailures = 3 // 杩炵画 3 娆″け璐ュ垽瀹氭柇绾匡紙45绉掞級

	for range ticker.C {
		if s.closing.Load() == 1 || s.isConnected.Load() == 0 {
			return
		}
		if s.client == nil {
			return
		}

		_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures >= maxFailures {
				s.handleDisconnect()
				return
			}
		} else {
			consecutiveFailures = 0
		}
	}
}

// Connect 杩炴帴鍒?SSH 鏈嶅姟鍣?func (s *SSHClient) Connect() error {
	var authMethods []ssh.AuthMethod

	if s.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(s.config.Password))
	}

	if s.config.PrivateKey != "" {
		key, err := parsePrivateKeyContent([]byte(s.config.PrivateKey))
		if err != nil {
			return fmt.Errorf("瑙ｆ瀽绉侀挜澶辫触: %v", err)
		}
		authMethods = append(authMethods, key)
	} else if s.config.KeyPath != "" {
		key, err := loadPrivateKey(s.config.KeyPath)
		if err != nil {
			return fmt.Errorf("鍔犺浇绉侀挜澶辫触: %v", err)
		}
		authMethods = append(authMethods, key)
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("鏈彁渚涜璇佹柟寮忥紙瀵嗙爜鎴栧瘑閽ワ級")
	}

	hostKeyCallback, err := buildHostKeyCallback(s.config)
	if err != nil {
		return fmt.Errorf("鏋勯€犱富鏈哄瘑閽ユ牎楠屽け璐? %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            s.config.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Duration(s.config.Timeout) * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	start := time.Now()

	var client *ssh.Client

	// 璺虫澘鏈轰唬鐞嗚繛鎺?	if s.config.JumpHost != "" {
		client, err = s.connectViaJumpHost(addr, sshConfig)
	} else {
		client, err = ssh.Dial("tcp", addr, sshConfig)
	}

	if err != nil {
		return fmt.Errorf("杩炴帴SSH鏈嶅姟鍣ㄥけ璐? %v", err)
	}

	s.latency = time.Since(start).Milliseconds()
	s.lastPingTime = start
	s.client = client
	s.isConnected.Store(1)

	// 鍚姩 SSH keepalive 蹇冭烦锛堟娴嬬綉缁滄柇寮€锛?	go s.keepaliveLoop()

	return nil
}

// connectViaJumpHost 閫氳繃璺虫澘鏈鸿繛鎺ョ洰鏍囨湇鍔″櫒
func (s *SSHClient) connectViaJumpHost(targetAddr string, targetConfig *ssh.ClientConfig) (*ssh.Client, error) {
	// 1. 鏋勫缓璺虫澘鏈鸿璇?	jumpAuth := []ssh.AuthMethod{}
	if s.config.JumpPassword != "" {
		jumpAuth = append(jumpAuth, ssh.Password(s.config.JumpPassword))
	}
	if s.config.JumpKeyPath != "" {
		key, err := loadPrivateKey(s.config.JumpKeyPath)
		if err == nil {
			jumpAuth = append(jumpAuth, key)
		}
	}
	// 濡傛灉璺虫澘鏈烘病鏈夊崟鐙厤缃璇侊紝浣跨敤鐩爣鏈嶅姟鍣ㄧ殑璁よ瘉
	if len(jumpAuth) == 0 {
		jumpAuth = targetConfig.Auth
	}

	jumpUser := s.config.JumpUsername
	if jumpUser == "" {
		jumpUser = s.config.Username
	}

	jumpConfig := &ssh.ClientConfig{
		User:            jumpUser,
		Auth:            jumpAuth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 璺虫澘鏈洪€氬父涓嶆牎楠?		Timeout:         time.Duration(s.config.Timeout) * time.Second,
	}

	// 2. 杩炴帴璺虫澘鏈?	jumpConn, err := ssh.Dial("tcp", s.config.JumpHost, jumpConfig)
	if err != nil {
		return nil, fmt.Errorf("杩炴帴璺虫澘鏈哄け璐? %v", err)
	}

	// 3. 閫氳繃璺虫澘鏈鸿繛鎺ョ洰鏍囨湇鍔″櫒
	targetConn, err := jumpConn.Dial("tcp", targetAddr)
	if err != nil {
		jumpConn.Close()
		return nil, fmt.Errorf("閫氳繃璺虫澘鏈烘嫧鍙风洰鏍囨湇鍔″櫒澶辫触: %v", err)
	}

	// 4. 寤虹珛 SSH 杩炴帴
	ncc, chans, reqs, err := ssh.NewClientConn(targetConn, targetAddr, targetConfig)
	if err != nil {
		targetConn.Close()
		jumpConn.Close()
		return nil, fmt.Errorf("閫氳繃璺虫澘鏈哄缓绔婼SH杩炴帴澶辫触: %v", err)
	}

	return ssh.NewClient(ncc, chans, reqs), nil
}

// buildHostKeyCallback 鏍规嵁绛栫暐鏋勯€?ssh.HostKeyCallback
func buildHostKeyCallback(cfg *SSHConfig) (ssh.HostKeyCallback, error) {
	khPath := cfg.KnownHostsFile
	if khPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("鑾峰彇鐢ㄦ埛涓荤洰褰曞け璐? %w", err)
		}
		khPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	if _, err := os.Stat(khPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(khPath), 0700); err != nil {
			return nil, fmt.Errorf("鍒涘缓 known_hosts 鐩綍澶辫触: %w", err)
		}
		f, err := os.OpenFile(khPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("鍒涘缓 known_hosts 鏂囦欢澶辫触: %w", err)
		}
		f.Close()
	}

	kh, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("鍔犺浇 known_hosts 澶辫触: %w", err)
	}

	policy := cfg.HostKeyPolicy
	if policy == "" {
		policy = HostKeyCallbackTOFU
	}

	switch policy {
	case HostKeyCallbackOff:
		log.Warn("涓绘満瀵嗛挜鏍￠獙宸插叧闂紝瀛樺湪 MITM 鏀诲嚮椋庨櫓")
		return ssh.InsecureIgnoreHostKey(), nil

	case HostKeyCallbackStrict:
		return ssh.HostKeyCallback(kh), nil

	case HostKeyCallbackTOFU:
		return wrapKnownHostsForTOFU(khPath, ssh.HostKeyCallback(kh)), nil

	default:
		return nil, fmt.Errorf("鏈煡鐨勪富鏈哄瘑閽ユ牎楠岀瓥鐣? %s", policy)
	}
}

// wrapKnownHostsForTOFU 鎶婁弗鏍兼牎楠屽寘瑁呮垚 ToFU 妯″紡
func wrapKnownHostsForTOFU(khPath string, strict ssh.HostKeyCallback) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := strict(hostname, remote, key)
		if err == nil {
			return nil
		}

		if knownhosts.IsHostUnknown(err) {
			f, openErr := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
			if openErr != nil {
				return fmt.Errorf("鎵撳紑 known_hosts 澶辫触: %w", openErr)
			}
			defer f.Close()
			if writeErr := knownhosts.WriteKnownHost(f, hostname, remote, key); writeErr != nil {
				return fmt.Errorf("鍐欏叆 known_hosts 澶辫触: %w", writeErr)
			}
			log.Info("ToFU: 宸蹭俊浠绘柊涓绘満", "host", hostname)
			return nil
		}

		return err
	}
}

// Close 鍏抽棴 SSH 杩炴帴
func (s *SSHClient) Close() {
	s.closing.Store(1)
	s.CloseAllShells()

	if s.sftpClient != nil {
		s.sftpClient.Close()
	}
	if s.client != nil {
		s.client.Close()
	}

	s.isConnected.Store(0)
}

// IsConnected 妫€鏌ユ槸鍚﹀凡杩炴帴
func (s *SSHClient) IsConnected() bool {
	return s.isConnected.Load() == 1
}

// ExecuteCommand 鎵ц杩滅▼鍛戒护
func (s *SSHClient) ExecuteCommand(command string) (*CommandResult, error) {
	if s.isConnected.Load() == 0 {
		return nil, fmt.Errorf("鏈繛鎺ュ埌SSH鏈嶅姟鍣?)
	}

	session, err := s.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("鍒涘缓浼氳瘽澶辫触: %v", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("鑾峰彇鏍囧噯杈撳嚭绠￠亾澶辫触: %v", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("鑾峰彇鏍囧噯閿欒绠￠亾澶辫触: %v", err)
	}

	if err := session.Start(command); err != nil {
		return nil, fmt.Errorf("鍚姩鍛戒护澶辫触: %v", err)
	}

	stdoutBuf, _ := io.ReadAll(stdout)
	stderrBuf, _ := io.ReadAll(stderr)

	err = session.Wait()
	exitCode := 0
	success := true
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
			success = false
		} else {
			return nil, fmt.Errorf("绛夊緟鍛戒护鎵ц澶辫触: %v", err)
		}
	}

	return &CommandResult{
		Stdout:   string(stdoutBuf),
		Stderr:   string(stderrBuf),
		ExitCode: exitCode,
		Success:  success,
	}, nil
}

// CreateShell 鍒涘缓鐙珛鐨?Shell 浼氳瘽
func (s *SSHClient) CreateShell(sessionID string) (*ssh.Session, error) {
	if s.isConnected.Load() == 0 {
		return nil, fmt.Errorf("鏈繛鎺ュ埌SSH鏈嶅姟鍣?)
	}

	s.CloseShell(sessionID)

	sess, err := s.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("鍒涘缓浼氳瘽澶辫触: %v", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err = sess.RequestPty("xterm-256color", 80, 40, modes); err != nil {
		sess.Close()
		return nil, fmt.Errorf("璇锋眰PTY澶辫触: %v", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("鑾峰彇stdin澶辫触: %v", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("鑾峰彇stdout澶辫触: %v", err)
	}
	if err = sess.Shell(); err != nil {
		sess.Close()
		return nil, fmt.Errorf("鍚姩Shell澶辫触: %v", err)
	}

	sh := &shellSession{session: sess, stdin: stdin, stdout: stdout}
	s.sessMu.Lock()
	s.sessions[sessionID] = sh
	s.sessMu.Unlock()

	go s.readShellOutput(sessionID, sh)
	return sess, nil
}


// readShellOutput 鎸佺画璇诲彇鍗曚釜 session 鐨勮緭鍑?func (s *SSHClient) readShellOutput(sessionID string, sh *shellSession) {
	buf := make([]byte, 4096)
	fmt.Printf("[readShellOutput] 鍚姩: sessionID=%s\n", sessionID)
	for sh.closing.Load() == 0 {
		n, err := sh.stdout.Read(buf)
		if err != nil {
			fmt.Printf("[readShellOutput] 璇诲彇閿欒: sessionID=%s, err=%v\n", sessionID, err)
			if sh.closing.Load() == 0 {
				if err == io.EOF || s.closing.Load() == 0 {
					s.handleDisconnect()
				}
			}
			break
		}
		if n > 0 {
			sh.bufMutex.Lock()
			sh.readBuf = append(sh.readBuf, buf[:n]...)
			sh.bufMutex.Unlock()
		}
	}
	fmt.Printf("[readShellOutput] 閫€鍑? sessionID=%s\n", sessionID)
}

// WriteToShell 鍚戞寚瀹?session 鍐欏叆鏁版嵁
func (s *SSHClient) WriteToShell(sessionID string, data []byte) error {
	s.sessMu.RLock()
	sh, ok := s.sessions[sessionID]
	s.sessMu.RUnlock()
	if !ok || sh.stdin == nil {
		return fmt.Errorf("Shell浼氳瘽鏈垵濮嬪寲: %s", sessionID)
	}
	_, err := sh.stdin.Write(data)
	return err
}

// ReadFromShell 浠庢寚瀹?session 璇诲彇鏁版嵁
func (s *SSHClient) ReadFromShell(sessionID string, buf []byte) (int, error) {
	s.sessMu.RLock()
	sh, ok := s.sessions[sessionID]
	s.sessMu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("Shell浼氳瘽涓嶅瓨鍦? %s", sessionID)
	}
	sh.bufMutex.Lock()
	defer sh.bufMutex.Unlock()
	if len(sh.readBuf) == 0 {
		return 0, nil
	}
	n := copy(buf, sh.readBuf)
	sh.readBuf = sh.readBuf[n:]
	return n, nil
}

// IsShellActive 妫€鏌?session 鏄惁娲昏穬
func (s *SSHClient) IsShellActive(sessionID string) bool {
	s.sessMu.RLock()
	sh, ok := s.sessions[sessionID]
	s.sessMu.RUnlock()
	return ok && sh.closing.Load() == 0
}

// CloseShell 鍏抽棴鎸囧畾 session
func (s *SSHClient) CloseShell(sessionID string) {
	s.sessMu.Lock()
	sh, ok := s.sessions[sessionID]
	if ok {
		sh.closing.Store(1)
		delete(s.sessions, sessionID)
	}
	s.sessMu.Unlock()
	if ok && sh.session != nil {
		sh.session.Close()
	}
}

// CloseAllShells 鍏抽棴鎵€鏈?session
func (s *SSHClient) CloseAllShells() {
	s.sessMu.Lock()
	sessions := make(map[string]*shellSession, len(s.sessions))
	for k, v := range s.sessions {
		sessions[k] = v
	}
	s.sessions = make(map[string]*shellSession)
	s.sessMu.Unlock()

	for _, sh := range sessions {
		sh.closing.Store(1)
		if sh.session != nil {
			sh.session.Close()
		}
	}
}

// GetSessionIDs 鑾峰彇鎵€鏈夋椿璺?session ID
func (s *SSHClient) GetSessionIDs() []string {
	s.sessMu.RLock()
	defer s.sessMu.RUnlock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	return ids
}

// UpdateLatency 鏇存柊寤惰繜锛堜娇鐢ㄨ交閲?SSH request 娴嬮噺锛屼笉鍒涘缓鏂颁細璇濓級
func (s *SSHClient) UpdateLatency() int64 {
	if s.isConnected.Load() == 0 || s.client == nil {
		return 0
	}

	start := time.Now()
	_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
	if err == nil {
		s.latency = time.Since(start).Milliseconds()
		s.lastPingTime = start
	}
	return s.latency
}

// ResizeTerminal 璋冩暣缁堢澶у皬锛堥粯璁?session锛?func (s *SSHClient) ResizeTerminal(cols, rows int) error {
	return s.ResizeTerminalByID("default", cols, rows)
}

// ResizeTerminalByID 璋冩暣鎸囧畾 session 鐨勭粓绔ぇ灏?func (s *SSHClient) ResizeTerminalByID(sessionID string, cols, rows int) error {
	s.sessMu.RLock()
	sh, ok := s.sessions[sessionID]
	s.sessMu.RUnlock()
	if !ok || sh.session == nil {
		return fmt.Errorf("浼氳瘽鏈垵濮嬪寲: %s", sessionID)
	}
	return sh.session.WindowChange(rows, cols)
}

// GetLatency 鑾峰彇杩炴帴寤惰繜
func (s *SSHClient) GetLatency() int64 {
	return s.latency
}

// loadPrivateKey 鍔犺浇绉侀挜鏂囦欢
func loadPrivateKey(keyPath string) (ssh.AuthMethod, error) {
	buffer, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("璇诲彇瀵嗛挜鏂囦欢澶辫触: %v", err)
	}
	return parsePrivateKeyContent(buffer)
}

// parsePrivateKeyContent 瑙ｆ瀽绉侀挜鍐呭
func parsePrivateKeyContent(buffer []byte) (ssh.AuthMethod, error) {
	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("瑙ｆ瀽绉侀挜澶辫触: %v", err)
	}
	return ssh.PublicKeys(key), nil
}

