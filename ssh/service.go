package ssh

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SSHService SSH 鏈嶅姟锛圵ails 鏆撮湶缁欏墠绔級銆?// 鑱岃矗锛氱紪鎺掕繛鎺ャ€丼hell銆佹枃浠舵搷浣溿€佸垎缁勩€佺獥鍙ｃ€佷簨浠跺箍鎾€?type SSHService struct {
	mu            sync.RWMutex
	clients       map[string]*SSHClient
	storage       *StorageManager
	windowManager *WindowManager
	groupManager  *GroupManager
	auditLogger   *AuditLogger
	monitorHistory *MonitorHistory
	app           *application.App
	shellSessions map[string]bool

	reconnectMu      sync.RWMutex
	reconnectConfigs map[string]*SSHConfig

	directoryUploadCancelMu sync.Mutex
	directoryUploadCancelled bool
}

// NewSSHService 鍒涘缓 SSH 鏈嶅姟瀹炰緥
func NewSSHService() *SSHService {
	return &SSHService{
		clients:          make(map[string]*SSHClient),
		storage:          NewStorageManager(),
		groupManager:     NewGroupManager(),
		auditLogger:      NewAuditLogger(),
		monitorHistory:   NewMonitorHistory(),
		shellSessions:    make(map[string]bool),
		reconnectConfigs: make(map[string]*SSHConfig),
	}
}

// SetApp 璁剧疆 Wails 搴旂敤瀹炰緥
func (s *SSHService) SetApp(app *application.App) {
	s.app = app
}

// GetClient 鑾峰彇鎸囧畾杩炴帴鐨?SSHClient
func (s *SSHService) GetClient(connID string) (*SSHClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	client, ok := s.clients[connID]
	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	return client, nil
}

// GetApp 鑾峰彇 Wails 搴旂敤瀹炰緥
func (s *SSHService) GetApp() *application.App {
	return s.app
}

// CancelDirectoryUpload 鍙栨秷鐩綍涓婁紶
func (s *SSHService) CancelDirectoryUpload() {
	s.directoryUploadCancelMu.Lock()
	defer s.directoryUploadCancelMu.Unlock()
	s.directoryUploadCancelled = true
}

// ResetDirectoryUploadCancel 閲嶇疆鍙栨秷鏍囧織
func (s *SSHService) ResetDirectoryUploadCancel() {
	s.directoryUploadCancelMu.Lock()
	defer s.directoryUploadCancelMu.Unlock()
	s.directoryUploadCancelled = false
}

// IsDirectoryUploadCancelled 妫€鏌ユ槸鍚﹁鍙栨秷
func (s *SSHService) IsDirectoryUploadCancelled() bool {
	s.directoryUploadCancelMu.Lock()
	defer s.directoryUploadCancelMu.Unlock()
	return s.directoryUploadCancelled
}

// SetWindowManager 璁剧疆绐楀彛绠＄悊鍣?func (s *SSHService) SetWindowManager(wm *WindowManager) {
	s.windowManager = wm
}

// ClearWindowPositions 娓呴櫎鎵€鏈夌獥鍙ｄ綅缃蹇?func (s *SSHService) ClearWindowPositions() {
	if s.windowManager != nil {
		s.windowManager.ClearPositions()
	}
}

