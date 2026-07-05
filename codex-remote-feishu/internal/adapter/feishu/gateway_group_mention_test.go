package feishu

import larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

func botMention() []*larkim.MentionEvent {
	return []*larkim.MentionEvent{{
		Key:  stringRef("@_user_1"),
		Id:   &larkim.UserId{OpenId: stringRef("ou_bot")},
		Name: stringRef("Codex Remote"),
	}}
}
