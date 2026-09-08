package ssh

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// FileInfo 鏂囦欢淇℃伅缁撴瀯浣?type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
	Owner   string    `json:"owner"`
	Group   string    `json:"group"`
}

// InitSFTP 鍒濆鍖?SFTP 瀹㈡埛绔?func (s *SSHClient) InitSFTP() error {
	if s.isConnected.Load() == 0 {
		return fmt.Errorf("鏈繛鎺ュ埌SSH鏈嶅姟鍣?)
	}
	if s.sftpClient != nil {
		return nil
	}

	client, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("鍒濆鍖朣FTP澶辫触: %v", err)
	}
	s.sftpClient = client
	return nil
}

// UploadFile 涓婁紶鏂囦欢
func (s *SSHClient) UploadFile(localPath, remotePath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("鎵撳紑鏈湴鏂囦欢澶辫触: %v", err)
	}
	defer localFile.Close()

	remoteFile, err := s.sftpClient.Create(remotePath)
	if err != nil {
		// SFTP 鏉冮檺涓嶈冻锛屽洖閫€鍒?sudo 鏂瑰紡
		localFile.Close()
		data, readErr := os.ReadFile(localPath)
		if readErr != nil {
			return fmt.Errorf("璇诲彇鏈湴鏂囦欢澶辫触: %v", readErr)
		}
		return s.uploadViaSudo(remotePath, data, nil)
	}
	defer remoteFile.Close()

	if _, err := io.Copy(remoteFile, localFile); err != nil {
		return fmt.Errorf("涓婁紶鏂囦欢澶辫触: %v", err)
	}
	return nil
}

// DownloadFile 涓嬭浇鏂囦欢锛堟敮鎸佹柇鐐圭画浼狅級
// 濡傛灉鏈湴鏂囦欢宸插瓨鍦ㄤ笖灏忎簬杩滅▼鏂囦欢澶у皬锛岃嚜鍔ㄤ粠鏂偣缁х画涓嬭浇
func (s *SSHClient) DownloadFile(remotePath, localPath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	remoteFile, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("鎵撳紑杩滅▼鏂囦欢澶辫触: %v", err)
	}
	defer remoteFile.Close()

	// 鑾峰彇杩滅▼鏂囦欢澶у皬
	remoteStat, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("鑾峰彇杩滅▼鏂囦欢淇℃伅澶辫触: %v", err)
	}
	remoteSize := remoteStat.Size()

	// 妫€鏌ユ湰鍦版枃浠舵槸鍚﹀凡瀛樺湪锛堟柇鐐圭画浼狅級
	var localFile *os.File
	offset := int64(0)
	if info, err := os.Stat(localPath); err == nil {
		existingSize := info.Size()
		if existingSize >= remoteSize {
			// 鏈湴鏂囦欢宸插畬鏁?			return nil
		}
		offset = existingSize
		localFile, err = os.OpenFile(localPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		localFile, err = os.Create(localPath)
	}
	if err != nil {
		return fmt.Errorf("鍒涘缓/鎵撳紑鏈湴鏂囦欢澶辫触: %v", err)
	}
	defer localFile.Close()

	// 浠庢柇鐐逛綅缃紑濮嬭鍙?	if offset > 0 {
		if _, err := remoteFile.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("seek 鍒版柇鐐逛綅缃け璐? %v", err)
		}
	}

	if _, err := io.Copy(localFile, remoteFile); err != nil {
		return fmt.Errorf("涓嬭浇鏂囦欢澶辫触 (宸蹭笅杞?%d/%d bytes): %v", offset, remoteSize, err)
	}
	return nil
}

