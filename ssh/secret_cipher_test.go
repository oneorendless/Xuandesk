package ssh

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestAESCipherRoundTrip 楠岃瘉 AES cipher 鐨勫姞瑙ｅ瘑寰€杩?func TestAESCipherRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	masterKeyPath := filepath.Join(tmpDir, "master.key")

	cipher := newAESCipher(masterKeyPath)

	plain := "my-super-secret-password-!@#$%^&*()_+"

	// 鍔犲瘑
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("鍔犲瘑澶辫触: %v", err)
	}
	if enc == "" {
		t.Fatal("鍔犲瘑缁撴灉涓虹┖")
	}
	if enc == plain {
		t.Fatal("鍔犲瘑缁撴灉涓庢槑鏂囩浉鍚岋紝cipher 娌″伐浣滐紒")
	}

	// 瑙ｅ瘑
	dec, err := cipher.Decrypt(enc)
	if err != nil {
		t.Fatalf("瑙ｅ瘑澶辫触: %v", err)
	}
	if dec != plain {
		t.Errorf("瑙ｅ瘑缁撴灉涓嶄竴鑷?\n  got:  %q\n  want: %q", dec, plain)
	}
}

// TestAESCipherWrongKey 楠岃瘉鐢ㄩ敊鐨?master key 鍔犲瘑鐨勫瘑鏂囪В涓嶅嚭鏉?//
// 杩欎繚璇?attacker 鍗充娇鎷垮埌瀵嗘枃锛岀疆鎹竴涓?key 涔熻В涓嶅嚭鏉ャ€?func TestAESCipherWrongKey(t *testing.T) {
	tmpDir := t.TempDir()

	// 鐢?key A 鍔犲瘑
	cipherA := newAESCipher(filepath.Join(tmpDir, "keyA.key"))
	enc, err := cipherA.Encrypt("secret payload")
	if err != nil {
		t.Fatalf("鍔犲瘑澶辫触: %v", err)
	}

	// 鐢?key B 瑙ｅ瘑 鈫?蹇呯劧澶辫触
	cipherB := newAESCipher(filepath.Join(tmpDir, "keyB.key"))
	_, err = cipherB.Decrypt(enc)
	if err == nil {
		t.Fatal("鐢ㄩ敊璇?key 瑙ｅ瘑绔熺劧鎴愬姛浜嗭紒cipher 娌℃湁鐪熸闅旂 key")
	}
}

// TestAESCipherNonceUniqueness 楠岃瘉澶氭鍔犲瘑鍚屼竴涓槑鏂囧緱鍒颁笉鍚屽瘑鏂囷紙GCM nonce 蹇呴』鍞竴锛?func TestAESCipherNonceUniqueness(t *testing.T) {
	tmpDir := t.TempDir()
	cipher := newAESCipher(filepath.Join(tmpDir, "master.key"))

	plain := "same input"
	enc1, _ := cipher.Encrypt(plain)
	enc2, _ := cipher.Encrypt(plain)

	if enc1 == enc2 {
		t.Fatal("涓ゆ鍔犲瘑鍚屾槑鏂囧緱鍒扮浉鍚屽瘑鏂囷紝nonce 閲嶅 = 涓ラ噸瀹夊叏婕忔礊锛?)
	}
}

// TestAESCipherTamperDetection 楠岃瘉瀵嗘枃琚鏀瑰悗鑳芥娴嬪嚭鏉?func TestAESCipherTamperDetection(t *testing.T) {
	tmpDir := t.TempDir()
	cipher := newAESCipher(filepath.Join(tmpDir, "master.key"))

	enc, _ := cipher.Encrypt("important data")

	// 缈昏浆瀵嗘枃鐨勬渶鍚庝竴涓瓧绗︼紙GCM 璁よ瘉 tag 搴旇兘鎹曡幏杩欎釜绡℃敼锛?	tampered := enc[:len(enc)-1]
	if enc[len(enc)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}

	_, err := cipher.Decrypt(tampered)
	if err == nil {
		t.Fatal("琚鏀圭殑瀵嗘枃绔熺劧瑙ｅ瘑鎴愬姛浜嗭紒AES-GCM 璁よ瘉澶辫触锛?)
	}
}

// TestMasterKeyPersistence 楠岃瘉 master key 鍦ㄩ噸鍚悗鑳界户缁В瀵嗗悓涓€鏂囦欢
func TestMasterKeyPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	masterKeyPath := filepath.Join(tmpDir, "master.key")

	// 绗竴娆″惎鍔細鐢熸垚 master key 骞跺姞瀵?	cipher1 := newAESCipher(masterKeyPath)
	enc, err := cipher1.Encrypt("persistent secret")
	if err != nil {
		t.Fatalf("棣栨鍔犲瘑澶辫触: %v", err)
	}

	// 妫€鏌?master key 鏂囦欢鏄惁瀛樺湪
	info, err := os.Stat(masterKeyPath)
	if err != nil {
		t.Fatalf("master key 鏂囦欢鏈敓鎴? %v", err)
	}
	// Unix 涓嬫鏌ユ潈闄愪负 0600锛學indows 涓嶉€傜敤 Unix 鏉冮檺妯″瀷
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("master key 鏂囦欢鏉冮檺搴旇鏄?0600锛屽疄闄呮槸 %o", perm)
		}
	}

	// 绗簩娆″惎鍔細閲嶆柊鏋勯€?cipher锛屽簲璇ヨ兘瑙ｅ瘑涔嬪墠鐨勬暟鎹?	cipher2 := newAESCipher(masterKeyPath)
	dec, err := cipher2.Decrypt(enc)
	if err != nil {
		t.Fatalf("閲嶅惎鍚庤В瀵嗗け璐? %v", err)
	}
	if dec != "persistent secret" {
		t.Errorf("閲嶅惎鍚庤В瀵嗙粨鏋滀笉瀵? got %q", dec)
	}
}

// TestGetMasterKeyPath 楠岃瘉 master key 璺緞鍑芥暟
func TestGetMasterKeyPath(t *testing.T) {
	path := getMasterKeyPath()
	if path == "" {
		t.Fatal("master key 璺緞涓嶅簲涓虹┖")
	}
	// 璺緞搴旇鍖呭惈 qssh 瀛愮洰褰?	if !filepath.IsAbs(path) && path[0] != '.' {
		t.Errorf("master key 璺緞搴旇涓虹粷瀵硅矾寰勬垨褰撳墠鐩綍鐩稿璺緞锛?s", path)
	}
}

