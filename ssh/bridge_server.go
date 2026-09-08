package ssh

// HTTP bridge used by the Flutter client. It deliberately lives beside the
// qssh service so the SSH/SFTP implementation remains shared by Windows,
// Linux and Android builds. Put it behind the Xuandesk account gateway in
// production; X-Xuandesk-Token is a lightweight local-development guard.

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

type BridgeServer struct {
	service *SSHService
	token   string
}

func NewBridgeServer(service *SSHService) *BridgeServer {
	return &BridgeServer{service: service, token: os.Getenv("XUANDESK_SSH_TOKEN")}
}

func (b *BridgeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if b.token != "" && r.Header.Get("X-Xuandesk-Token") != b.token {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if path == "api/ssh/connect" && r.Method == http.MethodPost {
		var req struct {
			ConnID string     `json:"connId"`
			Config SSHConfig  `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ConnID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid connect request"})
			return
		}
		if err := b.service.Connect(req.ConnID, &req.Config); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"connId": req.ConnID})
		return
	}
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "ssh" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	connID := parts[2]
	if len(parts) == 4 && parts[3] == "disconnect" && r.Method == http.MethodPost {
		if err := b.service.Disconnect(connID); err != nil { writeError(w, err); return }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if len(parts) == 4 && parts[3] == "stats" && r.Method == http.MethodGet {
		stats, err := b.service.GetSystemStats(connID)
		if err != nil { writeError(w, err); return }
		writeJSON(w, http.StatusOK, stats)
		return
	}
	if len(parts) == 4 && parts[3] == "files" && r.Method == http.MethodGet {
		files, err := b.service.ListFiles(connID, r.URL.Query().Get("path"))
		if err != nil { writeError(w, err); return }
		writeJSON(w, http.StatusOK, files)
		return
	}
	if len(parts) == 4 && parts[3] == "download" && r.Method == http.MethodGet {
		data, err := b.service.DownloadFile(connID, r.URL.Query().Get("path"))
		if err != nil { writeError(w, err); return }
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
		return
	}
	if len(parts) == 4 && parts[3] == "upload" && r.Method == http.MethodPost {
		data, err := readBody(r)
		if err != nil { writeError(w, err); return }
		if err = b.service.UploadFile(connID, r.URL.Query().Get("path"), base64.StdEncoding.EncodeToString(data)); err != nil {
			writeError(w, err); return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (b *BridgeServer) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, b)
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 512<<20+1))
	if err == nil && len(data) > 512<<20 {
		return nil, io.ErrShortBuffer
	}
	return data, err
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
