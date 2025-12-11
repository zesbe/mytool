package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

var (
	version   = "3.1.0"
	buildTime = time.Now().Format("2006-01-02")
)

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorPurple  = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorItalic  = "\033[3m"
	clearLine    = "\033[2K\r"
	cursorUp     = "\033[1A"
	saveCursor   = "\033[s"
	restoreCursor = "\033[u"
)

const minimaxAPIURL = "https://api.minimax.io/v1/chat/completions"
const modelName = "MiniMax-Text-01"
const maxContextTokens = 128000
const costPer1KTokens = 0.0001 // approximate cost

const (
	ModeAuto   = "auto"
	ModeAsk    = "ask"
	ModeManual = "manual"
)

var (
	currentMode     = ModeAuto
	currentDir      string
	undoStack       []UndoAction
	totalTokens     int
	totalCost       float64
	sessionID       string
	projectType     string
	lastResponse    string
	isThinking      bool
	thinkingFrames  = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	memory          = make(map[string]string)
	chatExportFile  string
	settings        Settings
	mcpServers      []MCPServer
)

// Settings structure
type Settings struct {
	Model             string `json:"model"`
	ReasoningLevel    string `json:"reasoning_level"`
	DiffDisplayMode   string `json:"diff_display_mode"`
	TodoDisplayMode   string `json:"todo_display_mode"`
	CloudSync         bool   `json:"cloud_sync"`
	ShowThinking      bool   `json:"show_thinking"`
	PlaySounds        bool   `json:"play_sounds"`
	CompletionSound   string `json:"completion_sound"`
	AllowBackground   bool   `json:"allow_background"`
	CustomDroids      bool   `json:"custom_droids"`
}

// MCP Server structure  
type MCPServer struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
	Tools     []string `json:"tools"`
}

type UndoAction struct {
	Type    string
	Path    string
	Content string
	Time    time.Time
}

type StreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type StreamResponse struct {
	Choices []StreamChoice `json:"choices"`
	Usage   struct {
		TotalTokens  int `json:"total_tokens"`
		PromptTokens int `json:"prompt_tokens"`
	} `json:"usage"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type Session struct {
	ID       string            `json:"id"`
	Dir      string            `json:"dir"`
	Mode     string            `json:"mode"`
	History  []ChatMessage     `json:"history"`
	Tokens   int               `json:"tokens"`
	Cost     float64           `json:"cost"`
	Memory   map[string]string `json:"memory"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
}

type Memory struct {
	Facts map[string]string `json:"facts"`
}

func main() {
	currentDir, _ = os.Getwd()
	sessionID = generateSessionID()
	detectProject()
	loadMemory()
	loadSettings()
	loadMCPServers()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("\n%s👋 Interrupted%s\n", colorYellow, colorReset)
		saveMemory()
		os.Exit(0)
	}()

	args := os.Args[1:]
	if len(args) > 0 && (strings.HasSuffix(args[0], "/mytool") || strings.HasSuffix(args[0], "\\mytool.exe")) {
		args = args[1:]
	}

	if len(args) < 1 {
		runChat([]string{})
		return
	}

	switch args[0] {
	case "version", "-v", "--version":
		printVersion()
	case "help", "-h", "--help":
		printHelp()
	case "resume":
		resumeSession()
	case "sessions":
		listSessions()
	case "export":
		if len(args) > 1 {
			exportChat(args[1])
		} else {
			exportChat("")
		}
	case "memory":
		showMemory()
	default:
		runChat(args)
	}
}

func generateSessionID() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s-%d", currentDir, time.Now().UnixNano()))))[:8]
}

func detectProject() {
	projectType = ""
	checks := map[string]string{
		"package.json": "nodejs", "go.mod": "go", "Cargo.toml": "rust",
		"requirements.txt": "python", "pom.xml": "java", "composer.json": "php",
		"Gemfile": "ruby", "pubspec.yaml": "flutter", "CMakeLists.txt": "cpp",
		"Makefile": "make", "docker-compose.yml": "docker",
	}
	for file, ptype := range checks {
		if _, err := os.Stat(filepath.Join(currentDir, file)); err == nil {
			projectType = ptype
			return
		}
	}
	if _, err := os.Stat(filepath.Join(currentDir, ".git")); err == nil {
		projectType = "git"
	}
}

// ==================== UI ====================

func printBanner() {
	fmt.Print("\033[H\033[2J") // Clear screen
	banner := `%s
    ███╗   ███╗██╗   ██╗████████╗ ██████╗  ██████╗ ██╗     
    ████╗ ████║╚██╗ ██╔╝╚══██╔══╝██╔═══██╗██╔═══██╗██║     
    ██╔████╔██║ ╚████╔╝    ██║   ██║   ██║██║   ██║██║     
    ██║╚██╔╝██║  ╚██╔╝     ██║   ██║   ██║██║   ██║██║     
    ██║ ╚═╝ ██║   ██║      ██║   ╚██████╔╝╚██████╔╝███████╗
    ╚═╝     ╚═╝   ╚═╝      ╚═╝    ╚═════╝  ╚═════╝ ╚══════╝%s
                                              %sv%s%s
`
	fmt.Printf(banner, colorCyan, colorReset, colorGray, version, colorReset)
}

func printHelp() {
	fmt.Printf(`
%smytool%s v%s - AI Terminal Assistant (Full Featured)

%sUSAGE%s
  mytool              Start interactive chat
  mytool "message"    Send single message
  mytool resume       Resume last session
  mytool sessions     List all sessions
  mytool export [f]   Export chat to file
  mytool memory       Show AI memory

%sFEATURES%s
  ✓ Full system access (read/write/execute)
  ✓ Git integration
  ✓ Web search & URL fetch
  ✓ Image analysis
  ✓ Code execution (Python/JS/Shell)
  ✓ Syntax highlighting
  ✓ Session save/resume
  ✓ Persistent memory
  ✓ Undo support
  ✓ Cost tracking
  ✓ Context window display
  ✓ Export conversations
  ✓ Multiple sessions

%sCOMMANDS%s
  /mode         Toggle mode (auto/ask/manual)
  /undo         Undo last file change
  /save         Save current session
  /export [f]   Export chat to file
  /copy         Copy last response
  /memory       Show/manage memory
  /forget <k>   Forget memory item
  /remember     Remember something
  /sessions     List sessions
  /clear        Clear history
  /context      Show context usage
  /cost         Show API cost
  /run <cmd>    Run shell command
  /python <c>   Run Python code
  /node <c>     Run JavaScript
  /git <cmd>    Git command
  /search <q>   Web search
  /read <f>     Read file
  /edit <f>     Edit file
  /ls [d]       List directory
  /find <n>     Find files
  /grep <p>     Search in files
  /img <f>      Analyze image
  /help         This help
  exit          Quit

%sSHORTCUTS%s
  @file         Include file content
  \             Multi-line input
  Ctrl+C        Cancel/Exit

`, colorCyan, colorReset, version,
		colorYellow, colorReset, colorYellow, colorReset,
		colorYellow, colorReset, colorYellow, colorReset)
}

