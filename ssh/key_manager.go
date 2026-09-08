package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHKeyInfo SSH 瀵嗛挜淇℃伅
type SSHKeyInfo struct {
	Name      string `json:"name"`       // 鏂囦欢鍚?	Path      string `json:"path"`       // 瀹屾暣璺緞
	Type      string `json:"type"`       // ed25519, rsa, ecdsa
	Bits      int    `json:"bits"`       // 瀵嗛挜浣嶆暟
	Comment   string `json:"comment"`    // 娉ㄩ噴
	CreatedAt string `json:"createdAt"`  // 鍒涘缓鏃堕棿
	HasPublic bool   `json:"hasPublic"`  // 鏄惁鏈夊叕閽?}

// KeyManager SSH 瀵嗛挜绠＄悊鍣?type KeyManager struct {
	sshDir string
}

// NewKeyManager 鍒涘缓瀵嗛挜绠＄悊鍣?func NewKeyManager() *KeyManager {
	home, err := os.UserHomeDir()
	if err != nil {
		return &KeyManager{sshDir: ".ssh"}
	}
	return &KeyManager{sshDir: filepath.Join(home, ".ssh")}
}

// ListKeys 鍒楀嚭鎵€鏈?SSH 瀵嗛挜
func (km *KeyManager) ListKeys() ([]SSHKeyInfo, error) {
	entries, err := os.ReadDir(km.sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SSHKeyInfo{}, nil
		}
		return nil, fmt.Errorf("璇诲彇 .ssh 鐩綍澶辫触: %w", err)
	}

	var keys []SSHKeyInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 璺宠繃闈炲瘑閽ユ枃浠?		if !isPrivateKeyFile(name) {
			continue
		}

		path := filepath.Join(km.sshDir, name)
		info, err := km.analyzeKey(path, name)
		if err != nil {
			continue
		}

		keys = append(keys, *info)
	}

	return keys, nil
}

// GenerateKey 鐢熸垚鏂板瘑閽?func (km *KeyManager) GenerateKey(name, keyType, comment string, bits int) error {
	if name == "" {
		return fmt.Errorf("瀵嗛挜鍚嶇О涓嶈兘涓虹┖")
	}

	// 纭繚 .ssh 鐩綍瀛樺湪
	if err := os.MkdirAll(km.sshDir, 0700); err != nil {
		return fmt.Errorf("鍒涘缓 .ssh 鐩綍澶辫触: %w", err)
	}

	privatePath := filepath.Join(km.sshDir, name)
	publicPath := privatePath + ".pub"

	// 妫€鏌ユ槸鍚﹀凡瀛樺湪
	if _, err := os.Stat(privatePath); err == nil {
		return fmt.Errorf("瀵嗛挜 %s 宸插瓨鍦?, name)
	}

	if comment == "" {
		comment = fmt.Sprintf("generated@pzssh-%s", time.Now().Format("20060102"))
	}

	var privateKeyPEM []byte
	var publicKeyBytes []byte

	switch keyType {
	case "ed25519":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return fmt.Errorf("鐢熸垚 ed25519 瀵嗛挜澶辫触: %w", err)
		}
		privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return fmt.Errorf("搴忓垪鍖栫閽ュけ璐? %w", err)
		}
		privateKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

		sshPub, err := ssh.NewPublicKey(pub)
		if err != nil {
			return fmt.Errorf("鍒涘缓鍏挜澶辫触: %w", err)
		}
		publicKeyBytes = append(ssh.MarshalAuthorizedKey(sshPub), []byte(" "+comment+"\n")...)

	case "rsa":
		if bits == 0 {
			bits = 4096
		}
		key, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			return fmt.Errorf("鐢熸垚 RSA 瀵嗛挜澶辫触: %w", err)
		}
		privBytes, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return fmt.Errorf("搴忓垪鍖栫閽ュけ璐? %w", err)
		}
		privateKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

		sshPub, err := ssh.NewPublicKey(&key.PublicKey)
		if err != nil {
			return fmt.Errorf("鍒涘缓鍏挜澶辫触: %w", err)
		}
		publicKeyBytes = append(ssh.MarshalAuthorizedKey(sshPub), []byte(" "+comment+"\n")...)

	default:
		return fmt.Errorf("涓嶆敮鎸佺殑瀵嗛挜绫诲瀷: %s锛堟敮鎸?ed25519, rsa锛?, keyType)
	}

	// 鍐欏叆绉侀挜锛堟潈闄?0600锛?	if err := os.WriteFile(privatePath, privateKeyPEM, 0600); err != nil {
		return fmt.Errorf("鍐欏叆绉侀挜澶辫触: %w", err)
	}

	// 鍐欏叆鍏挜锛堟潈闄?0644锛?	if err := os.WriteFile(publicPath, publicKeyBytes, 0644); err != nil {
		return fmt.Errorf("鍐欏叆鍏挜澶辫触: %w", err)
	}

	return nil
}