// formatSSHError 灏?SSH 閿欒杞崲涓哄弸濂芥彁绀?func formatSSHError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()

	switch {
	case contains(errMsg, "unable to authenticate", "authentication failed"):
		return &FriendlyError{Message: "璁よ瘉澶辫触锛氱敤鎴峰悕鎴栧瘑鐮侀敊璇?}
	case contains(errMsg, "timeout", "i/o timeout"):
		return &FriendlyError{Message: "杩炴帴瓒呮椂锛氳妫€鏌ョ綉缁滆繛鎺ユ垨鏈嶅姟鍣ㄥ湴鍧€"}
	case contains(errMsg, "connection refused"):
		return &FriendlyError{Message: "杩炴帴琚嫆缁濓細璇锋鏌ユ湇鍔″櫒鏄惁杩愯SSH鏈嶅姟"}
	case contains(errMsg, "no route to host", "network is unreachable"):
		return &FriendlyError{Message: "缃戠粶涓嶅彲杈撅細璇锋鏌ョ綉缁滆繛鎺?}
	case contains(errMsg, "handshake failed"):
		return &FriendlyError{Message: "杩炴帴澶辫触锛氭棤娉曚笌鏈嶅姟鍣ㄥ缓绔嬪畨鍏ㄨ繛鎺?}
	}

	if idx := strings.Index(errMsg, ":"); idx > 0 {
		return &FriendlyError{Message: "杩炴帴澶辫触锛? + errMsg[:idx]}
	}
	return &FriendlyError{Message: "杩炴帴澶辫触锛? + errMsg}
}

func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// FriendlyError 鍙嬪ソ鐨勯敊璇被鍨?type FriendlyError struct{ Message string }

func (e *FriendlyError) Error() string { return e.Message }

// TestConnection 娴嬭瘯 SSH 杩炴帴
func (s *SSHService) TestConnection(config *SSHConfig) error {
	if config.Password == "" && config.PrivateKey == "" && config.KeyPath == "" {
		if stored := s.findStoredPassword(config.Host, config.Port, config.Username); stored != "" {
			config.Password = stored
		}
	}
	client := NewSSHClient(config)
	if err := client.Connect(); err != nil {
		return formatSSHError(err)
	}
	client.Close()
	return nil
}

// Connect 杩炴帴鍒?SSH 鏈嶅姟鍣?func (s *SSHService) Connect(connID string, config *SSHConfig) error {
	if config.Password == "" && config.PrivateKey == "" && config.KeyPath == "" {
		if stored := s.findStoredPassword(config.Host, config.Port, config.Username); stored != "" {
			config.Password = stored
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	client := NewSSHClient(config)
	if err := client.Connect(); err != nil {
		return formatSSHError(err)
	}
	s.clients[connID] = client
	return nil
}

// CreateAndConnect 鍒涘缓鏂拌繛鎺ュ苟寤虹珛 SSH 杩炴帴
func (s *SSHService) CreateAndConnect(config *SSHConfig) (string, error) {
	result, err := s.CreateAndConnectWithGroup(config, "group-1")
	if err != nil {
		return "", err
	}
	return result["connID"], nil
}

// findStoredPassword 浠庡瓨鍌ㄤ腑鏌ユ壘宸蹭繚瀛樿繛鎺ョ殑瀵嗙爜锛堟寜 host:port:username 鍖归厤锛?func (s *SSHService) findStoredPassword(host string, port int, username string) string {
	for _, c := range s.storage.GetAllConnections() {
		if c.Host == host && c.Port == port && c.Username == username && c.Password != "" {
			return c.Password
		}
	}
	return ""
}

// CreateAndConnectWithGroup 鍒涘缓杩炴帴骞舵寚瀹氬垎缁?func (s *SSHService) CreateAndConnectWithGroup(config *SSHConfig, groupID string) (map[string]string, error) {
	if config == nil {
		return nil, &FriendlyError{Message: "杩炴帴閰嶇疆涓嶈兘涓虹┖"}
	}

	// 瀹夊叏璁捐锛氬墠绔笉鎺ユ敹瀵嗙爜锛坖son:"-"锛夛紝杩炴帴鏃跺悗绔嚜鍔ㄤ粠瀛樺偍琛ュ叏
	if config.Password == "" && config.PrivateKey == "" && config.KeyPath == "" {
		if stored := s.findStoredPassword(config.Host, config.Port, config.Username); stored != "" {
			config.Password = stored
		}
	}

	connID := generateConnectionID()

	connInfo := &ConnectionInfo{
		ID:           connID,
		Name:         config.Name,
		Host:         config.Host,
		Port:         config.Port,
		Username:     config.Username,
		Password:     config.Password,
		KeyPath:      config.KeyPath,
		PrivateKey:   config.PrivateKey,
		JumpHost:     config.JumpHost,
		JumpUsername: config.JumpUsername,
		JumpPassword: config.JumpPassword,
		JumpKeyPath:  config.JumpKeyPath,
		Status:       "disconnected",
		Saved:        true,
		GroupID:      groupID,
	}

	if err := s.storage.AddConnection(connInfo); err != nil {
		return nil, &FriendlyError{Message: "淇濆瓨杩炴帴閰嶇疆澶辫触: " + err.Error()}
	}

	type connectResult struct {
		client *SSHClient
		err    error
	}
	resultChan := make(chan connectResult, 1)

	go func() {
		client := NewSSHClient(config)
		client.SetDisconnectCallback(connID, s.onConnectionDisconnected)
		err := client.Connect()
		resultChan <- connectResult{client: client, err: err}
	}()

	timeout := time.Duration(30) * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	select {
	case result := <-resultChan:
		if result.err != nil {
			s.storage.DeleteFromCache(connID)
			return nil, formatSSHError(result.err)
		}
		s.mu.Lock()
		s.clients[connID] = result.client
		s.mu.Unlock()

	case <-time.After(timeout):
		s.storage.DeleteFromCache(connID)
		return nil, &FriendlyError{Message: fmt.Sprintf("杩炴帴瓒呮椂锛氭湇鍔″櫒鍦?%v 鍐呮湭鍝嶅簲", timeout)}
	}

	s.storage.UpdateConnectionStatus(connID, "connected")

	group := s.groupManager.GetGroup(groupID)
	if group == nil {
		s.groupManager.CreateGroupWithName(groupID, config.Name)
	}
	if err := s.groupManager.AddConnectionToGroup(groupID, connID); err != nil {
		return nil, err
	}

	return map[string]string{"connID": connID, "groupID": groupID}, nil
}

// Disconnect 鏂紑 SSH 杩炴帴
func (s *SSHService) Disconnect(connID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil
	}

	client.Close()
	delete(s.clients, connID)
	delete(s.shellSessions, connID)
	s.storage.DeleteFromCache(connID)
	s.broadcastConnections()
	return nil
}

// GetServerKey 鏍规嵁杩炴帴 ID 杩斿洖鏈嶅姟鍣ㄦ爣璇?func (s *SSHService) GetServerKey(connID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if client, ok := s.clients[connID]; ok {
		return fmt.Sprintf("%s:%d", client.config.Host, client.config.Port)
	}
	return ""
}

// ResolveCdPath 鍦ㄧ嫭绔?session 涓墽琛?cd 骞惰繑鍥炵湡瀹炶矾寰?func (s *SSHService) ResolveCdPath(connID, currentDir, targetPath string) (string, error) {
	shellArg := func(a string) string {
		if a == "~" || strings.HasPrefix(a, "~/") {
			return a
		}
		return "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
	}
	cmd := fmt.Sprintf("cd %s && cd %s && pwd", shellArg(currentDir), shellArg(targetPath))
	output, err := s.RunCommand(connID, cmd)
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(output)
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("璺緞瑙ｆ瀽澶辫触: %s", output)
	}
	return path, nil
}

