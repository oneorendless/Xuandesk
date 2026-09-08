package ssh

import (
	"fmt"
	"strings"
	"sync"
)

// shellQuote 鐢ㄥ崟寮曞彿鍖呰９瀛楃涓蹭互瀹夊叏宓屽叆 shell 鍛戒护
//
// 瀹夊叏淇锛氫箣鍓?AddIptablesRule / AddUfwRule / AddFirewalldRule 鐩存帴
// 鎶?comment銆乻ource銆乸ort 绛夌敤鎴峰彲鎺у瓧娈垫嫾鎺ュ埌鍛戒护瀛楃涓查噷锛?// 鏀诲嚮鑰呭彲浠ラ€氳繃杈撳叆 `'; rm -rf / #` 涔嬬被鎵ц浠绘剰鍛戒护銆?//
// 绠楁硶锛氭爣鍑?POSIX 鍗曞紩鍙疯浆涔?鈥斺€?鎶婂唴閮ㄦ墍鏈夊崟寮曞彿鏇挎崲涓?'\''
//
//	绀轰緥锛歠oo'bar 鈫?'foo'\''bar'
//
// 杩欐槸鎵€鏈?shell quoting 宸ュ叿锛坰hlex銆乻hellescape 绛夛級鐨勬爣鍑嗗仛娉曘€?func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// FirewallRule 闃茬伀澧欒鍒?type FirewallRule struct {
	Index    int    `json:"index"`    // 瑙勫垯搴忓彿
	Chain    string `json:"chain"`    // INPUT / OUTPUT / FORWARD / zone
	Target   string `json:"target"`   // ACCEPT / DROP / REJECT / allow / deny
	Protocol string `json:"protocol"` // tcp / udp / icmp / all
	Source   string `json:"source"`   // 婧愬湴鍧€
	Dest     string `json:"dest"`     // 鐩爣鍦板潃
	Port     string `json:"port"`     // 绔彛
	Comment  string `json:"comment"`  // 澶囨敞
	Raw      string `json:"raw"`      // 鍘熷琛?}

// FirewallInfo 闃茬伀澧欎俊鎭?type FirewallInfo struct {
	Type       string         `json:"type"`       // iptables / firewalld / ufw / unknown
	Status     string         `json:"status"`     // active / inactive / unknown
	Rules      []FirewallRule `json:"rules"`
	RawOutput  string         `json:"rawOutput"`  // 鍘熷杈撳嚭
	Chains     []string       `json:"chains"`     // 鍙敤鐨勯摼/鍖哄煙
}

// FirewallService 闃茬伀澧欑鐞嗘湇鍔?type FirewallService struct {
	mu     sync.RWMutex
	sshSvc *SSHService
}

// NewFirewallService 鍒涘缓闃茬伀澧欐湇鍔?func NewFirewallService(sshSvc *SSHService) *FirewallService {
	return &FirewallService{
		sshSvc: sshSvc,
	}
}

// runCmd 鎵ц杩滅▼鍛戒护
func (s *FirewallService) runCmd(connID, cmd string) (string, error) {
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

// detectType 妫€娴嬮槻鐏绫诲瀷锛堜紭鍏堢骇锛歶fw > firewalld > iptables锛?func (s *FirewallService) detectType(connID string) string {
	// ufw锛堟鏌ユ槸鍚﹀畨瑁咃紝涓嶇鏄惁鍚敤锛?	if out, err := s.runCmd(connID, "which ufw 2>/dev/null || command -v ufw 2>/dev/null"); err == nil && strings.TrimSpace(out) != "" {
		return "ufw"
	}
	// firewalld锛堟鏌ユ槸鍚﹀畨瑁咃紝涓嶇鏄惁杩愯锛?	if out, err := s.runCmd(connID, "which firewall-cmd 2>/dev/null || command -v firewall-cmd 2>/dev/null"); err == nil && strings.TrimSpace(out) != "" {
		return "firewalld"
	}
	// iptables
	if out, err := s.runCmd(connID, "which iptables 2>/dev/null || command -v iptables 2>/dev/null"); err == nil && strings.TrimSpace(out) != "" {
		return "iptables"
	}
	return "unknown"
}

