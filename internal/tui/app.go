package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/terbash/terbash/internal/llm"
	"github.com/terbash/terbash/internal/tools"
	"github.com/terbash/terbash/pkg/types"
)

type App struct {
	config     *types.Config
	llmManager *llm.Manager
	toolReg    *tools.Registry
	renderer   *glamour.TermRenderer
	messages   []types.Message
	input      string
	cursor     int
	streaming  bool
	streamBuf  strings.Builder
	styles     Styles
	quitting   bool
	paletteIdx int
}

type Styles struct {
	Base       lipgloss.Style
	UserMsg    lipgloss.Style
	AssistantMsg lipgloss.Style
	SystemMsg  lipgloss.Style
	ToolMsg    lipgloss.Style
	Input      lipgloss.Style
	Cursor     lipgloss.Style
	Help       lipgloss.Style
	PaletteSel lipgloss.Style
	PaletteNorm lipgloss.Style
}

// slashCmd describes one "/" command shown in the palette.
type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/help", "Show all commands"},
	{"/providers", "List configured LLM providers"},
	{"/tools", "List available tools"},
	{"/clear", "Clear conversation"},
	{"/config", "Show current config"},
	{"/update", "Self-update hint (run `terbash update` outside chat)"},
	{"/exit", "Quit (also /quit, Ctrl+C)"},
}

// paletteMatches returns slash commands matching the current input.
// Typing "/" alone shows everything; "/pr" filters to /providers, etc.
func paletteMatches(input string) []slashCmd {
	if !strings.HasPrefix(input, "/") {
		return nil
	}
	// Only complete the first word (command), not arguments.
	word := input
	if i := strings.IndexAny(input, " \t"); i >= 0 {
		word = input[:i]
	}
	filter := strings.ToLower(strings.TrimPrefix(word, "/"))
	var out []slashCmd
	for _, c := range slashCommands {
		if strings.HasPrefix(strings.ToLower(strings.TrimPrefix(c.name, "/")), filter) {
			out = append(out, c)
		}
	}
	return out
}

func NewApp(cfg *types.Config) *App {
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	llmManager, _ := llm.NewManager(cfg)
	toolReg := tools.NewRegistry(cfg, ".")

	return &App{
		config:     cfg,
		llmManager: llmManager,
		toolReg:    toolReg,
		renderer:   r,
		messages:   []types.Message{},
		styles:     defaultStyles(),
	}
}

func defaultStyles() Styles {
	return Styles{
		Base: lipgloss.NewStyle().Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Padding(0, 1),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Padding(0, 1),
		ToolMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Padding(0, 1),
		Input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1),
		Cursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Blink(true),
	Help: lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true),
	PaletteSel: lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("62")).
		Bold(true).
		Padding(0, 1),
	PaletteNorm: lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1),
	}
}

func (a *App) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKey(msg)
	case tea.WindowSizeMsg:
		return a, nil
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	matches := paletteMatches(a.input)

	switch msg.Type {
	case tea.KeyCtrlC:
		a.quitting = true
		return a, tea.Quit
	case tea.KeyEsc:
		// If the "/" palette is open, Esc closes it instead of quitting.
		if len(matches) > 0 {
			a.input = ""
			a.cursor = 0
			a.paletteIdx = 0
			return a, nil
		}
		a.quitting = true
		return a, tea.Quit
	case tea.KeyUp:
		if len(matches) > 0 {
			a.paletteIdx = (a.paletteIdx - 1 + len(matches)) % len(matches)
			return a, nil
		}
	case tea.KeyDown:
		if len(matches) > 0 {
			a.paletteIdx = (a.paletteIdx + 1) % len(matches)
			return a, nil
		}
	case tea.KeyTab:
		if len(matches) > 0 {
			a.completePalette(matches)
			return a, nil
		}
	case tea.KeyEnter:
		if a.input != "" && !a.streaming {
			// Partial command like "/pr" + Enter completes first instead
			// of submitting an unknown command.
			if len(matches) > 0 && !a.exactSlashMatch() {
				a.completePalette(matches)
				return a, nil
			}
			return a, a.processInput()
		}
	case tea.KeyBackspace:
		if a.cursor > 0 {
			a.input = a.input[:a.cursor-1] + a.input[a.cursor:]
			a.cursor--
			a.paletteIdx = 0
		}
	case tea.KeyLeft:
		if a.cursor > 0 {
			a.cursor--
		}
	case tea.KeyRight:
		if a.cursor < len(a.input) {
			a.cursor++
		}
	case tea.KeyRunes, tea.KeySpace:
		if len(msg.Runes) > 0 {
			a.input = a.input[:a.cursor] + string(msg.Runes) + a.input[a.cursor:]
			a.cursor += len(msg.Runes)
			a.paletteIdx = 0
		}
	}
	return a, nil
}

// exactSlashMatch reports whether the input is exactly a known command.
func (a *App) exactSlashMatch() bool {
	word := a.input
	if i := strings.IndexAny(a.input, " \t"); i >= 0 {
		word = a.input[:i]
	}
	for _, c := range slashCommands {
		if strings.EqualFold(word, c.name) {
			return true
		}
	}
	return false
}

// completePalette replaces the input with the selected palette entry.
func (a *App) completePalette(matches []slashCmd) {
	if len(matches) == 0 {
		return
	}
	if a.paletteIdx < 0 || a.paletteIdx >= len(matches) {
		a.paletteIdx = 0
	}
	a.input = matches[a.paletteIdx].name
	a.cursor = len(a.input)
}