// RunCommand 鎵ц鍛戒护骞惰繑鍥炶緭鍑哄瓧绗︿覆
func (s *SSHService) RunCommand(connID, command string) (string, error) {
	result, err := s.ExecuteCommand(connID, command)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return fmt.Sprintf("鍛戒护鎵ц澶辫触 (exit code %d):\n%s\n%s", result.ExitCode, result.Stdout, result.Stderr), nil
	}
	output := result.Stdout
	if result.Stderr != "" {
		output += "\n[stderr] " + result.Stderr
	}
	return output, nil
}

// ExecuteCommand 鎵ц杩滅▼鍛戒护锛堣嚜鍔ㄨ褰曞璁℃棩蹇楋級
func (s *SSHService) ExecuteCommand(connID string, command string) (*CommandResult, error) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	var host, username string
	if ok {
		host = client.config.Host
		username = client.config.Username
	}
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}

	result, err := client.ExecuteCommand(command)

	// 璁板綍瀹¤鏃ュ織
	success := err == nil && result != nil && result.Success
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	} else if result != nil && !result.Success {
		errMsg = result.Stderr
	}
	s.auditLogger.Log(connID, host, username, "command", command, success, errMsg)

	return result, err
}

// ExecuteCommandWithSudo 鎵ц杩滅▼鍛戒护锛堟潈闄愪笉瓒虫椂鑷姩鐢?SSH 瀵嗙爜 sudo 鎻愭潈锛?func (s *SSHService) ExecuteCommandWithSudo(connID string, command string) (*CommandResult, error) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}

	// 鍏堢洿鎺ユ墽琛?	result, err := client.ExecuteCommand(command)
	if err != nil {
		return nil, err
	}
	if result.Success {
		return result, nil
	}

	// 妫€鏌ユ槸鍚︽槸鏉冮檺闂
	errOut := strings.TrimSpace(result.Stderr)
	if errOut == "" {
		errOut = strings.TrimSpace(result.Stdout)
	}
	isPermErr := strings.Contains(errOut, "Permission denied") ||
		strings.Contains(errOut, "permission denied") ||
		strings.Contains(errOut, "Cannot open")

	if !isPermErr {
		return result, nil
	}

	// 鐢?SSH 瀵嗙爜閫氳繃 sudo -S 鎻愭潈
	password := client.config.Password
	if password == "" {
		return result, fmt.Errorf("鏉冮檺涓嶈冻涓旀湭閰嶇疆瀵嗙爜锛屾棤娉曟彁鏉?)
	}

	encodedPw := base64.StdEncoding.EncodeToString([]byte(password))
	sudoCmd := fmt.Sprintf("echo %s | base64 -d | sudo -S sh -c %s 2>&1",
		encodedPw, shellQuote(command))
	sudoResult, sudoErr := client.ExecuteCommand(sudoCmd)
	if sudoErr != nil {
		return result, fmt.Errorf("sudo 鎵ц澶辫触: %v", sudoErr)
	}
	if sudoResult.Success {
		return sudoResult, nil
	}

	errMsg := strings.TrimSpace(sudoResult.Stderr)
	if errMsg == "" {
		errMsg = strings.TrimSpace(sudoResult.Stdout)
	}
	return sudoResult, fmt.Errorf("sudo 鎵ц澶辫触: %s", errMsg)
}

// ReadFileForUpload 璇诲彇鏈湴鏂囦欢骞惰繑鍥?base64 缂栫爜锛堢敤浜庢嫋鏀句笂浼狅級
func (s *SSHService) ReadFileForUpload(connID string, filePath string) (string, error) {
	s.mu.RLock()
	_, ok := s.clients[connID]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("璇诲彇鏂囦欢澶辫触: %v", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// IsLocalDirectory 妫€鏌ユ湰鍦拌矾寰勬槸鍚︽槸鐩綍
func (s *SSHService) IsLocalDirectory(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// UploadFile 涓婁紶鏂囦欢
func (s *SSHService) UploadFile(connID string, remotePath string, data string) error {
	s.mu.RLock()
	client, ok := s.clients[connID]
	app := s.app
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}

	decodedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("base64 瑙ｇ爜澶辫触: %v", err)
	}

	progressCallback := func(percent int) {
		if app != nil {
			app.Event.Emit("ssh:upload-progress", map[string]interface{}{
				"connId":   connID,
				"path":     remotePath,
				"progress": percent,
			})
		}
	}

	return client.UploadFileFromBytes(remotePath, decodedData, progressCallback)
}

// DownloadFile 涓嬭浇鏂囦欢
func (s *SSHService) DownloadFile(connID string, remotePath string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	return client.DownloadFileToBytes(remotePath)
}

// DownloadFileRange 涓嬭浇鏂囦欢鎸囧畾鑼冨洿锛堢敤浜庢柇鐐圭画浼狅級
func (s *SSHService) DownloadFileRange(connID string, remotePath string, offset int64, limit int64) ([]byte, error) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	return client.DownloadFileRange(remotePath, offset, limit)
}

// GetRemoteFileSize 鑾峰彇杩滅▼鏂囦欢澶у皬锛堢敤浜庡墠绔垽鏂槸鍚﹂渶瑕佺画浼狅級
func (s *SSHService) GetRemoteFileSize(connID string, remotePath string) (int64, error) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	if client.sftpClient == nil {
		if err := client.InitSFTP(); err != nil {
			return 0, err
		}
	}
	stat, err := client.sftpClient.Stat(remotePath)
	if err != nil {
		return 0, nil // 鏂囦欢涓嶅瓨鍦ㄨ繑鍥?0
	}
	return stat.Size(), nil
}

