package tui

import (
	"encoding/json"
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

// maxToolIters bounds the agentic loop: user message -> LLM -> tools ->
// LLM ... stops after this many LLM turns even if tools keep coming.
const maxToolIters = 8

// Version is set by the CLI entrypoint (dev when built without ldflags).
var Version = "dev"

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
	// Provider picker overlay state (opened by /providers or /provider).
	pickingProvider bool
	providerIdx     int
	// Model picker overlay state (opened by /models).
	pickingModel bool
	loadingModel bool
	modelReq     int
	modelNames   []string
	modelIdx     int
	modelFor     string
	// Agentic tool-loop state for the current user message.
	streamCh      <-chan types.StreamChunk
	toolIter      int
	pendingCalls  []types.ToolCall
	fragTools     []*toolFrag
	confirmCall   *types.ToolCall
	confirmPrompt string
	// Terminal size for the bottom status bar (from WindowSizeMsg).
	termWidth int
}

// toolFrag accumulates one streamed tool call. Providers send arguments
// in fragments across chunks, so we stitch by call ID (falling back to
// the last open call when a fragment carries no ID).
type toolFrag struct {
	id   string
	name string
	args strings.Builder
}

func mergeToolFrag(frags *[]*toolFrag, tc types.ToolCall) {
	if tc.ID != "" {
		for _, f := range *frags {
			if f.id == tc.ID {
				if tc.Function.Name != "" && f.name == "" {
					f.name = tc.Function.Name
				}
				f.args.WriteString(tc.Function.Arguments)
				return
			}
		}
		f := &toolFrag{id: tc.ID, name: tc.Function.Name}
		f.args.WriteString(tc.Function.Arguments)
		*frags = append(*frags, f)
		return
	}
	if len(*frags) > 0 {
		last := (*frags)[len(*frags)-1]
		if tc.Function.Name == "" || last.name == "" || tc.Function.Name == last.name {
			if tc.Function.Name != "" && last.name == "" {
				last.name = tc.Function.Name
			}
			last.args.WriteString(tc.Function.Arguments)
			return
		}
	}
	f := &toolFrag{name: tc.Function.Name}
	f.args.WriteString(tc.Function.Arguments)
	*frags = append(*frags, f)
}

func fragsToCalls(frags []*toolFrag) []types.ToolCall {
	var out []types.ToolCall
	for _, f := range frags {
		if f.name == "" && f.args.Len() == 0 {
			continue
		}
		out = append(out, types.ToolCall{
			ID:   f.id,
			Type: "function",
			Function: types.FunctionCall{
				Name:      f.name,
				Arguments: f.args.String(),
			},
		})
	}
	return out
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
	Version    lipgloss.Style
	PaletteSel lipgloss.Style
	PaletteNorm lipgloss.Style
}

// bottomBar renders the hint left-aligned with the version pinned to the
// bottom-right corner. Falls back to inline when the width is unknown.
func (a *App) bottomBar(hint string) string {
	ver := "terbash " + Version
	if a.termWidth <= 0 {
		return a.styles.Help.Render(hint + " • " + ver)
	}
	gap := a.termWidth - lipgloss.Width(hint) - lipgloss.Width(ver)
	if gap < 1 {
		return a.styles.Help.Render(hint + " • " + ver)
	}
	return a.styles.Help.Render(hint) + strings.Repeat(" ", gap) + a.styles.Version.Render(ver)
}

// slashCmd describes one "/" command shown in the palette.
type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/help", "Show all commands"},
	{"/providers", "Pick the active LLM provider (interactive list)"},
	{"/provider", "Switch provider directly (e.g. /provider groq)"},
	{"/model", "Show or switch model (e.g. /model llama3.2:3b)"},
	{"/models", "Pick a model from a list"},
	{"/key", "Show key status or save one (/key groq sk-...)"},
	{"/status", "Show version, provider, model, counts"},
	{"/version", "Show terbash version"},
	{"/tools", "List available tools"},
	{"/clear", "Clear conversation"},
	{"/config", "Show current config"},
	{"/update", "Self-update hint (run `terbash update` outside chat)"},
	{"/exit", "Quit (also /quit, Ctrl+C)"},
}