// GetFirewallInfo 鑾峰彇闃茬伀澧欎俊鎭?func (s *FirewallService) GetFirewallInfo(connID string) *FirewallInfo {
	fwType := s.detectType(connID)
	info := &FirewallInfo{Type: fwType, Rules: []FirewallRule{}, Chains: []string{}}

	switch fwType {
	case "iptables":
		s.loadIptables(connID, info)
	case "firewalld":
		s.loadFirewalld(connID, info)
	case "ufw":
		s.loadUfw(connID, info)
	default:
		info.Status = "unknown"
		info.RawOutput = "鏈娴嬪埌鏀寔鐨勯槻鐏锛坕ptables / firewalld / ufw锛?
	}
	return info
}

// ==================== iptables ====================

func (s *FirewallService) loadIptables(connID string, info *FirewallInfo) {
	info.Chains = []string{"INPUT", "OUTPUT", "FORWARD"}

	hasRules := false
	for _, chain := range info.Chains {
		chainOut, _ := s.runCmd(connID, fmt.Sprintf("iptables -L %s -n -v --line-numbers 2>/dev/null", chain))
		info.RawOutput += chainOut + "\n\n"
		s.parseIptablesChain(chain, chainOut, info)
		// 妫€鏌ユ槸鍚︽湁瀹為檯瑙勫垯锛堜笉鍙槸鏍囬鍜岀瓥鐣ヨ锛?		lines := strings.Split(chainOut, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "Chain") || strings.HasPrefix(line, "num") || strings.HasPrefix(line, "target") {
				continue
			}
			hasRules = true
		}
	}

	if hasRules {
		info.Status = "active"
	} else {
		info.Status = "inactive"
	}
}

func (s *FirewallService) parseIptablesChain(chain, output string, info *FirewallInfo) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Chain") || strings.HasPrefix(line, "num") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// num pkts bytes target prot opt source dest [extra...]
		rule := FirewallRule{
			Raw:      line,
			Chain:    chain,
			Protocol: fields[3],
			Target:   fields[2],
			Source:   fields[7],
			Dest:     fields[8],
		}
		// 瑙ｆ瀽搴忓彿
		fmt.Sscanf(fields[0], "%d", &rule.Index)
		// 瑙ｆ瀽绔彛
		for i := 9; i < len(fields); i++ {
			if fields[i] == "dpt:" && i+1 < len(fields) {
				rule.Port = fields[i+1]
			}
			if fields[i] == "spt:" && i+1 < len(fields) {
				// 婧愮鍙ｆ殏涓嶅鐞?			}
		}
		// 瑙ｆ瀽娉ㄩ噴
		if idx := strings.Index(line, "/* "); idx != -1 {
			endIdx := strings.Index(line[idx:], " */")
			if endIdx != -1 {
				rule.Comment = line[idx+3 : idx+endIdx]
			}
		}
		info.Rules = append(info.Rules, rule)
	}
}

// AddIptablesRule 娣诲姞 iptables 瑙勫垯
func (s *FirewallService) AddIptablesRule(connID, chain, target, protocol, port, source, comment string) error {
	if chain == "" {
		chain = "INPUT"
	}
	if target == "" {
		target = "ACCEPT"
	}
	if protocol == "" {
		protocol = "tcp"
	}
	if source == "" {
		source = "0.0.0.0/0"
	}

	// 瀹夊叏淇锛氭墍鏈夌敤鎴峰彲鎺у瓧娈甸兘杩?shellQuote锛岄槻姝㈠懡浠ゆ敞鍏?	cmd := fmt.Sprintf("iptables -A %s -p %s -s %s -j %s",
		shellQuote(chain), shellQuote(protocol), shellQuote(source), shellQuote(target))
	if port != "" {
		cmd += fmt.Sprintf(" --dport %s", shellQuote(port))
	}
	if comment != "" {
		cmd += fmt.Sprintf(" -m comment --comment %s", shellQuote(comment))
	}

	_, err := s.runCmd(connID, cmd)
	if err != nil {
		return fmt.Errorf("娣诲姞瑙勫垯澶辫触: %v", err)
	}
	// 淇濆瓨瑙勫垯
	s.runCmd(connID, "iptables-save > /etc/iptables/rules.v4 2>/dev/null || true")
	return nil
}

