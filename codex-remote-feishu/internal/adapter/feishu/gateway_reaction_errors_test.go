package feishu

import "testing"

func TestIgnoredMissingReactionCreateError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{name: "english missing message", msg: "message not found", want: true},
		{name: "english missing message sentence", msg: "The message is not found", want: true},
		{name: "english missing target sentence", msg: "The target message is not found", want: true},
		{name: "english recalled message", msg: "target message has been recalled", want: true},
		{name: "chinese missing message", msg: "目标消息不存在", want: true},
		{name: "reaction id not found", msg: "reaction not found", want: false},
		{name: "empty", msg: "", want: false},
	}
	for _, tt := range tests {
		if got := ignoredMissingReactionCreateError(0, tt.msg); got != tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIgnoredMissingReactionDeleteError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{name: "english missing message", msg: "message not found", want: true},
		{name: "reaction id not found", msg: "reaction not found", want: true},
		{name: "reaction deleted", msg: "reaction deleted", want: true},
		{name: "chinese missing reaction", msg: "表情不存在", want: true},
		{name: "other error", msg: "permission denied", want: false},
		{name: "empty", msg: "", want: false},
	}
	for _, tt := range tests {
		if got := ignoredMissingReactionDeleteError(0, tt.msg); got != tt.want {
			t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}