// ListDirectory 鍒楀嚭鐩綍
func (s *SSHService) ListDirectory(connID string, remotePath string) ([]FileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil, nil
	}
	return client.ListDirectory(remotePath)
}

// DeleteFile 鍒犻櫎鏂囦欢
func (s *SSHService) DeleteFile(connID string, remotePath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	return client.DeleteFile(remotePath)
}

// CreateDirectory 鍒涘缓鐩綍
func (s *SSHService) CreateDirectory(connID string, remotePath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.CreateDirectory(remotePath)
}

// RenameFile 閲嶅懡鍚嶆枃浠?鐩綍
func (s *SSHService) RenameFile(connID string, oldPath, newPath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.RenameFile(oldPath, newPath)
}

// CopyFile 澶嶅埗鏂囦欢
func (s *SSHService) CopyFile(connID string, srcPath, destPath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.CopyFile(srcPath, destPath)
}

// UploadDirectory 涓婁紶鐩綍
func (s *SSHService) UploadDirectory(connID string, localPath string, remotePath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.UploadDirectory(localPath, remotePath)
}

// CreateArchive 鍒涘缓鍘嬬缉鍖?func (s *SSHService) CreateArchive(connID string, files []string, archiveName string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return "", fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.CreateArchive(files, archiveName)
}

// DeleteTempFile 鍒犻櫎涓存椂鏂囦欢
func (s *SSHService) DeleteTempFile(connID string, filePath string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.DeleteTempFile(filePath)
}

// ExtractArchive 瑙ｅ帇鍘嬬缉鍖?func (s *SSHService) ExtractArchive(connID string, archivePath string, targetDir string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.ExtractArchive(archivePath, targetDir)
}

// SelectLocalDirectoryAndUpload 閫夋嫨鏈湴鐩綍骞朵笂浼?func (s *SSHService) SelectLocalDirectoryAndUpload(connID string, remotePath string) error {
	if s.app == nil {
		return fmt.Errorf("搴旂敤瀹炰緥鏈垵濮嬪寲")
	}

	s.ResetDirectoryUploadCancel()

	localDir, err := s.app.Dialog.OpenFile().
		SetTitle("閫夋嫨瑕佷笂浼犵殑鐩綍").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()

	if err != nil || localDir == "" {
		return fmt.Errorf("鐢ㄦ埛鍙栨秷閫夋嫨鎴栧彂鐢熼敊璇?)
	}

	s.emitDirectoryProgress(connID, "compressing", 10, "姝ｅ湪鍘嬬缉鐩綍...")

	tempArchive := filepath.Join(os.TempDir(), fmt.Sprintf("directory_upload_%d.tar.gz", time.Now().Unix()))
	if err := s.createTarGzFromDirectory(localDir, tempArchive); err != nil {
		return fmt.Errorf("鍘嬬缉鏈湴鐩綍澶辫触: %v", err)
	}

	archiveInfo, _ := os.Stat(tempArchive)
	if archiveInfo != nil {
		s.emitDirectoryProgress(connID, "uploading", 40, "姝ｅ湪涓婁紶鍘嬬缉鍖?..")
	}

	archiveData, err := os.ReadFile(tempArchive)
	os.Remove(tempArchive)
	if err != nil {
		return fmt.Errorf("璇诲彇鍘嬬缉鏂囦欢澶辫触: %v", err)
	}

	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}

	remoteArchivePath := fmt.Sprintf("%s/temp_upload_%d.tar.gz", remotePath, time.Now().Unix())
	if err := s.uploadFileWithProgress(client, remoteArchivePath, archiveData); err != nil {
		if s.IsDirectoryUploadCancelled() {
			return fmt.Errorf("鐢ㄦ埛鍙栨秷浜嗕笂浼?)
		}
		return fmt.Errorf("涓婁紶鍘嬬缉鏂囦欢澶辫触: %v", err)
	}

	s.emitDirectoryProgress(connID, "extracting", 80, "姝ｅ湪瑙ｅ帇...")

	if err := client.ExtractArchive(remoteArchivePath, remotePath); err != nil {
		return fmt.Errorf("瑙ｅ帇澶辫触: %v", err)
	}

	client.DeleteTempFile(remoteArchivePath)
	return nil
}

func (s *SSHService) emitDirectoryProgress(connID, stage string, progress int, message string) {
	if s.app != nil {
		s.app.Event.Emit("directory-upload-progress", map[string]interface{}{
			"stage":    stage,
			"progress": progress,
			"message":  message,
		})
	}
}