func printVersion() {
	fmt.Printf(`%smytool%s v%s
  Model:    %s
  OS:       %s/%s
  Project:  %s
  Session:  %s
  Build:    %s
`, colorCyan, colorReset, version, modelName, runtime.GOOS, runtime.GOARCH,
		projectType, sessionID, buildTime)
}

func printStatusBar() {
	mode := getModeDisplay()
	tokens := fmt.Sprintf("%d/%dk", totalTokens/1000, maxContextTokens/1000)
	cost := fmt.Sprintf("$%.4f", totalCost)
	
	proj := ""
	if projectType != "" {
		proj = fmt.Sprintf("[%s]", projectType)
	}
	
	git := ""
	if branch := getGitBranch(); branch != "" {
		git = fmt.Sprintf("⎇ %s", branch)
	}
	
	bar := fmt.Sprintf("%s │ %s%s │ %s%s │ %s │ %s",
		mode, colorGray, tokens, cost, colorReset, currentDir, proj)
	if git != "" {
		bar += fmt.Sprintf(" %s%s%s", colorBlue, git, colorReset)
	}
	fmt.Println(bar)
}

func getModeDisplay() string {
	switch currentMode {
	case ModeAuto:
		return fmt.Sprintf("%s●Auto%s", colorGreen, colorReset)
	case ModeAsk:
		return fmt.Sprintf("%s●Ask%s", colorYellow, colorReset)
	case ModeManual:
		return fmt.Sprintf("%s●Manual%s", colorRed, colorReset)
	}
	return ""
}

func getModeColor() string {
	switch currentMode {
	case ModeAuto:
		return colorGreen
	case ModeAsk:
		return colorYellow
	default:
		return colorRed
	}
}

func cycleMode() {
	switch currentMode {
	case ModeAuto:
		currentMode = ModeAsk
	case ModeAsk:
		currentMode = ModeManual
	case ModeManual:
		currentMode = ModeAuto
	}
}

func showThinking() {
	isThinking = true
	go func() {
		i := 0
		for isThinking {
			fmt.Printf("\r%s%s Thinking...%s", colorGray, thinkingFrames[i%len(thinkingFrames)], colorReset)
			time.Sleep(80 * time.Millisecond)
			i++
		}
		fmt.Printf("\r%s\r", clearLine)
	}()
}

func stopThinking() {
	isThinking = false
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("\r%s\r", clearLine)
}

func showProgress(msg string, current, total int) {
	pct := float64(current) / float64(total) * 100
	bar := int(pct / 5)
	fmt.Printf("\r%s%s [%s%s] %.0f%%%s",
		colorGray, msg,
		strings.Repeat("█", bar),
		strings.Repeat("░", 20-bar),
		pct, colorReset)
}

// ==================== SYNTAX HIGHLIGHTING ====================

func highlightCode(code, lang string) string {
	keywords := map[string][]string{
		"go":     {"func", "return", "if", "else", "for", "range", "var", "const", "type", "struct", "interface", "package", "import", "defer", "go", "chan", "select", "case", "default", "switch", "break", "continue"},
		"python": {"def", "return", "if", "else", "elif", "for", "while", "in", "import", "from", "class", "try", "except", "finally", "with", "as", "yield", "lambda", "pass", "break", "continue", "True", "False", "None"},
		"js":     {"function", "return", "if", "else", "for", "while", "var", "let", "const", "class", "import", "export", "from", "try", "catch", "finally", "async", "await", "new", "this", "true", "false", "null", "undefined"},
	}

	kw, ok := keywords[lang]
	if !ok {
		return code
	}

	result := code
	for _, k := range kw {
		re := regexp.MustCompile(`\b(` + k + `)\b`)
		result = re.ReplaceAllString(result, colorPurple+"$1"+colorReset)
	}

	// Strings
	result = regexp.MustCompile(`"([^"]*)"'`).ReplaceAllString(result, colorGreen+`"$1"`+colorReset)
	result = regexp.MustCompile(`'([^']*)'`).ReplaceAllString(result, colorGreen+`'$1'`+colorReset)

	// Comments
	result = regexp.MustCompile(`(//.*)`).ReplaceAllString(result, colorGray+"$1"+colorReset)
	result = regexp.MustCompile(`(#.*)`).ReplaceAllString(result, colorGray+"$1"+colorReset)

	return result
}

func formatCodeBlock(code, lang string) string {
	lines := strings.Split(code, "\n")
	var result strings.Builder
	
	result.WriteString(fmt.Sprintf("%s┌─ %s ─%s\n", colorGray, lang, colorReset))
	for i, line := range lines {
		hl := highlightCode(line, lang)
		result.WriteString(fmt.Sprintf("%s│%3d%s %s\n", colorGray, i+1, colorReset, hl))
	}
	result.WriteString(fmt.Sprintf("%s└─────%s\n", colorGray, colorReset))
	
	return result.String()
}

// ==================== MEMORY ====================

func loadMemory() {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".mytool", "memory.json"))
	if err != nil {
		return
	}
	json.Unmarshal(data, &memory)
}

func saveMemory() {
	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".mytool"), 0755)
	data, _ := json.MarshalIndent(memory, "", "  ")
	os.WriteFile(filepath.Join(home, ".mytool", "memory.json"), data, 0644)
}

func showMemory() {
	if len(memory) == 0 {
		fmt.Println("No memories stored")
		return
	}
	fmt.Printf("%sMemory (%d items):%s\n", colorCyan, len(memory), colorReset)
	for k, v := range memory {
		fmt.Printf("  %s%s%s: %s\n", colorYellow, k, colorReset, truncate(v, 50))
	}
}

func rememberFact(key, value string) {
	memory[key] = value
	saveMemory()
}

func forgetFact(key string) {
	delete(memory, key)
	saveMemory()
}

// ==================== SETTINGS ====================

