package orchestrator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const (
	defaultGroupSummaryLimit = 80
	maxGroupSummaryLimit     = 200
)

type groupSummaryRequest struct {
	mode    string
	limit   int
	since   time.Time
	end     time.Time
	topic   string
	sync    bool
	status  bool
	enable  bool
	disable bool
}

type GroupSummaryHistoryReader interface {
	ListGroupSummaryMessages(ctx context.Context, gatewayID, surfaceSessionID, chatID string, start, end time.Time, limit int) ([]state.GroupSummaryMessageRecord, error)
}

func (s *Service) SetGroupSummaryHistoryReader(reader GroupSummaryHistoryReader) {
	if s == nil {
		return
	}
	s.groupSummaryHistoryReader = reader
}

func (s *Service) recordGroupSummaryText(surface *state.SurfaceConsoleRecord, action control.Action) {
	text := strings.TrimSpace(action.Text)
	if text == "" {
		return
	}
	s.recordGroupSummaryMessage(surface, action, state.SurfaceMessageKindText, text)
}

func (s *Service) recordGroupSummaryAttachment(surface *state.SurfaceConsoleRecord, action control.Action, kind state.SurfaceMessageKind, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.recordGroupSummaryMessage(surface, action, kind, text)
}

func (s *Service) recordGroupSummaryMessage(surface *state.SurfaceConsoleRecord, action control.Action, kind state.SurfaceMessageKind, text string) {
	if surface == nil || !surface.GroupSummary.Enabled {
		return
	}
	messageID := strings.TrimSpace(action.MessageID)
	if messageID == "" || strings.TrimSpace(surface.ChatID) == "" {
		return
	}
	for _, message := range surface.GroupSummary.Messages {
		if strings.TrimSpace(message.MessageID) == messageID {
			return
		}
	}
	now := s.now()
	createdAt := now
	if action.Inbound != nil && !action.Inbound.MessageCreateTime.IsZero() {
		createdAt = action.Inbound.MessageCreateTime
	}
	surface.GroupSummary.Messages = append(surface.GroupSummary.Messages, state.GroupSummaryMessageRecord{
		MessageID:   messageID,
		ActorUserID: strings.TrimSpace(action.ActorUserID),
		Text:        trimGroupSummaryText(text),
		MessageKind: kind,
		CreatedAt:   createdAt,
		RecordedAt:  now,
	})
	if overflow := len(surface.GroupSummary.Messages) - state.GroupSummaryMaxMessages; overflow > 0 {
		surface.GroupSummary.Messages = append([]state.GroupSummaryMessageRecord(nil), surface.GroupSummary.Messages[overflow:]...)
	}
}

func (s *Service) handleGroupSummaryCommand(surface *state.SurfaceConsoleRecord, action control.Action) []eventcontract.Event {
	req := parseGroupSummaryRequest(action.Text, s.now())
	switch {
	case req.enable:
		if !surface.GroupSummary.Enabled {
			surface.GroupSummary.Enabled = true
			surface.GroupSummary.EnabledAt = s.now()
		}
		return notice(surface, "group_summary_enabled", "已开启当前群的消息摘要记录。只会记录开启之后机器人收到的消息。")
	case req.disable:
		surface.GroupSummary = state.GroupSummaryRecord{}
		return notice(surface, "group_summary_disabled", "已关闭当前群的消息摘要记录，并清空已记录的摘要缓存。")
	case req.status:
		return notice(surface, "group_summary_status", groupSummaryStatusText(surface))
	}
	if req.sync {
		events := s.syncGroupSummaryMessages(surface, req)
		if len(events) > 0 {
			return events
		}
	}
	if !surface.GroupSummary.Enabled {
		return notice(surface, "group_summary_not_enabled", "当前群还没有开启消息摘要记录。请先发送 `/summary enable`。")
	}
	messages := selectGroupSummaryMessages(surface.GroupSummary.Messages, req)
	if len(messages) == 0 {
		return notice(surface, "group_summary_empty", "当前范围内还没有可摘要的群消息。第一期只记录 `/summary enable` 之后机器人收到的消息。")
	}
	prompt := buildGroupSummaryPrompt(messages, req)
	return s.handleText(surface, control.Action{
		Kind:             control.ActionTextMessage,
		GatewayID:        surface.GatewayID,
		SurfaceSessionID: surface.SurfaceSessionID,
		ChatID:           surface.ChatID,
		ActorUserID:      action.ActorUserID,
		MessageID:        action.MessageID,
		Text:             prompt,
		Inputs:           []agentproto.Input{{Type: agentproto.InputText, Text: prompt}},
	})
}

