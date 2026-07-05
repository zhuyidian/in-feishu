# in-feishu

把本机或服务器上的 Codex 接到飞书里使用。安装后可以在飞书中接管工作区、继续会话、新建任务、发送图片/文件，并查看 Codex 执行进度。

本仓库已经通过 GitHub Releases 提供预构建版本，普通使用者不需要 clone 仓库，也不需要本地编译。

## 快速安装

安装前请先确认目标机器上已经可以正常运行 `codex`。

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/zhuyidian/in-feishu/main/install-release.sh | bash
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/zhuyidian/in-feishu/main/install-release.ps1 | iex
```

安装脚本会自动下载本仓库最新的 production release，安装 `codex-remote`，启动本地 daemon，并打印 WebSetup 地址。

如果要安装指定版本，例如 `v1.7.4`：

```bash
curl -fsSL https://raw.githubusercontent.com/zhuyidian/in-feishu/main/install-release.sh | bash -s -- --version v1.7.4
```

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/zhuyidian/in-feishu/main/install-release.ps1))) -Version v1.7.4
```

## 接入飞书

安装完成后，打开命令行输出中的 `/setup` 或 WebSetup 链接，按向导完成飞书接入、权限配置和用户绑定。

飞书应用配置不需要在安装前手动准备；WebSetup 会引导完成必要步骤。如需提前了解飞书机器人菜单、事件订阅和权限配置，可参考：

- [飞书配置模板](https://github.com/zhuyidian/in-feishu/blob/main/codex-remote-feishu/deploy/feishu/app-template.json)
- [飞书配置说明](https://github.com/zhuyidian/in-feishu/blob/main/codex-remote-feishu/deploy/feishu/README.md)

## 飞书里怎么用

完成绑定后，可以先在飞书里发送：

```text
/menu
```

常用命令：

- `/list`：选择或添加工作区
- `/new`：在当前工作区新建会话
- `/use`：继续最近会话
- `/status`：查看当前连接和队列状态
- `/stop`：停止当前执行
- `/compact`：整理当前会话上下文
- `/sendfile`：从当前工作区发送文件回飞书
- `/upgrade latest`：升级到当前 release track 的最新版本

私聊机器人可以直接发送消息。群聊中建议先 `@机器人`；如果是在机器人已有回复下面继续上下文，也可以直接回复对应消息。

## 可选能力

- 图片输入：先发送图片，再发送文字说明，系统会把图片一起交给 Codex。
- 本地文档预览：可把最终回复中的本地 `.md` 或单文件 `.html` 链接转换为飞书云空间预览链接。
- `/cron` 定时任务：可为当前 daemon 实例配置定时任务，需要在 WebSetup 中开启相关权限。
- VS Code 跟随：默认不需要接入 VS Code；只有需要跟随编辑器当前焦点时，再在 WebSetup / Admin UI 中按需开启。

## 发布版本

正式版本通过 GitHub Actions 的 `Release` workflow 构建并发布到本仓库 Releases：

https://github.com/zhuyidian/in-feishu/releases

Release 包中包含各平台运行所需的 `codex-remote` / `codex-remote.exe`、校验文件和安装脚本。日常使用推荐通过上面的在线安装命令接入。

## 开发

源码位于 [`codex-remote-feishu/`](./codex-remote-feishu)。需要本地开发或联调时再进入该目录查看项目文档和构建脚本。
