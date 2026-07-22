package feishu

import (
	"context"
	"log"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	gatewaypkg "github.com/kxn/codex-remote-feishu/internal/adapter/feishu/gateway"
	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func gatewayDispatcher(handler ActionHandler) gatewaypkg.ActionDispatcher {
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, action control.Action) error {
		return handleGatewayEventAction(ctx, action, handler)
	}
}

func (g *LiveGateway) routingEnv() gatewaypkg.RoutingEnv {
	return gatewaypkg.RoutingEnv{
		GatewayID:            g.config.GatewayID,
		SurfaceForCardAction: g.surfaceForCardAction,
	}
}

func (g *LiveGateway) inboundEnv() gatewaypkg.InboundEnv {
	return gatewaypkg.InboundEnv{
		GatewayID:                     g.config.GatewayID,
		BotOpenID:                     g.botOpenID(),
		LookupSurfaceMessage:          g.lookupSurfaceMessage,
		IsReplyTargetBot:              g.isReplyTargetBot,
		ParseTextActionWithoutCatalog: control.ParseFeishuTextActionWithoutCatalog,
		QuotedInputs:                  g.quotedInputs,
		ParsePostInputs:               g.parsePostInputs,
		BuildMergeForwardStructuredInput: func(ctx context.Context, message *larkim.EventMessage) (string, []agentproto.Input, error) {
			payload, err := g.buildMergeForwardStructuredPayloadFromEvent(ctx, message)
			if err != nil {
				return "", nil, err
			}
			return payload.Summary, payload.Inputs, nil
		},
		RecordSurfaceMessage:       g.recordSurfaceMessage,
		DownloadImage:              g.downloadImageFn,
		DownloadFile:               g.downloadFileFn,
		DeliverAsyncInboundFailure: g.deliverAsyncInboundFailure,
	}
}

func (g *LiveGateway) botOpenID() string {
	if g == nil {
		return ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.TrimSpace(g.config.BotOpenID)
}

func (g *LiveGateway) ensureBotIdentity(ctx context.Context) {
	if g == nil || strings.TrimSpace(g.botOpenID()) != "" {
		return
	}
	identityCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	identity, err := FetchBotIdentity(identityCtx, g.config)
	if err != nil {
		log.Printf("feishu bot identity lookup failed: gateway=%s err=%v", strings.TrimSpace(g.config.GatewayID), err)
		return
	}
	if strings.TrimSpace(identity.OpenID) == "" {
		log.Printf("feishu bot identity lookup returned empty open_id: gateway=%s app_name=%s", strings.TrimSpace(g.config.GatewayID), strings.TrimSpace(identity.AppName))
		return
	}
	g.mu.Lock()
	g.config.BotOpenID = strings.TrimSpace(identity.OpenID)
	g.mu.Unlock()
	log.Printf("feishu bot identity loaded: gateway=%s app_name=%s", strings.TrimSpace(g.config.GatewayID), strings.TrimSpace(identity.AppName))
}

func (g *LiveGateway) lookupSurfaceMessage(messageID string) string {
	if g == nil {
		return ""
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return strings.TrimSpace(g.messages[messageID])
}

func (g *LiveGateway) surfaceForCardAction(messageID, chatID, operatorID string) string {
	if g == nil {
		return ""
	}
	if surfaceID := g.lookupSurfaceMessage(messageID); surfaceID != "" {
		return surfaceID
	}
	return gatewaypkg.SurfaceIDForInbound(g.config.GatewayID, chatID, "", operatorID)
}

func (g *LiveGateway) deliverAsyncInboundFailure(ctx context.Context, surfaceID, chatID, actorUserID, replyToMessageID, body string) {
	if g == nil || strings.TrimSpace(body) == "" {
		return
	}
	receiveID, receiveIDType := gatewaypkg.ResolveReceiveTarget(chatID, actorUserID)
	if receiveID == "" || receiveIDType == "" {
		return
	}
	op := Operation{
		Kind:             OperationSendCard,
		GatewayID:        g.config.GatewayID,
		SurfaceSessionID: strings.TrimSpace(surfaceID),
		ReceiveID:        receiveID,
		ReceiveIDType:    receiveIDType,
		ChatID:           strings.TrimSpace(chatID),
		ReplyToMessageID: strings.TrimSpace(replyToMessageID),
		CardTitle:        "消息未处理",
		CardBody:         body,
		CardThemeKey:     cardThemeError,
		cardEnvelope:     cardEnvelopeV2,
		card:             rawCardDocument("消息未处理", body, cardThemeError, nil),
	}
	applyCtx, cancel := newFeishuTimeoutContext(ctx, asyncInboundFailureNoticeTimeout)
	defer cancel()
	if err := g.Apply(applyCtx, []Operation{op}); err != nil {
		log.Printf("feishu async inbound failure notice delivery failed: surface=%s reply_to=%s err=%v", surfaceID, replyToMessageID, err)
	}
}