// DownloadFileRange 涓嬭浇鏂囦欢鎸囧畾鑼冨洿锛堢敤浜庣画浼犳垨鍒嗗潡涓嬭浇锛?func (s *SSHClient) DownloadFileRange(remotePath string, offset int64, limit int64) ([]byte, error) {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return nil, err
		}
	}

	remoteFile, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("鎵撳紑杩滅▼鏂囦欢澶辫触: %v", err)
	}
	defer remoteFile.Close()

	// seek 鍒版寚瀹氫綅缃?	if offset > 0 {
		if _, err := remoteFile.Seek(offset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek 澶辫触: %v", err)
		}
	}

	// 璇诲彇鎸囧畾鑼冨洿
	if limit > 0 {
		data := make([]byte, limit)
		n, err := io.ReadFull(remoteFile, data)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("璇诲彇澶辫触: %v", err)
		}
		return data[:n], nil
	}

	// 鏃犻檺鍒讹紝璇诲彇鍏ㄩ儴
	data, err := io.ReadAll(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("璇诲彇澶辫触: %v", err)
	}
	return data, nil
}

// ListDirectory 鍒楀嚭杩滅▼鐩綍鍐呭锛堟€ц兘浼樺寲锛氱Щ闄ら€愭枃浠?Lstat锛屽紓姝ヨ幏鍙?owner/group锛?func (s *SSHClient) ListDirectory(remotePath string) ([]FileInfo, error) {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return nil, err
		}
	}

	if remotePath == "" {
		remotePath = "/"
	}

	files, err := s.sftpClient.ReadDir(remotePath)
	if err != nil {
		if strings.Contains(err.Error(), "file does not exist") || strings.Contains(err.Error(), "no such file") {
			return nil, fmt.Errorf("鐩綍涓嶅瓨鍦? %s", remotePath)
		}
		return nil, fmt.Errorf("璇诲彇鐩綍澶辫触: %v", err)
	}

	var fileInfos []FileInfo
	for _, file := range files {
		fullPath := remotePath
		if !strings.HasSuffix(remotePath, "/") {
			fullPath += "/"
		}
		fullPath += file.Name()

		fileInfos = append(fileInfos, FileInfo{
			Name:    file.Name(),
			Path:    fullPath,
			Size:    file.Size(),
			Mode:    file.Mode().String(),
			ModTime: file.ModTime(),
			IsDir:   file.IsDir(),
			Owner:   "-",
			Group:   "-",
		})
	}

	return fileInfos, nil
}

// batchGetFileOwners 涓€娆?SSH 璋冪敤鑾峰彇鐩綍涓嬫墍鏈夋枃浠剁殑 owner 鍜?group
func (s *SSHClient) batchGetFileOwners(dir string, files []os.FileInfo) []string {
	n := len(files)
	result := make([]string, n*2)
	for i := range result {
		result[i] = "unknown"
	}
	if n == 0 {
		return result
	}

	args := make([]string, n)
	for i, f := range files {
		args[i] = fmt.Sprintf("'%s/%s'", strings.TrimSuffix(dir, "/"), f.Name())
	}
	cmd := fmt.Sprintf("stat -c '%%U %%G' %s", strings.Join(args, " "))

	res, err := s.ExecuteCommand(cmd)
	if err != nil || !res.Success {
		return result
	}

	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	for i, line := range lines {
		if i >= n {
			break
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			result[i] = fields[0]
			result[n+i] = fields[1]
		}
	}
	return result
}

// GetFileOwners 鎸夐渶鑾峰彇鏂囦欢鐨?owner/group锛堝墠绔彲寮傛璋冪敤锛?func (s *SSHClient) GetFileOwners(dir string, filenames []string) map[string][2]string {
	result := make(map[string][2]string, len(filenames))
	for _, name := range filenames {
		result[name] = [2]string{"unknown", "unknown"}
	}
	if len(filenames) == 0 {
		return result
	}

	args := make([]string, len(filenames))
	for i, name := range filenames {
		args[i] = fmt.Sprintf("'%s/%s'", strings.TrimSuffix(dir, "/"), name)
	}
	cmd := fmt.Sprintf("stat -c '%%U %%G' %s", strings.Join(args, " "))

	res, err := s.ExecuteCommand(cmd)
	if err != nil || !res.Success {
		return result
	}

	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	for i, line := range lines {
		if i >= len(filenames) {
			break
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			result[filenames[i]] = [2]string{fields[0], fields[1]}
		}
	}
	return result
}