func (s *SSHService) uploadFileWithProgress(client *SSHClient, remotePath string, data []byte) error {
	if client.sftpClient == nil {
		if err := client.InitSFTP(); err != nil {
			return err
		}
	}

	remoteFile, err := client.sftpClient.Create(remotePath)
	if err != nil {
		// SFTP 鏉冮檺涓嶈冻锛屽洖閫€鍒?sudo 鏂瑰紡
		return client.uploadViaSudo(remotePath, data, nil)
	}
	defer remoteFile.Close()

	const chunkSize = 1024 * 1024
	totalSize := len(data)
	uploadedSize := 0

	for uploadedSize < totalSize {
		if s.IsDirectoryUploadCancelled() {
			remoteFile.Close()
			client.DeleteTempFile(remotePath)
			return fmt.Errorf("鐢ㄦ埛鍙栨秷浜嗕笂浼?)
		}

		end := uploadedSize + chunkSize
		if end > totalSize {
			end = totalSize
		}

		written, err := remoteFile.Write(data[uploadedSize:end])
		if err != nil {
			client.DeleteTempFile(remotePath)
			return fmt.Errorf("鍐欏叆鏁版嵁澶辫触: %v", err)
		}

		uploadedSize += written

		if s.app != nil {
			progress := 40 + int(float64(uploadedSize)/float64(totalSize)*40)
			s.app.Event.Emit("directory-upload-progress", map[string]interface{}{
				"stage":    "uploading",
				"progress": progress,
				"message":  fmt.Sprintf("姝ｅ湪涓婁紶... (%.1f MB / %.1f MB)", float64(uploadedSize)/(1024*1024), float64(totalSize)/(1024*1024)),
			})
		}
	}

	return nil
}

func (s *SSHService) createTarGzFromDirectory(sourceDir string, destArchive string) error {
	file, err := os.Create(destArchive)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	baseName := filepath.Base(sourceDir)

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		relPath = baseName + "/" + relPath
		relPath = filepath.ToSlash(relPath)

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return err
			}
			defer data.Close()
			_, err = io.Copy(tarWriter, data)
			return err
		}

		return nil
	})
}

// IsConnected 妫€鏌ヨ繛鎺ョ姸鎬?func (s *SSHService) IsConnected(connID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return false
	}
	return client.IsConnected()
}

// GetConnectionList 鑾峰彇鎵€鏈夎繛鎺?ID 鍒楄〃
func (s *SSHService) GetConnectionList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.clients))
	for id := range s.clients {
		ids = append(ids, id)
	}
	return ids
}

// ========== 杩炴帴瀛樺偍绠＄悊 API ==========

func (s *SSHService) AddConnection(conn *ConnectionInfo) error       { return s.storage.AddConnection(conn) }
func (s *SSHService) UpdateConnection(conn *ConnectionInfo) error    { return s.storage.UpdateConnection(conn) }
func (s *SSHService) DeleteConnection(id string) error               { return s.storage.DeleteConnection(id) }
func (s *SSHService) GetConnection(id string) (*ConnectionInfo, error) { return s.storage.GetConnection(id) }

func (s *SSHService) SyncImportConnection(conn *ConnectionInfo) error {
	err := s.storage.SyncImportConnection(conn)
	if err == nil {
		s.broadcastConnections()
	}
	return err
}

// ImportSSHConfig 瑙ｆ瀽 ~/.ssh/config 鏂囦欢锛岃繑鍥炲彲鐢ㄧ殑杩炴帴鍒楄〃
func (s *SSHService) ImportSSHConfig() ([]SSHConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("鑾峰彇鐢ㄦ埛涓荤洰褰曞け璐? %v", err)
	}

	configPath := filepath.Join(home, ".ssh", "config")
	entries, err := ParseSSHConfig(configPath)
	if err != nil {
		return nil, err
	}

	var configs []SSHConfig
	for _, e := range entries {
		configs = append(configs, SSHConfig{
			Name:       e.Host,
			Host:       e.HostName,
			Port:       e.Port,
			Username:   e.User,
			KeyPath:    e.IdentityFile,
		})
	}

	return configs, nil
}

// GetAllConnections 鑾峰彇鎵€鏈夎繛鎺ワ紙鍚垎缁?ID 鍜岀湡瀹炲湪绾跨姸鎬侊級
func (s *SSHService) GetAllConnections() []*ConnectionInfo {
	connections := s.storage.GetAllConnections()
	var result []*ConnectionInfo
	for _, conn := range connections {
		if !conn.Saved {
			continue
		}
		group := s.groupManager.GetGroupByConnID(conn.ID)
		if group != nil {
			conn.GroupID = group.ID
		}
		// 鎸?host:port:username 鍖归厤鍦ㄧ嚎鐘舵€侊紙瀛樺偍 ID 鍙兘涓庢椿璺冭繛鎺?ID 涓嶅悓锛?		s.mu.RLock()
		isOnline := false
		for _, client := range s.clients {
			if client.config.Host == conn.Host && client.config.Port == conn.Port && client.config.Username == conn.Username {
				isOnline = true
				break
			}
		}
		s.mu.RUnlock()
		if isOnline {
			conn.Status = "connected"
		} else {
			conn.Status = "disconnected"
		}
		result = append(result, conn)
	}
	return result
}

func (s *SSHService) SaveConnection(id string) error {
	err := s.storage.MarkAsSaved(id)
	if err == nil {
		s.broadcastConnections()
	}
	return err
}

func (s *SSHService) UnsaveConnection(id string) {
	s.storage.MarkAsUnsaved(id)
}

func (s *SSHService) broadcastConnections() {
	if s.app == nil {
		return
	}
	all := s.GetAllConnections()
	if all == nil {
		all = []*ConnectionInfo{}
	}
	s.app.Event.Emit("ssh:connections-updated", map[string]interface{}{
		"connections": all,
		"timestamp":   time.Now().UnixMilli(),
	})
}

// ========== 鍒嗙粍绠＄悊 API ==========