func loadSettings() {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".mytool", "settings.json"))
	if err != nil {
		// Default settings
		settings = Settings{
			Model:           modelName,
			ReasoningLevel:  "High",
			DiffDisplayMode: "GitHub",
			TodoDisplayMode: "In message flow",
			CloudSync:       false,
			ShowThinking:    true,
			PlaySounds:      false,
			CompletionSound: "FX-OK01",
			AllowBackground: true,
			CustomDroids:    true,
		}
		return
	}
	json.Unmarshal(data, &settings)
}

func saveSettings() {
	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".mytool"), 0755)
	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(filepath.Join(home, ".mytool", "settings.json"), data, 0644)
}

func showSettings(scanner *bufio.Scanner) {
	for {
		options := []string{
			fmt.Sprintf("Model: %s", settings.Model),
			fmt.Sprintf("Reasoning: %s", settings.ReasoningLevel),
			fmt.Sprintf("Diff mode: %s", settings.DiffDisplayMode),
			fmt.Sprintf("Todo mode: %s", settings.TodoDisplayMode),
			fmt.Sprintf("Cloud sync: %s", boolToStr(settings.CloudSync)),
			fmt.Sprintf("Show thinking: %s", boolToStr(settings.ShowThinking)),
			fmt.Sprintf("Play sounds: %s", boolToStr(settings.PlaySounds)),
			fmt.Sprintf("Allow background: %s", boolToStr(settings.AllowBackground)),
			fmt.Sprintf("Custom droids: %s", boolToStr(settings.CustomDroids)),
			"← Back to chat",
		}
		
		choice := selectMenu("⚙️  Settings", options, 0)
		
		if choice == -1 || choice == len(options)-1 {
			saveSettings()
			return
		}
		
		switch choice {
		case 0: // Model
			fmt.Print("\033[H\033[2J")
			fmt.Printf("Current model: %s\n", settings.Model)
			fmt.Printf("Enter new model name (or press Enter to cancel): ")
			
			// Restore terminal for input
			if scanner.Scan() {
				if name := strings.TrimSpace(scanner.Text()); name != "" {
					settings.Model = name
				}
			}
		case 1: // Reasoning
			levels := []string{"High", "Medium", "Low", "← Back"}
			idx := selectMenu("Reasoning Level", levels, 0)
			if idx >= 0 && idx < 3 {
				settings.ReasoningLevel = levels[idx]
			}
		case 2: // Diff mode
			modes := []string{"GitHub", "Unified", "← Back"}
			idx := selectMenu("Diff Display Mode", modes, 0)
			if idx >= 0 && idx < 2 {
				settings.DiffDisplayMode = modes[idx]
			}
		case 3: // Todo mode
			modes := []string{"In message flow", "Sidebar", "← Back"}
			idx := selectMenu("Todo Display Mode", modes, 0)
			if idx >= 0 && idx < 2 {
				settings.TodoDisplayMode = modes[idx]
			}
		case 4:
			settings.CloudSync = !settings.CloudSync
		case 5:
			settings.ShowThinking = !settings.ShowThinking
		case 6:
			settings.PlaySounds = !settings.PlaySounds
		case 7:
			settings.AllowBackground = !settings.AllowBackground
		case 8:
			settings.CustomDroids = !settings.CustomDroids
		}
		saveSettings()
	}
}

func boolToStr(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

// Interactive menu with arrow keys
func selectMenu(title string, options []string, selected int) int {
	// Save terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return -1
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	cursor := selected
	if cursor < 0 || cursor >= len(options) {
		cursor = 0
	}

	for {
		// Clear and draw menu
		fmt.Print("\033[H\033[2J") // Clear screen
		fmt.Printf("%s%s%s\n\n", colorCyan, title, colorReset)
		
		for i, opt := range options {
			if i == cursor {
				fmt.Printf("  %s> %s%s\n", colorGreen, opt, colorReset)
			} else {
				fmt.Printf("    %s\n", opt)
			}
		}
		
		fmt.Printf("\n%s↑↓ Navigate • Enter Select • q Quit%s", colorGray, colorReset)

		// Read key
		buf := make([]byte, 3)
		n, _ := os.Stdin.Read(buf)
		
		if n == 1 {
			switch buf[0] {
			case 'q', 'Q', 27: // q or ESC
				return -1
			case 13, 10: // Enter
				return cursor
			case 'j', 'J': // vim down
				cursor = (cursor + 1) % len(options)
			case 'k', 'K': // vim up
				cursor = (cursor - 1 + len(options)) % len(options)
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65: // Up arrow
				cursor = (cursor - 1 + len(options)) % len(options)
			case 66: // Down arrow
				cursor = (cursor + 1) % len(options)
			}
		}
	}
}

func boolToOnOff(b bool) string {
	if b {
		return fmt.Sprintf("%sOn%s", colorGreen, colorReset)
	}
	return fmt.Sprintf("%sOff%s", colorRed, colorReset)
}

// ==================== MCP SERVERS ====================

func loadMCPServers() {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".mytool", "mcp_servers.json"))
	if err != nil {
		// Default MCP servers
		mcpServers = []MCPServer{
			{Name: "browser-use", URL: "localhost:3000", Type: "browser", Connected: false, Tools: []string{"browse", "click", "type", "screenshot"}},
			{Name: "context7", URL: "localhost:3001", Type: "context", Connected: false, Tools: []string{"search_docs", "get_context"}},
		}
		return
	}
	json.Unmarshal(data, &mcpServers)
}

func saveMCPServers() {
	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".mytool"), 0755)
	data, _ := json.MarshalIndent(mcpServers, "", "  ")
	os.WriteFile(filepath.Join(home, ".mytool", "mcp_servers.json"), data, 0644)
}

