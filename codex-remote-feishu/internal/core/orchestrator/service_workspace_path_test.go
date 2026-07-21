package orchestrator

import (
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestThreadsSnapshotNormalizesWindowsExtendedCWD(t *testing.T) {
	now := time.Date(2026, 7, 21, 14, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-headless-1",
		WorkspaceRoot: "E:/project/study/V7.0-Study-HeiBan",
		WorkspaceKey:  "E:/project/study/V7.0-Study-HeiBan",
		Online:        true,
	})

	svc.ApplyAgentEvent("inst-headless-1", agentproto.Event{
		Kind: agentproto.EventThreadsSnapshot,
		Threads: []agentproto.ThreadSnapshotRecord{{
			ThreadID: "thread-1",
			CWD:      `//?/E:\project\study\V7.0-Study-HeiBan`,
			Loaded:   true,
		}},
	})

	thread := svc.root.Instances["inst-headless-1"].Threads["thread-1"]
	if thread == nil {
		t.Fatal("expected thread from snapshot")
	}
	if got, want := thread.CWD, "E:/project/study/V7.0-Study-HeiBan"; got != want {
		t.Fatalf("thread CWD = %q, want %q", got, want)
	}
}