// SearchFiles 閫掑綊鎼滅储鏂囦欢
func (s *SSHClient) SearchFiles(basePath string, keyword string, app *application.App, searchID string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	keyword = strings.ToLower(keyword)
	foundCount := 0

	s.searchMutex.Lock()
	s.searchCancelMap[searchID] = false
	s.searchMutex.Unlock()

	if app != nil {
		app.Event.Emit("search-start", map[string]interface{}{
			"searchID": searchID,
			"basePath": basePath,
			"keyword":  keyword,
		})
	}

	var searchRecursive func(path string) error
	searchRecursive = func(path string) error {
		s.searchMutex.RLock()
		cancelled := s.searchCancelMap[searchID]
		s.searchMutex.RUnlock()
		if cancelled {
			return fmt.Errorf("鎼滅储宸插彇娑?)
		}

		files, err := s.sftpClient.ReadDir(path)
		if err != nil {
			return err
		}

		for _, file := range files {
			s.searchMutex.RLock()
			cancelled := s.searchCancelMap[searchID]
			s.searchMutex.RUnlock()
			if cancelled {
				return fmt.Errorf("鎼滅储宸插彇娑?)
			}

			if file.Name() == "." || file.Name() == ".." {
				continue
			}

			fullPath := path
			if !strings.HasSuffix(path, "/") {
				fullPath += "/"
			}
			fullPath += file.Name()

			if strings.Contains(strings.ToLower(file.Name()), keyword) {
				fileStat, err := s.sftpClient.Lstat(fullPath)
				isDir := false
				if err == nil && fileStat != nil {
					isDir = fileStat.IsDir()
				} else {
					isDir = file.IsDir()
				}

				relativePath := fullPath
				if strings.HasPrefix(fullPath, basePath) {
					relativePath = strings.TrimPrefix(fullPath, basePath)
					relativePath = strings.TrimPrefix(relativePath, "/")
				}

				if app != nil {
					foundCount++
					app.Event.Emit("search-result", map[string]interface{}{
						"searchID":     searchID,
						"file":         FileInfo{Name: file.Name(), Path: fullPath, Size: file.Size(), Mode: file.Mode().String(), ModTime: file.ModTime(), IsDir: isDir, Owner: "-", Group: "-"},
						"relativePath": relativePath,
					})
				}
			}

			if file.IsDir() {
				if err := searchRecursive(fullPath); err != nil {
					if strings.Contains(err.Error(), "鎼滅储宸插彇娑?) {
						return err
					}
				}
			}
		}
		return nil
	}

	err := searchRecursive(basePath)

	s.searchMutex.Lock()
	delete(s.searchCancelMap, searchID)
	s.searchMutex.Unlock()

	if app != nil {
		app.Event.Emit("search-complete", map[string]interface{}{
			"searchID":   searchID,
			"totalFound": foundCount,
			"error":      err != nil,
		})
	}

	if err != nil && strings.Contains(err.Error(), "鎼滅储宸插彇娑?) {
		return nil
	}
	return err
}

// CancelSearch 鍙栨秷鎼滅储
func (s *SSHClient) CancelSearch(searchID string) {
	s.searchMutex.Lock()
	defer s.searchMutex.Unlock()
	if _, ok := s.searchCancelMap[searchID]; ok {
		s.searchCancelMap[searchID] = true
	}
}

// DeleteFile 鍒犻櫎杩滅▼鏂囦欢鎴栫洰褰?func (s *SSHClient) DeleteFile(remotePath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	fileInfo, err := s.sftpClient.Lstat(remotePath)
	if err != nil {
		return fmt.Errorf("鑾峰彇鏂囦欢淇℃伅澶辫触: %v", err)
	}

	isSymlink := fileInfo.Mode()&os.ModeSymlink != 0

	var deleteErr error
	if isSymlink {
		deleteErr = s.sftpClient.Remove(remotePath)
	} else if fileInfo.IsDir() {
		deleteErr = s.removeDir(remotePath)
	} else {
		deleteErr = s.sftpClient.Remove(remotePath)
	}

	// SFTP 鍒犻櫎澶辫触锛屽皾璇?sudo
	if deleteErr != nil {
		return s.deleteViaSudo(remotePath, fileInfo.IsDir())
	}
	return nil
}

// removeDir 閫掑綊鍒犻櫎鐩綍
func (s *SSHClient) removeDir(path string) error {
	files, err := s.sftpClient.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.Name() == "." || file.Name() == ".." {
			continue
		}

		fullPath := path + "/" + file.Name()

		if file.IsDir() {
			err = s.removeDir(fullPath)
		} else {
			err = s.sftpClient.Remove(fullPath)
		}
		if err != nil {
			return err
		}
	}

	return s.sftpClient.RemoveDirectory(path)
}

