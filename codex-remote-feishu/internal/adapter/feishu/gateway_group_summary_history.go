package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	gatewaypkg "github.com/kxn/codex-remote-feishu/internal/adapter/feishu/gateway"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const (
	groupSummaryHistoryDefaultLimit = 80
	groupSummaryHistoryMaxLimit     = 200
	groupSummaryHistoryPageSize     = 50
)

func (g *LiveGateway) ListGroupSummaryMessages(ctx context.Context, _, _, chatID string, start, end time.Time, limit int) ([]state.GroupSummaryMessageRecord, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, fmt.Errorf("missing chat id")
	}
	if limit <= 0 {
		limit = groupSummaryHistoryDefaultLimit
	}
	if limit > groupSummaryHistoryMaxLimit {
		limit = groupSummaryHistoryMaxLimit
	}

	sortType := larkim.SortTypeListMessageByCreateTimeAsc
	reverseResult := false
	if start.IsZero() && end.IsZero() {
		sortType = larkim.SortTypeListMessageByCreateTimeDesc
		reverseResult = true
	}

	records := make([]state.GroupSummaryMessageRecord, 0, limit)
	pageToken := ""
	for len(records) < limit {
		pageSize := groupSummaryHistoryPageSize
		if remaining := limit - len(records); remaining < pageSize {
			pageSize = remaining
		}
		resp, err := g.listGroupSummaryMessagePage(ctx, chatID, start, end, sortType, pageSize, pageToken)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.Data == nil {
			break
		}
		for _, item := range resp.Data.Items {
			record, ok := groupSummaryRecordFromLarkMessage(item)
			if !ok {
				continue
			}
			records = append(records, record)
			if len(records) >= limit {
				break
			}
		}
		if resp.Data.HasMore == nil || !*resp.Data.HasMore || strings.TrimSpace(stringPtr(resp.Data.PageToken)) == "" {
			break
		}
		pageToken = strings.TrimSpace(stringPtr(resp.Data.PageToken))
	}
	if reverseResult {
		for left, right := 0, len(records)-1; left < right; left, right = left+1, right-1 {
			records[left], records[right] = records[right], records[left]
		}
	}
	return records, nil
}

func (g *LiveGateway) listGroupSummaryMessagePage(ctx context.Context, chatID string, start, end time.Time, sortType string, pageSize int, pageToken string) (*larkim.ListMessageResp, error) {
	return DoSDK(ctx, g.broker, CallSpec{
		GatewayID: g.config.GatewayID,
		API:       "im.v1.message.list",
		Class:     CallClassIMRead,
		Priority:  CallPriorityReadAssist,
		ResourceKey: FeishuResourceKey{
			ReceiveTarget: joinReceiveTarget("chat", chatID),
		},
		Retry:      RetryRateLimitOnly,
		Permission: PermissionCooldownOnly,
	}, func(callCtx context.Context, client *lark.Client) (*larkim.ListMessageResp, error) {
		builder := larkim.NewListMessageReqBuilder().
			ContainerIdType("chat").
			ContainerId(chatID).
			SortType(sortType).
			PageSize(pageSize)
		if !start.IsZero() {
			builder.StartTime(strconv.FormatInt(start.Unix(), 10))
		}
		if !end.IsZero() {
			builder.EndTime(strconv.FormatInt(end.Unix(), 10))
		}
		if strings.TrimSpace(pageToken) != "" {
			builder.PageToken(strings.TrimSpace(pageToken))
		}
		resp, err := client.Im.V1.Message.List(callCtx, builder.Build())
		if err != nil {
			return resp, err
		}
		if !resp.Success() {
			return resp, newAPIError("im.v1.message.list", resp.ApiResp, resp.CodeError)
		}
		return resp, nil
	})
}

func groupSummaryRecordFromLarkMessage(message *larkim.Message) (state.GroupSummaryMessageRecord, bool) {
	if message == nil || boolPtr(message.Deleted) {
		return state.GroupSummaryMessageRecord{}, false
	}
	messageID := strings.TrimSpace(stringPtr(message.MessageId))
	msgType := strings.ToLower(strings.TrimSpace(stringPtr(message.MsgType)))
	content := ""
	if message.Body != nil {
		content = strings.TrimSpace(stringPtr(message.Body.Content))
	}
	text, kind, ok := groupSummaryTextFromMessageContent(msgType, content)
	if !ok || strings.TrimSpace(text) == "" || messageID == "" {
		return state.GroupSummaryMessageRecord{}, false
	}
	createdAt := groupSummaryMessageCreatedAt(stringPtr(message.CreateTime))
	actorID := ""
	if message.Sender != nil {
		actorID = strings.TrimSpace(stringPtr(message.Sender.Id))
	}
	return state.GroupSummaryMessageRecord{
		MessageID:   messageID,
		ActorUserID: actorID,
		Text:        text,
		MessageKind: kind,
		CreatedAt:   createdAt,
		RecordedAt:  time.Now().UTC(),
	}, true
}

func groupSummaryTextFromMessageContent(msgType, content string) (string, state.SurfaceMessageKind, bool) {
	switch msgType {
	case "text":
		text, err := gatewaypkg.ParseTextContent(content)
		return strings.TrimSpace(text), state.SurfaceMessageKindText, err == nil
	case "post":
		text, err := groupSummaryPostText(content)
		return text, state.SurfaceMessageKindText, err == nil
	case "image":
		return "[图片]", state.SurfaceMessageKindImage, true
	case "file":
		name := strings.TrimSpace(gatewaypkg.ParseFileName(content))
		if name == "" {
			return "[文件]", state.SurfaceMessageKindCard, true
		}
		return "[文件] " + name, state.SurfaceMessageKindCard, true
	default:
		return "", state.SurfaceMessageKindText, false
	}
}

func groupSummaryPostText(rawContent string) (string, error) {
	var localized feishuLocalizedPostContent
	if err := json.Unmarshal([]byte(rawContent), &localized); err == nil && (localized.ZhCN.Title != "" || len(localized.ZhCN.Content) > 0) {
		return groupSummaryPostContentText(localized.ZhCN), nil
	}
	var content feishuPostContent
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		return "", err
	}
	return groupSummaryPostContentText(content), nil
}

func groupSummaryPostContentText(content feishuPostContent) string {
	parts := make([]string, 0, len(content.Content)+1)
	if title := strings.TrimSpace(content.Title); title != "" {
		parts = append(parts, title)
	}
	for _, paragraph := range content.Content {
		var segment strings.Builder
		for _, node := range paragraph {
			switch strings.ToLower(strings.TrimSpace(node.Tag)) {
			case "text":
				segment.WriteString(node.Text)
			case "a":
				segment.WriteString(firstNonEmpty(node.Text, node.Href))
			case "at":
				segment.WriteString(firstNonEmpty(node.Text, node.UserName, node.UserID))
			case "emotion":
				if emoji := strings.TrimSpace(node.EmojiType); emoji != "" {
					segment.WriteString(":" + emoji + ":")
				}
			case "code_block":
				if code := strings.TrimSpace(node.Text); code != "" {
					if segment.Len() > 0 {
						segment.WriteString("\n")
					}
					segment.WriteString(code)
				}
			case "img", "media":
				segment.WriteString("[图片]")
			}
		}
		if text := strings.TrimSpace(segment.String()); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func groupSummaryMessageCreatedAt(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(value, 10, 64)
	if err != nil || ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