// DeleteIptablesRule 鍒犻櫎 iptables 瑙勫垯
func (s *FirewallService) DeleteIptablesRule(connID, chain string, index int) error {
	if chain == "" {
		chain = "INPUT"
	}
	// 瀹夊叏淇锛歝hain 鍙傛暟杩?shellQuote 闃叉鍛戒护娉ㄥ叆
	cmd := fmt.Sprintf("iptables -D %s %d", shellQuote(chain), index)
	_, err := s.runCmd(connID, cmd)
	if err != nil {
		return fmt.Errorf("鍒犻櫎瑙勫垯澶辫触: %v", err)
	}
	s.runCmd(connID, "iptables-save > /etc/iptables/rules.v4 2>/dev/null || true")
	return nil
}

// ==================== firewalld ====================

func (s *FirewallService) loadFirewalld(connID string, info *FirewallInfo) {
	// 妫€鏌ユ槸鍚﹁繍琛?	state, _ := s.runCmd(connID, "firewall-cmd --state 2>/dev/null")
	if strings.TrimSpace(state) == "running" {
		info.Status = "active"
	} else {
		info.Status = "inactive"
	}

	info.Chains = []string{"public", "trusted", "drop"}

	// 鑾峰彇榛樿鍖哄煙
	defaultZone, _ := s.runCmd(connID, "firewall-cmd --get-default-zone 2>/dev/null")
	if defaultZone != "" {
		info.Chains = []string{defaultZone}
	}

	// 鑾峰彇鎵€鏈夊尯鍩?	zones, _ := s.runCmd(connID, "firewall-cmd --get-zones 2>/dev/null")
	if zones != "" {
		info.Chains = strings.Fields(zones)
	}

	// 鑾峰彇褰撳墠鍖哄煙鐨勮鍒?	zone := defaultZone
	if zone == "" {
		zone = "public"
	}

	out, _ := s.runCmd(connID, fmt.Sprintf("firewall-cmd --zone=%s --list-all 2>/dev/null", zone))
	info.RawOutput = out

	// 瑙ｆ瀽绔彛
	portsOut, _ := s.runCmd(connID, fmt.Sprintf("firewall-cmd --zone=%s --list-ports 2>/dev/null", zone))
	for i, port := range strings.Fields(portsOut) {
		info.Rules = append(info.Rules, FirewallRule{
			Index:    i + 1,
			Chain:    zone,
			Target:   "allow",
			Protocol: "tcp",
			Port:     port,
			Raw:      port,
		})
	}

	// 瑙ｆ瀽鏈嶅姟
	servicesOut, _ := s.runCmd(connID, fmt.Sprintf("firewall-cmd --zone=%s --list-services 2>/dev/null", zone))
	for _, svc := range strings.Fields(servicesOut) {
		info.Rules = append(info.Rules, FirewallRule{
			Index:    len(info.Rules) + 1,
			Chain:    zone,
			Target:   "allow",
			Protocol: "service",
			Port:     svc,
			Raw:      svc,
		})
	}
}

// AddFirewalldRule 娣诲姞 firewalld 瑙勫垯
func (s *FirewallService) AddFirewalldRule(connID, zone, port, protocol string) error {
	if zone == "" {
		zone = "public"
	}
	if protocol == "" {
		protocol = "tcp"
	}

	// 瀹夊叏淇锛氭墍鏈夌敤鎴峰彲鎺у瓧娈佃繃 shellQuote
	if strings.Contains(port, "/") || strings.Contains(port, ":") {
		// 绔彛鑼冨洿
		cmd := fmt.Sprintf("firewall-cmd --zone=%s --add-port=%s/%s --permanent 2>/dev/null",
			shellQuote(zone), shellQuote(port), shellQuote(protocol))
		_, err := s.runCmd(connID, cmd)
		if err != nil {
			return fmt.Errorf("娣诲姞绔彛瑙勫垯澶辫触: %v", err)
		}
	} else {
		// 灏濊瘯浣滀负鏈嶅姟娣诲姞
		cmd := fmt.Sprintf("firewall-cmd --zone=%s --add-service=%s --permanent 2>/dev/null",
			shellQuote(zone), shellQuote(port))
		_, err := s.runCmd(connID, cmd)
		if err != nil {
			// 浣滀负绔彛娣诲姞
			cmd = fmt.Sprintf("firewall-cmd --zone=%s --add-port=%s/%s --permanent 2>/dev/null",
				shellQuote(zone), shellQuote(port), shellQuote(protocol))
			_, err = s.runCmd(connID, cmd)
			if err != nil {
				return fmt.Errorf("娣诲姞瑙勫垯澶辫触: %v", err)
			}
		}
	}

	// 閲嶈浇
	s.runCmd(connID, "firewall-cmd --reload 2>/dev/null")
	return nil
}

