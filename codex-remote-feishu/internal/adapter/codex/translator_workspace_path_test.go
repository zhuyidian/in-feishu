package codex

import (
	"encoding/json"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestTranslatorNormalizesWindowsExtendedCWDForResumeAndRestart(t *testing.T) {
	tr := NewTranslator("inst-1")
	rawCWD := `//?/E:\project\study\V7.0-Study-HeiBan`
	wantCWD := "E:/project/study/V7.0-Study-HeiBan"

	result, err := tr.ObserveClient([]byte(`{"method":"thread/resume","params":{"threadId":"thread-current","cwd":"//?/E:\\project\\study\\V7.0-Study-HeiBan"}}`))
	if err != nil {
		t.Fatalf("observe thread/resume: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].CWD != wantCWD {
		t.Fatalf("expected normalized observed cwd, got %#v", result.Events)
	}
	if got := tr.knownThreadCWD["thread-current"]; got != wantCWD {
		t.Fatalf("known cwd = %q, want %q", got, wantCWD)
	}

	commands, err := tr.TranslateCommand(agentproto.Command{
		Kind:   agentproto.CommandPromptSend,
		Origin: agentproto.Origin{Surface: "surface-1"},
		Target: agentproto.Target{ThreadID: "thread-target", CWD: rawCWD},
	})
	if err != nil {
		t.Fatalf("translate prompt: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected one resume command, got %#v", commands)
	}
	var resume map[string]any
	if err := json.Unmarshal(commands[0], &resume); err != nil {
		t.Fatalf("unmarshal resume: %v", err)
	}
	params, _ := resume["params"].(map[string]any)
	if got := params["cwd"]; got != wantCWD {
		t.Fatalf("resume cwd = %#v, want %q", got, wantCWD)
	}

	frame, _, ok, err := tr.BuildChildRestartRestoreFrame("cmd-restart-1")
	if err != nil {
		t.Fatalf("build restart restore frame: %v", err)
	}
	if !ok {
		t.Fatal("expected restart restore frame")
	}
	var restore map[string]any
	if err := json.Unmarshal(frame, &restore); err != nil {
		t.Fatalf("unmarshal restart restore frame: %v", err)
	}
	restoreParams, _ := restore["params"].(map[string]any)
	if got := restoreParams["cwd"]; got != wantCWD {
		t.Fatalf("restart restore cwd = %#v, want %q", got, wantCWD)
	}
}
