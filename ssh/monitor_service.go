package ssh

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SystemStats 绯荤粺鏁翠綋缁熻
type SystemStats struct {
	Timestamp   int64        `json:"timestamp"`
	Uptime      string       `json:"uptime"`       // 绯荤粺杩愯鏃堕暱
	CPU         CPUStats     `json:"cpu"`
	Memory      MemoryStats  `json:"memory"`
	Disk        DiskStats    `json:"disk"`
	Network     NetworkStats `json:"network"`
}

// CPUStats CPU 缁熻
type CPUStats struct {
	UsagePercent float64   `json:"usagePercent"` // 浣跨敤鐜?%
	Cores        int       `json:"cores"`        // 鏍稿績鏁?	PerCPUUsage  []float64 `json:"perCpuUsage"`  // 姣忎釜鏍稿績鐨勪娇鐢ㄧ巼
	LoadAvg      LoadAvg   `json:"loadAvg"`      // 骞冲潎璐熻浇
}

// LoadAvg 骞冲潎璐熻浇
type LoadAvg struct {
	Load1  float64 `json:"load1"`  // 1鍒嗛挓
	Load5  float64 `json:"load5"`  // 5鍒嗛挓
	Load15 float64 `json:"load15"` // 15鍒嗛挓
}

// MemoryStats 鍐呭瓨缁熻
type MemoryStats struct {
	Total       uint64  `json:"total"`       // 鎬诲唴瀛?(bytes)
	Used        uint64  `json:"used"`        // 宸蹭娇鐢?(bytes)
	Free        uint64  `json:"free"`        // 绌洪棽 (bytes)
	UsedPercent float64 `json:"usedPercent"` // 浣跨敤鐜?%
	Cached      uint64  `json:"cached"`      // 缂撳瓨 (bytes)
	SwapTotal   uint64  `json:"swapTotal"`   // 浜ゆ崲鍖烘€诲ぇ灏?	SwapUsed    uint64  `json:"swapUsed"`    // 浜ゆ崲鍖哄凡浣跨敤
}

// DiskStats 纾佺洏缁熻
type DiskStats struct {
	Partitions []DiskPartition `json:"partitions"` // 鍒嗗尯鍒楄〃
	IOStats    DiskIOStats     `json:"ioStats"`    // IO 缁熻
}

// DiskPartition 纾佺洏鍒嗗尯
type DiskPartition struct {
	Device      string  `json:"device"`      // 璁惧鍚?	Mountpoint  string  `json:"mountpoint"`  // 鎸傝浇鐐?	Fstype      string  `json:"fstype"`      // 鏂囦欢绯荤粺绫诲瀷
	Total       uint64  `json:"total"`       // 鎬诲閲?(bytes)
	Used        uint64  `json:"used"`        // 宸蹭娇鐢?(bytes)
	Free        uint64  `json:"free"`        // 绌洪棽 (bytes)
	UsedPercent float64 `json:"usedPercent"` // 浣跨敤鐜?%
}

// DiskIOStats 纾佺洏 IO 缁熻
type DiskIOStats struct {
	ReadBytes  uint64 `json:"readBytes"`  // 璇诲彇瀛楄妭鏁?	WriteBytes uint64 `json:"writeBytes"` // 鍐欏叆瀛楄妭鏁?	ReadCount  uint64 `json:"readCount"`  // 璇诲彇娆℃暟
	WriteCount uint64 `json:"writeCount"` // 鍐欏叆娆℃暟
}

