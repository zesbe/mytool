package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

const minimaxAPIURL = "https://api.minimax.io/v1/chat/completions"
const modelName = "MiniMax-Text-01"

// Modes
const (
	ModeAuto   = "auto"
	ModeAsk    = "ask"
	ModeManual = "manual"
)

var currentMode = ModeAuto
var currentDir string

type StreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type StreamResponse struct {
	Choices []StreamChoice `json:"choices"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Messages  []ChatMessage `json:"messages"`
	Stream    bool          `json:"stream,omitempty"`
}

type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	currentDir, _ = os.Getwd()
	args := os.Args[1:]

	if len(args) > 0 && (strings.HasSuffix(args[0], "/mytool") || strings.HasSuffix(args[0], "\\mytool.exe")) {
		args = args[1:]
	}

	if len(args) < 1 {
		runChat([]string{})
		return
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "version", "-v", "--version":
		printVersion()
	case "info":
		printSystemInfo()
	case "help", "-h", "--help":
		printHelp()
	default:
		runChat(append([]string{command}, cmdArgs...))
	}
}

func printBanner() {
	banner := `
%s███╗   ███╗██╗   ██╗████████╗ ██████╗  ██████╗ ██╗     
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
	fmt.Printf("\n%smytool%s v%s - AI Terminal Assistant\n\n", colorGreen, colorReset, version)
	fmt.Printf("%sUsage:%s mytool [message]\n\n", colorYellow, colorReset)
	fmt.Printf("%sCommands:%s\n", colorYellow, colorReset)
	fmt.Println("  (no args)     Start interactive chat")
	fmt.Println("  \"message\"     Send single message to AI")
	fmt.Println("  help          Show this help")
	fmt.Println("  version       Show version")
	fmt.Printf("\n%sChat Features:%s\n", colorYellow, colorReset)
	fmt.Println("  @filename     Mention file to include its content")
	fmt.Println("  \\             Continue input on next line")
	fmt.Println("  /mode         Cycle through modes (auto/ask/manual)")
	fmt.Println("  /read <file>  Read file content")
	fmt.Println("  /ls [dir]     List directory")
	fmt.Println("  /run <cmd>    Execute command")
	fmt.Println("  /find <name>  Search files")
	fmt.Println("  /cd <dir>     Change directory")
	fmt.Println("  /clear        Clear chat history")
	fmt.Println("  exit          Exit mytool")
	fmt.Println()
}

