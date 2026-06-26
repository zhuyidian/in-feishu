package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestWorkspaceSkillMatchDispatchesSkillRunCommand(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	workspace := t.TempDir()
	skillDir := filepath.Join(workspace, ".agents", "skills", "gkprep-build-apk")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillBody := "---\nname: gkprep-build-apk\ndescription: Build GKPrep Android APKs and optionally upload the generated APK to Feishu.\n---\n# Build APK\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillBody), 0o644); err != nil {
		t.Fatal(err)
	}

	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "GKPrep",
		WorkspaceRoot: workspace,
		WorkspaceKey:  workspace,
		Online:        true,
		Threads:       map[string]*state.ThreadRecord{},
	})
	svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionAttachInstance,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		InstanceID:       "inst-1",
	})
	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		MessageID:        "msg-1",
		ActorUserID:      "user-1",
		Text:             "请帮我出一个 y41air release 的 APK，并发送到飞书群。",
	})

	found := false
	for _, event := range events {
		if event.DaemonCommand != nil && event.DaemonCommand.Kind == control.DaemonCommandSkillRun {
			found = true
			if event.DaemonCommand.SkillName != "gkprep-build-apk" {
				t.Fatalf("unexpected skill name: %#v", event.DaemonCommand)
			}
		}
	}
	if !found {
		t.Fatalf("expected daemon.skill.run event, got %#v", events)
	}
}
