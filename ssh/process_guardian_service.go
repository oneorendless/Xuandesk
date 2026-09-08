package ssh

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// GuardianProcess 瀹堟姢杩涚▼淇℃伅
type GuardianProcess struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Command     string `json:"command"`
	WorkDir     string `json:"workDir"`
	Status      string `json:"status"` // running / stopped / failed / unknown
	PID         int    `json:"pid"`
	AutoRestart bool   `json:"autoRestart"`
	LogPath     string `json:"logPath"`
	CreatedAt   int64  `json:"createdAt"`
	Restarts    int    `json:"restarts"`
}

// ProcessGuardianService 杩涚▼瀹堟姢鏈嶅姟锛堝熀浜?systemd锛屽吋瀹规墍鏈夌幇浠?Linux锛?type ProcessGuardianService struct {
	sshSvc *SSHService
}

func NewProcessGuardianService(sshSvc *SSHService) *ProcessGuardianService {
	return &ProcessGuardianService{sshSvc: sshSvc}
}

// runCmd 鎵ц杩滅▼鍛戒护
func (s *ProcessGuardianService) runCmd(connID, cmd string) (string, error) {
	client, err := s.sshSvc.GetClient(connID)
	if err != nil {
		return "", fmt.Errorf("杩炴帴涓嶅瓨鍦? %v", err)
	}
	if !client.IsConnected() {
		return "", fmt.Errorf("SSH杩炴帴宸叉柇寮€")
	}
	result, err := client.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// detectInit 妫€娴?init 绯荤粺
func (s *ProcessGuardianService) detectInit(connID string) string {
	// systemd
	if out, _ := s.runCmd(connID, "pidof systemd 2>/dev/null"); out != "" {
		return "systemd"
	}
	// OpenRC
	if _, err := s.runCmd(connID, "rc-status --help 2>/dev/null"); err == nil {
		return "openrc"
	}
	// sysvinit
	if _, err := s.runCmd(connID, "service --status-all 2>/dev/null"); err == nil {
		return "sysvinit"
	}
	return "unknown"
}

// GetGuardians 鑾峰彇鎵€鏈夊畧鎶よ繘绋?func (s *ProcessGuardianService) GetGuardians(connID string) []GuardianProcess {
	initSystem := s.detectInit(connID)
	switch initSystem {
	case "systemd":
		return s.getSystemdServices(connID)
	default:
		return s.getSystemdServices(connID) // 榛樿灏濊瘯 systemd
	}
}

// getSystemdServices 鑾峰彇鑷畾涔夌殑 pzssh 瀹堟姢鏈嶅姟
func (s *ProcessGuardianService) getSystemdServices(connID string) []GuardianProcess {
	// 鍒楀嚭鎵€鏈?pzssh-managed 鏈嶅姟
	out, _ := s.runCmd(connID, "systemctl list-units --type=service --all --no-pager 2>/dev/null | grep 'pzssh-' | awk '{print $1}'")
	if out == "" {
		return []GuardianProcess{}
	}

	var processes []GuardianProcess
	for _, unit := range strings.Split(out, "\n") {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		// 鑾峰彇鏈嶅姟璇︽儏
		name := strings.TrimPrefix(unit, "pzssh-")
		name = strings.TrimSuffix(name, ".service")

		status := s.getSystemdStatus(connID, unit)
		pid := s.getSystemdPID(connID, unit)
		cmd := s.getSystemdCommand(connID, name)
		restarts := s.getSystemdRestarts(connID, unit)

		processes = append(processes, GuardianProcess{
			ID:          name,
			Name:        name,
			Command:     cmd,
			Status:      status,
			PID:         pid,
			AutoRestart: true,
			Restarts:    restarts,
		})
	}

	return processes
}

func (s *ProcessGuardianService) getSystemdStatus(connID, unit string) string {
	out, _ := s.runCmd(connID, fmt.Sprintf("systemctl is-active %s 2>/dev/null", unit))
	switch strings.TrimSpace(out) {
	case "active":
		return "running"
	case "activating":
		return "running"
	case "reloading":
		return "running"
	case "inactive":
		return "stopped"
	case "deactivating":
		return "stopped"
	case "failed":
		return "failed"
	default:
		// 灏濊瘯鐢?status 鍛戒护鑾峰彇鏇磋缁嗕俊鎭?		statusOut, _ := s.runCmd(connID, fmt.Sprintf("systemctl show %s --property=ActiveState --value 2>/dev/null", unit))
		switch strings.TrimSpace(statusOut) {
		case "active", "activating", "reloading":
			return "running"
		case "inactive", "deactivating":
			return "stopped"
		case "failed":
			return "failed"
		default:
			return "stopped"
		}
	}
}

