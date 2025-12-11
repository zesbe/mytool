package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
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
)

var (
	version   = "2.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorPurple  = "\033[35m"
	colorCyan    = "\033[36m"
	colorGray    = "\033[90m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	clearLine    = "\033[2K\r"
)

const minimaxAPIURL = "https://api.minimax.io/v1/chat/completions"
const modelName = "MiniMax-Text-01"

// Modes
const (
	ModeAuto   = "auto"
	ModeAsk    = "ask" 
	ModeManual = "manual"
)

var (
	currentMode    = ModeAuto
	currentDir     string
	undoStack      []UndoAction
	totalTokens    int
	sessionFile    string
	projectType    string
	lastResponse   string
)

type UndoAction struct {
	Type    string
	Path    string
	Content string
	Desc    string
}

type StreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type StreamResponse struct {
	Choices []StreamChoice `json:"choices"`
	Usage   struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
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

type Session struct {
	Dir      string        `json:"dir"`
	Mode     string        `json:"mode"`
	History  []ChatMessage `json:"history"`
	Tokens   int           `json:"tokens"`
}

func main() {
	currentDir, _ = os.Getwd()
	detectProject()
	
	// Handle Ctrl+C gracefully
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("\n%sInterrupted. Bye!%s\n", colorYellow, colorReset)
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
	default:
		runChat(args)
	}
}

func detectProject() {
	projectType = "unknown"
	
	checks := map[string]string{
		"package.json": "nodejs",
		"go.mod":       "golang",
		"Cargo.toml":   "rust",
		"requirements.txt": "python",
		"pom.xml":      "java",
		"composer.json": "php",
		"Gemfile":      "ruby",
		"pubspec.yaml": "flutter",
	}
	
	for file, ptype := range checks {
		if _, err := os.Stat(filepath.Join(currentDir, file)); err == nil {
			projectType = ptype
			break
		}
	}
	
	// Check if git repo
	if _, err := os.Stat(filepath.Join(currentDir, ".git")); err == nil {
		if projectType == "unknown" {
			projectType = "git"
		}
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
	fmt.Printf("\n%smytool%s v%s - AI Terminal Assistant (Droid-like)\n\n", colorCyan, colorReset, version)
	fmt.Printf("%sUsage:%s mytool [message]\n\n", colorYellow, colorReset)
	
	fmt.Printf("%sFeatures:%s\n", colorYellow, colorReset)
	fmt.Println("  • AI with full system access (read/write/execute)")
	fmt.Println("  • Git integration (status, commit, diff, push)")
	fmt.Println("  • URL fetching and web content")
	fmt.Println("  • @file mentions to include files")
	fmt.Println("  • Multi-line input with \\")
	fmt.Println("  • Undo support for file changes")
	fmt.Println("  • Session save/resume")
	fmt.Println("  • Three modes: auto/ask/manual")
	
	fmt.Printf("\n%sCommands:%s\n", colorYellow, colorReset)
	fmt.Println("  /mode         - Cycle modes (auto→ask→manual)")
	fmt.Println("  /undo         - Undo last file change")
	fmt.Println("  /git <cmd>    - Run git command")
	fmt.Println("  /save         - Save session")
	fmt.Println("  /read <file>  - Read file")
	fmt.Println("  /edit <file>  - Edit file interactively")
	fmt.Println("  /run <cmd>    - Run shell command")
	fmt.Println("  /find <name>  - Find files")
	fmt.Println("  /ls [dir]     - List directory")
	fmt.Println("  /cd <dir>     - Change directory")
	fmt.Println("  /clear        - Clear history")
	fmt.Println("  /help         - Show this help")
	fmt.Println("  exit          - Exit mytool")
	
	fmt.Printf("\n%sResume:%s mytool resume\n\n", colorYellow, colorReset)
}

func printVersion() {
	fmt.Printf("%smytool%s v%s\n", colorCyan, colorReset, version)
	fmt.Printf("  Model:   %s\n", modelName)
	fmt.Printf("  OS:      %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Project: %s\n", projectType)
}

func getModeColor() string {
	switch currentMode {
	case ModeAuto:
		return colorGreen
	case ModeAsk:
		return colorYellow
	case ModeManual:
		return colorRed
	}
	return colorReset
}

func getModeDisplay() string {
	c := getModeColor()
	switch currentMode {
	case ModeAuto:
		return fmt.Sprintf("%sAuto%s", c, colorReset)
	case ModeAsk:
		return fmt.Sprintf("%sAsk%s", c, colorReset)
	case ModeManual:
		return fmt.Sprintf("%sManual%s", c, colorReset)
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
	tokens := fmt.Sprintf("%s%d tokens%s", colorGray, totalTokens, colorReset)
	proj := ""
	if projectType != "unknown" {
		proj = fmt.Sprintf(" %s[%s]%s", colorPurple, projectType, colorReset)
	}
	
	// Check git status
	gitStatus := ""
	if isGitRepo() {
		branch := getGitBranch()
		if branch != "" {
			gitStatus = fmt.Sprintf(" %s⎇ %s%s", colorBlue, branch, colorReset)
		}
	}
	
	fmt.Printf("\n%s │ %s │ %s%s%s\n", mode, tokens, currentDir, proj, gitStatus)
}

func isGitRepo() bool {
	_, err := os.Stat(filepath.Join(currentDir, ".git"))
	return err == nil
}

func getGitBranch() string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = currentDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ==================== FILE OPERATIONS ====================

func saveForUndo(path, desc string) {
	fullPath := resolvePath(path)
	content := ""
	if data, err := os.ReadFile(fullPath); err == nil {
		content = string(data)
	}
	undoStack = append(undoStack, UndoAction{
		Type:    "file",
		Path:    fullPath,
		Content: content,
		Desc:    desc,
	})
	// Keep only last 10 undos
	if len(undoStack) > 10 {
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
		// File was created, delete it
		os.Remove(action.Path)
		return fmt.Sprintf("Undone: removed %s", action.Path)
	} else {
		// Restore previous content
		os.WriteFile(action.Path, []byte(action.Content), 0644)
		return fmt.Sprintf("Undone: restored %s", action.Path)
	}
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
	
	// Add line numbers
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%sFile: %s (%d lines)%s\n", colorCyan, fullPath, len(lines), colorReset))
	result.WriteString(strings.Repeat("─", 50) + "\n")
	
	maxLines := 150
	for i, line := range lines {
		if i >= maxLines {
			result.WriteString(fmt.Sprintf("%s... (%d more lines)%s\n", colorGray, len(lines)-maxLines, colorReset))
			break
		}
		result.WriteString(fmt.Sprintf("%s%4d│%s %s\n", colorGray, i+1, colorReset, line))
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
	result.WriteString(strings.Repeat("─", 50) + "\n")
	
	// Sort: dirs first, then files
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	
	for _, entry := range dirs {
		result.WriteString(fmt.Sprintf("%s📁 %s/%s\n", colorBlue, entry.Name(), colorReset))
	}
	for _, entry := range files {
		info, _ := entry.Info()
		size := ""
		if info != nil {
			size = formatSize(info.Size())
		}
		icon := getFileIcon(entry.Name())
		result.WriteString(fmt.Sprintf("%s %s %s%s%s\n", icon, entry.Name(), colorGray, size, colorReset))
	}
	
	result.WriteString(fmt.Sprintf("%s\n%d dirs, %d files%s\n", colorGray, len(dirs), len(files), colorReset))
	return result.String()
}

func getFileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	icons := map[string]string{
		".go":   "🔵", ".js":  "🟡", ".ts":  "🔷", ".py":  "🐍",
		".rs":   "🦀", ".rb":  "💎", ".java": "☕", ".php": "🐘",
		".html": "🌐", ".css": "🎨", ".json": "📋", ".md":  "📝",
		".yml":  "⚙️", ".yaml": "⚙️", ".sh":  "📜", ".sql": "🗃️",
		".jpg":  "🖼️", ".png": "🖼️", ".gif": "🖼️", ".svg": "🖼️",
		".mp3":  "🎵", ".mp4": "🎬", ".pdf": "📕", ".zip": "📦",
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
		return fmt.Sprintf("%s[blocked] Manual mode - command not executed%s", colorRed, colorReset)
	}
	
	if currentMode == ModeAsk {
		fmt.Printf("%sRun: %s%s ? [y/N] ", colorYellow, command, colorReset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			return "Cancelled"
		}
	}
	
	fmt.Printf("%s$ %s%s\n", colorGray, command, colorReset)
	
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
		result += fmt.Sprintf("\n%sExit: %s%s", colorRed, err, colorReset)
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
		return fmt.Sprintf("Error: not a directory")
	}
	
	currentDir = newPath
	detectProject()
	return fmt.Sprintf("→ %s", currentDir)
}

func cmdFind(pattern string) string {
	if pattern == "" {
		return "Usage: /find <pattern>"
	}
	
	cmd := exec.Command("find", currentDir, "-maxdepth", "5", "-iname", "*"+pattern+"*", "-not", "-path", "*/node_modules/*", "-not", "-path", "*/.git/*")
	output, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	
	if result == "" {
		return fmt.Sprintf("No files found: %s", pattern)
	}
	
	lines := strings.Split(result, "\n")
	if len(lines) > 30 {
		result = strings.Join(lines[:30], "\n") + fmt.Sprintf("\n%s... +%d more%s", colorGray, len(lines)-30, colorReset)
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
	
	cmd := exec.Command("grep", "-r", "-n", "-i", "--include=*.*", "--exclude-dir=node_modules", "--exclude-dir=.git", pattern, searchPath)
	output, _ := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	
	if result == "" {
		return fmt.Sprintf("No matches: %s", pattern)
	}
	
	lines := strings.Split(result, "\n")
	if len(lines) > 20 {
		result = strings.Join(lines[:20], "\n") + fmt.Sprintf("\n%s... +%d more%s", colorGray, len(lines)-20, colorReset)
	}
	
	return fmt.Sprintf("%sMatches: %d%s\n%s", colorGreen, len(lines), colorReset, result)
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
	
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	
	var filtered []os.DirEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
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
			result.WriteString(fmt.Sprintf("%s%s%s%s/%s\n", prefix, connector, colorBlue, entry.Name(), colorReset))
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

func cmdWrite(args string) string {
	parts := strings.SplitN(args, "|||", 2)
	if len(parts) < 2 {
		return "Error: format path|||content"
	}
	
	path := strings.TrimSpace(parts[0])
	content := parts[1]
	fullPath := resolvePath(path)
	
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked] Manual mode%s", colorRed, colorReset)
	}
	
	if currentMode == ModeAsk {
		fmt.Printf("%sWrite to %s?%s [y/N] ", colorYellow, fullPath, colorReset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			return "Cancelled"
		}
	}
	
	// Save for undo
	saveForUndo(path, "write "+path)
	
	// Create dir if needed
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	
	return fmt.Sprintf("%s✓ Written: %s (%d bytes)%s", colorGreen, fullPath, len(content), colorReset)
}

func cmdReplace(args string) string {
	parts := strings.SplitN(args, "|||", 3)
	if len(parts) < 3 {
		return "Error: format path|||old|||new"
	}
	
	path := strings.TrimSpace(parts[0])
	oldText := parts[1]
	newText := parts[2]
	fullPath := resolvePath(path)
	
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked] Manual mode%s", colorRed, colorReset)
	}
	
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	
	content := string(data)
	if !strings.Contains(content, oldText) {
		return "Error: text not found"
	}
	
	// Show diff
	fmt.Printf("%s--- %s%s\n", colorRed, fullPath, colorReset)
	fmt.Printf("%s- %s%s\n", colorRed, truncate(oldText, 100), colorReset)
	fmt.Printf("%s+ %s%s\n", colorGreen, truncate(newText, 100), colorReset)
	
	if currentMode == ModeAsk {
		fmt.Printf("%sApply change?%s [y/N] ", colorYellow, colorReset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			return "Cancelled"
		}
	}
	
	saveForUndo(path, "replace in "+path)
	
	newContent := strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	
	return fmt.Sprintf("%s✓ Replaced in: %s%s", colorGreen, fullPath, colorReset)
}

func cmdAppend(args string) string {
	parts := strings.SplitN(args, "|||", 2)
	if len(parts) < 2 {
		return "Error: format path|||content"
	}
	
	path := strings.TrimSpace(parts[0])
	content := parts[1]
	fullPath := resolvePath(path)
	
	if currentMode == ModeManual {
		return fmt.Sprintf("%s[blocked] Manual mode%s", colorRed, colorReset)
	}
	
	saveForUndo(path, "append to "+path)
	
	f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("Error: %s", err)
	}
	defer f.Close()
	
	f.WriteString(content)
	return fmt.Sprintf("%s✓ Appended to: %s%s", colorGreen, fullPath, colorReset)
}

