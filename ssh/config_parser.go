package ssh

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SSHConfigEntry SSH config 涓殑涓€涓?Host 鍧?type SSHConfigEntry struct {
	Host       string // 鍘熷 Host 鍚嶇О
	HostName   string // 瀹為檯涓绘満鍦板潃
	Port       int
	User       string
	IdentityFile string
}

// ParseSSHConfig 瑙ｆ瀽 SSH config 鏂囦欢锛岃繑鍥炶繛鎺ュ垪琛?func ParseSSHConfig(path string) ([]SSHConfigEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("鎵撳紑 SSH config 澶辫触: %w", err)
	}
	defer file.Close()

	var entries []SSHConfigEntry
	var current *SSHConfigEntry

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value := parseConfigLine(line)
		if key == "" {
			continue
		}

		keyLower := strings.ToLower(key)

		if keyLower == "host" {
			// 鏂扮殑 Host 鍧?			if current != nil && current.Host != "" {
				entries = append(entries, *current)
			}
			current = &SSHConfigEntry{
				Host: value,
				Port: 22,
			}
			continue
		}

		if current == nil {
			continue
		}

		switch keyLower {
		case "hostname":
			current.HostName = value
		case "port":
			if p, err := strconv.Atoi(value); err == nil && p > 0 {
				current.Port = p
			}
		case "user":
			current.User = value
		case "identityfile":
			// 灞曞紑 ~ 涓虹敤鎴蜂富鐩綍
			if strings.HasPrefix(value, "~") {
				if home, err := os.UserHomeDir(); err == nil {
					value = filepath.Join(home, value[1:])
				}
			}
			current.IdentityFile = value
		}
	}

	// 淇濆瓨鏈€鍚庝竴涓?Host 鍧?	if current != nil && current.Host != "" {
		entries = append(entries, *current)
	}

	// 杩囨护鎺夐€氶厤绗?Host锛堝 *锛?	var result []SSHConfigEntry
	for _, e := range entries {
		if e.Host == "*" || strings.Contains(e.Host, "*") || strings.Contains(e.Host, "?") {
			continue
		}
		// 濡傛灉娌℃湁 HostName锛屼娇鐢?Host 浣滀负涓绘満鍦板潃
		if e.HostName == "" {
			e.HostName = e.Host
		}
		// 濡傛灉娌℃湁 User锛屼娇鐢ㄩ粯璁ゅ€?		if e.User == "" {
			e.User = "root"
		}
		result = append(result, e)
	}

	return result, nil
}

// parseConfigLine 瑙ｆ瀽涓€琛?SSH config锛岃繑鍥?key 鍜?value
func parseConfigLine(line string) (string, string) {
	// 璺宠繃娉ㄩ噴
	if idx := strings.Index(line, "#"); idx == 0 {
		return "", ""
	}

	// 鍒嗗壊 key 鍜?value锛堢涓€涓┖鏍硷級
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return strings.TrimSpace(parts[0]), ""
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// 鍘绘帀寮曞彿
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}

	return key, value
}

