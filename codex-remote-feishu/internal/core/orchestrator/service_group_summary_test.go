package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func TestGroupSummaryRecordsOnlyAfterEnable(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)

	svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "msg-before",
		Text:             "开启前不应该记录",
	})
	surface := svc.root.Surfaces["surface-1"]
	if got := len(surface.GroupSummary.Messages); got != 0 {
		t.Fatalf("expected disabled summary to record nothing, got %d", got)
	}

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionGroupSummaryCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "cmd-enable",
		Text:             "/summary enable",
	})
	if !surface.GroupSummary.Enabled || len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "group_summary_enabled" {
		t.Fatalf("expected enable notice and enabled state, events=%#v state=%#v", events, surface.GroupSummary)
	}

	svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionTextMessage,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-2",
		MessageID:        "msg-after",
		Text:             "发布计划明天确认",
		Inbound:          &control.ActionInboundMeta{MessageCreateTime: now.Add(time.Minute)},
	})
	if got := len(surface.GroupSummary.Messages); got != 1 {
		t.Fatalf("expected one recorded message, got %d", got)
	}
	if got := surface.GroupSummary.Messages[0].Text; got != "发布计划明天确认" {
		t.Fatalf("recorded text = %q", got)
	}
}

func TestGroupSummaryCommandDispatchesPromptFromRecordedMessages(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	setupGroupSummaryAttachedSurface(t, svc)

	svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionGroupSummaryCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "cmd-enable",
		Text:             "/summary enable",
	})
	surface := svc.root.Surfaces["surface-1"]
	svc.recordGroupSummaryText(surface, control.Action{
		ActorUserID: "user-2",
		MessageID:   "msg-1",
		Text:        "今天先完成接口联调",
		Inbound:     &control.ActionInboundMeta{MessageCreateTime: now.Add(time.Minute)},
	})
	svc.recordGroupSummaryText(surface, control.Action{
		ActorUserID: "user-3",
		MessageID:   "msg-2",
		Text:        "发布计划需要等权限审批",
		Inbound:     &control.ActionInboundMeta{MessageCreateTime: now.Add(2 * time.Minute)},
	})

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionGroupSummaryCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "cmd-summary",
		Text:             "/summary topic 发布计划",
	})
	command := firstPromptCommand(events)
	if command == nil {
		t.Fatalf("expected summary to dispatch a prompt command, got %#v", events)
	}
	if command.Origin.MessageID != "cmd-summary" {
		t.Fatalf("origin message = %q", command.Origin.MessageID)
	}
	if len(command.Prompt.Inputs) != 1 {
		t.Fatalf("expected one prompt input, got %#v", command.Prompt.Inputs)
	}
	prompt := command.Prompt.Inputs[0].Text
	if !strings.Contains(prompt, "请基于下面这些飞书群消息做中文摘要和分析") ||
		!strings.Contains(prompt, "发布计划需要等权限审批") ||
		strings.Contains(prompt, "今天先完成接口联调") {
		t.Fatalf("unexpected prompt:\n%s", prompt)
	}
}

func TestGroupSummaryDisableClearsRecordedMessages(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	setupGroupSummaryAttachedSurface(t, svc)

	svc.ApplySurfaceAction(control.Action{Kind: control.ActionGroupSummaryCommand, SurfaceSessionID: "surface-1", ChatID: "chat-1", ActorUserID: "user-1", MessageID: "cmd-enable", Text: "/summary enable"})
	svc.ApplySurfaceAction(control.Action{Kind: control.ActionTextMessage, SurfaceSessionID: "surface-1", ChatID: "chat-1", ActorUserID: "user-2", MessageID: "msg-1", Text: "需要被清理"})

	events := svc.ApplySurfaceAction(control.Action{Kind: control.ActionGroupSummaryCommand, SurfaceSessionID: "surface-1", ChatID: "chat-1", ActorUserID: "user-1", MessageID: "cmd-disable", Text: "/summary disable"})
	surface := svc.root.Surfaces["surface-1"]
	if surface.GroupSummary.Enabled || len(surface.GroupSummary.Messages) != 0 {
		t.Fatalf("expected disabled empty summary state, got %#v", surface.GroupSummary)
	}
	if len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "group_summary_disabled" {
		t.Fatalf("expected disable notice, got %#v", events)
	}
}