// CreateDirectory 鍒涘缓鐩綍
func (s *SSHClient) CreateDirectory(remotePath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	err := s.sftpClient.MkdirAll(remotePath)
	if err != nil {
		// SFTP 澶辫触锛屽皾璇?sudo
		return s.mkdirViaSudo(remotePath)
	}
	return nil
}

// mkdirViaSudo 閫氳繃 sudo 鍒涘缓鐩綍
func (s *SSHClient) mkdirViaSudo(remotePath string) error {
	password := s.config.Password
	if password == "" {
		return fmt.Errorf("鏉冮檺涓嶈冻涓旀湭閰嶇疆瀵嗙爜锛屾棤娉曞垱寤虹洰褰?%s", remotePath)
	}

	encodedPw := base64.StdEncoding.EncodeToString([]byte(password))
	cmd := fmt.Sprintf("echo %s | base64 -d | sudo -S mkdir -p %s 2>&1",
		encodedPw, remotePath)
	result, err := s.ExecuteCommand(cmd)
	if err != nil || !result.Success {
		return fmt.Errorf("鏉冮檺涓嶈冻锛屾棤娉曞垱寤虹洰褰?%s", remotePath)
	}
	return nil
}

// RenameFile 閲嶅懡鍚嶆枃浠?鐩綍
func (s *SSHClient) RenameFile(oldPath, newPath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	err := s.sftpClient.Rename(oldPath, newPath)
	if err != nil {
		// SFTP 澶辫触锛屽皾璇?sudo
		return s.renameViaSudo(oldPath, newPath)
	}
	return nil
}

// renameViaSudo 閫氳繃 sudo 閲嶅懡鍚嶆枃浠?func (s *SSHClient) renameViaSudo(oldPath, newPath string) error {
	password := s.config.Password
	if password == "" {
		return fmt.Errorf("鏉冮檺涓嶈冻涓旀湭閰嶇疆瀵嗙爜锛屾棤娉曢噸鍛藉悕 %s", oldPath)
	}

	encodedPw := base64.StdEncoding.EncodeToString([]byte(password))
	cmd := fmt.Sprintf("echo %s | base64 -d | sudo -S mv %s %s 2>&1",
		encodedPw, oldPath, newPath)
	result, err := s.ExecuteCommand(cmd)
	if err != nil || !result.Success {
		return fmt.Errorf("鏉冮檺涓嶈冻锛屾棤娉曢噸鍛藉悕 %s", oldPath)
	}
	return nil
}

// CopyFile 澶嶅埗鏂囦欢
func (s *SSHClient) CopyFile(srcPath, destPath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	srcFile, err := s.sftpClient.Open(srcPath)
	if err != nil {
		return fmt.Errorf("鎵撳紑婧愭枃浠跺け璐? %v", err)
	}

	srcInfo, err := srcFile.Stat()
	if err != nil {
		srcFile.Close()
		return fmt.Errorf("鑾峰彇婧愭枃浠朵俊鎭け璐? %v", err)
	}

	destFile, err := s.sftpClient.Create(destPath)
	if err != nil {
		srcFile.Close()
		return s.copyFileViaShell(srcPath, destPath)
	}

	if _, err := io.Copy(destFile, srcFile); err != nil {
		srcFile.Close()
		destFile.Close()
		return s.copyFileViaShell(srcPath, destPath)
	}
	srcFile.Close()
	destFile.Close()

	_ = s.sftpClient.Chmod(destPath, srcInfo.Mode())
	return nil
}