func showMCPServers(scanner *bufio.Scanner) {
	for {
		// Build options list
		options := []string{}
		for _, server := range mcpServers {
			status := "○"
			if server.Connected {
				status = "●"
			}
			options = append(options, fmt.Sprintf("%s %s", status, server.Name))
		}
		options = append(options, "+ Add MCP server")
		options = append(options, "← Back to chat")
		
		choice := selectMenu("🔌 MCP Servers", options, 0)
		
		if choice == -1 || choice == len(options)-1 {
			return
		}
		
		// Add new server
		if choice == len(options)-2 {
			fmt.Print("\033[H\033[2J")
			fmt.Printf("%s=== Add MCP Server ===%s\n\n", colorCyan, colorReset)
			
			fmt.Printf("Server name: ")
			if !scanner.Scan() {
				return
			}
			name := strings.TrimSpace(scanner.Text())
			if name == "" {
				continue
			}
			
			fmt.Printf("Server URL: ")
			if !scanner.Scan() {
				return
			}
			url := strings.TrimSpace(scanner.Text())
			if url == "" {
				continue
			}
			
			mcpServers = append(mcpServers, MCPServer{
				Name:      name,
				URL:       url,
				Type:      "custom",
				Connected: false,
				Tools:     []string{},
			})
			saveMCPServers()
			continue
		}
		
		// Toggle or manage existing server
		if choice >= 0 && choice < len(mcpServers) {
			serverIdx := choice
			actions := []string{
				"Toggle connection",
				"Delete server",
				"← Back",
			}
			
			actionChoice := selectMenu(mcpServers[serverIdx].Name, actions, 0)
			
			switch actionChoice {
			case 0: // Toggle
				mcpServers[serverIdx].Connected = !mcpServers[serverIdx].Connected
				saveMCPServers()
			case 1: // Delete
				confirm := []string{"Yes, delete", "No, cancel"}
				if selectMenu("Delete "+mcpServers[serverIdx].Name+"?", confirm, 1) == 0 {
					mcpServers = append(mcpServers[:serverIdx], mcpServers[serverIdx+1:]...)
					saveMCPServers()
				}
			}
		}
	}
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func getMCPTools() []string {
	var tools []string
	for _, server := range mcpServers {
		if server.Connected {
			for _, tool := range server.Tools {
				tools = append(tools, fmt.Sprintf("%s:%s", server.Name, tool))
			}
		}
	}
	return tools
}

// ==================== SESSIONS ====================

func saveSession(history []ChatMessage) {
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".mytool", "sessions")
	os.MkdirAll(sessionDir, 0755)

	session := Session{
		ID:      sessionID,
		Dir:     currentDir,
		Mode:    currentMode,
		History: history,
		Tokens:  totalTokens,
		Cost:    totalCost,
		Memory:  memory,
		Updated: time.Now(),
	}

	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(filepath.Join(sessionDir, sessionID+".json"), data, 0644)
	fmt.Printf("%s✓ Session saved: %s%s\n", colorGreen, sessionID, colorReset)
}

func loadSession(id string) (*Session, error) {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".mytool", "sessions", id+".json"))
	if err != nil {
		return nil, err
	}
	var session Session
	json.Unmarshal(data, &session)
	return &session, nil
}

func resumeSession() {
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".mytool", "sessions")
	
	// Find most recent session for this directory
	entries, _ := os.ReadDir(sessionDir)
	var latest *Session
	var latestTime time.Time
	
	for _, e := range entries {
		if s, err := loadSession(strings.TrimSuffix(e.Name(), ".json")); err == nil {
			if s.Dir == currentDir && s.Updated.After(latestTime) {
				latest = s
				latestTime = s.Updated
			}
		}
	}
	
	if latest == nil {
		fmt.Printf("%sNo session found for this directory%s\n", colorYellow, colorReset)
		runChat([]string{})
		return
	}
	
	sessionID = latest.ID
	currentMode = latest.Mode
	totalTokens = latest.Tokens
	totalCost = latest.Cost
	memory = latest.Memory
	
	fmt.Printf("%s✓ Resumed: %s (%d msgs)%s\n", colorGreen, sessionID, len(latest.History), colorReset)
	runChatWithHistory(latest.History)
}

func listSessions() {
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".mytool", "sessions")
	entries, err := os.ReadDir(sessionDir)
	if err != nil || len(entries) == 0 {
		fmt.Println("No sessions found")
		return
	}
	
	fmt.Printf("%sSessions:%s\n", colorCyan, colorReset)
	for _, e := range entries {
		if s, err := loadSession(strings.TrimSuffix(e.Name(), ".json")); err == nil {
			age := time.Since(s.Updated).Round(time.Minute)
			fmt.Printf("  %s%s%s  %s  %d msgs  %s ago\n",
				colorYellow, s.ID, colorReset, truncate(s.Dir, 30), len(s.History), age)
		}
	}
}

// ==================== EXPORT ====================

func exportChat(filename string) {
	if filename == "" {
		filename = fmt.Sprintf("chat_%s_%s.md", sessionID, time.Now().Format("20060102_150405"))
	}
	
	if chatExportFile == "" {
		fmt.Printf("%sNo chat to export%s\n", colorYellow, colorReset)
		return
	}
	
	os.WriteFile(filename, []byte(chatExportFile), 0644)
	fmt.Printf("%s✓ Exported: %s%s\n", colorGreen, filename, colorReset)
}

func appendToExport(role, content string) {
	chatExportFile += fmt.Sprintf("\n## %s\n%s\n", role, content)
}

// ==================== CODE EXECUTION ====================

func runPython(code string) string {
	tmpFile := filepath.Join(os.TempDir(), "mytool_py.py")
	os.WriteFile(tmpFile, []byte(code), 0644)
	defer os.Remove(tmpFile)
	
	cmd := exec.Command("python3", tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s%s\n%s%s", string(output), colorRed, err, colorReset)
	}
	return string(output)
}

func runNode(code string) string {
	tmpFile := filepath.Join(os.TempDir(), "mytool_js.js")
	os.WriteFile(tmpFile, []byte(code), 0644)
	defer os.Remove(tmpFile)
	
	cmd := exec.Command("node", tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s%s\n%s%s", string(output), colorRed, err, colorReset)
	}
	return string(output)
}

// ==================== IMAGE ANALYSIS ====================

func analyzeImage(path string) string {
	fullPath := resolvePath(path)
	
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	
	// Check file size
	if len(data) > 5*1024*1024 {
		return "Error: Image too large (max 5MB)"
	}
	
	// Get mime type
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".gif": "image/gif", ".webp": "image/webp",
	}
	mime, ok := mimeTypes[ext]
	if !ok {
		return "Error: Unsupported image format"
	}
	
	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("Image loaded: %s (%s, %d bytes)\nBase64: %s...%s",
		fullPath, mime, len(data), b64[:50], b64[len(b64)-20:])
}

// ==================== WEB SEARCH ====================

func webSearch(query string) string {
	// Using DuckDuckGo instant answers API (free, no auth needed)
	url := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1", strings.ReplaceAll(query, " ", "+"))
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Sprintf("Search error: %s", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	
	var output strings.Builder
	output.WriteString(fmt.Sprintf("%sSearch: %s%s\n", colorCyan, query, colorReset))
	
	if abstract, ok := result["Abstract"].(string); ok && abstract != "" {
		output.WriteString(fmt.Sprintf("\n%s\n", abstract))
	}
	
	if relatedTopics, ok := result["RelatedTopics"].([]interface{}); ok {
		for i, topic := range relatedTopics {
			if i >= 5 {
				break
			}
			if t, ok := topic.(map[string]interface{}); ok {
				if text, ok := t["Text"].(string); ok {
					output.WriteString(fmt.Sprintf("• %s\n", truncate(text, 100)))
				}
			}
		}
	}
	
	return output.String()
}

// ==================== CLIPBOARD ====================

func copyToClipboard(text string) string {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return "Clipboard not supported on this OS"
	}
	
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	return fmt.Sprintf("%s✓ Copied to clipboard (%d chars)%s", colorGreen, len(text), colorReset)
}

