# Xuandesk 三平台发布构建

目标产物：

- Windows x64：EXE / MSI，以及便携目录 artifact
- Linux x64：DEB + `Xuandesk-linux-x64.tar.gz`
- Android：arm64 + armv7 + x86_64 universal APK

## 推荐：GitHub Actions

将整个项目提交到一个 GitHub 仓库后，打开 **Actions → Xuandesk - Windows Linux Android Release → Run workflow**。
默认三个开关都为 `true`。完成后在该 workflow run 的 **Artifacts** 区域下载：

- `Xuandesk-windows-x64`
- `Xuandesk-linux-x64-portable`
- `rustdesk-1.5.0-x86_64.deb`
- `Xuandesk-android-universal`

本发布工作流只上传 Actions artifacts，不创建 GitHub Release，也不会自动签名 Windows 文件。Android 若未配置正式签名 secret，会采用 debug signing 生成可安装 APK。

## 本机构建

`scripts/build-windows.ps1`、`scripts/build-linux.sh`、`scripts/build-android.ps1` 已补上原始简化脚本缺失的 Rust native library 构建步骤。

RustDesk 完整构建依赖 vcpkg、Flutter、Rust、C/C++ toolchain；Android 还依赖 Android SDK/NDK。因此干净环境优先使用 GitHub Actions。

## 许可

发行时保留根目录 `LICENCE` 以及上游依赖的许可证/NOTICE 文件。项目包含 RustDesk 派生代码与 `ssh/NOTICE-qssh.txt`，商业分发前应再次核对相应许可证义务。