// copyFileViaShell 鐢?shell 鍛戒护澶嶅埗锛圫FTP 鏉冮檺涓嶈冻鏃剁殑 fallback锛夈€?func (s *SSHClient) copyFileViaShell(srcPath, destPath string) error {
	cmd := fmt.Sprintf("cp -p %s %s 2>&1", srcPath, destPath)
	result, err := s.ExecuteCommand(cmd)
	if err == nil && result.Success {
		return nil
	}

	password := s.config.Password
	if password == "" {
		return fmt.Errorf("鏉冮檺涓嶈冻涓旀湭閰嶇疆瀵嗙爜锛屾棤娉曞鍒舵枃浠?)
	}

	encodedPw := base64.StdEncoding.EncodeToString([]byte(password))
	cmd = fmt.Sprintf("echo %s | base64 -d | sudo -S cp -p %s %s 2>&1",
		encodedPw, srcPath, destPath)
	result, err = s.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("sudo cp 鎵ц澶辫触: %v", err)
	}
	if !result.Success {
		errMsg := strings.TrimSpace(result.Stderr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.Stdout)
		}
		return fmt.Errorf("sudo cp 澶辫触: %s", errMsg)
	}
	return nil
}

// UploadFileFromBytes 浠庡瓧鑺傛暟缁勪笂浼犳枃浠讹紙鏀寔鏂偣缁紶锛?// 濡傛灉杩滅▼鏂囦欢宸插瓨鍦ㄤ笖灏忎簬鏈湴鏁版嵁澶у皬锛岃嚜鍔ㄤ粠鏂偣缁х画鍐欏叆
func (s *SSHClient) UploadFileFromBytes(remotePath string, data []byte, progressCallback func(percent int)) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	totalSize := len(data)
	offset := int64(0)

	// 妫€鏌ヨ繙绋嬫枃浠舵槸鍚﹀凡瀛樺湪锛堟柇鐐圭画浼狅級
	if stat, err := s.sftpClient.Stat(remotePath); err == nil {
		existingSize := stat.Size()
		if existingSize >= int64(totalSize) {
			// 杩滅▼鏂囦欢宸插畬鏁达紝璺宠繃涓婁紶
			if progressCallback != nil {
				progressCallback(100)
			}
			return nil
		}
		offset = existingSize
	}

	var remoteFile *sftp.File
	var err error

	if offset > 0 {
		// 杩藉姞妯″紡鎵撳紑
		remoteFile, err = s.sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_APPEND)
	} else {
		// 鏂板缓鏂囦欢
		remoteFile, err = s.sftpClient.Create(remotePath)
	}

	if err != nil {
		return s.uploadViaSudo(remotePath, data, progressCallback)
	}
	defer remoteFile.Close()

	// 浠庢柇鐐逛綅缃紑濮嬪啓鍏?	startPos := int(offset)
	chunkSize := 32 * 1024
	written := startPos
	lastReported := 0

	// 璁＄畻鍒濆杩涘害
	if progressCallback != nil && startPos > 0 {
		lastReported = startPos * 100 / totalSize
		progressCallback(lastReported)
	}

	for written < totalSize {
		end := written + chunkSize
		if end > totalSize {
			end = totalSize
		}

		n, err := remoteFile.Write(data[written:end])
		if err != nil {
			return fmt.Errorf("鍐欏叆杩滅▼鏂囦欢澶辫触 (宸插啓鍏?%d/%d bytes): %v", written, totalSize, err)
		}
		written += n

		if progressCallback != nil {
			percent := written * 100 / totalSize
			if percent >= lastReported+5 || percent >= 100 {
				lastReported = percent
				progressCallback(percent)
			}
		}
	}

	// 楠岃瘉鏈€缁堟枃浠跺ぇ灏?	fileInfo, err := s.sftpClient.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("楠岃瘉鏂囦欢澶辫触: %v", err)
	}
	if fileInfo.Size() != int64(totalSize) {
		return fmt.Errorf("鏂囦欢澶у皬涓嶅尮閰? 鏈熸湜 %d bytes, 瀹為檯 %d bytes", totalSize, fileInfo.Size())
	}

	return nil
}