// ==================== FILE OPERATIONS ====================

func saveForUndo(path, desc string) {
	fullPath := resolvePath(path)
	content := ""
	if data, err := os.ReadFile(fullPath); err == nil {
		content = string(data)
	}
	undoStack = append(undoStack, UndoAction{
		Type: "file", Path: fullPath, Content: content, Time: time.Now(),
	})
	if len(undoStack) > 20 {
		undoStack = undoStack[1:]
	}
}

func doUndo() string {
	if len(undoStack) == 0 {
		return "Nothing to undo"
	}
	action := undoStack[len(undoStack)-1]
	undoStack = undoStack[:len(undoStack)-1]
	
	if action.Content == "" {
		os.Remove(action.Path)
		return fmt.Sprintf("%s✓ Undone: removed %s%s", colorGreen, action.Path, colorReset)
	}
	os.WriteFile(action.Path, []byte(action.Content), 0644)
	return fmt.Sprintf("%s✓ Undone: restored %s%s", colorGreen, action.Path, colorReset)
}

func cmdRead(path string) string {
	if path == "" {
		return "Usage: /read <file>"
	}
	fullPath := resolvePath(path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	
	content := string(data)
	lines := strings.Split(content, "\n")
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s─── %s (%d lines) ───%s\n", colorCyan, fullPath, len(lines), colorReset))
	
	for i, line := range lines {
		if i >= 200 {
			result.WriteString(fmt.Sprintf("%s... +%d more lines%s\n", colorGray, len(lines)-200, colorReset))
			break
		}
		hl := highlightCode(line, ext)
		result.WriteString(fmt.Sprintf("%s%4d│%s %s\n", colorGray, i+1, colorReset, hl))
	}
	
	return result.String()
}

func cmdList(path string) string {
	if path == "" {
		path = currentDir
	} else {
		path = resolvePath(path)
	}
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s%s%s\n", colorCyan, path, colorReset))
	
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	
	for _, e := range dirs {
		result.WriteString(fmt.Sprintf("%s📁 %s/%s\n", colorBlue, e.Name(), colorReset))
	}
	for _, e := range files {
		info, _ := e.Info()
		size := ""
		if info != nil {
			size = formatSize(info.Size())
		}
		icon := getFileIcon(e.Name())
		result.WriteString(fmt.Sprintf("%s %-30s %s%s%s\n", icon, e.Name(), colorGray, size, colorReset))
	}
	
	result.WriteString(fmt.Sprintf("\n%s%d dirs, %d files%s", colorGray, len(dirs), len(files), colorReset))
	return result.String()
}

func getFileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	icons := map[string]string{
		".go": "🔵", ".js": "🟡", ".ts": "🔷", ".py": "🐍", ".rs": "🦀",
		".rb": "💎", ".java": "☕", ".php": "🐘", ".html": "🌐", ".css": "🎨",
		".json": "📋", ".md": "📝", ".yml": "⚙️", ".yaml": "⚙️", ".sh": "📜",
		".sql": "🗃️", ".jpg": "🖼️", ".png": "🖼️", ".gif": "🖼️", ".svg": "🖼️",
		".mp3": "🎵", ".mp4": "🎬", ".pdf": "📕", ".zip": "📦", ".exe": "⚡",
	}
	if icon, ok := icons[ext]; ok {
		return icon
	}
	return "📄"
}

func cmdRun(command string) string {
	if command == "" {
		return "Usage: /run <command>"
	}
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked] Manual mode%s", colorRed, colorReset)
	}
	if currentMode == ModeAsk {
		fmt.Printf("%sRun:%s %s [y/N] ", colorYellow, colorReset, command)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			return "Cancelled"
		}
	}
	
	fmt.Printf("%s$ %s%s\n", colorGray, command, colorReset)
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = currentDir
	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		result += fmt.Sprintf("\n%sExit: %s%s", colorRed, err, colorReset)
	}
	return result
}

func cmdCd(path string) string {
	if path == "" {
		path = os.Getenv("HOME")
	}
	newPath := resolvePath(path)
	if info, err := os.Stat(newPath); err != nil || !info.IsDir() {
		return "Error: not a directory"
	}
	currentDir = newPath
	detectProject()
	return fmt.Sprintf("→ %s", currentDir)
}

func cmdFind(pattern string) string {
	if pattern == "" {
		return "Usage: /find <pattern>"
	}
	cmd := exec.Command("find", currentDir, "-maxdepth", "6", "-iname", "*"+pattern+"*",
		"-not", "-path", "*/node_modules/*", "-not", "-path", "*/.git/*")
	output, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "No files found"
	}
	lines := strings.Split(result, "\n")
	if len(lines) > 30 {
		result = strings.Join(lines[:30], "\n") + fmt.Sprintf("\n%s+%d more%s", colorGray, len(lines)-30, colorReset)
	}
	return fmt.Sprintf("%sFound %d:%s\n%s", colorGreen, len(lines), colorReset, result)
}

func cmdGrep(args string) string {
	parts := strings.SplitN(args, " ", 2)
	pattern := parts[0]
	searchPath := currentDir
	if len(parts) > 1 {
		searchPath = resolvePath(parts[1])
	}
	cmd := exec.Command("grep", "-rn", "-i", "--include=*.*",
		"--exclude-dir=node_modules", "--exclude-dir=.git", pattern, searchPath)
	output, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "No matches"
	}
	lines := strings.Split(result, "\n")
	if len(lines) > 25 {
		result = strings.Join(lines[:25], "\n") + fmt.Sprintf("\n%s+%d more%s", colorGray, len(lines)-25, colorReset)
	}
	return fmt.Sprintf("%sMatched %d:%s\n%s", colorGreen, len(lines), colorReset, result)
}

func cmdTree(path string) string {
	if path == "" {
		path = currentDir
	} else {
		path = resolvePath(path)
	}
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s%s%s\n", colorCyan, path, colorReset))
	walkDir(path, "", &result, 0, 3)
	return result.String()
}

