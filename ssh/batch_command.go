package ssh

import (
	"fmt"
	"strings"
	"sync"
)

// BatchCommand 鎵归噺鍛戒护
type BatchCommand struct {
	Command  string `json:"command"`  // 鍛戒护妯℃澘锛堟敮鎸?{{host}}, {{user}}, {{port}} 绛夊彉閲忥級
	Targets  []string `json:"targets"` // 鐩爣杩炴帴 ID 鍒楄〃
}

// BatchResult 鎵归噺鎵ц缁撴灉
type BatchResult struct {
	ConnID  string `json:"connId"`
	Host    string `json:"host"`
	Command string `json:"command"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// TemplateVars 妯℃澘鍙橀噺
type TemplateVars struct {
	Host     string
	Port     int
	Username string
	Name     string
}

// ExpandTemplate 灞曞紑鍛戒护妯℃澘鍙橀噺
// 鏀寔鐨勫彉閲忥細{{host}}, {{port}}, {{user}}, {{name}}
func ExpandTemplate(template string, vars TemplateVars) string {
	result := template
	result = strings.ReplaceAll(result, "{{host}}", vars.Host)
	result = strings.ReplaceAll(result, "{{port}}", fmt.Sprintf("%d", vars.Port))
	result = strings.ReplaceAll(result, "{{user}}", vars.Username)
	result = strings.ReplaceAll(result, "{{name}}", vars.Name)
	return result
}

// ExecuteBatch 鎵归噺鎵ц鍛戒护
func (s *SSHService) ExecuteBatch(commands []BatchCommand) []BatchResult {
	var results []BatchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, batch := range commands {
		for _, connID := range batch.Targets {
			wg.Add(1)
			go func(connID, cmdTmpl string) {
				defer wg.Done()

				// 鑾峰彇杩炴帴淇℃伅鐢ㄤ簬妯℃澘鍙橀噺
				s.mu.RLock()
				client, ok := s.clients[connID]
				var vars TemplateVars
				if ok {
					vars = TemplateVars{
						Host:     client.config.Host,
						Port:     client.config.Port,
						Username: client.config.Username,
						Name:     client.config.Name,
					}
				}
				s.mu.RUnlock()

				if !ok {
					mu.Lock()
					results = append(results, BatchResult{
						ConnID:  connID,
						Command: cmdTmpl,
						Success: false,
						Error:   "杩炴帴涓嶅瓨鍦?,
					})
					mu.Unlock()
					return
				}

				// 灞曞紑妯℃澘鍙橀噺
				cmd := ExpandTemplate(cmdTmpl, vars)

				// 鎵ц鍛戒护
				result, err := client.ExecuteCommand(cmd)

				batchResult := BatchResult{
					ConnID:  connID,
					Host:    vars.Host,
					Command: cmd,
				}

				if err != nil {
					batchResult.Success = false
					batchResult.Error = err.Error()
				} else if result != nil {
					batchResult.Success = result.Success
					batchResult.Output = result.Stdout
					if !result.Success {
						batchResult.Error = result.Stderr
					}
				}

				// 璁板綍瀹¤鏃ュ織
				s.auditLogger.Log(connID, vars.Host, vars.Username, "batch-command", cmd, batchResult.Success, batchResult.Error)

				mu.Lock()
				results = append(results, batchResult)
				mu.Unlock()
			}(connID, batch.Command)
		}
	}

	wg.Wait()
	return results
}

// CompareResults 瀵规瘮鎵归噺鎵ц缁撴灉
func CompareResults(results []BatchResult) map[string][]BatchResult {
	grouped := make(map[string][]BatchResult)
	for _, r := range results {
		key := r.Command
		grouped[key] = append(grouped[key], r)
	}
	return grouped
}