// ==================== GIT ====================

func cmdGit(args string) string {
	if !isGitRepo() {
		return "Not a git repository"
	}
	
	if args == "" {
		args = "status"
	}
	
	cmd := exec.Command("sh", "-c", "git "+args)
	cmd.Dir = currentDir
	output, err := cmd.CombinedOutput()
	
	result := string(output)
	if err != nil {
		result += fmt.Sprintf("\n%sError: %s%s", colorRed, err, colorReset)
	}
	return result
}

// ==================== WEB ====================

func cmdFetch(url string) string {
	if url == "" {
		return "Usage: fetch <url>"
	}
	
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
	
	// Truncate if too long
	if len(content) > 5000 {
		content = content[:5000] + "\n... (truncated)"
	}
	
	return fmt.Sprintf("%sURL: %s (%d bytes)%s\n%s", colorCyan, url, len(body), colorReset, content)
}

// ==================== SESSION ====================

func saveSession(history []ChatMessage) {
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".mytool")
	os.MkdirAll(sessionDir, 0755)
	
	// Use dir hash as session name
	hash := fmt.Sprintf("%x", md5.Sum([]byte(currentDir)))[:8]
	sessionFile = filepath.Join(sessionDir, "session_"+hash+".json")
	
	session := Session{
		Dir:     currentDir,
		Mode:    currentMode,
		History: history,
		Tokens:  totalTokens,
	}
	
	data, _ := json.MarshalIndent(session, "", "  ")
	os.WriteFile(sessionFile, data, 0644)
	
	fmt.Printf("%s✓ Session saved: %s%s\n", colorGreen, sessionFile, colorReset)
}

