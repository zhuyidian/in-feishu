package feishu

import (
	"context"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestPlanInboundMessageEventIgnoresGroupReplyToHumanMessage(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", BotOpenID: "ou_bot"})
	gateway.fetchMessageFn = func(_ context.Context, messageID string) (*gatewayMessage, error) {
		if messageID != "om-human-parent-plan-1" {
			t.Fatalf("unexpected parent message lookup: %s", messageID)
		}
		return &gatewayMessage{MessageID: messageID, MessageType: "text", SenderID: "ou_human"}, nil
	}
	event := testTextMessageEvent("evt-human-reply-plan-1", "om-human-reply-plan-1", "@_user_1 ok")
	event.Event.Message.ChatType = stringRef("group")
	event.Event.Message.ParentId = stringRef("om-human-parent-plan-1")
	event.Event.Message.Mentions = []*larkim.MentionEvent{{
		Key: stringRef("@_user_1"),
		Id:  &larkim.UserId{OpenId: stringRef("ou_human")},
	}}

	plan, ok, err := gateway.planInboundMessageEvent(event)
	if err != nil {
		t.Fatalf("planInboundMessageEvent returned error: %v", err)
	}
	if ok || plan.action != nil || plan.queue != nil {
		t.Fatalf("expected human reply to be ignored, got %#v", plan)
	}
}

func TestPlanInboundMessageEventHandlesGroupReplyToBotMessage(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", BotOpenID: "ou_bot"})
	gateway.fetchMessageFn = func(_ context.Context, messageID string) (*gatewayMessage, error) {
		if messageID != "om-bot-parent-plan-1" {
			t.Fatalf("unexpected parent message lookup: %s", messageID)
		}
		return &gatewayMessage{MessageID: messageID, MessageType: "text", SenderID: "ou_bot"}, nil
	}
	event := testTextMessageEvent("evt-bot-reply-plan-1", "om-bot-reply-plan-1", "继续处理")
	event.Event.Message.ChatType = stringRef("group")
	event.Event.Message.ParentId = stringRef("om-bot-parent-plan-1")

	plan, ok, err := gateway.planInboundMessageEvent(event)
	if err != nil {
		t.Fatalf("planInboundMessageEvent returned error: %v", err)
	}
	if !ok || plan.action != nil || plan.queue == nil {
		t.Fatalf("expected bot reply to enter the queue, got %#v", plan)
	}
}