func (s *SSHService) CreateGroup(name string) string    { return s.groupManager.CreateGroup(name).ID }
func (s *SSHService) GetAllGroups() []*SSHGroup         { return s.groupManager.GetAllGroups() }
func (s *SSHService) GetDefaultGroupID() string         { return "group-1" }
func (s *SSHService) AddToGroup(groupID, connID string) error { return s.groupManager.AddConnectionToGroup(groupID, connID) }
func (s *SSHService) RemoveFromGroup(groupID, connID string) error { return s.groupManager.RemoveConnectionFromGroup(groupID, connID) }

// UpdateGroup 鏇存柊鍒嗙粍灞炴€э紙鍚嶇О銆侀鑹层€佺埗鍒嗙粍锛?func (s *SSHService) UpdateGroup(groupID, name, color, parentID string) error {
	return s.groupManager.UpdateGroup(groupID, name, color, parentID)
}

// ReorderGroups 鏇存柊鍒嗙粍鎺掑簭
func (s *SSHService) ReorderGroups(groupIDs []string) error {
	return s.groupManager.ReorderGroups(groupIDs)
}

// GetChildGroups 鑾峰彇瀛愬垎缁?func (s *SSHService) GetChildGroups(parentID string) []*SSHGroup {
	return s.groupManager.GetChildGroups(parentID)
}

func (s *SSHService) GetGroupConnections(groupID string) []string {
	group := s.groupManager.GetGroup(groupID)
	if group == nil {
		return []string{}
	}
	return group.ConnIDs
}

func (s *SSHService) GetGroupConnectionInfos(groupID string) []*ConnectionInfo {
	group := s.groupManager.GetGroup(groupID)
	if group == nil {
		return []*ConnectionInfo{}
	}
	var infos []*ConnectionInfo
	for _, connID := range group.ConnIDs {
		conn, err := s.storage.GetConnection(connID)
		if err == nil && conn != nil {
			infos = append(infos, conn)
		}
	}
	return infos
}

// CloseGroup 鍏抽棴鍒嗙粍
func (s *SSHService) CloseGroup(groupID string) error {
	group := s.groupManager.GetGroup(groupID)
	if group == nil {
		return nil
	}

	connIDs := make([]string, len(group.ConnIDs))
	copy(connIDs, group.ConnIDs)

	for _, connID := range connIDs {
		s.mu.Lock()
		if client, ok := s.clients[connID]; ok {
			client.Close()
			delete(s.clients, connID)
			s.storage.DeleteFromCache(connID)
		}
		s.mu.Unlock()
	}

	s.groupManager.ClearGroup(groupID)

	if s.app != nil {
		s.app.Event.Emit("ssh:group-updated", map[string]interface{}{
			"groupID":     groupID,
			"connections": []string{},
		})
	}

	s.groupManager.DeleteGroup(groupID)
	return nil
}

func (s *SSHService) OpenSSHWindow(groupID string, groupName string, activeConnID string) error {
	if s.windowManager == nil {
		return &FriendlyError{Message: "绐楀彛绠＄悊鍣ㄦ湭鍒濆鍖?}
	}
	return s.windowManager.CreateSSHWindow(groupID, groupName, activeConnID)
}

func (s *SSHService) GetGroupByConnID(connID string) *SSHGroup {
	return s.groupManager.GetGroupByConnID(connID)
}

// ========== Shell 浼氳瘽 API ==========

func (s *SSHService) StartShellSession(connID string) error {
	return s.StartShellSessionWithID(connID, "default")
}

func (s *SSHService) StartShellSessionWithID(connID, sessionID string) error {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}

	if _, err := client.CreateShell(sessionID); err != nil {
		// 妫€鏌ユ槸鍚︽槸 SSH 浼氳瘽鏁伴檺鍒堕棶棰?		errMsg := err.Error()
		if strings.Contains(errMsg, "open failed") || strings.Contains(errMsg, "rejected") {
			return fmt.Errorf("鍒涘缓缁堢澶辫触锛歋SH 鏈嶅姟鍣ㄥ彲鑳藉凡杈惧埌鏈€澶т細璇濇暟闄愬埗銆傝鍏抽棴鍏朵粬缁堢鍚庨噸璇?)
		}
		return fmt.Errorf("鍒涘缓缁堢澶辫触: %v", err)
	}

	s.mu.Lock()
	s.shellSessions[connID] = true
	s.mu.Unlock()
	return nil
}

func (s *SSHService) WriteToTerminal(connID string, data string) error {
	return s.WriteToTerminalByID(connID, "default", data)
}

func (s *SSHService) WriteToTerminalByID(connID, sessionID string, data string) error {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	if !client.IsShellActive(sessionID) {
		return fmt.Errorf("Shell浼氳瘽鏈垵濮嬪寲: %s", sessionID)
	}
	return client.WriteToShell(sessionID, []byte(data))
}

func (s *SSHService) ReadFromShellSession(connID, sessionID string, buf []byte) (int, error) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.ReadFromShell(sessionID, buf)
}

func (s *SSHService) IsShellSessionActive(connID, sessionID string) bool {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return client.IsShellActive(sessionID)
}

func (s *SSHService) GetSessionIDs(connID string) []string {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return client.GetSessionIDs()
}

func (s *SSHService) ResizeTerminal(connID string, cols, rows int) error {
	return s.ResizeTerminalByID(connID, "default", cols, rows)
}

func (s *SSHService) ResizeTerminalByID(connID, sessionID string, cols, rows int) error {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.ResizeTerminalByID(sessionID, cols, rows)
}

func (s *SSHService) IsShellActive(connID string) bool {
	return s.IsShellSessionActive(connID, "default")
}

func (s *SSHService) CloseShellSession(connID string) error {
	return s.CloseShellSessionByID(connID, "default")
}

