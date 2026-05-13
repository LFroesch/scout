package suitechrome

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type AppMeta struct {
	ID    string
	Name  string
	Icon  string
	Color string
}

type Tab struct {
	Label  string
	Active bool
}

type Action struct {
	Key   string
	Label string
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	activeTabStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Underline(true)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Bold(true)

	actionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230"))

	sepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
)

var catalog = map[string]AppMeta{
	"backup-xd": {ID: "backup-xd", Name: "backup-xd", Icon: "💾", Color: "141"},
	"bobdb":     {ID: "bobdb", Name: "bobdb", Icon: "🧮", Color: "111"},
	"dwight":    {ID: "dwight", Name: "dwight", Icon: "🤖", Color: "51"},
	"logdog":    {ID: "logdog", Name: "logdog", Icon: "🐶", Color: "203"},
	"portmon":   {ID: "portmon", Name: "portmon", Icon: "📡", Color: "214"},
	"runx":      {ID: "runx", Name: "runx", Icon: "📜", Color: "117"},
	"sb":        {ID: "sb", Name: "sb", Icon: "📓", Color: "149"},
	"scout":     {ID: "scout", Name: "scout", Icon: "🔎", Color: "81"},
	"seedbank":  {ID: "seedbank", Name: "seedbank", Icon: "🌱", Color: "78"},
	"stickies":  {ID: "stickies", Name: "stickies", Icon: "📝", Color: "229"},
	"tui-hub":   {ID: "tui-hub", Name: "tui-hub", Icon: "🧰", Color: "117"},
	"unrot":     {ID: "unrot", Name: "unrot", Icon: "🧠", Color: "177"},
	"zap":       {ID: "zap", Name: "zap", Icon: "⚡", Color: "220"},
}

func App(id string) AppMeta {
	if meta, ok := catalog[id]; ok {
		return meta
	}
	return AppMeta{ID: id, Name: id, Icon: "•", Color: "117"}
}

func Dim(text string) string {
	return dimStyle.Render(text)
}

func RenderTitle(appID, version string) string {
	meta := App(appID)
	icon := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(meta.Color)).
		Render(meta.Icon)
	title := titleStyle.Render(meta.Name)
	if strings.TrimSpace(version) == "" {
		return icon + " " + title
	}
	return icon + " " + title + " " + dimStyle.Render(version)
}

func RenderTabs(tabs []Tab) string {
	var rendered []string
	for i, tab := range tabs {
		if i > 0 {
			rendered = append(rendered, dimStyle.Render("  │  "))
		}
		if tab.Active {
			rendered = append(rendered, activeTabStyle.Render(tab.Label))
		} else {
			rendered = append(rendered, inactiveTabStyle.Render(tab.Label))
		}
	}
	return strings.Join(rendered, "")
}

func RenderActions(actions []Action) string {
	var parts []string
	for _, action := range actions {
		if strings.TrimSpace(action.Key) == "" && strings.TrimSpace(action.Label) == "" {
			continue
		}
		if len(parts) > 0 {
			parts = append(parts, sepStyle.Render(" · "))
		}
		parts = append(parts, keyStyle.Render(action.Key), " ", actionStyle.Render(action.Label))
	}
	return strings.Join(parts, "")
}

func JoinHeader(width int, left, right string) string {
	if strings.TrimSpace(right) == "" {
		return left
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func JoinLine(width int, left, right string) string {
	return JoinHeader(width, left, right)
}
