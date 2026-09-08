# Xuandesk 二次开发说明

Xuandesk 基于 RustDesk 客户端源码，保留原有远程桌面、终端和文件传输能力，并加入了账户与 SSH 扩展。

## 已接入

- 账户注册：Flutter 登录窗口提供“创建账户”，请求 `POST /api/register`，并复用登录响应格式。
- 账户绑定服务器：登录/注册响应可以带 `server_config`（或 `serverConfig`），字段为 `api`、`id`、`relay`、`key`。客户端会写入 `api-server`、`custom-rendezvous-server`、`relay-server` 和 `key`，同一账户再次登录即可自动使用对应服务端。
- 账户设备列表：沿用 RustDesk 的 address-book/group 同步，登录成功后自动刷新，因此同一账户下的设备会直接显示在地址簿。
- qssh SSH/SFTP：`ssh/` 是从 qssh 合并的共享 Go 包，包含 SSH、SFTP、进程/CPU/内存/网络监控、文件上传下载和目录浏览。
- 跨平台 SSH Bridge：`ssh/bridge_server.go` 提供统一 HTTP 接口，Flutter 模型 `flutter/lib/models/ssh_service.dart` 可用于 Windows、Linux 和 Android。

## SSH Bridge 接口

启动时设置 `XUANDESK_SSH_TOKEN` 可启用请求鉴权。接口路径如下：

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| POST | `/api/ssh/connect` | 建立 SSH 连接（JSON: `connId`, `config`） |
| GET | `/api/ssh/{id}/stats` | CPU、内存、网络和进程统计 |
| GET | `/api/ssh/{id}/files?path=/` | 远程目录列表 |
| GET | `/api/ssh/{id}/download?path=/tmp/a` | 下载文件 |
| POST | `/api/ssh/{id}/upload?path=/tmp/a` | 上传文件（二进制 body） |
| POST | `/api/ssh/{id}/disconnect` | 断开 SSH 连接 |

`SSHService` 仍可直接用于 Wails/桌面端；移动端通过相同的 HTTP 合约接入，不需要维护两套 SSH 实现。

## 构建

先初始化 RustDesk 的依赖（压缩包没有携带 git submodule）：

```powershell
cd F:\vs\RUST\Xuandesk
git submodule update --init --recursive
cd flutter
flutter pub get
```

桌面端：

```powershell
flutter build windows
flutter build linux
```

Android APK：

```powershell
flutter build apk --release
```

SSH 扩展单独检查：

```powershell
cd ssh
go test ./...
```

本机工具链安装与构建脚本位于 `scripts/`：`setup-toolchains.ps1`、`build-windows.ps1`、`build-android.ps1` 和 `build-linux.sh`。Windows 构建可直接运行 `powershell -ExecutionPolicy Bypass -File scripts/build-windows.ps1`；Linux 构建建议在 WSL2/Ubuntu 或 Linux CI 上运行。完整 RustDesk 构建还需要 Visual Studio C++、Android SDK/NDK 和 vcpkg 依赖。

服务端一键部署文件位于 `server/`。将该目录上传阿里云 ECS，编辑 `.env` 后运行 `sudo bash install-aliyun.sh`；脚本监听 `51114-51119`，不会停止旧 RustDesk 服务。

账户 API 需要由部署方提供 `/api/register`、`/api/login`、`/api/currentUser` 和设备/地址簿接口；`server_config` 是 Xuandesk 的新增响应字段。

## 上游许可

RustDesk 与 qssh 的原始许可证和版权声明必须随发行包保留。qssh 目录沿用其原项目许可证，商业发布前请先核对 qssh 当前许可证限制。
