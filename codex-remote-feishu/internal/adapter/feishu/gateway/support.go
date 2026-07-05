package gateway

import (
	"context"
	"sort"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const (
	PlatformFeishu             = "feishu"
	ScopeKindUser              = "user"
	ScopeKindChat              = "chat"
	inboundMessageParseTimeout = 30 * time.Second
)

type SurfaceRef struct {
	Platform  string
	GatewayID string
	ScopeKind string
	ScopeID   string
}

type feishuTextContent struct {
	Text string `json:"text"`
}

type feishuPostContent struct {
	Title   string             `json:"title"`
	Content [][]feishuPostNode `json:"content"`
}

type feishuLocalizedPostContent struct {
	ZhCN feishuPostContent `json:"zh_cn"`
}

type feishuPostNode struct {
	Tag       string `json:"tag"`
	Text      string `json:"text"`
	Href      string `json:"href"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	ImageKey  string `json:"image_key"`
	EmojiType string `json:"emoji_type"`
	Language  string `json:"language"`
}

func (r SurfaceRef) SurfaceID() string {
	if !r.valid() {
		return ""
	}
	return strings.Join([]string{
		PlatformFeishu,
		normalizeGatewayID(r.GatewayID),
		r.ScopeKind,
		r.ScopeID,
	}, ":")
}

func (r SurfaceRef) valid() bool {
	if strings.TrimSpace(r.Platform) != PlatformFeishu {
		return false
	}
	if strings.TrimSpace(r.GatewayID) == "" {
		return false
	}
	if strings.TrimSpace(r.ScopeID) == "" {
		return false
	}
	switch strings.TrimSpace(r.ScopeKind) {
	case ScopeKindUser, ScopeKindChat:
		return true
	default:
		return false
	}
}

func normalizeGatewayID(gatewayID string) string {
	return strings.TrimSpace(gatewayID)
}

func newFeishuTimeoutContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	base := context.Background()
	if parent != nil {
		base = parent
	}
	if timeout <= 0 {
		return context.WithCancel(base)
	}
	return context.WithTimeout(base, timeout)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func referencedMessageID(message *larkim.EventMessage) string {
	if message == nil {
		return ""
	}
	targetMessageID := strings.TrimSpace(stringPtr(message.ParentId))
	if targetMessageID == "" {
		targetMessageID = strings.TrimSpace(stringPtr(message.RootId))
	}
	return targetMessageID
}

func parseFeishuEventText(rawContent string, mentions []*larkim.MentionEvent) (displayText string, commandText string, err error) {
	rawText, err := ParseTextContent(rawContent)
	if err != nil {
		return "", "", err
	}
	return normalizeFeishuTextMentions(rawText, mentions), normalizeFeishuCommandCandidate(rawText, mentions), nil
}

func normalizeFeishuTextMentions(rawText string, mentions []*larkim.MentionEvent) string {
	replacements := feishuMentionReplacements(mentions)
	if len(replacements) == 0 {
		return rawText
	}
	pairs := make([]string, 0, len(replacements)*2)
	for _, item := range replacements {
		pairs = append(pairs, item.key, item.label)
	}
	return strings.NewReplacer(pairs...).Replace(rawText)
}

func shouldHandleInboundGroupMessage(chatType, messageType, rawContent string, mentions []*larkim.MentionEvent, botOpenID, replyTargetMessageID string) bool {
	if strings.ToLower(strings.TrimSpace(chatType)) != "group" {
		return true
	}
	if strings.TrimSpace(replyTargetMessageID) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(messageType)) {
	case "text":
		rawText, err := ParseTextContent(rawContent)
		if err != nil {
			return false
		}
		mentionedBot := leadingFeishuMentionMatchesBot(rawText, mentions, botOpenID)
		if strings.TrimSpace(botOpenID) != "" {
			return mentionedBot
		}
		commandText, ok := stripLeadingFeishuMentionKeys(strings.TrimSpace(rawText), feishuMentionKeys(mentions))
		return ok && strings.HasPrefix(strings.TrimSpace(commandText), "/")
	default:
		if strings.TrimSpace(botOpenID) == "" {
			return len(feishuMentionKeys(mentions)) != 0
		}
		return feishuMentionMatchesBot(mentions, botOpenID)
	}
}

func leadingFeishuMentionMatchesBot(rawText string, mentions []*larkim.MentionEvent, botOpenID string) bool {
	botOpenID = strings.TrimSpace(botOpenID)
	if botOpenID == "" {
		return false
	}
	for _, mention := range leadingFeishuMentions(rawText, mentions) {
		if feishuMentionMatchesBot([]*larkim.MentionEvent{mention}, botOpenID) {
			return true
		}
	}
	return false
}

func feishuMentionMatchesBot(mentions []*larkim.MentionEvent, botOpenID string) bool {
	botOpenID = strings.TrimSpace(botOpenID)
	if botOpenID == "" {
		return false
	}
	for _, mention := range mentions {
		if mention == nil || mention.Id == nil {
			continue
		}
		if preferredFeishuUserID(stringPtr(mention.Id.OpenId), stringPtr(mention.Id.UserId), stringPtr(mention.Id.UnionId)) == botOpenID {
			return true
		}
	}
	return false
}

func leadingFeishuMentions(rawText string, mentions []*larkim.MentionEvent) []*larkim.MentionEvent {
	byKey := map[string]*larkim.MentionEvent{}
	for _, mention := range mentions {
		if mention == nil {
			continue
		}
		key := strings.TrimSpace(stringPtr(mention.Key))
		if key != "" {
			byKey[key] = mention
		}
	}
	if len(byKey) == 0 {
		return nil
	}
	rest := strings.TrimSpace(rawText)
	var values []*larkim.MentionEvent
	for {
		var matchedKey string
		var matchedMention *larkim.MentionEvent
		for key, mention := range byKey {
			if strings.HasPrefix(rest, key) && len(key) > len(matchedKey) {
				matchedKey = key
				matchedMention = mention
			}
		}
		if matchedKey == "" || matchedMention == nil {
			return values
		}
		values = append(values, matchedMention)
		rest = strings.TrimLeft(rest[len(matchedKey):], " \t\r\n")
		if rest == "" {
			return values
		}
	}
}

func normalizeFeishuCommandCandidate(rawText string, mentions []*larkim.MentionEvent) string {
	trimmed := strings.TrimSpace(rawText)
	if trimmed == "" {
		return rawText
	}
	if commandText, ok := stripLeadingFeishuMentionKeys(trimmed, feishuMentionKeys(mentions)); ok {
		return commandText
	}
	if len(mentions) == 0 && !strings.Contains(trimmed, "@_user_") {
		return rawText
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return rawText
	}
	index := 0
	for index < len(fields) && strings.HasPrefix(fields[index], "@") {
		index++
	}
	if index > 0 && index < len(fields) && strings.HasPrefix(fields[index], "/") {
		return strings.Join(fields[index:], " ")
	}
	return rawText
}

func stripLeadingFeishuMentionKeys(text string, keys []string) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	rest := strings.TrimSpace(text)
	stripped := false
	for {
		matched := false
		for _, key := range keys {
			if !strings.HasPrefix(rest, key) {
				continue
			}
			next := strings.TrimLeft(rest[len(key):], " \t\r\n")
			if next == "" {
				return "", false
			}
			rest = next
			stripped = true
			matched = true
			break
		}
		if !matched {
			break
		}
	}
	if stripped && strings.HasPrefix(rest, "/") {
		return rest, true
	}
	return "", false
}

type feishuMentionReplacement struct {
	key   string
	label string
}

func feishuMentionKeys(mentions []*larkim.MentionEvent) []string {
	replacements := feishuMentionReplacements(mentions)
	keys := make([]string, 0, len(replacements))
	for _, item := range replacements {
		keys = append(keys, item.key)
	}
	return keys
}

func feishuMentionReplacements(mentions []*larkim.MentionEvent) []feishuMentionReplacement {
	replacements := make([]feishuMentionReplacement, 0, len(mentions))
	seen := map[string]struct{}{}
	for _, mention := range mentions {
		if mention == nil {
			continue
		}
		key := strings.TrimSpace(stringPtr(mention.Key))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		replacements = append(replacements, feishuMentionReplacement{
			key:   key,
			label: feishuMentionDisplayLabel(mention, key),
		})
	}
	sort.SliceStable(replacements, func(i, j int) bool {
		return len(replacements[i].key) > len(replacements[j].key)
	})
	return replacements
}

func feishuMentionDisplayLabel(mention *larkim.MentionEvent, fallback string) string {
	if mention == nil {
		return fallback
	}
	name := strings.TrimSpace(stringPtr(mention.Name))
	if name == "" {
		return fallback
	}
	if strings.HasPrefix(name, "@") {
		return name
	}
	return "@" + name
}