func (s *Service) syncGroupSummaryMessages(surface *state.SurfaceConsoleRecord, req groupSummaryRequest) []eventcontract.Event {
	if s.groupSummaryHistoryReader == nil {
		return notice(surface, "group_summary_sync_unavailable", "当前服务还没有接入飞书群历史读取能力，暂时不能执行 `/summary sync`。")
	}
	if strings.TrimSpace(surface.ChatID) == "" {
		return notice(surface, "group_summary_sync_missing_chat", "当前飞书会话缺少群 ID，不能同步群历史消息。")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	records, err := s.groupSummaryHistoryReader.ListGroupSummaryMessages(ctx, surface.GatewayID, surface.SurfaceSessionID, surface.ChatID, req.since, req.end, req.limit)
	if err != nil {
		return notice(surface, "group_summary_sync_failed", "同步群历史消息失败："+err.Error())
	}
	if !surface.GroupSummary.Enabled {
		surface.GroupSummary.Enabled = true
		surface.GroupSummary.EnabledAt = s.now()
	}
	s.mergeGroupSummaryMessages(surface, records)
	return nil
}

func (s *Service) mergeGroupSummaryMessages(surface *state.SurfaceConsoleRecord, records []state.GroupSummaryMessageRecord) {
	if surface == nil || len(records) == 0 {
		return
	}
	seen := make(map[string]bool, len(surface.GroupSummary.Messages)+len(records))
	for _, message := range surface.GroupSummary.Messages {
		if id := strings.TrimSpace(message.MessageID); id != "" {
			seen[id] = true
		}
	}
	now := s.now()
	for _, record := range records {
		record.MessageID = strings.TrimSpace(record.MessageID)
		record.Text = trimGroupSummaryText(record.Text)
		if record.MessageID == "" || strings.TrimSpace(record.Text) == "" || seen[record.MessageID] {
			continue
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		if record.RecordedAt.IsZero() {
			record.RecordedAt = now
		}
		if record.MessageKind == "" {
			record.MessageKind = state.SurfaceMessageKindText
		}
		surface.GroupSummary.Messages = append(surface.GroupSummary.Messages, record)
		seen[record.MessageID] = true
	}
	if overflow := len(surface.GroupSummary.Messages) - state.GroupSummaryMaxMessages; overflow > 0 {
		surface.GroupSummary.Messages = append([]state.GroupSummaryMessageRecord(nil), surface.GroupSummary.Messages[overflow:]...)
	}
}

func parseGroupSummaryRequest(text string, now time.Time) groupSummaryRequest {
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "/summary"))
	lower := strings.ToLower(arg)
	req := groupSummaryRequest{mode: "recent", limit: defaultGroupSummaryLimit}
	switch {
	case arg == "", lower == "status":
		req.status = true
	case lower == "enable", lower == "on":
		req.enable = true
	case lower == "disable", lower == "off":
		req.disable = true
	case lower == "today":
		y, m, d := now.Date()
		req.mode = "today"
		req.since = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case lower == "sync today":
		y, m, d := now.Date()
		req.sync = true
		req.mode = "today"
		req.limit = maxGroupSummaryLimit
		req.since = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		req.end = now
	case lower == "sync 24h":
		req.sync = true
		req.mode = "24h"
		req.limit = maxGroupSummaryLimit
		req.since = now.Add(-24 * time.Hour)
		req.end = now
	case strings.HasPrefix(lower, "sync "):
		req.sync = true
		req.mode = "recent"
		if limit, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(lower, "sync "))); err == nil && limit > 0 {
			req.limit = limit
		}
	case strings.HasSuffix(lower, "h"):
		if hours, err := strconv.Atoi(strings.TrimSuffix(lower, "h")); err == nil && hours > 0 {
			req.mode = lower
			req.since = now.Add(-time.Duration(hours) * time.Hour)
		}
	case strings.HasPrefix(lower, "topic "):
		req.mode = "topic"
		req.topic = strings.TrimSpace(arg[len("topic "):])
	case strings.HasPrefix(lower, "topic:"):
		req.mode = "topic"
		req.topic = strings.TrimSpace(arg[len("topic:"):])
	default:
		if limit, err := strconv.Atoi(lower); err == nil && limit > 0 {
			req.mode = "recent"
			req.limit = limit
		}
	}
	if req.limit <= 0 {
		req.limit = defaultGroupSummaryLimit
	}
	if req.limit > maxGroupSummaryLimit {
		req.limit = maxGroupSummaryLimit
	}
	return req
}

