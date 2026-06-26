package workspaceskills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

const (
	MaxCount          = 20
	maxDescriptionLen = 500
)

type Skill struct {
	Name        string
	Description string
	Path        string
}

func MatchRequestedSkill(text string, skills []Skill) (*Skill, bool) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || len(skills) == 0 {
		return nil, false
	}
	for i := range skills {
		skill := &skills[i]
		name := strings.ToLower(strings.TrimSpace(skill.Name))
		if name == "" {
			continue
		}
		if strings.Contains(text, name) {
			return skill, true
		}
		if name == "gkprep-build-apk" && looksLikeGKPrepAPKRequest(text) {
			return skill, true
		}
	}
	return nil, false
}

func looksLikeGKPrepAPKRequest(text string) bool {
	hasAPK := strings.Contains(text, "apk") || strings.Contains(text, "安装包") || strings.Contains(text, "打包")
	hasBuildIntent := strings.Contains(text, "build") || strings.Contains(text, "package") || strings.Contains(text, "release") || strings.Contains(text, "debug") || strings.Contains(text, "构建") || strings.Contains(text, "出一个")
	hasVariant := strings.Contains(text, "y41air") || strings.Contains(text, "y41")
	return hasAPK && (hasBuildIntent || hasVariant)
}

func InjectHints(inputs []agentproto.Input, workspace string) []agentproto.Input {
	hint := Hint(workspace)
	if hint == "" {
		return inputs
	}
	out := append([]agentproto.Input(nil), inputs...)
	for i := range out {
		if out[i].Type != agentproto.InputText {
			continue
		}
		text := strings.TrimSpace(out[i].Text)
		if text == "" {
			continue
		}
		out[i].Text = hint + "\n\n" + out[i].Text
		return out
	}
	return inputs
}

func Hint(workspace string) string {
	skills := Read(workspace)
	if len(skills) == 0 {
		return ""
	}
	return Instructions(skills)
}

func Instructions(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	lines := []string{
		"[codex-remote workspace skills]",
		"The selected workspace exposes these local Codex skills from .agents/skills:",
	}
	for _, skill := range skills {
		line := fmt.Sprintf("- %s", skill.Name)
		if skill.Description != "" {
			line += ": " + skill.Description
		}
		line += fmt.Sprintf(" (file: %s)", skill.Path)
		lines = append(lines, line)
	}
	lines = append(lines,
		"Mandatory routing rule: before taking action, compare the user's request with the skill names and descriptions above. If a request matches a skill, you must use that skill.",
		"Using a skill means first opening and reading its SKILL.md, then following its workflow and scripts. Resolve relative files from that skill directory.",
		"[/codex-remote workspace skills]",
	)
	return strings.Join(lines, "\n")
}

func Read(workspace string) []Skill {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(workspace, ".agents", "skills"))
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(workspace, ".agents", "skills", entry.Name())
		skillPath := filepath.Join(skillDir, "SKILL.md")
		body, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		name, description := ParseHeader(string(body))
		if name == "" {
			name = entry.Name()
		}
		skills = append(skills, Skill{
			Name:        name,
			Description: truncateDescription(description),
			Path:        skillPath,
		})
		if len(skills) >= MaxCount {
			break
		}
	}
	return skills
}

func ParseHeader(body string) (string, string) {
	body = strings.TrimPrefix(body, "\ufeff")
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", ""
	}
	var name, description string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}
	return name, description
}

func truncateDescription(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= maxDescriptionLen {
		return value
	}
	return strings.TrimSpace(value[:maxDescriptionLen]) + "..."
}

func Tokenize(value string) []string {
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) >= 3 {
			out = append(out, part)
		}
	}
	return out
}
