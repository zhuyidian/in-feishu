# codex-remote-feishu

这是 `in-feishu` 的核心源码目录，包含飞书接入、Codex wrapper、daemon、WebSetup、release 打包脚本和飞书应用配置模板。

普通用户建议从仓库根目录开始：

https://github.com/zhuyidian/in-feishu

## 快速安装

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/zhuyidian/in-feishu/main/install-release.sh | bash
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/zhuyidian/in-feishu/main/install-release.ps1 | iex
```

安装后打开命令行输出中的 `/setup` 或 WebSetup 链接，按向导完成飞书接入、权限配置和用户绑定。

## 这个目录里有什么

- `cmd/`：`codex-remote` 等命令入口。
- `internal/`：核心业务逻辑，包括飞书网关、会话控制、安装升级、WebSetup 和 Codex wrapper。
- `web/`：管理页和 WebSetup 前端。
- `deploy/feishu/`：飞书应用菜单、事件订阅和权限配置参考。
- `scripts/`：构建、检查、打包和 release 辅助脚本。
- `docs/`：设计文档和开发说明。

## Release 包说明

正式 release 会把本文件、`QUICKSTART.md`、`CHANGELOG.md` 和 `deploy/` 一起打进平台压缩包。用户如果已经下载并解压 release 包，可以直接运行：

macOS / Linux:

```bash
./codex-remote install -bootstrap-only -start-daemon
```

Windows PowerShell:

```powershell
.\codex-remote.exe install -bootstrap-only -start-daemon
```

## 相关文档

- [快速开始](./QUICKSTART.md)
- [版本变更](./CHANGELOG.md)
- [开发者说明](./DEVELOPER.md)
- [飞书应用配置说明](./deploy/feishu/README.md)
- [文档索引](./docs/README.md)