// windowSelection returns the [start:end] row window of n items (at most
// max rows) that keeps the selected index visible. Used by both pickers
// so scrolling past the visible rows follows the highlight.
func windowSelection(n, sel, max int) (int, int) {
	if n <= max || max <= 0 {
		return 0, n
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= n {
		sel = n - 1
	}
	start := sel - max + 1
	if start < 0 {
		start = 0
	}
	if start > n-max {
		start = n - max
	}
	return start, start + max
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
	// The chat loop shows its own approval overlay before executing, so
	// tools must never block on stdin while the fullscreen TUI owns it.
	tools.ConfirmFunc = func(enabled bool, prompt string) bool { return true }

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
	Version: lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")),
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
		a.termWidth = msg.Width
		return a, nil
	case waitChunkMsg:
		return a, waitForChunk(msg.stream)
	case streamChunkMsg:
		return a.handleStreamChunk(msg)
	case streamDoneMsg:
		return a.finishStream()
	case streamErrorMsg:
		a.streaming = false
		a.toolIter = 0
		a.pendingCalls = nil
		a.fragTools = nil
		a.addSystemMessage(fmt.Sprintf("LLM error: %v", msg.err))
		return a, nil
	case toolResultMsg:
		return a.handleToolResult(msg)
	case modelsMsg:
		if msg.req != a.modelReq {
			return a, nil // superseded (user cancelled or re-ran /models)
		}
		a.loadingModel = false
		if msg.err != nil {
			a.addSystemMessage(fmt.Sprintf("Could not list models for %s: %v", a.modelFor, msg.err))
			return a, nil
		}
		if len(msg.models) == 0 {
			a.addSystemMessage(fmt.Sprintf("No models reported by %s.", a.modelFor))
			return a, nil
		}
		a.modelNames = msg.models
		a.modelIdx = 0
		for i, m := range msg.models {
			if m == a.llmManager.ProviderModel(a.modelFor) {
				a.modelIdx = i
				break
			}
		}
		a.pickingModel = true
		return a, nil
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Tool approval overlay takes over ALL keys while open: the chat box
	// must not eat the y/n answer, and Enter must not send a message.
	if a.confirmCall != nil {
		switch msg.Type {
		case tea.KeyCtrlC:
			a.quitting = true
			return a, tea.Quit
		case tea.KeyRunes, tea.KeySpace:
			if len(msg.Runes) == 1 {
				switch strings.ToLower(string(msg.Runes)) {
				case "y":
					call := *a.confirmCall
					a.confirmCall = nil
					return a, a.execToolCmd(call)
				case "n":
					a.denyConfirm()
					return a, a.driveToolQueue()
				}
			}
			return a, nil
		case tea.KeyEnter, tea.KeyEsc:
			// [y/N] defaults to No.
			a.denyConfirm()
			return a, a.driveToolQueue()
		default:
			return a, nil
		}
	}

	// Model picker overlay: same keyboard scheme as the provider picker.
	if a.pickingModel {
		n := len(a.modelNames)
		move := func(delta int) {
			if n > 0 {
				a.modelIdx = (a.modelIdx + delta + n) % n
			}
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			a.quitting = true
			return a, tea.Quit
		case tea.KeyEsc:
			a.pickingModel = false
			return a, nil
		case tea.KeyUp, tea.KeyCtrlP:
			move(-1)
			return a, nil
		case tea.KeyDown, tea.KeyCtrlN:
			move(1)
			return a, nil
		case tea.KeyEnter:
			a.chooseModel()
			return a, nil
		case tea.KeyRunes, tea.KeySpace:
			if len(msg.Runes) == 1 {
				switch r := msg.Runes[0]; {
				case r == 'k':
					move(-1)
					return a, nil
				case r == 'j':
					move(1)
					return a, nil
				case r >= '1' && r <= '9':
					if idx := int(r - '1'); idx < n {
						a.modelIdx = idx
						a.chooseModel()
					}
					return a, nil
				}
			}
		}
		a.pickingModel = false
	}

	// Provider picker overlay takes over keys while open.
	// Arrow keys do not exist on some mobile keyboards, so j/k,
	// Ctrl+P/Ctrl+N and number keys work as alternatives.
	if a.pickingProvider {
		names := a.llmManager.AllProviderNames()
		move := func(delta int) {
			if len(names) > 0 {
				a.providerIdx = (a.providerIdx + delta + len(names)) % len(names)
			}
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			a.quitting = true
			return a, tea.Quit
		case tea.KeyEsc:
			a.pickingProvider = false
			return a, nil
		case tea.KeyUp, tea.KeyCtrlP:
			move(-1)
			return a, nil
		case tea.KeyDown, tea.KeyCtrlN:
			move(1)
			return a, nil
		case tea.KeyEnter:
			a.chooseProvider()
			return a, nil
		case tea.KeyRunes, tea.KeySpace:
			if len(msg.Runes) == 1 {
				switch r := msg.Runes[0]; {
				case r == 'k':
					move(-1)
					return a, nil
				case r == 'j':
					move(1)
					return a, nil
				case r >= '1' && r <= '9':
					if idx := int(r - '1'); idx < len(names) {
						a.providerIdx = idx
						a.chooseProvider()
					}
					return a, nil
				}
			}
		}
		// Any other key (typing, etc.) closes the picker and is
		// handled normally below.
		a.pickingProvider = false
	}

	matches := paletteMatches(a.input)

	switch msg.Type {
	case tea.KeyCtrlC:
		a.quitting = true
		return a, tea.Quit
	case tea.KeyEsc:
		// A pending model fetch is cancelled first (its late reply is
		// ignored via the request generation counter).
		if a.loadingModel {
			a.loadingModel = false
			a.modelReq++
			return a, nil
		}
		// If the "/" palette is open, Esc closes it instead of quitting.
		if len(matches) > 0 {
			a.input = ""
			a.cursor = 0
			a.paletteIdx = 0
			return a, nil
		}
		a.quitting = true
		return a, tea.Quit
	case tea.KeyUp, tea.KeyCtrlP:
		if len(matches) > 0 {
			a.paletteIdx = (a.paletteIdx - 1 + len(matches)) % len(matches)
			return a, nil
		}
	case tea.KeyDown, tea.KeyCtrlN:
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
			// j/k navigate the "/" palette while still completing the
			// command word (no arguments typed yet) - mobile keyboards
			// often lack arrow keys.
			if len(matches) > 0 && len(msg.Runes) == 1 && !strings.ContainsAny(a.input, " \t") {
				switch msg.Runes[0] {
				case 'k':
					a.paletteIdx = (a.paletteIdx - 1 + len(matches)) % len(matches)
					return a, nil
				case 'j':
					a.paletteIdx = (a.paletteIdx + 1) % len(matches)
					return a, nil
				}
			}
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
	a.toolIter = 1
	a.pendingCalls = nil
	a.fragTools = nil
	a.confirmCall = nil
	a.pickingModel = false
	a.loadingModel = false
	a.modelReq++

	return a.streamLLM()
}

// denyConfirm records a user-rejected tool call as a tool message so the
// model sees the refusal and can adapt.
func (a *App) denyConfirm() {
	if a.confirmCall == nil {
		return
	}
	call := *a.confirmCall
	a.confirmCall = nil
	a.messages = append(a.messages, types.Message{
		Role:       "tool",
		ToolCallID: call.ID,
		Name:       call.Function.Name,
		Content:    "Denied by user - do not retry this call, work around it or ask.",
	})
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
		a.openProviderPicker()
	case "/provider":
		if len(parts) > 1 {
			if err := a.llmManager.SetDefaultProvider(parts[1]); err != nil {
				a.addSystemMessage(err.Error())
			} else {
				name := a.llmManager.DefaultProviderName()
				a.addSystemMessage(fmt.Sprintf("Switched provider to %s (model: %s)%s", name, a.llmManager.ProviderModel(parts[1]), a.keyNote(name)))
			}
		} else {
			a.openProviderPicker()
		}
	case "/tools":
		var toolNames []string
		for _, t := range a.toolReg.List() {
			toolNames = append(toolNames, t.Name())
		}
		a.addSystemMessage(fmt.Sprintf("Available tools: %s", strings.Join(toolNames, ", ")))
	case "/clear":
		a.messages = []types.Message{}
	case "/config":
		a.addSystemMessage(fmt.Sprintf("Active provider: %s, providers: %d, tools: %d", a.llmManager.DefaultProviderName(), len(a.llmManager.ListProviders()), len(a.toolReg.List())))
	case "/status":
		active := a.llmManager.DefaultProviderName()
		a.addSystemMessage(fmt.Sprintf("terbash %s\nprovider: %s (model: %s)\nproviders: %d, tools: %d", Version, active, a.llmManager.ProviderModel(active), len(a.llmManager.ListProviders()), len(a.toolReg.List())))
	case "/version":
		a.addSystemMessage(fmt.Sprintf("terbash %s", Version))
	case "/model":
		active := a.llmManager.DefaultProviderName()
		if len(parts) > 1 {
			a.llmManager.SetActiveModel(parts[1])
			a.addSystemMessage(fmt.Sprintf("Model for %s set to %s (this session)", active, strings.TrimSpace(parts[1])))
		} else {
			a.addSystemMessage(fmt.Sprintf("Model for %s: %s (usage: /model <name>, or /models to pick)", active, a.llmManager.ProviderModel(active)))
		}
	case "/models":
		return a.openModelPicker()
	case "/key":
		a.handleKeyCommand(parts)
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

// handleKeyCommand implements /key: with "<provider> <api-key>" it saves
// the key (live, no restart); bare "/key" lists which providers have one.
// Values are never printed - only set/missing.
func (a *App) handleKeyCommand(parts []string) {
	if len(parts) >= 3 {
		name := strings.ToLower(strings.TrimSpace(parts[1]))
		key := strings.TrimSpace(strings.Join(parts[2:], ""))
		if key == "" {
			a.addSystemMessage("API key must not be empty.")
			return
		}
		if err := a.llmManager.SetProviderAPIKey(name, key); err != nil {
			a.addSystemMessage(err.Error())
			return
		}
		if path, err := config.GetConfigPath(); err == nil {
			_ = config.SetProviderAPIKey(path, types.Provider(name), key)
		}
		a.addSystemMessage(fmt.Sprintf("API key for %s saved and active.", name))
		return
	}
	if len(parts) == 2 {
		a.addSystemMessage("Usage: /key <provider> <api-key>  (key is applied immediately)")
		return
	}
	var b strings.Builder
	b.WriteString("API keys (values never shown):\n")
	for _, n := range a.llmManager.AllProviderNames() {
		p := types.Provider(n)
		if !types.RequiresKey(p) {
			fmt.Fprintf(&b, "  %s - local, no key needed\n", n)
			continue
		}
		status := "missing"
		if a.llmManager.EffectiveAPIKey(n) != "" {
			status = "set"
		}
		fmt.Fprintf(&b, "  %s - %s\n", n, status)
	}
	a.addSystemMessage(strings.TrimRight(b.String(), "\n"))
}

// openModelPicker fetches the active provider's models in the background
// and opens the picker when they arrive.
func (a *App) openModelPicker() tea.Cmd {
	a.pickingModel = false
	a.loadingModel = true
	a.modelReq++
	req := a.modelReq
	a.modelNames = nil
	a.modelIdx = 0
	a.modelFor = a.llmManager.DefaultProviderName()
	return func() tea.Msg {
		p, err := a.llmManager.GetProvider(a.modelFor)
		if err != nil {
			return modelsMsg{req: req, err: err}
		}
		models, err := p.GetModels()
		return modelsMsg{req: req, models: models, err: err}
	}
}

// chooseModel applies the highlighted model to the active provider.
func (a *App) chooseModel() {
	a.pickingModel = false
	if len(a.modelNames) == 0 {
		return
	}
	if a.modelIdx < 0 || a.modelIdx >= len(a.modelNames) {
		a.modelIdx = 0
	}
	name := a.modelNames[a.modelIdx]
	a.llmManager.SetActiveModel(name)
	a.addSystemMessage(fmt.Sprintf("Model for %s set to %s (this session)", a.modelFor, name))
}

// openProviderPicker shows every selectable provider (configured plus all
// built-ins), preselecting the currently active one.
func (a *App) openProviderPicker() {
	names := a.llmManager.AllProviderNames()
	a.providerIdx = 0
	for i, n := range names {
		if n == a.llmManager.DefaultProviderName() {
			a.providerIdx = i
			break
		}
	}
	a.pickingProvider = true
}

// pickerModel returns the model shown for a picker row: the configured one,
// the built-in default, or "-" when setup is needed.
func (a *App) pickerModel(name string) string {
	if a.llmManager.IsConfigured(name) {
		if m := a.llmManager.ProviderModel(name); m != "" {
			return m
		}
	}
	if m, ok := types.DefaultModel(types.Provider(name)); ok {
		return m
	}
	return "- (needs setup)"
}

// chooseProvider switches to the highlighted provider and closes the picker.
// Picking a provider that is not configured yet scaffolds it (in memory and,
// best-effort, in the config file) so it works right away.
func (a *App) chooseProvider() {
	names := a.llmManager.AllProviderNames()
	a.pickingProvider = false
	if len(names) == 0 {
		return
	}
	if a.providerIdx < 0 || a.providerIdx >= len(names) {
		a.providerIdx = 0
	}
	name := names[a.providerIdx]
	note := ""
	if !a.llmManager.IsConfigured(name) {
		model, _ := types.DefaultModel(types.Provider(name))
		entry := types.ProviderConfig{Model: model, Temperature: 0.7, MaxTokens: 4096}
		if err := a.llmManager.EnsureProvider(name); err != nil {
			a.addSystemMessage(err.Error())
			return
		}
		if path, err := config.GetConfigPath(); err == nil {
			if err := config.EnsureProviderEntry(path, types.Provider(name), entry); err != nil {
				note = " (session only - could not save to config)"
			} else {
				note = " (saved to config)"
			}
		}
		if key := types.EnvKey(types.Provider(name)); key != "" {
			note += fmt.Sprintf(" - set %s or add api_key", key)
		}
	}
	if err := a.llmManager.SetDefaultProvider(name); err != nil {
		a.addSystemMessage(err.Error())
		return
	}
	model := a.llmManager.ProviderModel(name)
	if model != "" {
		a.addSystemMessage(fmt.Sprintf("Switched provider to %s (model: %s)%s%s", name, model, note, a.keyNote(name)))
	} else {
		a.addSystemMessage(fmt.Sprintf("Switched provider to %s%s%s", name, note, a.keyNote(name)))
	}
}

// keyNote tells the user how to add a missing API key, or "" when the
// provider is ready (key in config/env) or needs none (local providers).
func (a *App) keyNote(name string) string {
	p := types.Provider(name)
	if !types.RequiresKey(p) || a.llmManager.EffectiveAPIKey(name) != "" {
		return ""
	}
	if key := types.EnvKey(p); key != "" {
		return fmt.Sprintf(" — no API key: exit chat and run `terbash config set-key %s` (or set %s)", name, key)
	}
	return fmt.Sprintf(" — no API key: exit chat and run `terbash config set-key %s`", name)
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

		return waitChunkMsg{stream: stream}
	}
}

// waitForChunk reads one streamed chunk, then yields back so the UI stays
// live. The stream ends as streamDoneMsg.
func waitForChunk(stream <-chan types.StreamChunk) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-stream
		if !ok {
			return streamDoneMsg{}
		}
		return streamChunkMsg{chunk: chunk, stream: stream}
	}
}