func TestGroupSummarySyncFetchesHistoryAndDispatchesPrompt(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	setupGroupSummaryAttachedSurface(t, svc)
	reader := &fakeGroupSummaryHistoryReader{
		records: []state.GroupSummaryMessageRecord{
			{MessageID: "msg-old", ActorUserID: "user-2", Text: "already cached", CreatedAt: now.Add(-2 * time.Hour)},
			{MessageID: "msg-new", ActorUserID: "user-3", Text: "release plan needs owner", CreatedAt: now.Add(-time.Hour)},
		},
	}
	svc.SetGroupSummaryHistoryReader(reader)
	surface := svc.root.Surfaces["surface-1"]
	surface.GroupSummary.Messages = []state.GroupSummaryMessageRecord{
		{MessageID: "msg-old", ActorUserID: "user-2", Text: "already cached", CreatedAt: now.Add(-2 * time.Hour), RecordedAt: now},
	}

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionGroupSummaryCommand,
		GatewayID:        "gateway-1",
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		MessageID:        "cmd-sync",
		Text:             "/summary sync 200",
	})

	if !surface.GroupSummary.Enabled {
		t.Fatal("expected sync to enable group summary cache")
	}
	if got := len(surface.GroupSummary.Messages); got != 2 {
		t.Fatalf("expected deduplicated cached messages, got %d: %#v", got, surface.GroupSummary.Messages)
	}
	if reader.limit != 200 || reader.start != (time.Time{}) || reader.end != (time.Time{}) {
		t.Fatalf("unexpected sync request: limit=%d start=%v end=%v", reader.limit, reader.start, reader.end)
	}
	command := firstPromptCommand(events)
	if command == nil {
		t.Fatalf("expected sync summary to dispatch a prompt command, got %#v", events)
	}
	prompt := command.Prompt.Inputs[0].Text
	if !strings.Contains(prompt, "release plan needs owner") || strings.Count(prompt, "already cached") != 1 {
		t.Fatalf("unexpected prompt:\n%s", prompt)
	}
}

func TestParseGroupSummarySyncRequests(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)

	today := parseGroupSummaryRequest("/summary sync today", now)
	if !today.sync || today.mode != "today" || today.limit != maxGroupSummaryLimit || !today.since.Equal(time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)) || !today.end.Equal(now) {
		t.Fatalf("unexpected today sync request: %#v", today)
	}

	last24h := parseGroupSummaryRequest("/summary sync 24h", now)
	if !last24h.sync || last24h.mode != "24h" || last24h.limit != maxGroupSummaryLimit || !last24h.since.Equal(now.Add(-24*time.Hour)) || !last24h.end.Equal(now) {
		t.Fatalf("unexpected 24h sync request: %#v", last24h)
	}

	recent := parseGroupSummaryRequest("/summary sync 200", now)
	if !recent.sync || recent.mode != "recent" || recent.limit != 200 || !recent.since.IsZero() || !recent.end.IsZero() {
		t.Fatalf("unexpected recent sync request: %#v", recent)
	}
}

func setupGroupSummaryAttachedSurface(t *testing.T, svc *Service) {
	t.Helper()
	svc.UpsertInstance(&state.InstanceRecord{
		InstanceID:    "inst-1",
		DisplayName:   "repo",
		WorkspaceRoot: "/data/repo",
		WorkspaceKey:  "/data/repo",
		Online:        true,
		Threads: map[string]*state.ThreadRecord{
			"thread-1": {ThreadID: "thread-1", Name: "主会话", CWD: "/data/repo", Loaded: true},
		},
	})
	surface := svc.ensureSurface(control.Action{SurfaceSessionID: "surface-1", GatewayID: "gateway-1", ChatID: "chat-1", ActorUserID: "user-1"})
	surface.AttachedInstanceID = "inst-1"
	surface.SelectedThreadID = "thread-1"
	surface.RouteMode = state.RouteModePinned
	if !svc.claimKnownThread(surface, svc.root.Instances["inst-1"], "thread-1") {
		t.Fatal("failed to claim test thread")
	}
}

type fakeGroupSummaryHistoryReader struct {
	records          []state.GroupSummaryMessageRecord
	err              error
	gatewayID        string
	surfaceSessionID string
	chatID           string
	start            time.Time
	end              time.Time
	limit            int
}

func (f *fakeGroupSummaryHistoryReader) ListGroupSummaryMessages(_ context.Context, gatewayID, surfaceSessionID, chatID string, start, end time.Time, limit int) ([]state.GroupSummaryMessageRecord, error) {
	f.gatewayID = gatewayID
	f.surfaceSessionID = surfaceSessionID
	f.chatID = chatID
	f.start = start
	f.end = end
	f.limit = limit
	if f.err != nil {
		return nil, f.err
	}
	return append([]state.GroupSummaryMessageRecord(nil), f.records...), nil
}

func firstPromptCommand(events []eventcontract.Event) *agentproto.Command {
	for _, event := range events {
		if event.Command != nil && event.Command.Kind == agentproto.CommandPromptSend {
			return event.Command
		}
	}
	return nil
}
