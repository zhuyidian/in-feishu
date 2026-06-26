package daemon

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestNormalizeSkillRunWindowsPathRemovesExtendedPrefix(t *testing.T) {
	got := normalizeSkillRunWindowsPath(`//?/E:/project/study/V5.0-Study-GKPrep-Plus/.agents/skills/gkprep-build-apk/SKILL.md`)
	want := `E:\project\study\V5.0-Study-GKPrep-Plus\.agents\skills\gkprep-build-apk\SKILL.md`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestSkillRunPowerShellScriptUsesLiteralPathsAndUTF8(t *testing.T) {
	script := skillRunPowerShellScript(control.DaemonCommand{
		SkillPlatform:     "y41air",
		SkillBuildType:    "release",
		SkillSendToFeishu: true,
		SkillChatID:       "oc_1",
	}, `E:\project\study\V5.0-Study-GKPrep-Plus`, `E:\project\study\V5.0-Study-GKPrep-Plus\.agents\skills\gkprep-build-apk\scripts\package_apk.ps1`)
	if !strings.Contains(script, "[Console]::OutputEncoding = $utf8NoBom") {
		t.Fatalf("expected UTF-8 console setup, got:\n%s", script)
	}
	if !strings.Contains(script, "Set-Location -LiteralPath 'E:\\project\\study\\V5.0-Study-GKPrep-Plus'") {
		t.Fatalf("expected literal workspace location, got:\n%s", script)
	}
	if !strings.Contains(script, "& 'E:\\project\\study\\V5.0-Study-GKPrep-Plus\\.agents\\skills\\gkprep-build-apk\\scripts\\package_apk.ps1'") {
		t.Fatalf("expected literal script invocation, got:\n%s", script)
	}
	if !strings.Contains(script, "-SendToFeishu") || !strings.Contains(script, "-ChatId 'oc_1'") {
		t.Fatalf("expected Feishu send arguments, got:\n%s", script)
	}
}

func TestEncodePowerShellCommandUsesUTF16LEBase64(t *testing.T) {
	command := "Write-Host '\u4e2d\u6587'"
	encoded, err := base64.StdEncoding.DecodeString(encodePowerShellCommand(command))
	if err != nil {
		t.Fatalf("decode command: %v", err)
	}
	if len(encoded)%2 != 0 {
		t.Fatalf("expected UTF-16LE byte pairs, got %d bytes", len(encoded))
	}
	words := make([]uint16, 0, len(encoded)/2)
	for i := 0; i < len(encoded); i += 2 {
		words = append(words, uint16(encoded[i])|uint16(encoded[i+1])<<8)
	}
	if got := string(utf16.Decode(words)); got != command {
		t.Fatalf("decoded command = %q", got)
	}
}
