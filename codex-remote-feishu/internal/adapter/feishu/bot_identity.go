package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type BotIdentity struct {
	AppName string
	OpenID  string
}

type botTenantAccessTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

type botInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Bot  struct {
		AppName string `json:"app_name"`
		OpenID  string `json:"open_id"`
	} `json:"bot"`
}

func FetchBotIdentity(ctx context.Context, cfg LiveGatewayConfig) (BotIdentity, error) {
	appID := strings.TrimSpace(cfg.AppID)
	appSecret := strings.TrimSpace(cfg.AppSecret)
	if appID == "" || appSecret == "" {
		return BotIdentity{}, fmt.Errorf("missing app credentials")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.Domain), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(lark.FeishuBaseUrl, "/")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	broker := NewFeishuCallBrokerWithHTTPClient("bot-identity-"+appID, nil, client)

	payload, err := json.Marshal(map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	})
	if err != nil {
		return BotIdentity{}, err
	}

	var tokenResp botTenantAccessTokenResponse
	_, err = DoHTTP(ctx, broker, CallSpec{
		GatewayID:  cfg.GatewayID,
		API:        "auth.v3.tenant_access_token.internal",
		Class:      CallClassMetaHTTP,
		Priority:   CallPriorityInteractive,
		Retry:      RetryOff,
		Permission: PermissionFailFast,
	}, func(callCtx context.Context, httpClient *http.Client) (struct{}, error) {
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, baseURL+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(payload))
		if err != nil {
			return struct{}{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			return struct{}{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return struct{}{}, fmt.Errorf("tenant access token request failed: status=%d", resp.StatusCode)
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tokenResp); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		return BotIdentity{}, err
	}
	if tokenResp.Code != 0 || strings.TrimSpace(tokenResp.TenantAccessToken) == "" {
		return BotIdentity{}, fmt.Errorf("tenant access token failed: code=%d msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	var infoResp botInfoResponse
	_, err = DoHTTP(ctx, broker, CallSpec{
		GatewayID:  cfg.GatewayID,
		API:        "bot.v3.info",
		Class:      CallClassMetaHTTP,
		Priority:   CallPriorityInteractive,
		Retry:      RetrySafe,
		Permission: PermissionFailFast,
	}, func(callCtx context.Context, httpClient *http.Client) (struct{}, error) {
		req, err := http.NewRequestWithContext(callCtx, http.MethodGet, baseURL+"/open-apis/bot/v3/info", nil)
		if err != nil {
			return struct{}{}, err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(tokenResp.TenantAccessToken))
		resp, err := httpClient.Do(req)
		if err != nil {
			return struct{}{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return struct{}{}, fmt.Errorf("bot info request failed: status=%d", resp.StatusCode)
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&infoResp); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		return BotIdentity{}, err
	}
	if infoResp.Code != 0 {
		return BotIdentity{}, fmt.Errorf("bot info failed: code=%d msg=%s", infoResp.Code, infoResp.Msg)
	}
	return BotIdentity{
		AppName: strings.TrimSpace(infoResp.Bot.AppName),
		OpenID:  strings.TrimSpace(infoResp.Bot.OpenID),
	}, nil
}
