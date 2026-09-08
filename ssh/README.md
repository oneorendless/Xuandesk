# Xuandesk SSH/SFTP Bridge

This directory contains the qssh SSH implementation vendored into Xuandesk.
`SSHService` owns connections and provides terminal, SFTP, process and system
monitoring methods. `BridgeServer` exposes the small cross-platform HTTP
surface consumed by Flutter (`/api/ssh/{id}/stats`, `files`, `download`, and
`upload`).

Example host integration:

```go
service := ssh.NewSSHService()
bridge := ssh.NewBridgeServer(service)
go bridge.ListenAndServe("127.0.0.1:23891")
```

Set `XUANDESK_SSH_TOKEN` before starting the bridge when requests must include
`X-Xuandesk-Token`. The bridge is intended to bind to localhost or sit behind
the authenticated Xuandesk API gateway; do not expose it directly to the
internet.