// NetworkStats 缃戠粶缁熻
type NetworkStats struct {
	Interfaces []NetInterface `json:"interfaces"` // 缃戠粶鎺ュ彛鍒楄〃
	TotalRx    uint64         `json:"totalRx"`    // 鎬绘帴鏀跺瓧鑺?	TotalTx    uint64         `json:"totalTx"`    // 鎬诲彂閫佸瓧鑺?}

// NetInterface 缃戠粶鎺ュ彛
type NetInterface struct {
	Name        string `json:"name"`        // 鎺ュ彛鍚嶇О
	BytesSent   uint64 `json:"bytesSent"`   // 鍙戦€佸瓧鑺?	BytesRecv   uint64 `json:"bytesRecv"`   // 鎺ユ敹瀛楄妭
	PacketsSent uint64 `json:"packetsSent"` // 鍙戦€佸寘鏁?	PacketsRecv uint64 `json:"packetsRecv"` // 鎺ユ敹鍖呮暟
}

// ProcessInfo 杩涚▼淇℃伅
type ProcessInfo struct {
	PID        int32   `json:"pid"`        // 杩涚▼ ID
	Name       string  `json:"name"`       // 杩涚▼鍚嶇О
	CPUPercent float64 `json:"cpuPercent"` // CPU 浣跨敤鐜?%
	MemPercent float64 `json:"memPercent"` // 鍐呭瓨浣跨敤鐜?%
	MemRSS     uint64  `json:"memRss"`     // 鐗╃悊鍐呭瓨浣跨敤 (bytes)
	Status     string  `json:"status"`     // 杩涚▼鐘舵€?	Username   string  `json:"username"`   // 鐢ㄦ埛鍚?	StartTime  string  `json:"startTime"`  // 鍚姩鏃堕棿
	ElapsedTime string `json:"elapsedTime"` // 杩愯鏃堕暱
	Cmdline    string  `json:"cmdline"`    // 鍛戒护琛?	NumThreads int32   `json:"numThreads"` // 绾跨▼鏁?	Priority   int     `json:"priority"`   // 浼樺厛绾?	Nice       int     `json:"nice"`       // Nice 鍊?}

// GetSystemStats 鑾峰彇绯荤粺璧勬簮缁熻锛堥€氳繃 SSH 鍦ㄨ繙绋嬫湇鍔″櫒鎵ц鍛戒护锛?func (c *SSHClient) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	if c.isConnected.Load() == 0 {
		return nil, fmt.Errorf("SSH 鏈繛鎺?)
	}

	stats := &SystemStats{
		Timestamp: time.Now().UnixMilli(),
	}

	// 0. 鑾峰彇绯荤粺杩愯鏃堕暱
	// 鐩存帴璇诲彇 /proc/uptime 骞舵牸寮忓寲涓轰腑鏂?	uptimeRaw, err := c.ExecuteCommand("cat /proc/uptime | awk '{print $1}'")
	if err == nil && uptimeRaw.Success {
		seconds := parseFloat64(strings.TrimSpace(uptimeRaw.Stdout))
		stats.Uptime = formatUptime(seconds)
	} else {
		stats.Uptime = "鏈煡"
	}

	// 1. 鑾峰彇 CPU 浣跨敤鐜?	cpuResult, err := c.ExecuteCommand("top -bn1 | grep 'Cpu(s)' | awk '{print $2}'")
	if err == nil && cpuResult.Success {
		cpuPercent, parseErr := strconv.ParseFloat(strings.TrimSpace(cpuResult.Stdout), 64)
		if parseErr == nil {
			stats.CPU.UsagePercent = cpuPercent
		}
	}

	// 2. 鑾峰彇 CPU 鏍稿績鏁?	coresResult, err := c.ExecuteCommand("nproc")
	if err == nil && coresResult.Success {
		cores, parseErr := strconv.Atoi(strings.TrimSpace(coresResult.Stdout))
		if parseErr == nil {
			stats.CPU.Cores = cores
		}
	}

	// 3. 鑾峰彇骞冲潎璐熻浇
	loadResult, err := c.ExecuteCommand("cat /proc/loadavg | awk '{print $1,$2,$3}'")
	if err == nil && loadResult.Success {
		fields := strings.Fields(strings.TrimSpace(loadResult.Stdout))
		if len(fields) >= 3 {
			stats.CPU.LoadAvg.Load1, _ = strconv.ParseFloat(fields[0], 64)
			stats.CPU.LoadAvg.Load5, _ = strconv.ParseFloat(fields[1], 64)
			stats.CPU.LoadAvg.Load15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}

	// 4. 鑾峰彇鍐呭瓨淇℃伅
	memResult, err := c.ExecuteCommand("free -b | grep '^Mem:'")
	if err == nil && memResult.Success {
		fields := strings.Fields(strings.TrimSpace(memResult.Stdout))
		if len(fields) >= 7 {
			stats.Memory.Total, _ = strconv.ParseUint(fields[1], 10, 64)
			stats.Memory.Used, _ = strconv.ParseUint(fields[2], 10, 64)
			stats.Memory.Free, _ = strconv.ParseUint(fields[3], 10, 64)
			stats.Memory.Cached, _ = strconv.ParseUint(fields[6], 10, 64)
			if stats.Memory.Total > 0 {
				stats.Memory.UsedPercent = float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
			}
		}
	}

	// 5. 鑾峰彇浜ゆ崲鍖轰俊鎭?	swapResult, err := c.ExecuteCommand("free -b | grep '^Swap:'")
	if err == nil && swapResult.Success {
		fields := strings.Fields(strings.TrimSpace(swapResult.Stdout))
		if len(fields) >= 3 {
			stats.Memory.SwapTotal, _ = strconv.ParseUint(fields[1], 10, 64)
			stats.Memory.SwapUsed, _ = strconv.ParseUint(fields[2], 10, 64)
		}
	}

	// 6. 鑾峰彇纾佺洏鍒嗗尯淇℃伅
	dfResult, err := c.ExecuteCommand("df -B1 --output=target,size,used,avail,pcent,fstype | tail -n +2")
	if err == nil && dfResult.Success {
		lines := strings.Split(strings.TrimSpace(dfResult.Stdout), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				mountpoint := fields[0]
				total, _ := strconv.ParseUint(fields[1], 10, 64)
				used, _ := strconv.ParseUint(fields[2], 10, 64)
				free, _ := strconv.ParseUint(fields[3], 10, 64)
				usedPercentStr := strings.TrimSuffix(fields[4], "%")
				usedPercent, _ := strconv.ParseFloat(usedPercentStr, 64)
				fstype := fields[5]

				// 璺宠繃铏氭嫙鏂囦欢绯荤粺
				if fstype == "tmpfs" || fstype == "devtmpfs" || mountpoint == "/run" || mountpoint == "/dev/shm" {
					continue
				}

				stats.Disk.Partitions = append(stats.Disk.Partitions, DiskPartition{
					Device:      "",
					Mountpoint:  mountpoint,
					Fstype:      fstype,
					Total:       total,
					Used:        used,
					Free:        free,
					UsedPercent: usedPercent,
				})
			}
		}
	}

	// 7. 鑾峰彇纾佺洏 IO 缁熻
	ioResult, err := c.ExecuteCommand("cat /proc/diskstats | awk '{read+=$4; write+=$8} END {print read, write}'")
	if err == nil && ioResult.Success {
		fields := strings.Fields(strings.TrimSpace(ioResult.Stdout))
		if len(fields) >= 2 {
			// /proc/diskstats 鐨勫崟浣嶆槸鎵囧尯锛?12瀛楄妭锛?			readSectors, _ := strconv.ParseUint(fields[0], 10, 64)
			writeSectors, _ := strconv.ParseUint(fields[1], 10, 64)
			stats.Disk.IOStats.ReadBytes = readSectors * 512
			stats.Disk.IOStats.WriteBytes = writeSectors * 512
		}
	}

	// 8. 鑾峰彇缃戠粶鎺ュ彛淇℃伅
	netResult, err := c.ExecuteCommand("cat /proc/net/dev | tail -n +3")
	if err == nil && netResult.Success {
		lines := strings.Split(strings.TrimSpace(netResult.Stdout), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// 绉婚櫎鍐掑彿骞跺垎鍓?			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			ifaceName := strings.TrimSpace(parts[0])
			fields := strings.Fields(parts[1])
			if len(fields) >= 10 {
				bytesRecv, _ := strconv.ParseUint(fields[0], 10, 64)
				packetsRecv, _ := strconv.ParseUint(fields[1], 10, 64)
				bytesSent, _ := strconv.ParseUint(fields[8], 10, 64)
				packetsSent, _ := strconv.ParseUint(fields[9], 10, 64)

				stats.Network.Interfaces = append(stats.Network.Interfaces, NetInterface{
					Name:        ifaceName,
					BytesSent:   bytesSent,
					BytesRecv:   bytesRecv,
					PacketsSent: packetsSent,
					PacketsRecv: packetsRecv,
				})
				stats.Network.TotalRx += bytesRecv
				stats.Network.TotalTx += bytesSent
			}
		}
	}

	return stats, nil
}

// GetProcessList 鑾峰彇杩涚▼鍒楄〃
func (c *SSHClient) GetProcessList(ctx context.Context) ([]ProcessInfo, error) {
	if c.isConnected.Load() == 0 {
		return nil, fmt.Errorf("SSH 浼氳瘽鏈缓绔?)
	}

	// 涓€娆?SSH 璋冪敤鎵ц涓ゆ潯 ps 鍛戒护锛岀敤鍒嗛殧琛屽尯鍒嗚緭鍑?	// 绗竴鏉★細鍩虹瀛楁锛坧id user cpu mem rss stat etime comm锛?	// 绗簩鏉★細pid + lstart + args锛堢敤 | 鍋?pid 鍜?lstart 鐨勫垎闅旓級
	cmd := `echo __PS_BASIC__ && ps -eo pid,user,%cpu,%mem,rss,stat,etime,comm --sort=-%cpu --no-headers && echo __PS_DETAIL__ && ps -eo pid,lstart,args --sort=-%cpu --no-headers`
	result, err := c.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("鎵ц ps 鍛戒护澶辫触: %v", err)
	}

	processes, err := parsePsOutput(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("瑙ｆ瀽杩涚▼淇℃伅澶辫触: %v", err)
	}

	return processes, nil
}

// KillProcess 缁堟杩涚▼
func (c *SSHClient) KillProcess(ctx context.Context, pid int32) error {
	if c.isConnected.Load() == 0 {
		return fmt.Errorf("SSH 鏈繛鎺?)
	}

	// 鎵ц kill 鍛戒护
	cmd := fmt.Sprintf("kill -9 %d", pid)
	result, err := c.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("鎵ц kill 鍛戒护澶辫触: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("缁堟杩涚▼澶辫触: %s", result.Stderr)
	}

	return nil
}

// SendSignal 鍚戣繘绋嬪彂閫佷俊鍙?func (c *SSHClient) SendSignal(ctx context.Context, pid int32, signal string) error {
	if c.isConnected.Load() == 0 {
		return fmt.Errorf("SSH 鏈繛鎺?)
	}

	// 鎵ц kill 鍛戒护鍙戦€佷俊鍙?	cmd := fmt.Sprintf("kill -%s %d", signal, pid)
	result, err := c.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("鍙戦€佷俊鍙峰け璐? %v", err)
	}

	if !result.Success {
		return fmt.Errorf("鍙戦€佷俊鍙峰け璐? %s", result.Stderr)
	}

	return nil
}

// GetProcessDetail 鑾峰彇杩涚▼璇︾粏淇℃伅
func (c *SSHClient) GetProcessDetail(ctx context.Context, pid int32) (string, error) {
	if c.isConnected.Load() == 0 {
		return "", fmt.Errorf("SSH 鏈繛鎺?)
	}

	// 鑾峰彇杩涚▼璇︾粏淇℃伅
	cmd := fmt.Sprintf("ps -p %d -f", pid)
	result, err := c.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("鑾峰彇杩涚▼淇℃伅澶辫触: %v", err)
	}

	if !result.Success {
		return "", fmt.Errorf("杩涚▼涓嶅瓨鍦ㄦ垨鏃犳潈璁块棶")
	}

	return result.Stdout, nil
}

// parsePsOutput 瑙ｆ瀽 ps 鍛戒护杈撳嚭锛堝弻娈垫牸寮忥級
// 绗竴娈?__PS_BASIC__锛歱id user cpu mem rss stat etime comm锛堢┖鏍煎垎闅旓級
// 绗簩娈?__PS_DETAIL__锛歱id lstart args锛坙start 鍚┖鏍硷紝args 浠庣7涓瓧娈靛紑濮嬶級
func parsePsOutput(output string) ([]ProcessInfo, error) {
	// 鎸?__PS_BASIC__ 鍜?__PS_DETAIL__ 鍒嗗壊
	basicSection := ""
	detailSection := ""
	if idx := strings.Index(output, "__PS_BASIC__"); idx >= 0 {
		rest := output[idx+len("__PS_BASIC__"):]
		rest = strings.TrimPrefix(rest, "\n")
		if didx := strings.Index(rest, "__PS_DETAIL__"); didx >= 0 {
			basicSection = rest[:didx]
			detailSection = rest[didx+len("__PS_DETAIL__"):]
			detailSection = strings.TrimPrefix(detailSection, "\n")
		} else {
			basicSection = rest
		}
	} else {
		basicSection = output
	}

	// 瑙ｆ瀽绗簩娈碉細pid 鈫?(lstart, args) 鏄犲皠
	detailMap := make(map[string][2]string) // pid 鈫?[lstart, args]
	for _, line := range strings.Split(detailSection, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 鐢?Fields 鎷嗗垎锛屽墠6涓瓧娈垫槸 pid + lstart(5瀛楁)锛屽墿浣欐槸 args
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		pidStr := fields[0]
		startTime := strings.Join(fields[1:6], " ")
		fullCmd := strings.Join(fields[6:], " ")
		detailMap[pidStr] = [2]string{startTime, fullCmd}
	}

	// 瑙ｆ瀽绗竴娈碉細鍩虹瀛楁
	var processes []ProcessInfo
	for _, line := range strings.Split(basicSection, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		pidStr := fields[0]
		pid := parseInt32(pidStr)
		username := fields[1]
		cpuPercent := parseFloat64(fields[2])
		memPercent := parseFloat64(fields[3])
		memRSS := parseUint64(fields[4]) * 1024
		statusRaw := fields[5]
		elapsedTime := fields[6]
		name := fields[7]

		// 浠?detailMap 琛ュ厖 lstart 鍜?args
		var startTime, fullCmd string
		if detail, ok := detailMap[pidStr]; ok {
			startTime = detail[0]
			fullCmd = detail[1]
		}

		// 鐢?args 淇杩涚▼鍚?		if fullCmd != "" {
			cmdParts := strings.Fields(fullCmd)
			if len(cmdParts) > 0 {
				binName := cmdParts[0]
				if strings.HasPrefix(binName, "[") && strings.HasSuffix(binName, "]") {
					name = binName[1 : len(binName)-1]
				} else {
					name = filepath.Base(binName)
				}
			}
		}

		// 杩囨护鎺?ps 鍛戒护鑷韩
		if name == "ps" || strings.HasPrefix(name, "ps ") {
			continue
		}

		status := convertProcessStatus(statusRaw)

		processes = append(processes, ProcessInfo{
			PID:         pid,
			Name:        name,
			CPUPercent:  cpuPercent,
			MemPercent:  memPercent,
			MemRSS:      memRSS,
			Status:      status,
			ElapsedTime: elapsedTime,
			StartTime:   startTime,
			Username:    username,
			Cmdline:     fullCmd,
		})
	}

	return processes, nil
}