func printVersion() {
	fmt.Printf("%smytool%s v%s\n", colorCyan, colorReset, version)
	fmt.Printf("  Model: %s\n", modelName)
	fmt.Printf("  OS:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printSystemInfo() {
	hostname, _ := os.Hostname()
	fmt.Printf("\n%sSystem Info%s\n", colorCyan, colorReset)
	fmt.Printf("  Hostname: %s\n", hostname)
	fmt.Printf("  OS:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  CPUs:     %d\n", runtime.NumCPU())
	fmt.Printf("  Dir:      %s\n", currentDir)
	fmt.Println()
}

func getModeDisplay() string {
	switch currentMode {
	case ModeAuto:
		return fmt.Sprintf("%sAuto%s %s(allow all)%s", colorGreen, colorReset, colorGray, colorReset)
	case ModeAsk:
		return fmt.Sprintf("%sAsk%s %s(confirm actions)%s", colorYellow, colorReset, colorGray, colorReset)
	case ModeManual:
		return fmt.Sprintf("%sManual%s %s(no auto actions)%s", colorRed, colorReset, colorGray, colorReset)
	}
	return ""
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

func printStatusBar() {
	mode := getModeDisplay()
	model := fmt.Sprintf("%s%s%s", colorPurple, modelName, colorReset)
	fmt.Printf("\n%s • %s                    %sCurrent folder:%s %s\n", mode, model, colorGray, colorReset, currentDir)
}

// Process @mentions in input - replace @filename with file content
func processAtMentions(input string) string {
	re := regexp.MustCompile(`@([\w./\-_]+)`)
	matches := re.FindAllStringSubmatch(input, -1)
	
	if len(matches) == 0 {
		return input
	}
	
	result := input
	var fileContents []string
	
	for _, match := range matches {
		filename := match[1]
		fullPath := resolvePath(filename)
		
		data, err := os.ReadFile(fullPath)
		if err != nil {
			fileContents = append(fileContents, fmt.Sprintf("[Error reading %s: %s]", filename, err))
			continue
		}
		
		content := string(data)
		// Truncate if too long
		lines := strings.Split(content, "\n")
		if len(lines) > 100 {
			content = strings.Join(lines[:100], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-100)
		}
		
		fileContents = append(fileContents, fmt.Sprintf("=== File: %s ===\n%s\n=== End: %s ===", fullPath, content, filename))
		
		// Show that file was loaded
		fmt.Printf("%s  ✓ Loaded: %s%s\n", colorGray, fullPath, colorReset)
	}
	
	if len(fileContents) > 0 {
		result = result + "\n\n" + strings.Join(fileContents, "\n\n")
	}
	
	return result
}

// Read multi-line input (lines ending with \ continue)
func readMultiLineInput(scanner *bufio.Scanner) string {
	var lines []string
	
	for {
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		
		if strings.HasSuffix(line, "\\") {
			// Continue to next line
			lines = append(lines, strings.TrimSuffix(line, "\\"))
			fmt.Printf("%s. %s", colorGray, colorReset)
			continue
		}
		
		lines = append(lines, line)
		break
	}
	
	return strings.Join(lines, "\n")
}

// ==================== FILE OPERATIONS ====================

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
	if len(lines) > 100 {
		content = strings.Join(lines[:100], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-100)
	}
	return fmt.Sprintf("File: %s\n%s\n%s", fullPath, strings.Repeat("-", 40), content)
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
	result.WriteString(fmt.Sprintf("Directory: %s\n%s\n", path, strings.Repeat("-", 40)))
	
	for _, entry := range entries {
		info, _ := entry.Info()
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("%s[DIR]%s  %s/\n", colorBlue, colorReset, entry.Name()))
		} else {
			size := ""
			if info != nil {
				size = formatSize(info.Size())
			}
			result.WriteString(fmt.Sprintf("       %s %s\n", entry.Name(), size))
		}
	}
	return result.String()
}

func cmdRun(command string) string {
	if command == "" {
		return "Usage: /run <command>"
	}
	
	// Check mode
	if currentMode == ModeManual {
		return "Manual mode: command execution disabled. Use /mode to change."
	}
	
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Dir = currentDir
	
	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		result += fmt.Sprintf("\nExit: %s", err)
	}
	return result
}

func cmdCd(path string) string {
	if path == "" {
		path = os.Getenv("HOME")
	}
	newPath := resolvePath(path)
	
	info, err := os.Stat(newPath)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	if !info.IsDir() {
		return fmt.Sprintf("Error: %s is not a directory", newPath)
	}
	
	currentDir = newPath
	return fmt.Sprintf("Changed to: %s", currentDir)
}

func cmdFind(pattern string) string {
	if pattern == "" {
		return "Usage: /find <pattern>"
	}
	
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "dir", "/s", "/b", "*"+pattern+"*")
	} else {
		cmd = exec.Command("find", currentDir, "-maxdepth", "5", "-iname", "*"+pattern+"*")
	}
	cmd.Dir = currentDir
	
	output, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	
	if result == "" {
		return fmt.Sprintf("No files found matching: %s", pattern)
	}
	
	lines := strings.Split(result, "\n")
	if len(lines) > 30 {
		result = strings.Join(lines[:30], "\n") + fmt.Sprintf("\n... and %d more", len(lines)-30)
	}
	
	return fmt.Sprintf("Found %d items:\n%s", len(lines), result)
}