func (a *App) handleStreamChunk(msg streamChunkMsg) (tea.Model, tea.Cmd) {
	for _, choice := range msg.chunk.Choices {
		if choice.Delta.Content != "" {
			a.streamBuf.WriteString(choice.Delta.Content)
		}
		for _, tc := range choice.Delta.ToolCalls {
			mergeToolFrag(&a.fragTools, tc)
		}
	}
	return a, waitForChunk(msg.stream)
}

// finishStream closes one LLM turn: records the assistant message and,
// when the model asked for tools, queues them instead of ending the turn.
func (a *App) finishStream() (tea.Model, tea.Cmd) {
	calls := fragsToCalls(a.fragTools)
	a.fragTools = nil
	content := a.streamBuf.String()
	if content != "" || len(calls) > 0 {
		a.messages = append(a.messages, types.Message{
			Role:      "assistant",
			Content:   content,
			ToolCalls: calls,
		})
	}
	if len(calls) == 0 {
		a.streaming = false
		a.toolIter = 0
		return a, nil
	}
	if a.toolIter >= maxToolIters {
		a.addSystemMessage(fmt.Sprintf("Stopped after %d tool rounds.", maxToolIters))
		a.streaming = false
		a.toolIter = 0
		return a, nil
	}
	a.pendingCalls = append([]types.ToolCall(nil), calls...)
	return a, a.driveToolQueue()
}