// convertProcessStatus 灏?ps 鐘舵€佺爜杞崲涓虹畝鐭嫳鏂囩姸鎬佸悕锛堝墠绔礋璐?i18n 缈昏瘧锛?func convertProcessStatus(status string) string {
	if len(status) == 0 {
		return "unknown"
	}

	switch status[0] {
	case 'R':
		return "running"
	case 'S', 'I':
		return "sleeping"
	case 'D':
		return "uninterruptible"
	case 'Z':
		return "zombie"
	case 'T', 't':
		return "stopped"
	case 'X':
		return "dead"
	default:
		return "unknown"
	}
}

// formatUptime 鏍煎紡鍖栫郴缁熻繍琛屾椂闂达紙杩斿洖鑻辨枃鏍煎紡锛屽墠绔礋璐ｇ炕璇戯級
func formatUptime(seconds float64) string {
	days := int(seconds) / 86400
	hours := (int(seconds) % 86400) / 3600
	minutes := (int(seconds) % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// 杈呭姪鍑芥暟
func parseInt32(s string) int32 {
	var n int32
	fmt.Sscanf(s, "%d", &n)
	return n
}

func parseFloat64(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func parseUint64(s string) uint64 {
	var n uint64
	fmt.Sscanf(s, "%d", &n)
	return n
}

func extractProcessName(cmdline string) string {
	// 鎻愬彇鍛戒护鐨勭涓€閮ㄥ垎浣滀负杩涚▼鍚?	parts := strings.Fields(cmdline)
	if len(parts) > 0 {
		name := parts[0]
		// 鍘婚櫎璺緞鍓嶇紑
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		return name
	}
	return cmdline
}