func cmdGrep(args string) string {
	parts := strings.SplitN(args, ":", 2)
	if len(parts) < 1 || parts[0] == "" {
		return "Usage: /grep <pattern>:<path>"
	}
	
	pattern := parts[0]
	searchPath := currentDir
	if len(parts) > 1 && parts[1] != "" {
		searchPath = resolvePath(parts[1])
	}
	
	cmd := exec.Command("grep", "-r", "-n", "-i", "--include=*", pattern, searchPath)
	output, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	
	if result == "" {
		return fmt.Sprintf("No matches found for: %s", pattern)
	}
	
	lines := strings.Split(result, "\n")
	if len(lines) > 20 {
		result = strings.Join(lines[:20], "\n") + fmt.Sprintf("\n... and %d more", len(lines)-20)
	}
	
	return fmt.Sprintf("Found %d matches:\n%s", len(lines), result)
}

func cmdTree(path string) string {
	if path == "" {
		path = currentDir
	} else {
		path = resolvePath(path)
	}
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s\n", path))
	walkDir(path, "", &result, 0, 3)
	return result.String()
}

func walkDir(path string, prefix string, result *strings.Builder, depth int, maxDepth int) {
	if depth >= maxDepth {
		return
	}
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	
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
	
	for i, entry := range filtered {
		isLast := i == len(filtered)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("%s%s%s/\n", prefix, connector, entry.Name()))
			newPrefix := prefix + "│   "
			if isLast {
				newPrefix = prefix + "    "
			}
			walkDir(filepath.Join(path, entry.Name()), newPrefix, result, depth+1, maxDepth)
		} else {
			result.WriteString(fmt.Sprintf("%s%s%s\n", prefix, connector, entry.Name()))
		}
	}
}

func cmdEdit(path string, scanner *bufio.Scanner) string {
	if path == "" {
		return "Usage: /edit <file>"
	}
	fullPath := resolvePath(path)
	
	if data, err := os.ReadFile(fullPath); err == nil {
		fmt.Printf("%sFile exists:%s\n", colorYellow, colorReset)
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i >= 15 {
				fmt.Printf("%s... (%d more lines)%s\n", colorGray, len(lines)-15, colorReset)
				break
			}
			fmt.Printf("%s%3d:%s %s\n", colorGray, i+1, colorReset, line)
		}
	} else {
		fmt.Printf("%sNew file: %s%s\n", colorYellow, fullPath, colorReset)
	}
	
	fmt.Printf("\n%sEnter content (/save to save, /cancel to cancel):%s\n", colorYellow, colorReset)
	
	var content strings.Builder
	for {
		fmt.Printf("%s|%s ", colorGray, colorReset)
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if line == "/save" {
			if err := os.WriteFile(fullPath, []byte(content.String()), 0644); err != nil {
				return fmt.Sprintf("Error: %s", err)
			}
			return fmt.Sprintf("%sSaved: %s%s", colorGreen, fullPath, colorReset)
		}
		if line == "/cancel" {
			return "Cancelled"
		}
		content.WriteString(line + "\n")
	}
	return "Cancelled"
}

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

// ==================== TOOL EXECUTION ====================

func parseAndExecuteTools(response string) (string, []string) {
	var toolResults []string
	
	for {
		startIdx := strings.Index(response, "<tool>")
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(response[startIdx:], "</tool>")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx
		
		toolCall := response[startIdx+6 : endIdx]
		parts := strings.SplitN(toolCall, ":", 2)
		if len(parts) < 2 {
			parts = append(parts, "")
		}
		
		toolName := strings.TrimSpace(parts[0])
		toolArg := strings.TrimSpace(parts[1])
		
		// Check mode before executing
		if currentMode == ModeManual && (toolName == "run" || toolName == "write") {
			toolResults = append(toolResults, fmt.Sprintf("[%s blocked - manual mode]", toolName))
			response = response[:startIdx] + response[endIdx+7:]
			continue
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
		default:
			result = fmt.Sprintf("Unknown tool: %s", toolName)
		}
		
		toolResults = append(toolResults, fmt.Sprintf("[%s:%s]\n%s", toolName, toolArg, result))
		response = response[:startIdx] + response[endIdx+7:]
	}
	
	return strings.TrimSpace(response), toolResults
}