func loadSession() ([]ChatMessage, bool) {
	home, _ := os.UserHomeDir()
	hash := fmt.Sprintf("%x", md5.Sum([]byte(currentDir)))[:8]
	sessionFile = filepath.Join(home, ".mytool", "session_"+hash+".json")
	
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, false
	}
	
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, false
	}
	
	currentMode = session.Mode
	totalTokens = session.Tokens
	
	return session.History, true
}

func resumeSession() {
	history, ok := loadSession()
	if !ok {
		fmt.Printf("%sNo session found for this directory%s\n", colorYellow, colorReset)
		runChat([]string{})
		return
	}
	
	fmt.Printf("%s✓ Resumed session (%d messages, %d tokens)%s\n", colorGreen, len(history), totalTokens, colorReset)
	runChatWithHistory(history)
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
	
	var fileContents []string
	for _, match := range matches {
		filename := match[1]
		fullPath := resolvePath(filename)
		
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		
		content := string(data)
		lines := strings.Split(content, "\n")
		if len(lines) > 100 {
			content = strings.Join(lines[:100], "\n") + fmt.Sprintf("\n... (%d more)", len(lines)-100)
		}
		
		fileContents = append(fileContents, fmt.Sprintf("=== %s ===\n%s", fullPath, content))
		fmt.Printf("%s  ✓ @%s%s\n", colorGray, filename, colorReset)
	}
	
	if len(fileContents) > 0 {
		return input + "\n\n" + strings.Join(fileContents, "\n\n")
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

// ==================== TOOL PARSING ====================

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
		default:
			result = fmt.Sprintf("Unknown tool: %s", toolName)
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
	
	gitInfo := ""
	if isGitRepo() {
		gitInfo = fmt.Sprintf("\n- Git branch: %s", getGitBranch())
	}
	
	return fmt.Sprintf(`Kamu adalah mytool v%s, AI assistant terminal dengan akses penuh ke sistem.

SISTEM:
- Host: %s | OS: %s/%s | User: %s
- Dir: %s
- Project: %s%s
- Mode: %s

TOOLS (format: <tool>nama:arg</tool>):
READ/BROWSE:
- <tool>read:file</tool> - Baca file dengan line numbers
- <tool>ls:dir</tool> - List direktori
- <tool>tree:dir</tool> - Struktur folder
- <tool>find:nama</tool> - Cari file
- <tool>grep:pattern path</tool> - Cari teks

WRITE/EDIT:
- <tool>write:path|||content</tool> - Tulis file baru
- <tool>replace:path|||old|||new</tool> - Ganti teks
- <tool>append:path|||content</tool> - Tambah ke file

EXECUTE:
- <tool>run:command</tool> - Jalankan command shell
- <tool>git:command</tool> - Jalankan git command

WEB:
- <tool>fetch:url</tool> - Ambil konten dari URL

NAVIGATION:
- <tool>cd:path</tool> - Pindah direktori

ATURAN PENTING:
1. LANGSUNG gunakan tools - jangan suruh user melakukan sendiri
2. Untuk edit file: baca dulu, lalu gunakan replace dengan exact text
3. Untuk buat file: gunakan write dengan full content
4. Jelaskan singkat apa yang kamu lakukan
5. Bahasa Indonesia jika user pakai Indonesia
6. Tampilkan diff/perubahan sebelum edit`, 
		version, hostname, runtime.GOOS, runtime.GOARCH, os.Getenv("USER"), 
		currentDir, projectType, gitInfo, currentMode)
}

func runChat(args []string) {
	apiKey := getAPIKey()
	if apiKey == "" {
		fmt.Printf("\n%smytool%s - Setup\n\n", colorCyan, colorReset)
		fmt.Println("API key required. Get one at: https://platform.minimax.io/")
		fmt.Printf("\nEnter API Key: ")
		
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				saveAPIKey(apiKey)
				fmt.Printf("%s✓ Saved%s\n\n", colorGreen, colorReset)
			}
		}
		if apiKey == "" {
			os.Exit(1)
		}
	}

	// Single message mode
	if len(args) > 0 {
		msg := processAtMentions(strings.Join(args, " "))
		messages := []ChatMessage{
			{Role: "system", Content: getSystemPrompt()},
			{Role: "user", Content: msg},
		}
		fmt.Printf("%s", colorGreen)
		response, _ := sendStream(apiKey, messages)
		fmt.Printf("%s\n", colorReset)
		
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
	fmt.Printf("\nENTER send • \\ newline • @file include • /help commands\n")
	printStatusBar()
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	hints := []string{
		"\"List all files here\"",
		"\"Read and explain package.json\"",
		"\"Find all TODO comments\"",
		"\"Create a hello.py file\"",
		"\"What's the git status?\"",
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

		// Commands
		if input == "exit" || input == "quit" {
			fmt.Printf("%sBye! 👋%s\n", colorCyan, colorReset)
			break
		}

		if input == "/mode" {
			cycleMode()
			history[0] = ChatMessage{Role: "system", Content: getSystemPrompt()}
			fmt.Printf("Mode: %s\n\n", getModeDisplay())
			continue
		}

		if input == "/undo" {
			fmt.Println(doUndo())
			fmt.Println()
			continue
		}

		if input == "/save" {
			saveSession(history)
			fmt.Println()
			continue
		}

		if strings.HasPrefix(input, "/") {
			result := handleCommand(input, scanner)
			fmt.Println(result)
			fmt.Println()
			continue
		}

		// Process @mentions
		input = processAtMentions(input)

		// Send to AI
		history = append(history, ChatMessage{Role: "user", Content: input})
		
		// Show thinking indicator
		fmt.Printf("%s⠋ Thinking...%s", colorGray, colorReset)
		
		fmt.Printf("\r%s              %s\r", clearLine, colorReset)
		fmt.Printf("%s", colorGreen)
		response, _ := sendStream(apiKey, history)
		fmt.Printf("%s", colorReset)
		lastResponse = response
		
		// Parse tools
		_, results := parseAndExecuteTools(response)
		
		if len(results) > 0 {
			fmt.Printf("\n\n%s─── Executing ───%s\n", colorCyan, colorReset)
			for _, r := range results {
				fmt.Println(r)
			}
			fmt.Printf("%s─────────────────%s\n", colorCyan, colorReset)
			
			history = append(history, ChatMessage{Role: "assistant", Content: response})
			
			// Follow up
			history = append(history, ChatMessage{
				Role: "user", 
				Content: "Results:\n" + strings.Join(results, "\n") + "\n\nJelaskan singkat.",
			})
			
			fmt.Printf("\n%s", colorGreen)
			followUp, _ := sendStream(apiKey, history)
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
	case "/help", "/?":
		return `/read <f>  - Read file
/ls [d]    - List directory  
/run <c>   - Run command
/find <n>  - Find files
/grep <p>  - Search in files
/tree [d]  - Show structure
/git <c>   - Git command
/cd <d>    - Change directory
/edit <f>  - Edit file
/mode      - Cycle mode
/undo      - Undo last change
/save      - Save session
/clear     - Clear history
exit       - Quit`
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
		return "History cleared"
	default:
		return fmt.Sprintf("Unknown: %s", cmd)
	}
}

func cmdEdit(path string, scanner *bufio.Scanner) string {
	if path == "" {
		return "Usage: /edit <file>"
	}
	fullPath := resolvePath(path)
	
	// Show current content
	if data, err := os.ReadFile(fullPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i >= 20 {
				fmt.Printf("%s... (%d more)%s\n", colorGray, len(lines)-20, colorReset)
				break
			}
			fmt.Printf("%s%3d│%s %s\n", colorGray, i+1, colorReset, line)
		}
	} else {
		fmt.Printf("%sNew file: %s%s\n", colorYellow, fullPath, colorReset)
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
			saveForUndo(path, "edit "+path)
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			if err := os.WriteFile(fullPath, []byte(content.String()), 0644); err != nil {
				return fmt.Sprintf("Error: %s", err)
			}
			return fmt.Sprintf("%s✓ Saved: %s%s", colorGreen, fullPath, colorReset)
		}
		if line == "/cancel" {
			return "Cancelled"
		}
		content.WriteString(line + "\n")
	}
	return "Cancelled"
}

func sendStream(apiKey string, messages []ChatMessage) (string, error) {
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

	var full strings.Builder
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
			var sr StreamResponse
			if json.Unmarshal([]byte(data), &sr) == nil && len(sr.Choices) > 0 {
				content := sr.Choices[0].Delta.Content
				if content != "" {
					fmt.Print(content)
					full.WriteString(content)
				}
				if sr.Usage.TotalTokens > 0 {
					totalTokens = sr.Usage.TotalTokens
				}
			}
		}
	}

	return full.String(), nil
}