func (s *ProcessGuardianService) getSystemdPID(connID, unit string) int {
	out, _ := s.runCmd(connID, fmt.Sprintf("systemctl show %s --property=MainPID --value 2>/dev/null", unit))
	var pid int
	fmt.Sscanf(strings.TrimSpace(out), "%d", &pid)
	return pid
}

func (s *ProcessGuardianService) getSystemdCommand(connID, name string) string {
	out, _ := s.runCmd(connID, fmt.Sprintf("cat /etc/systemd/system/pzssh-%s.service 2>/dev/null | grep ExecStart | head -1 | sed 's/ExecStart=//'", name))
	cmd := strings.TrimSpace(out)
	// 鍘绘帀 /bin/sh -c '...' 鍖呰
	if strings.HasPrefix(cmd, "/bin/sh -c '") && strings.HasSuffix(cmd, "'") {
		cmd = cmd[len("/bin/sh -c '") : len(cmd)-1]
	}
	return cmd
}

func (s *ProcessGuardianService) getSystemdRestarts(connID, unit string) int {
	out, _ := s.runCmd(connID, fmt.Sprintf("systemctl show %s --property=NRestarts --value 2>/dev/null", unit))
	var n int
	fmt.Sscanf(strings.TrimSpace(out), "%d", &n)
	return n
}

