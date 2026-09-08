package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	dataDirOnce sync.Once
	dataDirPath string
)

// GetDataDir 杩斿洖搴旂敤鏁版嵁鐩綍锛堢粺涓€瀹炵幇锛屼緵 ssh 鍜?ai 妯″潡鍏辩敤锛夈€?//
// 浼樺厛绾э細
//  1. 鍙墽琛屾枃浠舵墍鍦ㄧ洰褰曚笅鐨?data/
//  2. 褰撳墠宸ヤ綔鐩綍涓嬬殑 data/锛坉ev 妯″紡锛?//  3. 褰撳墠鐩綍锛堟渶缁?fallback锛?func GetDataDir() string {
	dataDirOnce.Do(func() {
		// 浼樺厛鐢ㄥ彲鎵ц鏂囦欢鐩綍锛堢敓浜фā寮忥級
		if exePath, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exePath), "data")
			if mkErr := os.MkdirAll(candidate, 0755); mkErr == nil {
				dataDirPath = candidate
				return
			}
		}

		// 鍥為€€鍒板綋鍓嶅伐浣滅洰褰曪紙dev 妯″紡锛?		if cwd, err := os.Getwd(); err == nil {
			candidate := filepath.Join(cwd, "data")
			if mkErr := os.MkdirAll(candidate, 0755); mkErr == nil {
				dataDirPath = candidate
				return
			}
		}

		dataDirPath = "."
		fmt.Fprintln(os.Stderr, "[pathutil] 璀﹀憡锛氭棤娉曠‘瀹氭暟鎹洰褰曪紝浣跨敤褰撳墠鐩綍")
	})
	return dataDirPath
}

