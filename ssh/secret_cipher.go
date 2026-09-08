package ssh

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// aesCipher 鐢?AES-256-GCM 鍔犲瘑锛屽瘑閽ヤ粠鎸佷箙鍖栫殑 master key 鏂囦欢鍔犺浇
//
// 杩欐槸涓€涓?fallback cipher锛屼富瑕佺粰 macOS / Linux 鐢ㄣ€?// 涓嶆槸鏈€鐞嗘兂鐨勶紙鐞嗘兂鏂规鏄?Keychain / Secret Service锛夛紝浣嗘瘮涔嬪墠鐨勬槑鏂囧己 N 鍊嶃€?//
// 瀹夊叏娉ㄦ剰鐐癸細
//   - master key 鏂囦欢鏉冮檺 0600锛堜粎褰撳墠鐢ㄦ埛鍙鍐欙級
//   - 鏂囦欢璺緞鍦ㄧ敤鎴烽厤缃洰褰曚笅锛岃法鐢ㄦ埛闅旂
//   - 涓€鏃︽硠闇诧紝鏀诲嚮鑰呰兘瑙ｅ瘑鎵€鏈夎繛鎺?鈫?骞虫椂瑕佺粰鏂囦欢鍔?ACL / 鐢?umask
type aesCipher struct {
	key        []byte
	masterPath string
}

// newAESCipher 鏋勯€?AES cipher锛屼細鎸夐渶鐢熸垚鎴栬鍙?master key
func newAESCipher(masterKeyPath string) *aesCipher {
	key, err := loadOrCreateMasterKey(masterKeyPath)
	if err != nil {
		// 鏈€鍚庣殑 fallback锛氶殢鏈虹敓鎴愪竴涓紙浠呮湰娆¤繘绋嬫湁鏁堬紝閲嶅惎鍚庢棤娉曡В瀵嗭級
		// 杩欑鎯呭喌涓嬬浉褰撲簬闄嶇骇鍒版棤鍔犲瘑鐘舵€侊紝蹇呴』澶у０鎶ラ敊鎻愰啋鐢ㄦ埛
		fmt.Printf("[SecretCipher] 鈿狅笍 涓ラ噸璀﹀憡锛歮aster key 鍔犺浇澶辫触 (%v)锛屾湰浼氳瘽瀵嗙爜灏嗘棤娉曟寔涔呭寲\n", err)
		key = make([]byte, 32)
		_, _ = rand.Read(key)
	}
	return &aesCipher{key: key, masterPath: masterKeyPath}
}

func (a *aesCipher) Kind() string {
	return "aes-gcm"
}

func (a *aesCipher) Encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// 鎶?nonce 鎷兼帴鍒板瘑鏂囧墠闈紝鏂逛究瑙ｅ瘑鏃跺彇鍑烘潵
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (a *aesCipher) Decrypt(cipherText string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("瀵嗘枃杩囩煭")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// loadOrCreateMasterKey 浠庢枃浠跺姞杞?master key锛屾枃浠朵笉瀛樺湪鍒欑敓鎴愪竴涓柊鐨?//
// 杩欎釜 key 鍐冲畾鎵€鏈夊瘑鏂囪兘鍚﹁瑙ｅ瘑锛屼竴鏃︿涪澶辨剰鍛崇潃鎵€鏈夊凡淇濆瓨杩炴帴澶辨晥锛堥渶鐢ㄦ埛閲嶆柊杈撳叆瀵嗙爜锛?func loadOrCreateMasterKey(path string) ([]byte, error) {
	// 1. 纭繚鐩綍瀛樺湪涓旀潈闄愭纭?	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("鍒涘缓 master key 鐩綍澶辫触: %w", err)
	}

	// 2. 鏂囦欢瀛樺湪 鈫?璇诲彇
	if data, err := os.ReadFile(path); err == nil {
		if len(data) == 32 {
			return data, nil
		}
		// 鏂囦欢瀛樺湪浣嗛暱搴︿笉瀵癸紝涓㈠純骞堕噸鏂扮敓鎴愶紙杩欐槸涓槑鏄鹃敊璇姸鎬侊級
		fmt.Printf("[SecretCipher] 鈿狅笍 master key 闀垮害寮傚父 (%d 瀛楄妭)锛岄噸鏂扮敓鎴愶紙宸插姞瀵嗘暟鎹皢鏃犳硶瑙ｅ瘑锛侊級\n", len(data))
	}

	// 3. 鐢熸垚鏂?key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("鐢熸垚 master key 澶辫触: %w", err)
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, fmt.Errorf("鍐欏叆 master key 澶辫触: %w", err)
	}
	fmt.Printf("[SecretCipher] 鉁?宸茬敓鎴愭柊鐨?master key: %s\n", path)
	return key, nil
}

// getMasterKeyPath 杩斿洖 master key 鏂囦欢鐨勮矾寰勶紙macOS / Linux 鐢級
//
// 浼樺厛绾э細$XDG_CONFIG_HOME/qssh/master.key > $HOME/.config/qssh/master.key
func getMasterKeyPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "qssh", "master.key")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// 鏈€鍚?fallback锛氬綋鍓嶇洰褰曠殑涓存椂鏂囦欢
		return filepath.Join(".qssh", "master.key")
	}
	return filepath.Join(home, ".config", "qssh", "master.key")
}