// driveToolQueue runs the next queued tool call: approval overlay first
// when the tool declares it needs one, background execution otherwise.
// An empty queue means all results are in - feed them back to the model.
func (a *App) driveToolQueue() tea.Cmd {
	if len(a.pendingCalls) == 0 {
		a.toolIter++
		return a.streamLLM()
	}
	call := a.pendingCalls[0]
	a.pendingCalls = a.pendingCalls[1:]
	if ok, prompt := toolNeedsConfirm(a.toolReg, call); ok {
		c := call
		a.confirmCall = &c
		a.confirmPrompt = prompt
		return nil
	}
	return a.execToolCmd(call)
}

func (a *App) execToolCmd(call types.ToolCall) tea.Cmd {
	return func() tea.Msg {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			args = map[string]interface{}{}
		}
		res, err := a.toolReg.Execute(call.Function.Name, args)
		if err != nil {
			res = &types.ToolResult{Success: false, Error: err.Error()}
		}
		return toolResultMsg{call: call, result: res}
	}
}

func (a *App) handleToolResult(msg toolResultMsg) (tea.Model, tea.Cmd) {
	content := msg.result.Output
	if !msg.result.Success {
		if msg.result.Error != "" {
			content = "Error: " + msg.result.Error
		} else if content == "" {
			content = "Error: tool failed with no output."
		}
	}
	a.messages = append(a.messages, types.Message{
		Role:       "tool",
		ToolCallID: msg.call.ID,
		Name:       msg.call.Function.Name,
		Content:    content,
	})
	return a, a.driveToolQueue()
}

