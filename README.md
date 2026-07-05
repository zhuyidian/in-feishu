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

## Skill 接入

in-feishu 会在当前被接管的工作区里读取项目级 skill：

```text
<workspace>/.agents/skills/<skill-name>/SKILL.md
```

也就是说，skill 应该放在你通过 `/list` 选择的目标项目里，而不是放在本仓库根目录。

当前已接入直连执行通道的 skill 是 `gkprep-build-apk`。如果某个项目需要从飞书里直接触发 APK 构建，请在该项目根目录放置：

```text
<workspace>/.agents/skills/gkprep-build-apk/SKILL.md
<workspace>/.agents/skills/gkprep-build-apk/scripts/package_apk.ps1
```

例如：

```text
E:\project\study\V5.0-Study-GKPrep-Plus\.agents\skills\gkprep-build-apk\SKILL.md
E:\project\study\V5.0-Study-GKPrep-Plus\.agents\skills\gkprep-build-apk\scripts\package_apk.ps1
```

本仓库里的 `codex-remote-feishu/.codex/skills/` 是维护 in-feishu 源码时给 Codex 使用的 repo skill，不会作为飞书运行时的用户项目 skill 自动接管。

## 版本说明

每个正式版本都会通过 GitHub tag / Release 记录对应修改。下面是当前主要版本的功能变化摘要：

| 版本 | 主要变化 |
| --- | --- |
| [v1.7.4](https://github.com/zhuyidian/in-feishu/releases/tag/v1.7.4) | 本仓库开始提供可直接安装的 GitHub Release；修复根目录 release workflow、测试阻塞和群聊回复上下文处理。 |
| [v1.7.3](https://github.com/zhuyidian/in-feishu/releases/tag/v1.7.3) | 支持引用飞书文件消息后交给 Codex 处理；增强 APK skill 在 Windows 下的直接执行稳定性。 |
| [v1.7.2](https://github.com/zhuyidian/in-feishu/releases/tag/v1.7.2) | 工作区可暴露本地 `.agents/skills`，并支持项目内 APK 构建 skill 直接运行。 |
| [v1.7.1](https://github.com/zhuyidian/in-feishu/releases/tag/v1.7.1) | 群聊消息改为仅在明确提及当前机器人时响应，减少误触发。 |
| [v1.7.0](https://github.com/zhuyidian/in-feishu/releases/tag/v1.7.0) | 增加飞书群历史摘要同步，支持 `/summary sync today`、`24h`、`200` 等用法。 |
| v1.6.0 | 优化飞书中继续当前执行、工作区选择、过程卡、定时任务、升级和多实例部署体验。 |
| v1.5.0 | 明确默认使用路径，完善 WebSetup、会话恢复、多工作区切换和用户升级入口。 |
| v1.4.0 | 补齐 release track、安装引导、飞书菜单、旧卡片失效提示、恢复和升级回滚基础。 |

更完整的逐版本说明见 [CHANGELOG.md](./codex-remote-feishu/CHANGELOG.md)。

## 发布版本

正式版本通过 GitHub Actions 的 `Release` workflow 构建并发布到本仓库 Releases：

https://github.com/zhuyidian/in-feishu/releases

Release 包中包含各平台运行所需的 `codex-remote` / `codex-remote.exe`、校验文件和安装脚本。日常使用推荐通过上面的在线安装命令接入。

## 开发

源码位于 [`codex-remote-feishu/`](./codex-remote-feishu)。需要本地开发或联调时再进入该目录查看项目文档和构建脚本。