// DeleteFirewalldRule 鍒犻櫎 firewalld 瑙勫垯
func (s *FirewallService) DeleteFirewalldRule(connID, zone, port, protocol string) error {
	if zone == "" {
		zone = "public"
	}
	if protocol == "" {
		protocol = "tcp"
	}

	cmd := fmt.Sprintf("firewall-cmd --zone=%s --remove-port=%s/%s --permanent 2>/dev/null", zone, port, protocol)
	_, err := s.runCmd(connID, cmd)
	if err != nil {
		// 灏濊瘯浣滀负鏈嶅姟鍒犻櫎
		cmd = fmt.Sprintf("firewall-cmd --zone=%s --remove-service=%s --permanent 2>/dev/null", zone, port)
		_, err = s.runCmd(connID, cmd)
		if err != nil {
			return fmt.Errorf("鍒犻櫎瑙勫垯澶辫触: %v", err)
		}
	}
	s.runCmd(connID, "firewall-cmd --reload 2>/dev/null")
	return nil
}

// ==================== ufw ====================

func (s *FirewallService) loadUfw(connID string, info *FirewallInfo) {
	out, _ := s.runCmd(connID, "ufw status numbered 2>/dev/null")
	info.RawOutput = out

	if strings.Contains(out, "Status: active") {
		info.Status = "active"
	} else {
		info.Status = "inactive"
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Status:") || strings.HasPrefix(line, "To") || strings.HasPrefix(line, "--") {
			continue
		}

		// 鏍煎紡: [ N] to                  action      FROM
		// 鎴? [ N] 22/tcp                ALLOW IN    Anywhere
		if !strings.HasPrefix(line, "[") {
			continue
		}

		rule := FirewallRule{Raw: line}

		// 瑙ｆ瀽搴忓彿
		if idxEnd := strings.Index(line, "]"); idxEnd > 1 {
			fmt.Sscanf(line[1:idxEnd], "%d", &rule.Index)
		}

		// 瑙ｆ瀽鍓╀綑閮ㄥ垎
		rest := strings.TrimSpace(line[strings.Index(line, "]")+1:])
		fields := strings.Fields(rest)

		if len(fields) >= 3 {
			rule.Port = fields[0]
			rule.Target = fields[1]
			rule.Chain = fields[2] // IN / OUT
			if len(fields) >= 4 {
				rule.Source = fields[3]
			}
		}

		// 瑙ｆ瀽鍗忚
		if parts := strings.Split(rule.Port, "/"); len(parts) == 2 {
			rule.Protocol = parts[1]
			rule.Port = parts[0]
		}

		info.Rules = append(info.Rules, rule)
	}

	info.Chains = []string{"IN", "OUT"}
}

// AddUfwRule 娣诲姞 ufw 瑙勫垯
func (s *FirewallService) AddUfwRule(connID, action, port, protocol, source string) error {
	if action == "" {
		action = "allow"
	}
	action = strings.ToLower(action)

	// 瀹夊叏淇锛氭墍鏈夌敤鎴峰彲鎺у瓧娈佃繃 shellQuote
	cmd := fmt.Sprintf("ufw %s", shellQuote(action))
	if port != "" {
		cmd += " " + shellQuote(port)
		if protocol != "" && protocol != "all" {
			cmd += "/" + shellQuote(protocol)
		}
	}
	if source != "" && source != "anywhere" && source != "0.0.0.0/0" {
		cmd += " from " + shellQuote(source)
	}

	_, err := s.runCmd(connID, cmd)
	if err != nil {
		return fmt.Errorf("娣诲姞瑙勫垯澶辫触: %v", err)
	}
	return nil
}

// DeleteUfwRule 鍒犻櫎 ufw 瑙勫垯
func (s *FirewallService) DeleteUfwRule(connID string, index int) error {
	// ufw delete 闇€瑕佺‘璁わ紝浣跨敤 --force
	cmd := fmt.Sprintf("echo y | ufw delete %d 2>/dev/null", index)
	_, err := s.runCmd(connID, cmd)
	if err != nil {
		return fmt.Errorf("鍒犻櫎瑙勫垯澶辫触: %v", err)
	}
	return nil
}