// CreateGuardian 鍒涘缓瀹堟姢杩涚▼
func (s *ProcessGuardianService) CreateGuardian(connID, name, command, workDir string, autoRestart bool) error {
	if name == "" || command == "" {
		return fmt.Errorf("鍚嶇О鍜屽懡浠や笉鑳戒负绌?)
	}

	// 娓呯悊鍚嶇О锛堝彧鍏佽瀛楁瘝鏁板瓧鍜岃繛瀛楃锛?	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)

	if workDir == "" {
		workDir = "/tmp"
	}

	restartPolicy := "on-failure"
	if !autoRestart {
		restartPolicy = "no"
	}

	// 鐢熸垚 systemd service 鏂囦欢锛堟棩蹇楃敱 systemd journal 鑷姩绠＄悊锛?	serviceContent := fmt.Sprintf(`[Unit]
Description=pzssh guardian: %s
After=network.target

[Service]
Type=simple
ExecStart=/bin/sh -c '%s'
WorkingDirectory=%s
Restart=%s
RestartSec=3

[Install]
WantedBy=multi-user.target
`, name, command, workDir, restartPolicy)

	// 鍐欏叆 service 鏂囦欢锛堜娇鐢?base64 閬垮厤鐗规畩瀛楃闂锛?	unitName := fmt.Sprintf("pzssh-%s.service", safeName)
	servicePath := fmt.Sprintf("/etc/systemd/system/%s", unitName)

	encoded := base64.StdEncoding.EncodeToString([]byte(serviceContent))
	writeCmd := fmt.Sprintf("echo %s | base64 -d > %s", encoded, servicePath)
	if _, err := s.runCmd(connID, writeCmd); err != nil {
		return fmt.Errorf("鍐欏叆鏈嶅姟鏂囦欢澶辫触: %v", err)
	}

	// 閲嶈浇 systemd
	s.runCmd(connID, "systemctl daemon-reload")

	// 鍚敤鏈嶅姟
	s.runCmd(connID, fmt.Sprintf("systemctl enable %s 2>/dev/null", unitName))

	// 鍚姩鏈嶅姟
	if _, err := s.runCmd(connID, fmt.Sprintf("systemctl start %s", unitName)); err != nil {
		return fmt.Errorf("鍚姩鏈嶅姟澶辫触: %v", err)
	}

	return nil
}

// StartGuardian 鍚姩瀹堟姢杩涚▼
func (s *ProcessGuardianService) StartGuardian(connID, name string) error {
	unitName := fmt.Sprintf("pzssh-%s.service", name)
	_, err := s.runCmd(connID, fmt.Sprintf("systemctl start %s", unitName))
	return err
}

// StopGuardian 鍋滄瀹堟姢杩涚▼
func (s *ProcessGuardianService) StopGuardian(connID, name string) error {
	unitName := fmt.Sprintf("pzssh-%s.service", name)
	_, err := s.runCmd(connID, fmt.Sprintf("systemctl stop %s", unitName))
	return err
}

// RestartGuardian 閲嶅惎瀹堟姢杩涚▼
func (s *ProcessGuardianService) RestartGuardian(connID, name string) error {
	unitName := fmt.Sprintf("pzssh-%s.service", name)
	_, err := s.runCmd(connID, fmt.Sprintf("systemctl restart %s", unitName))
	return err
}

// DeleteGuardian 鍒犻櫎瀹堟姢杩涚▼
func (s *ProcessGuardianService) DeleteGuardian(connID, name string) error {
	unitName := fmt.Sprintf("pzssh-%s.service", name)
	servicePath := fmt.Sprintf("/etc/systemd/system/%s", unitName)

	// 鍋滄骞剁鐢?	s.runCmd(connID, fmt.Sprintf("systemctl stop %s 2>/dev/null", unitName))
	s.runCmd(connID, fmt.Sprintf("systemctl disable %s 2>/dev/null", unitName))

	// 鍒犻櫎鏂囦欢
	s.runCmd(connID, fmt.Sprintf("rm -f %s", servicePath))
	s.runCmd(connID, fmt.Sprintf("rm -f /var/log/pzssh-%s.log", name))

	// 閲嶈浇
	s.runCmd(connID, "systemctl daemon-reload")

	return nil
}

// GetGuardianLogs 鑾峰彇瀹堟姢杩涚▼鏃ュ織锛堜粠 systemd journal 璇诲彇锛?func (s *ProcessGuardianService) GetGuardianLogs(connID, name string, lines int) string {
	if lines <= 0 {
		lines = 100
	}
	unitName := fmt.Sprintf("pzssh-%s.service", name)
	out, _ := s.runCmd(connID, fmt.Sprintf("journalctl -u %s -n %d --no-pager -o short-iso 2>/dev/null", unitName, lines))
	return out
}

// GetGuardianStats 鑾峰彇瀹堟姢杩涚▼缁熻
func (s *ProcessGuardianService) GetGuardianStats(connID, name string) map[string]interface{} {
	unitName := fmt.Sprintf("pzssh-%s.service", name)

	status := s.getSystemdStatus(connID, unitName)
	pid := s.getSystemdPID(connID, unitName)
	restarts := s.getSystemdRestarts(connID, unitName)

	// 鑾峰彇杩愯鏃堕棿
	uptime := ""
	if pid > 0 {
		out, _ := s.runCmd(connID, fmt.Sprintf("ps -o etime= -p %d 2>/dev/null", pid))
		uptime = strings.TrimSpace(out)
	}

	// 鑾峰彇鍐呭瓨浣跨敤
	mem := ""
	if pid > 0 {
		out, _ := s.runCmd(connID, fmt.Sprintf("ps -o rss= -p %d 2>/dev/null", pid))
		mem = strings.TrimSpace(out)
	}

	return map[string]interface{}{
		"status":   status,
		"pid":      pid,
		"restarts": restarts,
		"uptime":   uptime,
		"memory":   mem,
	}
}

// ClearGuardianLogs 娓呯┖瀹堟姢杩涚▼鏃ュ織锛堟竻绌?systemd journal 涓鏈嶅姟鐨勮褰曪級
func (s *ProcessGuardianService) ClearGuardianLogs(connID, name, logPath string) error {
	unitName := fmt.Sprintf("pzssh-%s.service", name)

	fmt.Printf("[Guardian] ClearGuardianLogs: name=%s, unit=%s\n", name, unitName)

	// 浣跨敤 journalctl 娓呯┖鎸囧畾鏈嶅姟鐨勬棩蹇?	out, err := s.runCmd(connID, fmt.Sprintf("journalctl --rotate --unit=%s 2>&1 && journalctl --vacuum-time=1s --unit=%s 2>&1", unitName, unitName))
	fmt.Printf("[Guardian] 娓呯┖缁撴灉: %s, err=%v\n", strings.TrimSpace(out), err)

	return err
}