func walkDir(path, prefix string, result *strings.Builder, depth, maxDepth int) {
	if depth >= maxDepth {
		return
	}
	entries, _ := os.ReadDir(path)
	var filtered []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			continue
		}
		filtered = append(filtered, e)
		if len(filtered) >= 15 {
			break
		}
	}
	for i, e := range filtered {
		isLast := i == len(filtered)-1
		conn := "├── "
		if isLast {
			conn = "└── "
		}
		if e.IsDir() {
			result.WriteString(fmt.Sprintf("%s%s%s%s/%s\n", prefix, conn, colorBlue, e.Name(), colorReset))
			newPre := prefix + "│   "
			if isLast {
				newPre = prefix + "    "
			}
			walkDir(filepath.Join(path, e.Name()), newPre, result, depth+1, maxDepth)
		} else {
			result.WriteString(fmt.Sprintf("%s%s%s\n", prefix, conn, e.Name()))
		}
	}
}

func cmdWrite(args string) string {
	parts := strings.SplitN(args, "|||", 2)
	if len(parts) < 2 {
		return "Error: format path|||content"
	}
	path, content := strings.TrimSpace(parts[0]), parts[1]
	fullPath := resolvePath(path)
	
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked]%s", colorRed, colorReset)
	}
	if currentMode == ModeAsk {
		fmt.Printf("%sWrite %s?%s [y/N] ", colorYellow, fullPath, colorReset)
		reader := bufio.NewReader(os.Stdin)
		if in, _ := reader.ReadString('\n'); strings.ToLower(strings.TrimSpace(in)) != "y" {
			return "Cancelled"
		}
	}
	
	saveForUndo(path, "write")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte(content), 0644)
	return fmt.Sprintf("%s✓ Written: %s (%d bytes)%s", colorGreen, fullPath, len(content), colorReset)
}

func cmdReplace(args string) string {
	parts := strings.SplitN(args, "|||", 3)
	if len(parts) < 3 {
		return "Error: format path|||old|||new"
	}
	path, old, new := strings.TrimSpace(parts[0]), parts[1], parts[2]
	fullPath := resolvePath(path)
	
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked]%s", colorRed, colorReset)
	}
	
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	content := string(data)
	if !strings.Contains(content, old) {
		return "Text not found"
	}
	
	fmt.Printf("%s--- %s%s\n%s- %s%s\n%s+ %s%s\n",
		colorRed, fullPath, colorReset,
		colorRed, truncate(old, 80), colorReset,
		colorGreen, truncate(new, 80), colorReset)
	
	if currentMode == ModeAsk {
		fmt.Printf("%sApply?%s [y/N] ", colorYellow, colorReset)
		reader := bufio.NewReader(os.Stdin)
		if in, _ := reader.ReadString('\n'); strings.ToLower(strings.TrimSpace(in)) != "y" {
			return "Cancelled"
		}
	}
	
	saveForUndo(path, "replace")
	os.WriteFile(fullPath, []byte(strings.Replace(content, old, new, 1)), 0644)
	return fmt.Sprintf("%s✓ Replaced in %s%s", colorGreen, fullPath, colorReset)
}

func cmdAppend(args string) string {
	parts := strings.SplitN(args, "|||", 2)
	if len(parts) < 2 {
		return "Error: format path|||content"
	}
	path, content := strings.TrimSpace(parts[0]), parts[1]
	fullPath := resolvePath(path)
	
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked]%s", colorRed, colorReset)
	}
	
	saveForUndo(path, "append")
	f, _ := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.WriteString(content)
	f.Close()
	return fmt.Sprintf("%s✓ Appended to %s%s", colorGreen, fullPath, colorReset)
}

func cmdGit(args string) string {
	if args == "" {
		args = "status"
	}
	cmd := exec.Command("sh", "-c", "git "+args)
	cmd.Dir = currentDir
	output, _ := cmd.CombinedOutput()
	return string(output)
}

func cmdFetch(url string) string {
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	if len(content) > 8000 {
		content = content[:8000] + "\n... (truncated)"
	}
	return fmt.Sprintf("%sURL: %s (%d bytes)%s\n%s", colorCyan, url, len(body), colorReset, content)
}

func getGitBranch() string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = currentDir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func cmdEdit(path string, scanner *bufio.Scanner) string {
	if path == "" {
		return "Usage: /edit <file>"
	}
	fullPath := resolvePath(path)
	
	if data, err := os.ReadFile(fullPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i >= 25 {
				fmt.Printf("%s... +%d more%s\n", colorGray, len(lines)-25, colorReset)
				break
			}
			fmt.Printf("%s%3d│%s %s\n", colorGray, i+1, colorReset, line)
		}
	} else {
		fmt.Printf("%sNew file%s\n", colorYellow, colorReset)
	}
	
	fmt.Printf("\n%sEnter content (/save or /cancel):%s\n", colorYellow, colorReset)
	var content strings.Builder
	for {
		fmt.Printf("%s │%s ", colorGray, colorReset)
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if line == "/save" {
			saveForUndo(path, "edit")
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			os.WriteFile(fullPath, []byte(content.String()), 0644)
			return fmt.Sprintf("%s✓ Saved%s", colorGreen, colorReset)
		}
		if line == "/cancel" {
			return "Cancelled"
		}
		content.WriteString(line + "\n")
	}
	return "Cancelled"
}

// ==================== HELPERS ====================

func resolvePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(currentDir, path)
	}
	return filepath.Clean(path)
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func processAtMentions(input string) string {
	re := regexp.MustCompile(`@([\w./\-_]+)`)
	matches := re.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input
	}
	
	var files []string
	for _, m := range matches {
		filename := m[1]
		fullPath := resolvePath(filename)
		if data, err := os.ReadFile(fullPath); err == nil {
			content := string(data)
			if lines := strings.Split(content, "\n"); len(lines) > 100 {
				content = strings.Join(lines[:100], "\n") + fmt.Sprintf("\n... +%d lines", len(lines)-100)
			}
			files = append(files, fmt.Sprintf("=== %s ===\n%s", fullPath, content))
			fmt.Printf("%s  ✓ @%s%s\n", colorGray, filename, colorReset)
		}
	}
	
	if len(files) > 0 {
		return input + "\n\n" + strings.Join(files, "\n\n")
	}
	return input
}

func readMultiLine(scanner *bufio.Scanner) string {
	var lines []string
	for {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasSuffix(line, "\\") {
			lines = append(lines, strings.TrimSuffix(line, "\\"))
			fmt.Printf("%s. %s", colorGray, colorReset)
			continue
		}
		lines = append(lines, line)
		break
	}
	return strings.Join(lines, "\n")
}

// ==================== TOOLS ====================