// toolNeedsConfirm parses a streamed call and asks the tool whether these
// args need approval. Unknown tools and unparseable args fail closed: the
// former is reported by Execute, the latter needs a human look.
func toolNeedsConfirm(reg *tools.Registry, call types.ToolCall) (bool, string) {
	tool, ok := reg.Get(call.Function.Name)
	if !ok {
		return false, ""
	}
	checker, ok := tool.(types.ConfirmChecker)
	if !ok {
		return false, ""
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return true, fmt.Sprintf("Run %s with unparseable args?", call.Function.Name)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	return checker.RequiresConfirm(args)
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

	// Tool approval overlay: blocks the input box until answered.
	if a.confirmCall != nil {
		b.WriteString(a.styles.ToolMsg.Render(fmt.Sprintf("🔧 %s [y/N]", a.confirmPrompt)))
		b.WriteString("\n")
		b.WriteString(a.styles.Help.Render("y: allow once • n / Enter / Esc: deny"))
		b.WriteString("\n\n")
	} else if a.streaming && (len(a.pendingCalls) > 0 || a.toolIter > 1) {
		b.WriteString(a.styles.ToolMsg.Render("🔧 running tools…"))
		b.WriteString("\n\n")
	}

	inputView := a.styles.Input.Render(a.input[:a.cursor] + " " + a.input[a.cursor:])
	b.WriteString(inputView)
	b.WriteString("\n")

	// Model loading indicator while the provider is queried.
	if a.loadingModel {
		b.WriteString(a.styles.ToolMsg.Render(fmt.Sprintf("… loading models for %s", a.modelFor)))
		b.WriteString("\n")
		b.WriteString(a.bottomBar("Esc: cancel"))
		return b.String()
	}

	// Model picker overlay under the input box.
	if a.pickingModel {
		names := a.modelNames
		if a.modelIdx < 0 || (len(names) > 0 && a.modelIdx >= len(names)) {
			a.modelIdx = 0
		}
		const maxShow = 8
		start, end := windowSelection(len(names), a.modelIdx, maxShow)
		if start > 0 {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d above", start)))
			b.WriteString("\n")
		}
		current := a.llmManager.ProviderModel(a.modelFor)
		for i, m := range names[start:end] {
			line := "  " + m
			if m == current {
				line += "  (active)"
			}
			if start+i == a.modelIdx {
				b.WriteString(a.styles.PaletteSel.Render("›" + line))
			} else {
				b.WriteString(a.styles.PaletteNorm.Render(" " + line))
			}
			b.WriteString("\n")
		}
		if end < len(names) {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d more", len(names)-end)))
			b.WriteString("\n")
		}
		b.WriteString(a.bottomBar(fmt.Sprintf("Models for %s • ↑↓jk: move • 1-9/Enter: use • Esc: cancel", a.modelFor)))
		return b.String()
	}

	// Provider picker overlay: every selectable provider under the input.
	if a.pickingProvider {
		names := a.llmManager.AllProviderNames()
		if a.providerIdx < 0 || (len(names) > 0 && a.providerIdx >= len(names)) {
			a.providerIdx = 0
		}
		const maxShow = 8
		start, end := windowSelection(len(names), a.providerIdx, maxShow)
		if start > 0 {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d above", start)))
			b.WriteString("\n")
		}
		active := a.llmManager.DefaultProviderName()
		for i, n := range names[start:end] {
			marker := "+"
			if n == active {
				marker = "●"
			} else if a.llmManager.IsConfigured(n) {
				marker = "○"
			}
			line := fmt.Sprintf("  %s  %s", n, a.pickerModel(n))
			if n == active {
				line += "  (active)"
			}
			if start+i == a.providerIdx {
				b.WriteString(a.styles.PaletteSel.Render("› "+marker+line))
			} else {
				b.WriteString(a.styles.PaletteNorm.Render("  "+marker+line))
			}
			b.WriteString("\n")
		}
		if end < len(names) {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d more", len(names)-end)))
			b.WriteString("\n")
		}
		b.WriteString(a.bottomBar("↑↓jk: move • 1-9/Enter: use • Esc: cancel • ● active ○ ready + setup needed"))
		return b.String()
	}

	// "/" command palette: live-filtered list under the input box.
	if matches := paletteMatches(a.input); len(matches) > 0 {
		if a.paletteIdx < 0 || a.paletteIdx >= len(matches) {
			a.paletteIdx = 0
		}
		const maxShow = 8
		start, end := windowSelection(len(matches), a.paletteIdx, maxShow)
		if start > 0 {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d above", start)))
			b.WriteString("\n")
		}
		for i, c := range matches[start:end] {
			line := fmt.Sprintf("  %s  %s", c.name, c.desc)
			if start+i == a.paletteIdx {
				b.WriteString(a.styles.PaletteSel.Render("›"+line))
			} else {
				b.WriteString(a.styles.PaletteNorm.Render(" "+line))
			}
			b.WriteString("\n")
		}
		if end < len(matches) {
			b.WriteString(a.styles.PaletteNorm.Render(fmt.Sprintf("  … +%d more", len(matches)-end)))
			b.WriteString("\n")
		}
		b.WriteString(a.bottomBar("↑↓jk: move • Tab/Enter: complete • Esc: close"))
	} else {
		b.WriteString(a.bottomBar("Enter: send • Ctrl+C: quit • /: commands"))
	}

	return b.String()
}

type waitChunkMsg struct {
	stream <-chan types.StreamChunk
}

type streamChunkMsg struct {
	chunk  types.StreamChunk
	stream <-chan types.StreamChunk
}

type streamDoneMsg struct{}

type streamErrorMsg struct {
	err error
}

type toolResultMsg struct {
	call   types.ToolCall
	result *types.ToolResult
}

type modelsMsg struct {
	req    int
	models []string
	err    error
}

func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}