// DownloadFileToBytes 涓嬭浇杩滅▼鏂囦欢骞惰繑鍥炲瓧鑺傛暟缁?func (s *SSHClient) DownloadFileToBytes(remotePath string) ([]byte, error) {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return nil, err
		}
	}

	remoteFile, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("鎵撳紑杩滅▼鏂囦欢澶辫触: %v", err)
	}
	defer remoteFile.Close()

	data, err := io.ReadAll(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("璇诲彇杩滅▼鏂囦欢澶辫触: %v", err)
	}
	return data, nil
}

// UploadDirectory 涓婁紶鐩綍锛堥€掑綊锛?func (s *SSHClient) UploadDirectory(localPath, remotePath string) error {
	if s.sftpClient == nil {
		if err := s.InitSFTP(); err != nil {
			return err
		}
	}

	if err := s.sftpClient.MkdirAll(remotePath); err != nil {
		// SFTP 鏉冮檺涓嶈冻锛屽皾璇?sudo
		if mkdirErr := s.mkdirViaSudo(remotePath); mkdirErr != nil {
			return fmt.Errorf("鍒涘缓杩滅▼鐩綍澶辫触: %v", mkdirErr)
		}
	}

	entries, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("璇诲彇鏈湴鐩綍澶辫触: %v", err)
	}

	for _, entry := range entries {
		localFilePath := localPath + "/" + entry.Name()
		remoteFilePath := remotePath + "/" + entry.Name()

		if entry.IsDir() {
			if err := s.UploadDirectory(localFilePath, remoteFilePath); err != nil {
				return err
			}
		} else {
			if err := s.UploadFile(localFilePath, remoteFilePath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateArchive 鍒涘缓鍘嬬缉鍖?func (s *SSHClient) CreateArchive(files []string, archiveName string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("娌℃湁鏂囦欢闇€瑕佸帇缂?)
	}

	tempArchive := fmt.Sprintf("/tmp/%s.tar.gz", archiveName)

	cmd := fmt.Sprintf("tar czf '%s'", tempArchive)
	for _, file := range files {
		cmd += fmt.Sprintf(" '%s'", file)
	}

	result, err := s.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("鎵ц鍘嬬缉鍛戒护澶辫触: %v", err)
	}
	if !result.Success {
		return "", fmt.Errorf("鍘嬬缉澶辫触: %s", result.Stderr)
	}

	return tempArchive, nil
}

// DeleteTempFile 鍒犻櫎涓存椂鏂囦欢
func (s *SSHClient) DeleteTempFile(filePath string) error {
	cmd := fmt.Sprintf("rm -f '%s'", filePath)
	result, err := s.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("鍒犻櫎涓存椂鏂囦欢澶辫触: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("鍒犻櫎澶辫触: %s", result.Stderr)
	}
	return nil
}

// ExtractArchive 瑙ｅ帇鍘嬬缉鍖?func (s *SSHClient) ExtractArchive(archivePath string, targetDir string) error {
	var cmd string
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		cmd = fmt.Sprintf("cd '%s' && unzip -o '%s'", targetDir, archivePath)
	case strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz"):
		cmd = fmt.Sprintf("cd '%s' && tar xzf '%s'", targetDir, archivePath)
	case strings.HasSuffix(archivePath, ".tar"):
		cmd = fmt.Sprintf("cd '%s' && tar xf '%s'", targetDir, archivePath)
	default:
		return fmt.Errorf("涓嶆敮鎸佺殑鍘嬬缉鏍煎紡: %s", archivePath)
	}

	result, err := s.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("鎵ц瑙ｅ帇鍛戒护澶辫触: %v", err)
	}
	if !result.Success {
		return fmt.Errorf("瑙ｅ帇澶辫触: %s", result.Stderr)
	}
	return nil
}

