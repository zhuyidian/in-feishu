# AGENTS

This repository is a wrapper repository. The core project lives in:

```text
codex-remote-feishu/
```

For work on the core project code, docs, tests, release scripts, Feishu behavior, daemon behavior, or repository maintenance workflows:

1. Treat `codex-remote-feishu/` as the project root.
2. Read and follow `codex-remote-feishu/AGENTS.md`.
3. Resolve repo skills from `codex-remote-feishu/.codex/skills/`.
4. Run project-local commands from `codex-remote-feishu/`, unless the command explicitly targets this wrapper root.

Examples:

```powershell
Set-Location codex-remote-feishu
```

```bash
cd codex-remote-feishu
```

Root-level files such as `README.md`, `install-release.sh`, `install-release.ps1`, and `.github/workflows/` are wrapper/release-entry files for `zhuyidian/in-feishu`. When editing those files, stay at the wrapper root as needed, but still consult the core project guidance if the change affects packaged behavior or release validation.

Do not duplicate the full core project rules here. Keep `codex-remote-feishu/AGENTS.md` as the source of truth for core project workflow and skill routing.