func (s *SSHService) CloseShellSessionByID(connID, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	client.CloseShell(sessionID)
	return nil
}

func (s *SSHService) ReadFromShell(connID string, buf []byte) (int, error) {
	return s.ReadFromShellSession(connID, "default", buf)
}

var connIDCounter int64

func generateConnectionID() string {
	connIDCounter++
	return fmt.Sprintf("conn_%d_%d", time.Now().UnixNano(), connIDCounter)
}

// ========== 绯荤粺鐩戞帶 API ==========

func (s *SSHService) GetLatency(connID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return 0
	}
	return client.UpdateLatency()
}

func (s *SSHService) GetSystemStats(connID string) (*SystemStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}

	stats, err := client.GetSystemStats(context.Background())
	if err != nil {
		return nil, err
	}

	// 鑷姩璁板綍鐩戞帶蹇収
	if stats != nil {
		var diskPercent float64
		var diskUsed, diskTotal uint64
		if len(stats.Disk.Partitions) > 0 {
			diskPercent = stats.Disk.Partitions[0].UsedPercent
			diskUsed = stats.Disk.Partitions[0].Used
			diskTotal = stats.Disk.Partitions[0].Total
		}
		s.monitorHistory.Add(connID, MonitorSnapshot{
			Timestamp:   time.Now().UnixMilli(),
			CPUPercent:  stats.CPU.UsagePercent,
			MemPercent:  stats.Memory.UsedPercent,
			MemUsed:     stats.Memory.Used,
			MemTotal:    stats.Memory.Total,
			DiskPercent: diskPercent,
			DiskUsed:    diskUsed,
			DiskTotal:   diskTotal,
			NetRx:       stats.Network.TotalRx,
			NetTx:       stats.Network.TotalTx,
		})
	}

	return stats, nil
}

// GetMonitorHistory 鑾峰彇鐩戞帶鍘嗗彶鏁版嵁
func (s *SSHService) GetMonitorHistory(connID string, count int) []MonitorSnapshot {
	return s.monitorHistory.Get(connID, count)
}

// GetMonitorHistorySince 鑾峰彇鎸囧畾鏃堕棿浠ユ潵鐨勭洃鎺у巻鍙?func (s *SSHService) GetMonitorHistorySince(connID string, since int64) []MonitorSnapshot {
	return s.monitorHistory.GetSince(connID, since)
}

func (s *SSHService) GetProcessList(connID string) ([]ProcessInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.GetProcessList(context.Background())
}

func (s *SSHService) KillProcess(connID string, pid int32) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.KillProcess(context.Background(), pid)
}

func (s *SSHService) SendSignal(connID string, pid int32, signal string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.SendSignal(context.Background(), pid, signal)
}

func (s *SSHService) GetProcessDetail(connID string, pid int32) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return "", fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.GetProcessDetail(context.Background(), pid)
}

// ========== 鏂囦欢鎼滅储 API ==========

func (s *SSHService) ListFiles(connID string, path string) ([]FileInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.ListDirectory(path)
}

// GetFileOwners 鎸夐渶鑾峰彇鏂囦欢鐨?owner/group锛堝墠绔紓姝ヨ皟鐢級
func (s *SSHService) GetFileOwners(connID string, dir string, filenames []string) map[string][2]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return nil
	}
	return client.GetFileOwners(dir, filenames)
}

func (s *SSHService) SearchFiles(connID string, basePath string, keyword string, searchID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	return client.SearchFiles(basePath, keyword, s.app, searchID)
}

func (s *SSHService) CancelSearch(connID string, searchID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦?)
	}
	client.CancelSearch(searchID)
	return nil
}

// ========== 鏂嚎閲嶈繛 ==========

func (s *SSHService) onConnectionDisconnected(connID string) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	if !ok {
		s.mu.RUnlock()
		return
	}
	config := client.config
	s.mu.RUnlock()

	s.reconnectMu.Lock()
	s.reconnectConfigs[connID] = config
	s.reconnectMu.Unlock()

	s.storage.UpdateConnectionStatus(connID, "disconnected")

	if s.app != nil {
		s.app.Event.Emit("ssh:connection-disconnected", map[string]interface{}{
			"connID":    connID,
			"host":      config.Host,
			"timestamp": time.Now().UnixMilli(),
		})
	}

	s.broadcastConnections()

	// 鑷姩閲嶈繛锛堝悗鍙版墽琛岋紝涓嶉樆濉烇級
	s.AutoReconnect(connID)
}