func groupSummaryStatusText(surface *state.SurfaceConsoleRecord) string {
	if surface == nil || !surface.GroupSummary.Enabled {
		return "当前群未开启消息摘要记录。发送 `/summary enable` 后开始记录。"
	}
	return fmt.Sprintf("当前群已开启消息摘要记录，已记录 %d 条消息。第一期只包含开启之后机器人收到的消息。", len(surface.GroupSummary.Messages))
}

func selectGroupSummaryMessages(messages []state.GroupSummaryMessageRecord, req groupSummaryRequest) []state.GroupSummaryMessageRecord {
	selected := make([]state.GroupSummaryMessageRecord, 0, len(messages))
	topic := strings.ToLower(strings.TrimSpace(req.topic))
	for _, message := range messages {
		if !req.since.IsZero() && message.CreatedAt.Before(req.since) {
			continue
		}
		if topic != "" && !strings.Contains(strings.ToLower(message.Text), topic) {
			continue
		}
		selected = append(selected, message)
	}
	if len(selected) > req.limit {
		selected = selected[len(selected)-req.limit:]
	}
	return selected
}

func buildGroupSummaryPrompt(messages []state.GroupSummaryMessageRecord, req groupSummaryRequest) string {
	var b strings.Builder
	b.WriteString("请基于下面这些飞书群消息做中文摘要和分析。\n")
	b.WriteString("要求：\n")
	b.WriteString("1. 先给出 3-6 条关键结论。\n")
	b.WriteString("2. 提取明确待办、负责人线索、风险/阻塞点。\n")
	b.WriteString("3. 如果信息不足，请直接说明不足，不要编造。\n")
	b.WriteString("4. 不要逐字复述所有原文。\n\n")
	b.WriteString(fmt.Sprintf("摘要范围：%s，共 %d 条消息。\n\n", groupSummaryModeLabel(req), len(messages)))
	b.WriteString("消息：\n")
	for i, message := range messages {
		created := message.CreatedAt.Format("2006-01-02 15:04")
		actor := firstNonEmptyGroupSummaryString(message.ActorUserID, "unknown")
		b.WriteString(fmt.Sprintf("%d. [%s] %s: %s\n", i+1, created, actor, message.Text))
	}
	return b.String()
}

func groupSummaryModeLabel(req groupSummaryRequest) string {
	if req.topic != "" {
		return "topic " + req.topic
	}
	if req.mode != "" {
		return req.mode
	}
	return "recent"
}

func trimGroupSummaryText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	const maxRunes = 1000
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func firstNonEmptyGroupSummaryString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