// DeployPublicKey 閮ㄧ讲鍏挜鍒拌繙绋嬫湇鍔″櫒
func (km *KeyManager) DeployPublicKey(keyName string, client *SSHClient) error {
	publicPath := filepath.Join(km.sshDir, keyName+".pub")
	pubBytes, err := os.ReadFile(publicPath)
	if err != nil {
		return fmt.Errorf("璇诲彇鍏挜澶辫触: %w", err)
	}

	// 鑾峰彇杩滅▼鐢ㄦ埛鐨?home 鐩綍
	homeResult, err := client.ExecuteCommand("echo $HOME")
	if err != nil || !homeResult.Success {
		return fmt.Errorf("鑾峰彇杩滅▼ home 鐩綍澶辫触: %v", err)
	}
	remoteHome := strings.TrimSpace(homeResult.Stdout)

	// 纭繚 .ssh 鐩綍瀛樺湪
	remoteSSHDir := remoteHome + "/.ssh"
	client.ExecuteCommand(fmt.Sprintf("mkdir -p %s && chmod 700 %s", remoteSSHDir, remoteSSHDir))

	// 杩藉姞鍏挜鍒?authorized_keys
	authorizedKeys := remoteSSHDir + "/authorized_keys"
	pubKeyStr := strings.TrimSpace(string(pubBytes))

	// 妫€鏌ユ槸鍚﹀凡瀛樺湪
	checkCmd := fmt.Sprintf("grep -qF '%s' %s 2>/dev/null && echo 'exists' || echo 'not_found'", pubKeyStr, authorizedKeys)
	checkResult, err := client.ExecuteCommand(checkCmd)
	if err == nil && strings.TrimSpace(checkResult.Stdout) == "exists" {
		return nil // 宸插瓨鍦紝鏃犻渶閲嶅閮ㄧ讲
	}

	// 杩藉姞鍏挜
	appendCmd := fmt.Sprintf("echo '%s' >> %s && chmod 600 %s", pubKeyStr, authorizedKeys, authorizedKeys)
	result, err := client.ExecuteCommand(appendCmd)
	if err != nil {
		return fmt.Errorf("杩藉姞鍏挜澶辫触: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("杩藉姞鍏挜澶辫触: %s", result.Stderr)
	}

	return nil
}

// GetPublicKey 鑾峰彇鍏挜鍐呭
func (km *KeyManager) GetPublicKey(keyName string) (string, error) {
	publicPath := filepath.Join(km.sshDir, keyName+".pub")
	data, err := os.ReadFile(publicPath)
	if err != nil {
		return "", fmt.Errorf("璇诲彇鍏挜澶辫触: %w", err)
	}
	return string(data), nil
}

// DeleteKey 鍒犻櫎瀵嗛挜
func (km *KeyManager) DeleteKey(keyName string) error {
	privatePath := filepath.Join(km.sshDir, keyName)
	publicPath := privatePath + ".pub"

	os.Remove(publicPath)
	if err := os.Remove(privatePath); err != nil {
		return fmt.Errorf("鍒犻櫎绉侀挜澶辫触: %w", err)
	}

	return nil
}

// analyzeKey 鍒嗘瀽瀵嗛挜鏂囦欢
func (km *KeyManager) analyzeKey(path, name string) (*SSHKeyInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("鏃犳硶瑙ｆ瀽 PEM")
	}

	keyType := "unknown"
	bits := 0
	comment := ""

	// 灏濊瘯瑙ｆ瀽瀵嗛挜鑾峰彇绫诲瀷鍜屼綅鏁?	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			keyType = "rsa"
			bits = k.N.BitLen()
		case ed25519.PrivateKey:
			keyType = "ed25519"
			bits = 256
		}
	}

	// 浠庡叕閽ユ枃浠惰幏鍙栨敞閲?	pubPath := path + ".pub"
	if pubData, err := os.ReadFile(pubPath); err == nil {
		parts := strings.Fields(string(pubData))
		if len(parts) >= 3 {
			comment = parts[2]
		}
	}

	// 鑾峰彇鏂囦欢淇℃伅
	stat, _ := os.Stat(path)
	createdAt := ""
	if stat != nil {
		createdAt = stat.ModTime().Format("2006-01-02 15:04")
	}

	return &SSHKeyInfo{
		Name:      name,
		Path:      path,
		Type:      keyType,
		Bits:      bits,
		Comment:   comment,
		CreatedAt: createdAt,
		HasPublic: fileExists(pubPath),
	}, nil
}

// isPrivateKeyFile 鍒ゆ柇鏄惁鏄閽ユ枃浠?func isPrivateKeyFile(name string) bool {
	if strings.HasSuffix(name, ".pub") {
		return false
	}
	skipFiles := []string{
		"known_hosts", "authorized_keys", "config",
		"known_hosts.old", "config.old",
	}
	for _, skip := range skipFiles {
		if name == skip {
			return false
		}
	}
	// 绉侀挜鏂囦欢閫氬父娌℃湁鎵╁睍鍚嶏紝鎴栬€呮槸 .pem
	return true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