func parseAndExecuteTools(response string) (string, []string) {
	var results []string
	for {
		start := strings.Index(response, "<tool>")
		if start == -1 {
			break
		}
		end := strings.Index(response[start:], "</tool>")
		if end == -1 {
			break
		}
		end += start
		
		toolCall := response[start+6 : end]
		parts := strings.SplitN(toolCall, ":", 2)
		toolName := strings.TrimSpace(parts[0])
		toolArg := ""
		if len(parts) > 1 {
			toolArg = strings.TrimSpace(parts[1])
		}
		
		var result string
		switch toolName {
		case "read":
			result = cmdRead(toolArg)
		case "ls":
			result = cmdList(toolArg)
		case "run":
			result = cmdRun(toolArg)
		case "find":
			result = cmdFind(toolArg)
		case "grep":
			result = cmdGrep(toolArg)
		case "tree":
			result = cmdTree(toolArg)
		case "write":
			result = cmdWrite(toolArg)
		case "replace":
			result = cmdReplace(toolArg)
		case "append":
			result = cmdAppend(toolArg)
		case "git":
			result = cmdGit(toolArg)
		case "fetch":
			result = cmdFetch(toolArg)
		case "cd":
			result = cmdCd(toolArg)
		case "python":
			result = runPython(toolArg)
		case "node":
			result = runNode(toolArg)
		case "search":
			result = webSearch(toolArg)
		case "image":
			result = analyzeImage(toolArg)
		case "remember":
			p := strings.SplitN(toolArg, ":", 2)
			if len(p) == 2 {
				rememberFact(p[0], p[1])
				result = "Remembered: " + p[0]
			}
		default:
			result = "Unknown tool: " + toolName
		}
		
		results = append(results, fmt.Sprintf("[%s] %s", toolName, result))
		response = response[:start] + response[end+7:]
	}
	return strings.TrimSpace(response), results
}

// ==================== CHAT ====================

func getAPIKey() string {
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		return key
	}
	home, _ := os.UserHomeDir()
	if data, err := os.ReadFile(filepath.Join(home, ".mytool_key")); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func saveAPIKey(key string) {
	home, _ := os.UserHomeDir()
	os.WriteFile(filepath.Join(home, ".mytool_key"), []byte(key), 0600)
}

func getSystemPrompt() string {
	hostname, _ := os.Hostname()
	
	memoryStr := ""
	if len(memory) > 0 {
		var facts []string
		for k, v := range memory {
			facts = append(facts, fmt.Sprintf("- %s: %s", k, v))
		}
		memoryStr = "\n\nMEMORY:\n" + strings.Join(facts, "\n")
	}
	
	return fmt.Sprintf(`Kamu mytool v%s, AI terminal assistant dengan akses penuh ke sistem.

SISTEM:
- Host: %s | OS: %s/%s | User: %s
- Dir: %s | Project: %s | Mode: %s%s

TOOLS (format: <tool>nama:arg</tool>):

READ:
- <tool>read:file</tool> - Baca file
- <tool>ls:dir</tool> - List direktori
- <tool>tree:dir</tool> - Struktur folder
- <tool>find:pattern</tool> - Cari file
- <tool>grep:pattern path</tool> - Cari teks
- <tool>image:file</tool> - Analisa gambar

WRITE:
- <tool>write:path|||content</tool> - Buat/tulis file
- <tool>replace:path|||old|||new</tool> - Ganti teks
- <tool>append:path|||content</tool> - Tambah ke file

EXECUTE:
- <tool>run:cmd</tool> - Shell command
- <tool>git:cmd</tool> - Git command
- <tool>python:code</tool> - Jalankan Python
- <tool>node:code</tool> - Jalankan JavaScript

WEB:
- <tool>fetch:url</tool> - Ambil konten URL
- <tool>search:query</tool> - Cari di web

MEMORY:
- <tool>remember:key:value</tool> - Ingat sesuatu

ATURAN:
1. LANGSUNG gunakan tools - jangan suruh user manual
2. Untuk edit: baca dulu, lalu replace dengan exact text
3. Tampilkan diff sebelum edit
4. Bahasa Indonesia jika user pakai Indonesia
5. Respons singkat dan informatif`,
		version, hostname, runtime.GOOS, runtime.GOARCH, os.Getenv("USER"),
		currentDir, projectType, currentMode, memoryStr)
}

func runChat(args []string) {
	apiKey := getAPIKey()
	if apiKey == "" {
		fmt.Printf("\n%smytool Setup%s\n\n", colorCyan, colorReset)
		fmt.Println("API key required: https://platform.minimax.io/")
		fmt.Printf("\nEnter API Key: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				saveAPIKey(apiKey)
				fmt.Printf("%s✓ Saved%s\n", colorGreen, colorReset)
			}
		}
		if apiKey == "" {
			os.Exit(1)
		}
	}

	if len(args) > 0 {
		msg := processAtMentions(strings.Join(args, " "))
		messages := []ChatMessage{
			{Role: "system", Content: getSystemPrompt()},
			{Role: "user", Content: msg},
		}
		showThinking()
		response, _ := sendStream(apiKey, messages)
		stopThinking()
		fmt.Printf("%s%s%s\n", colorGreen, response, colorReset)
		
		_, results := parseAndExecuteTools(response)
		if len(results) > 0 {
			fmt.Printf("\n%s─── Results ───%s\n", colorCyan, colorReset)
			for _, r := range results {
				fmt.Println(r)
			}
		}
		return
	}

	history := []ChatMessage{{Role: "system", Content: getSystemPrompt()}}
	runChatWithHistory(history)
}