// ==================== CHAT ====================

func getAPIKey() string {
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		return key
	}
	home, _ := os.UserHomeDir()
	if data, err := os.ReadFile(home + "/.mytool_key"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func saveAPIKey(key string) error {
	home, _ := os.UserHomeDir()
	return os.WriteFile(home+"/.mytool_key", []byte(key), 0600)
}

func getSystemPrompt() string {
	hostname, _ := os.Hostname()
	modeDesc := "auto - dapat menjalankan semua tools"
	if currentMode == ModeAsk {
		modeDesc = "ask - konfirmasi sebelum menjalankan"
	} else if currentMode == ModeManual {
		modeDesc = "manual - tidak menjalankan tools otomatis"
	}
	
	return fmt.Sprintf(`Kamu adalah mytool, AI assistant di terminal dengan akses sistem.

SISTEM:
- Host: %s | OS: %s/%s | User: %s
- Dir: %s
- Mode: %s

TOOLS (gunakan format <tool>nama:arg</tool>):
- <tool>read:file</tool> - Baca file
- <tool>ls:dir</tool> - List direktori  
- <tool>run:cmd</tool> - Jalankan command
- <tool>find:nama</tool> - Cari file/folder
- <tool>grep:pattern:path</tool> - Cari teks
- <tool>tree:dir</tool> - Struktur folder

ATURAN:
1. LANGSUNG gunakan tools saat user minta akses file/folder/command
2. Jangan suruh user melakukan sendiri - KAMU yang lakukan
3. Respons singkat dan informatif
4. Bahasa Indonesia jika user pakai Indonesia`, 
		hostname, runtime.GOOS, runtime.GOARCH, os.Getenv("USER"), currentDir, modeDesc)
}

func runChat(args []string) {
	apiKey := getAPIKey()
	if apiKey == "" {
		fmt.Printf("\n%smytool%s - Setup\n\n", colorGreen, colorReset)
		fmt.Println("API key required. Get one at: https://platform.minimax.io/")
		fmt.Printf("\nEnter API Key: ")
		
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				saveAPIKey(apiKey)
				fmt.Printf("%sAPI key saved!%s\n\n", colorGreen, colorReset)
			}
		}
		if apiKey == "" {
			fmt.Printf("%sAPI key required%s\n", colorRed, colorReset)
			os.Exit(1)
		}
	}

	// Single message mode
	if len(args) > 0 {
		message := processAtMentions(strings.Join(args, " "))
		messages := []ChatMessage{
			{Role: "system", Content: getSystemPrompt()},
			{Role: "user", Content: message},
		}
		fmt.Printf("%s", colorGreen)
		response, _ := sendMessageStream(apiKey, messages)
		fmt.Printf("%s\n", colorReset)
		
		// Handle tool calls
		_, toolResults := parseAndExecuteTools(response)
		if len(toolResults) > 0 {
			fmt.Printf("\n%s--- Tools ---%s\n", colorCyan, colorReset)
			for _, r := range toolResults {
				fmt.Printf("%s%s%s\n", colorGray, r, colorReset)
			}
		}
		return
	}

	// Interactive mode
	printBanner()
	fmt.Printf("\n%sYou are standing in an open terminal. An AI awaits your commands.%s\n", colorGray, colorReset)
	fmt.Printf("\nENTER to send • \\ for newline • @file to include • /mode to switch\n")
	printStatusBar()
	fmt.Println()

	history := []ChatMessage{
		{Role: "system", Content: getSystemPrompt()},
	}
	
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	placeholders := []string{
		"Try: \"What files are in this folder?\"",
		"Try: \"Read package.json and explain it\"",
		"Try: \"Find all .go files\"",
		"Try: \"Show folder structure\"",
	}
	phIdx := 0

	for {
		fmt.Printf("%s> %s%s%s ", colorYellow, colorGray, placeholders[phIdx%len(placeholders)], colorReset)
		fmt.Printf("\r%s> %s", colorYellow, colorReset)
		
		input := readMultiLineInput(scanner)
		input = strings.TrimSpace(input)
		
		if input == "" {
			continue
		}
		
		phIdx++

		// Exit
		if input == "exit" || input == "quit" || input == "/exit" {
			fmt.Printf("%sBye!%s\n", colorCyan, colorReset)
			break
		}

		// Mode switch
		if input == "/mode" {
			cycleMode()
			fmt.Printf("Mode: %s\n\n", getModeDisplay())
			history[0] = ChatMessage{Role: "system", Content: getSystemPrompt()}
			continue
		}

		// Slash commands
		if strings.HasPrefix(input, "/") {
			result := handleCommand(input, scanner)
			if result != "" {
				fmt.Println(result)
			}
			fmt.Println()
			continue
		}

		// Process @mentions
		input = processAtMentions(input)

		// Send to AI
		history = append(history, ChatMessage{Role: "user", Content: input})
		
		fmt.Printf("%s", colorGreen)
		response, err := sendMessageStream(apiKey, history)
		fmt.Printf("%s", colorReset)
		
		if err != nil {
			fmt.Printf("\n%sError: %s%s\n\n", colorRed, err, colorReset)
			history = history[:len(history)-1]
			continue
		}
		
		// Handle tool calls
		_, toolResults := parseAndExecuteTools(response)
		
		if len(toolResults) > 0 {
			fmt.Printf("\n\n%s--- Executing ---%s\n", colorCyan, colorReset)
			for _, r := range toolResults {
				fmt.Printf("%s%s%s\n", colorGray, r, colorReset)
			}
			fmt.Printf("%s-----------------%s\n", colorCyan, colorReset)
			
			history = append(history, ChatMessage{Role: "assistant", Content: response})
			
			// Get AI explanation
			toolContext := "Results:\n" + strings.Join(toolResults, "\n\n")
			history = append(history, ChatMessage{Role: "user", Content: toolContext + "\n\nJelaskan singkat ke user."})
			
			fmt.Printf("\n%s", colorGreen)
			followUp, _ := sendMessageStream(apiKey, history)
			fmt.Printf("%s", colorReset)
			
			if followUp != "" {
				history = append(history, ChatMessage{Role: "assistant", Content: followUp})
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
	case "/help":
		return `Commands:
  /read <file>  - Read file
  /ls [dir]     - List directory
  /run <cmd>    - Run command
  /find <name>  - Find files
  /grep <p>:<f> - Search in files
  /tree [dir]   - Show structure
  /edit <file>  - Edit file
  /cd <dir>     - Change directory
  /pwd          - Current directory
  /mode         - Cycle mode (auto/ask/manual)
  /clear        - Clear history
  exit          - Quit`
	case "/read", "/cat":
		return cmdRead(arg)
	case "/ls", "/dir":
		return cmdList(arg)
	case "/run", "/exec":
		return cmdRun(arg)
	case "/find":
		return cmdFind(arg)
	case "/grep":
		return cmdGrep(arg)
	case "/tree":
		return cmdTree(arg)
	case "/cd":
		return cmdCd(arg)
	case "/pwd":
		return fmt.Sprintf("Directory: %s", currentDir)
	case "/edit":
		return cmdEdit(arg, scanner)
	case "/clear":
		return "History cleared"
	default:
		return fmt.Sprintf("Unknown: %s (try /help)", cmd)
	}
}

func sendMessageStream(apiKey string, messages []ChatMessage) (string, error) {
	reqBody := ChatRequest{
		Model:     modelName,
		MaxTokens: 4096,
		Messages:  messages,
		Stream:    true,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", minimaxAPIURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var fullResponse strings.Builder
	reader := bufio.NewReader(resp.Body)

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
			var streamResp StreamResponse
			if json.Unmarshal([]byte(data), &streamResp) == nil && len(streamResp.Choices) > 0 {
				content := streamResp.Choices[0].Delta.Content
				if content != "" {
					fmt.Print(content)
					fullResponse.WriteString(content)
				}
			}
		}
	}

	return fullResponse.String(), nil
}