// deleteViaSudo 閫氳繃 sudo 鍒犻櫎鏂囦欢锛圫FTP 鏉冮檺涓嶈冻鏃剁殑鍥為€€鏂规锛?func (s *SSHClient) deleteViaSudo(remotePath string, isDir bool) error {
	password := s.config.Password
	if password == "" {
		return fmt.Errorf("鏉冮檺涓嶈冻涓旀湭閰嶇疆瀵嗙爜锛屾棤娉曞垹闄?%s", remotePath)
	}

	rmFlag := "-f"
	if isDir {
		rmFlag = "-rf"
	}

	encodedPw := base64.StdEncoding.EncodeToString([]byte(password))
	cmd := fmt.Sprintf("echo %s | base64 -d | sudo -S rm %s %s 2>&1",
		encodedPw, rmFlag, remotePath)
	result, err := s.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("sudo rm 鎵ц澶辫触: %v", err)
	}
	if !result.Success {
		errMsg := strings.TrimSpace(result.Stderr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.Stdout)
		}
		return fmt.Errorf("鏉冮檺涓嶈冻锛屾棤娉曞垹闄?%s: %s", remotePath, errMsg)
	}
	return nil
}

// uploadViaSudo 閫氳繃 sudo 涓婁紶鏂囦欢锛圫FTP 鏉冮檺涓嶈冻鏃剁殑鍥為€€鏂规锛?// 鍏堜笂浼犲埌 /tmp锛屽啀鐢?sudo mv 鍒扮洰鏍囦綅缃?func (s *SSHClient) uploadViaSudo(remotePath string, data []byte, progressCallback func(percent int)) error {
	// 1. 涓婁紶鍒?/tmp
	tmpPath := fmt.Sprintf("/tmp/pzssh_upload_%d", time.Now().UnixNano())
	tmpFile, err := s.sftpClient.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("鍒涘缓涓存椂鏂囦欢澶辫触: %v", err)
	}

	// 鍐欏叆鏁版嵁
	written := 0
	totalSize := len(data)
	for written < totalSize {
		end := written + 32*1024
		if end > totalSize {
			end = totalSize
		}
		n, err := tmpFile.Write(data[written:end])
		if err != nil {
			tmpFile.Close()
			s.sftpClient.Remove(tmpPath)
			return fmt.Errorf("鍐欏叆涓存椂鏂囦欢澶辫触: %v", err)
		}
		written += n
		if progressCallback != nil {
			progressCallback(written * 100 / totalSize)
		}
	}
	tmpFile.Close()

	// 2. 鐢?sudo mv 绉诲姩鍒扮洰鏍囦綅缃?	password := s.config.Password
	if password == "" {
		s.sftpClient.Remove(tmpPath)
		return fmt.Errorf("鏉冮檺涓嶈冻涓旀湭閰嶇疆瀵嗙爜锛屾棤娉曚笂浼犲埌 %s", remotePath)
	}

	encodedPw := base64.StdEncoding.EncodeToString([]byte(password))
	cmd := fmt.Sprintf("echo %s | base64 -d | sudo -S mv %s %s 2>&1",
		encodedPw, tmpPath, remotePath)
	result, err := s.ExecuteCommand(cmd)
	if err != nil {
		s.sftpClient.Remove(tmpPath)
		return fmt.Errorf("sudo mv 鎵ц澶辫触: %v", err)
	}
	if !result.Success {
		s.sftpClient.Remove(tmpPath)
		errMsg := strings.TrimSpace(result.Stderr)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.Stdout)
		}
		return fmt.Errorf("sudo mv 澶辫触: %s", errMsg)
	}

	return nil
}