// ToggleUfw 鍚敤/绂佺敤 ufw
func (s *FirewallService) ToggleUfw(connID string, enable bool) error {
	cmd := "ufw --force disable"
	if enable {
		cmd = "ufw --force enable"
	}
	_, err := s.runCmd(connID, cmd)
	return err
}

// ==================== 閫氱敤鎺ュ彛 ====================

// AddRule 娣诲姞瑙勫垯锛堟牴鎹槻鐏绫诲瀷鑷姩閫傞厤锛?func (s *FirewallService) AddRule(connID, chain, target, protocol, port, source, comment string) error {
	fwType := s.detectType(connID)
	switch fwType {
	case "iptables":
		return s.AddIptablesRule(connID, chain, target, protocol, port, source, comment)
	case "firewalld":
		return s.AddFirewalldRule(connID, chain, port, protocol)
	case "ufw":
		action := "allow"
		if target == "DROP" || target == "REJECT" {
			action = "deny"
		}
		return s.AddUfwRule(connID, action, port, protocol, source)
	default:
		return fmt.Errorf("涓嶆敮鎸佺殑闃茬伀澧欑被鍨?)
	}
}

// DeleteRule 鍒犻櫎瑙勫垯锛堟牴鎹槻鐏绫诲瀷鑷姩閫傞厤锛?func (s *FirewallService) DeleteRule(connID, chain string, index int, port, protocol string) error {
	fwType := s.detectType(connID)
	switch fwType {
	case "iptables":
		return s.DeleteIptablesRule(connID, chain, index)
	case "firewalld":
		return s.DeleteFirewalldRule(connID, chain, port, protocol)
	case "ufw":
		return s.DeleteUfwRule(connID, index)
	default:
		return fmt.Errorf("涓嶆敮鎸佺殑闃茬伀澧欑被鍨?)
	}
}

// ToggleFirewall 鍚敤/绂佺敤闃茬伀澧?func (s *FirewallService) ToggleFirewall(connID string, enable bool) error {
	fwType := s.detectType(connID)
	switch fwType {
	case "ufw":
		return s.ToggleUfw(connID, enable)
	case "firewalld":
		cmd := "systemctl stop firewalld 2>/dev/null"
		if enable {
			cmd = "systemctl start firewalld 2>/dev/null"
		}
		_, err := s.runCmd(connID, cmd)
		return err
	case "iptables":
		if enable {
			// 鎭㈠宸蹭繚瀛樼殑瑙勫垯
			_, err := s.runCmd(connID, "iptables-restore < /etc/iptables/rules.v4 2>/dev/null")
			if err != nil {
				// 娌℃湁淇濆瓨鐨勮鍒欙紝璁剧疆榛樿鏀捐绛栫暐
				s.runCmd(connID, "iptables -P INPUT ACCEPT")
				s.runCmd(connID, "iptables -P OUTPUT ACCEPT")
				s.runCmd(connID, "iptables -P FORWARD ACCEPT")
			}
			return nil
		}
		// 鍏堜繚瀛樺綋鍓嶈鍒欙紝鍐嶆竻绌?		s.runCmd(connID, "mkdir -p /etc/iptables && iptables-save > /etc/iptables/rules.v4 2>/dev/null")
		// 娓呯┖鎵€鏈夎鍒欙紝璁剧疆榛樿鏀捐
		_, err := s.runCmd(connID, "iptables -F && iptables -X && iptables -P INPUT ACCEPT && iptables -P OUTPUT ACCEPT && iptables -P FORWARD ACCEPT")
		return err
	default:
		return fmt.Errorf("涓嶆敮鎸佺殑闃茬伀澧欑被鍨?)
	}
}

// RunCustomCommand 鎵ц鑷畾涔夐槻鐏鍛戒护
func (s *FirewallService) RunCustomCommand(connID, command string) (string, error) {
	// 瀹夊叏妫€鏌ワ細鍙厑璁搁槻鐏鐩稿叧鍛戒护
	allowed := false
	prefixes := []string{"iptables", "ufw", "firewall-cmd", "nft", "ip6tables"}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.TrimSpace(command), p) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("鍙厑璁告墽琛岄槻鐏鐩稿叧鍛戒护 (iptables/ufw/firewall-cmd/nft)")
	}

	return s.runCmd(connID, command)
}