// Reconnect 閲嶆柊杩炴帴
func (s *SSHService) Reconnect(connID string) error {
	s.reconnectMu.RLock()
	config, exists := s.reconnectConfigs[connID]
	s.reconnectMu.RUnlock()

	if !exists {
		connInfo, err := s.storage.GetConnection(connID)
		if err != nil || connInfo == nil {
			return &FriendlyError{Message: "鏃犳硶鎵惧埌杩炴帴閰嶇疆锛岃閲嶆柊杩炴帴"}
		}
		config = &SSHConfig{
			Name:         connInfo.Name,
			Host:         connInfo.Host,
			Port:         connInfo.Port,
			Username:     connInfo.Username,
			Password:     connInfo.Password,
			KeyPath:      connInfo.KeyPath,
			PrivateKey:   connInfo.PrivateKey,
			JumpHost:     connInfo.JumpHost,
			JumpUsername: connInfo.JumpUsername,
			JumpPassword: connInfo.JumpPassword,
			JumpKeyPath:  connInfo.JumpKeyPath,
		}
	}

	s.mu.Lock()
	if oldClient, ok := s.clients[connID]; ok {
		oldClient.Close()
		delete(s.clients, connID)
	}
	delete(s.shellSessions, connID)
	s.mu.Unlock()

	client := NewSSHClient(config)
	client.SetDisconnectCallback(connID, s.onConnectionDisconnected)

	if s.app != nil {
		s.app.Event.Emit("ssh:connection-reconnecting", map[string]interface{}{"connID": connID})
	}

	if err := client.Connect(); err != nil {
		if s.app != nil {
			s.app.Event.Emit("ssh:connection-reconnect-failed", map[string]interface{}{
				"connID": connID,
				"error":  formatSSHError(err).Error(),
			})
		}
		return formatSSHError(err)
	}

	s.mu.Lock()
	s.clients[connID] = client
	s.mu.Unlock()

	s.storage.UpdateConnectionStatus(connID, "connected")

	s.reconnectMu.Lock()
	delete(s.reconnectConfigs, connID)
	s.reconnectMu.Unlock()

	if s.app != nil {
		s.app.Event.Emit("ssh:connection-reconnected", map[string]interface{}{"connID": connID})
	}

	s.broadcastConnections()
	return nil
}

// AutoReconnect 鑷姩閲嶈繛锛堟寚鏁伴€€閬匡紝鏈€澶氶噸璇?10 娆★紝绾?3 鍒嗛挓锛?func (s *SSHService) AutoReconnect(connID string) {
	go func() {
		maxRetries := 10
		retryDelay := 2 * time.Second
		maxDelay := 30 * time.Second

		for i := 0; i < maxRetries; i++ {
			time.Sleep(retryDelay)

			if err := s.Reconnect(connID); err == nil {
				return
			}

			// 鎸囨暟閫€閬匡紝涓婇檺 30 绉?			retryDelay *= 2
			if retryDelay > maxDelay {
				retryDelay = maxDelay
			}
		}

		if s.app != nil {
			s.app.Event.Emit("ssh:connection-reconnect-failed", map[string]interface{}{
				"connID":  connID,
				"error":   "鑷姩閲嶈繛澶辫触锛岃鎵嬪姩閲嶈繛",
				"autoMax": true,
			})
		}
	}()
}

// ========== SSH 瀵嗛挜绠＄悊 API ==========

var defaultKeyManager = NewKeyManager()

// ListSSHKeys 鍒楀嚭鎵€鏈?SSH 瀵嗛挜
func (s *SSHService) ListSSHKeys() ([]SSHKeyInfo, error) {
	return defaultKeyManager.ListKeys()
}

// GenerateSSHKey 鐢熸垚鏂板瘑閽?func (s *SSHService) GenerateSSHKey(name, keyType, comment string, bits int) error {
	return defaultKeyManager.GenerateKey(name, keyType, comment, bits)
}

// DeploySSHPublicKey 閮ㄧ讲鍏挜鍒拌繙绋嬫湇鍔″櫒
func (s *SSHService) DeploySSHPublicKey(keyName, connID string) error {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	return defaultKeyManager.DeployPublicKey(keyName, client)
}

// GetSSHPublicKey 鑾峰彇鍏挜鍐呭
func (s *SSHService) GetSSHPublicKey(keyName string) (string, error) {
	return defaultKeyManager.GetPublicKey(keyName)
}

// DeleteSSHKey 鍒犻櫎瀵嗛挜
func (s *SSHService) DeleteSSHKey(keyName string) error {
	return defaultKeyManager.DeleteKey(keyName)
}

// ========== 瀹¤鏃ュ織 API ==========

// GetAuditLogs 鑾峰彇瀹¤鏃ュ織锛堟渶杩?N 鏉★級
func (s *SSHService) GetAuditLogs(limit int) []AuditEntry {
	return s.auditLogger.GetLogs(limit)
}

// GetAuditLogsByDate 鎸夋棩鏈熻幏鍙栧璁℃棩蹇?func (s *SSHService) GetAuditLogsByDate(date string) []AuditEntry {
	return s.auditLogger.GetLogsByDate(date)
}

// ExportAuditLogs 瀵煎嚭瀹¤鏃ュ織
func (s *SSHService) ExportAuditLogs(startDate, endDate string) (string, error) {
	return s.auditLogger.ExportLogs(startDate, endDate)
}

// CheckConnectionHealth 妫€鏌ヨ繛鎺ュ仴搴风姸鎬?func (s *SSHService) CheckConnectionHealth(connID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[connID]
	if !ok {
		return false
	}
	return client.IsConnected()
}

// StartHealthCheck 鍚姩鍋ュ悍妫€鏌ュ畾鏃跺櫒
func (s *SSHService) StartHealthCheck() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.mu.RLock()
			connIDs := make([]string, 0, len(s.clients))
			for id := range s.clients {
				connIDs = append(connIDs, id)
			}
			s.mu.RUnlock()

			for _, connID := range connIDs {
				if !s.CheckConnectionHealth(connID) {
					s.onConnectionDisconnected(connID)
				}
			}
		}
	}()
}

// ========== 绯荤粺鐜妫€娴?API ==========

func (s *SSHService) GetSystemEnvironments(connID string) ([]EnvInfo, error) {
	s.mu.RLock()
	client, ok := s.clients[connID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("杩炴帴涓嶅瓨鍦? %s", connID)
	}
	return client.GetSystemEnvironments()
}