func runChatWithHistory(history []ChatMessage) {
	apiKey := getAPIKey()
	
	printBanner()
	fmt.Printf("\n%sYou are standing in an open terminal. An AI awaits your commands.%s\n", colorGray, colorReset)
	fmt.Printf("\nENTER send • \\ newline • @file include • /help commands • Ctrl+C exit\n")
	printStatusBar()
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 2*1024*1024), 2*1024*1024)

	hints := []string{
		"\"What's in this folder?\"",
		"\"Read and explain package.json\"",
		"\"Find all TODO comments\"",
		"\"Create a Python hello world\"",
		"\"Search how to parse JSON in Go\"",
	}
	hintIdx := 0

	for {
		hint := hints[hintIdx%len(hints)]
		fmt.Printf("%s❯%s %s%s%s", getModeColor(), colorReset, colorGray, hint, colorReset)
		fmt.Printf("\r%s❯%s ", getModeColor(), colorReset)
		
		input := readMultiLine(scanner)
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		hintIdx++
		
		appendToExport("User", input)

		// Commands
		switch {
		case input == "exit" || input == "quit":
			saveMemory()
			fmt.Printf("%s👋 Bye!%s\n", colorCyan, colorReset)
			return
		case input == "/mode":
			cycleMode()
			history[0] = ChatMessage{Role: "system", Content: getSystemPrompt()}
			fmt.Printf("Mode: %s\n\n", getModeDisplay())
			continue
		case input == "/undo":
			fmt.Println(doUndo())
			fmt.Println()
			continue
		case input == "/save":
			saveSession(history)
			continue
		case input == "/copy":
			fmt.Println(copyToClipboard(lastResponse))
			continue
		case input == "/cost":
			fmt.Printf("Tokens: %d | Cost: $%.4f\n\n", totalTokens, totalCost)
			continue
		case input == "/context":
			pct := float64(totalTokens) / float64(maxContextTokens) * 100
			fmt.Printf("Context: %d/%d (%.1f%%)\n\n", totalTokens, maxContextTokens, pct)
			continue
		case input == "/memory":
			showMemory()
			fmt.Println()
			continue
		case input == "/sessions":
			listSessions()
			fmt.Println()
			continue
		case strings.HasPrefix(input, "/export"):
			parts := strings.SplitN(input, " ", 2)
			f := ""
			if len(parts) > 1 {
				f = parts[1]
			}
			exportChat(f)
			continue
		case strings.HasPrefix(input, "/forget "):
			key := strings.TrimPrefix(input, "/forget ")
			forgetFact(key)
			fmt.Printf("Forgot: %s\n\n", key)
			continue
		case strings.HasPrefix(input, "/remember "):
			parts := strings.SplitN(strings.TrimPrefix(input, "/remember "), "=", 2)
			if len(parts) == 2 {
				rememberFact(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				fmt.Printf("Remembered: %s\n\n", parts[0])
			}
			continue
		case strings.HasPrefix(input, "/python "):
			code := strings.TrimPrefix(input, "/python ")
			fmt.Println(runPython(code))
			continue
		case strings.HasPrefix(input, "/node "):
			code := strings.TrimPrefix(input, "/node ")
			fmt.Println(runNode(code))
			continue
		case strings.HasPrefix(input, "/search "):
			query := strings.TrimPrefix(input, "/search ")
			fmt.Println(webSearch(query))
			continue
		case strings.HasPrefix(input, "/img "):
			path := strings.TrimPrefix(input, "/img ")
			fmt.Println(analyzeImage(path))
			continue
		case strings.HasPrefix(input, "/"):
			result := handleCommand(input, scanner)
			fmt.Println(result)
			fmt.Println()
			continue
		}

		// Process mentions
		input = processAtMentions(input)

		// Send to AI
		history = append(history, ChatMessage{Role: "user", Content: input})
		
		showThinking()
		response, _ := sendStream(apiKey, history)
		stopThinking()
		lastResponse = response
		
		appendToExport("Assistant", response)
		totalCost = float64(totalTokens) / 1000 * costPer1KTokens

		// Parse tools
		_, results := parseAndExecuteTools(response)
		
		if len(results) > 0 {
			fmt.Printf("\n\n%s─── Executing ───%s\n", colorCyan, colorReset)
			for _, r := range results {
				fmt.Println(r)
			}
			fmt.Printf("%s─────────────────%s\n", colorCyan, colorReset)
			
			history = append(history, ChatMessage{Role: "assistant", Content: response})
			history = append(history, ChatMessage{
				Role:    "user",
				Content: "Results:\n" + strings.Join(results, "\n") + "\n\nJelaskan singkat.",
			})
			
			fmt.Printf("\n%s", colorGreen)
			followUp, _ := sendStream(apiKey, history)
			fmt.Printf("%s", colorReset)
			lastResponse = followUp
			
			if followUp != "" {
				history = append(history, ChatMessage{Role: "assistant", Content: followUp})
				appendToExport("Assistant", followUp)
			}
		} else {
			history = append(history, ChatMessage{Role: "assistant", Content: response})
		}
		
		fmt.Printf("\n\n")
	}
}

func handleCommand(input string, scanner *bufio.Scanner) string {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/help", "/?":
		return `/read <f>   Read file
/ls [d]     List directory
/run <c>    Run command
/find <n>   Find files
/grep <p>   Search in files
/tree [d]   Show structure
/git <c>    Git command
/edit <f>   Edit file
/cd <d>     Change directory
/python <c> Run Python
/node <c>   Run JavaScript
/search <q> Web search
/img <f>    Analyze image
/settings   Open settings menu
/mcp        Manage MCP servers
/mode       Toggle mode
/undo       Undo change
/save       Save session
/export [f] Export chat
/copy       Copy last response
/cost       Show API cost
/context    Context usage
/memory     Show memory
/remember   Remember fact
/forget <k> Forget fact
/clear      Clear history
exit        Quit`
	case "/settings":
		showSettings(scanner)
		return ""
	case "/mcp":
		showMCPServers(scanner)
		return ""
	case "/read", "/cat":
		return cmdRead(arg)
	case "/ls", "/dir":
		return cmdList(arg)
	case "/run", "/exec", "/$":
		return cmdRun(arg)
	case "/find":
		return cmdFind(arg)
	case "/grep":
		return cmdGrep(arg)
	case "/tree":
		return cmdTree(arg)
	case "/git":
		return cmdGit(arg)
	case "/cd":
		return cmdCd(arg)
	case "/pwd":
		return currentDir
	case "/edit":
		return cmdEdit(arg, scanner)
	case "/clear":
		return "Cleared"
	default:
		return "Unknown: " + cmd
	}
}

func sendStream(apiKey string, messages []ChatMessage) (string, error) {
	reqBody := ChatRequest{
		Model:       modelName,
		MaxTokens:   4096,
		Messages:    messages,
		Stream:      true,
		Temperature: 0.7,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", minimaxAPIURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var full strings.Builder
	reader := bufio.NewReader(resp.Body)
	
	fmt.Printf("%s", colorGreen)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var sr StreamResponse
			if json.Unmarshal([]byte(data), &sr) == nil {
				if len(sr.Choices) > 0 {
					content := sr.Choices[0].Delta.Content
					if content != "" {
						fmt.Print(content)
						full.WriteString(content)
					}
				}
				if sr.Usage.TotalTokens > 0 {
					totalTokens = sr.Usage.TotalTokens
				}
			}
		}
	}
	
	fmt.Printf("%s", colorReset)
	return full.String(), nil
}
