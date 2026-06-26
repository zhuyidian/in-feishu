package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
)

const skillRunTimeout = 90 * time.Minute
const skillRunOutputLimit = 3800

func (a *App) handleSkillRunCommandLocked(command control.DaemonCommand) []eventcontract.Event {
	if strings.TrimSpace(command.SkillName) != "gkprep-build-apk" {
		return []eventcontract.Event{skillRunNotice(command.SurfaceSessionID, "skill_direct_run_unsupported", fmt.Sprintf("current direct run only supports `gkprep-build-apk`, got `%s`", command.SkillName))}
	}

	workspace := strings.TrimSpace(command.WorkspaceKey)
	if workspace == "" {
		return []eventcontract.Event{skillRunNotice(command.SurfaceSessionID, "skill_workspace_missing", "skill run is missing workspace path")}
	}

	script := filepath.Join(workspace, ".agents", "skills", "gkprep-build-apk", "scripts", "package_apk.ps1")
	if strings.TrimSpace(command.SkillPath) != "" {
		script = filepath.Join(filepath.Dir(command.SkillPath), "scripts", "package_apk.ps1")
	}
	if _, err := os.Stat(script); err != nil {
		return []eventcontract.Event{skillRunNotice(command.SurfaceSessionID, "skill_script_missing", fmt.Sprintf("skill script not found: %s", script))}
	}

	started := skillRunNotice(command.SurfaceSessionID, "skill_run_started", fmt.Sprintf("matched `gkprep-build-apk`, starting direct run: %s %s", firstNonEmpty(command.SkillPlatform, "y41air"), firstNonEmpty(command.SkillBuildType, "release")))
	go a.runGKPrepBuildAPKSkill(command, workspace, script)
	return []eventcontract.Event{started}
}

func (a *App) runGKPrepBuildAPKSkill(command control.DaemonCommand, workspace, script string) {
	ctx, cancel := context.WithTimeout(context.Background(), skillRunTimeout)
	defer cancel()

	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", script,
		"-Platform", firstNonEmpty(strings.TrimSpace(command.SkillPlatform), "y41air"),
		"-BuildType", firstNonEmpty(strings.TrimSpace(command.SkillBuildType), "release"),
	}
	if command.SkillSendToFeishu {
		args = append(args, "-SendToFeishu")
	}
	if strings.TrimSpace(command.SkillChatID) != "" {
		args = append(args, "-ChatId", strings.TrimSpace(command.SkillChatID))
	}
	if strings.TrimSpace(command.SkillFolderToken) != "" {
		args = append(args, "-FolderToken", strings.TrimSpace(command.SkillFolderToken))
	}

	cmd := exec.CommandContext(ctx, "powershell", args...)
	cmd.Dir = workspace
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	statusText := buildSkillRunResultText(err, ctx.Err(), text)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shuttingDown {
		return
	}
	a.handleUIEventsLocked(context.Background(), []eventcontract.Event{
		skillRunNotice(command.SurfaceSessionID, skillRunResultCode(err, ctx.Err()), statusText),
	})
}

func buildSkillRunResultText(runErr error, ctxErr error, output string) string {
	output = trimSkillRunOutput(output)
	switch {
	case ctxErr != nil:
		return firstNonEmpty(output, "skill run timed out")
	case runErr != nil:
		return firstNonEmpty(output, runErr.Error())
	default:
		return "skill run completed"
	}
}

func skillRunResultCode(runErr error, ctxErr error) string {
	if ctxErr != nil {
		return "skill_run_timeout"
	}
	if runErr != nil {
		return "skill_run_failed"
	}
	return "skill_run_completed"
}

func trimSkillRunOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= skillRunOutputLimit {
		return output
	}
	return output[len(output)-skillRunOutputLimit:]
}

func skillRunNotice(surfaceID, code, text string) eventcontract.Event {
	return eventcontract.Event{
		Kind:             eventcontract.KindNotice,
		SurfaceSessionID: surfaceID,
		Notice: &control.Notice{
			Code:  code,
			Title: "skill run",
			Text:  strings.TrimSpace(text),
		},
	}
}