func (a *App) processInput() tea.Cmd {
	userInput := strings.TrimSpace(a.input)
	a.input = ""
	a.cursor = 0
	a.paletteIdx = 0

	if userInput == "" {
		return nil
	}

	if strings.HasPrefix(userInput, "/") {
		return a.handleCommand(userInput)
	}

	a.messages = append(a.messages, types.Message{Role: "user", Content: userInput})
	a.streaming = true
	a.streamBuf.Reset()

	return a.streamLLM()
}

func (a *App) handleCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/help":
		var b strings.Builder
		b.WriteString("Commands:\n")
		for _, c := range slashCommands {
			fmt.Fprintf(&b, "  %s - %s\n", c.name, c.desc)
		}
		a.addSystemMessage(strings.TrimRight(b.String(), "\n"))
	case "/providers":
		providers := a.llmManager.ListProviders()
		a.addSystemMessage(fmt.Sprintf("Available providers: %s", strings.Join(providers, ", ")))
	case "/tools":
		var toolNames []string
		for _, t := range a.toolReg.List() {
			toolNames = append(toolNames, t.Name())
		}
		a.addSystemMessage(fmt.Sprintf("Available tools: %s", strings.Join(toolNames, ", ")))
	case "/clear":
		a.messages = []types.Message{}
	case "/config":
		a.addSystemMessage(fmt.Sprintf("Providers configured: %d, tools: %d (default: ollama)", len(a.llmManager.ListProviders()), len(a.toolReg.List())))
	case "/update":
		a.addSystemMessage("To self-update, exit chat and run: terbash update")
	case "/exit", "/quit":
		a.quitting = true
		return tea.Quit
	default:
		a.addSystemMessage(fmt.Sprintf("Unknown command: %s (type / for the list)", cmd))
	}
	return nil
}

func (a *App) streamLLM() tea.Cmd {
	return func() tea.Msg {
		provider := a.llmManager.GetDefaultProvider()
		tools := a.toolReg.GetSchemas()
		var toolSchemas []types.ToolInterface
		for _, t := range tools {
			toolSchemas = append(toolSchemas, t)
		}

		req := types.ChatCompletionRequest{
			Model:       provider.GetConfig().Model,
			Messages:    a.messages,
			Tools:       a.convertTools(toolSchemas),
			Temperature: provider.GetConfig().Temperature,
			MaxTokens:   provider.GetConfig().MaxTokens,
			Stream:      true,
		}

		stream, err := provider.ChatCompletionStream(req)
		if err != nil {
			return streamErrorMsg{err: err}
		}

		return streamStartMsg{stream: stream}
	}
}

func (a *App) convertTools(tools []types.ToolInterface) []types.Tool {
	var result []types.Tool
	for _, t := range tools {
		result = append(result, types.Tool{
			Type: "function",
			Function: types.Function{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}
	return result
}

func (a *App) addSystemMessage(content string) {
	a.messages = append(a.messages, types.Message{Role: "system", Content: content})
}

func (a *App) View() string {
	if a.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	for _, msg := range a.messages {
		switch msg.Role {
		case "user":
			b.WriteString(a.styles.UserMsg.Render("▌ You"))
			b.WriteString("\n")
			rendered, _ := a.renderer.Render(msg.Content)
			b.WriteString(a.styles.Base.Render(rendered))
		case "assistant":
			b.WriteString(a.styles.AssistantMsg.Render("▌ Assistant"))
			b.WriteString("\n")
			if msg.Content != "" {
				rendered, _ := a.renderer.Render(msg.Content)
				b.WriteString(a.styles.Base.Render(rendered))
			}
			for _, tc := range msg.ToolCalls {
				b.WriteString(a.styles.ToolMsg.Render(fmt.Sprintf("  🔧 %s(%s)", tc.Function.Name, tc.Function.Arguments)))
			}
		case "system":
			b.WriteString(a.styles.SystemMsg.Render(msg.Content))
		case "tool":
			b.WriteString(a.styles.ToolMsg.Render(fmt.Sprintf("🔧 Tool Result: %s", msg.Content)))
		}
		b.WriteString("\n\n")
	}

	if a.streaming {
		b.WriteString(a.styles.AssistantMsg.Render("▌ Assistant"))
		b.WriteString("\n")
		if a.streamBuf.Len() > 0 {
			rendered, _ := a.renderer.Render(a.streamBuf.String())
			b.WriteString(a.styles.Base.Render(rendered))
		}
		b.WriteString(a.styles.Cursor.Render("█"))
		b.WriteString("\n\n")
	}

	inputView := a.styles.Input.Render(a.input[:a.cursor] + " " + a.input[a.cursor:])
	b.WriteString(inputView)
	b.WriteString("\n")

	// "/" command palette: live-filtered list under the input box.
	if matches := paletteMatches(a.input); len(matches) > 0 {
		if a.paletteIdx < 0 || a.paletteIdx >= len(matches) {
			a.paletteIdx = 0
		}
		const maxShow = 8
		shown := matches
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		for i, c := range shown {
			line := fmt.Sprintf("  %s  %s", c.name, c.desc)
			if i == a.paletteIdx {
				b.WriteString(a.styles.PaletteSel.Render("›"+line))
			} else {
				b.WriteString(a.styles.PaletteNorm.Render(" "+line))
			}
			b.WriteString("\n")
		}
		if len(matches) > maxShow {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d more", len(matches)-maxShow)))
			b.WriteString("\n")
		}
		b.WriteString(a.styles.Help.Render("↑↓: move • Tab/Enter: complete • Esc: close"))
	} else {
		b.WriteString(a.styles.Help.Render("Enter: send • Ctrl+C: quit • /: commands"))
	}

	return b.String()
}

type streamStartMsg struct {
	stream <-chan types.StreamChunk
}

type streamErrorMsg struct {
	err error
}

func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}