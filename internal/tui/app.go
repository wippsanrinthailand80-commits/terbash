package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/terbash/terbash/internal/config"
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
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		a.quitting = true
		return a, tea.Quit
	case tea.KeyEnter:
		if a.input != "" && !a.streaming {
			return a, a.processInput()
		}
	case tea.KeyBackspace:
		if a.cursor > 0 {
			a.input = a.input[:a.cursor-1] + a.input[a.cursor:]
			a.cursor--
		}
	case tea.KeyLeft:
		if a.cursor > 0 {
			a.cursor--
		}
	case tea.KeyRight:
		if a.cursor < len(a.input) {
			a.cursor++
		}
	case tea.KeyRunes:
		a.input = a.input[:a.cursor] + string(msg.Runes) + a.input[a.cursor:]
		a.cursor += len(msg.Runes)
	}
	return a, nil
}

func (a *App) processInput() tea.Cmd {
	userInput := strings.TrimSpace(a.input)
	a.input = ""
	a.cursor = 0

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
		a.addSystemMessage("Commands: /help, /providers, /tools, /clear, /config, /exit")
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
		a.addSystemMessage(fmt.Sprintf("Config: provider=%s, tools=%d", a.llmManager.GetDefaultProvider(), len(a.toolReg.List())))
	case "/exit", "/quit":
		a.quitting = true
		return tea.Quit
	default:
		a.addSystemMessage(fmt.Sprintf("Unknown command: %s", cmd))
	}
	return nil
}

func (a *App) streamLLM() tea.Cmd {
	return func() tea.Msg {
		provider := a.llmManager.GetDefaultProvider()
		tools := a.toolReg.GetSchemas()
		var toolSchemas []types.Tool
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

func (a *App) convertTools(tools []types.Tool) []types.Tool {
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
	b.WriteString(a.styles.Help.Render("Enter: send • Ctrl+C: quit • /help: commands"))

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