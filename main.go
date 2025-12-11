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
)

const minimaxAPIURL = "https://api.minimax.io/v1/chat/completions"

type StreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
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

var currentDir string

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
	case "info", "sysinfo":
		printSystemInfo()
	case "ip":
		printIP()
	case "time":
		printTime()
	case "env":
		printEnv()
	case "disk":
		printDisk()
	case "chat", "ai":
		runChat(cmdArgs)
	case "help", "-h", "--help":
		printHelp()
	default:
		runChat(args)
	}
}

func printHelp() {
	fmt.Printf("\n%smytool%s v%s - AI Assistant dengan akses sistem\n\n", colorGreen, colorReset, version)
	fmt.Printf("%sUsage:%s mytool [command]\n\n", colorYellow, colorReset)
	fmt.Printf("%sCommands:%s\n", colorYellow, colorReset)
	fmt.Println("  (tanpa args)  Masuk mode chat interaktif")
	fmt.Println("  \"pesan\"       Kirim pesan langsung ke AI")
	fmt.Println("  help          Tampilkan bantuan ini")
	fmt.Println("  version       Tampilkan versi")
	fmt.Println("  info          Info sistem")
	fmt.Printf("\n%sChat Commands:%s\n", colorYellow, colorReset)
	fmt.Println("  /read <file>      Baca isi file")
	fmt.Println("  /ls [dir]         List direktori")
	fmt.Println("  /edit <file>      Edit file (interactive)")
	fmt.Println("  /run <cmd>        Jalankan command")
	fmt.Println("  /cd <dir>         Pindah direktori")
	fmt.Println("  /pwd              Tampilkan direktori saat ini")
	fmt.Println("  /clear            Hapus history chat")
	fmt.Println("  /help             Tampilkan bantuan")
	fmt.Println("  exit              Keluar")
	fmt.Println()
}

func printVersion() {
	fmt.Printf("%smytool%s version %s%s%s\n", colorCyan, colorReset, colorGreen, version, colorReset)
	fmt.Printf("  Build time: %s\n", buildTime)
	fmt.Printf("  Git commit: %s\n", gitCommit)
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printSystemInfo() {
	fmt.Printf("\n%s[ System Information ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("-", 40))
	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()
	fmt.Printf("%sHostname:%s     %s\n", colorYellow, colorReset, hostname)
	fmt.Printf("%sOS:%s           %s/%s\n", colorYellow, colorReset, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("%sCPUs:%s         %d\n", colorYellow, colorReset, runtime.NumCPU())
	fmt.Printf("%sWorking Dir:%s  %s\n", colorYellow, colorReset, wd)
	fmt.Printf("%sUser:%s         %s\n", colorYellow, colorReset, os.Getenv("USER"))
	fmt.Printf("%sHome:%s         %s\n", colorYellow, colorReset, os.Getenv("HOME"))
	fmt.Println()
}

func printIP() {
	fmt.Printf("%sFetching public IP...%s\n", colorYellow, colorReset)
	cmd := exec.Command("curl", "-s", "-m", "5", "https://ifconfig.me")
	output, err := cmd.Output()
	if err == nil {
		fmt.Printf("%sPublic IP:%s %s%s%s\n", colorCyan, colorReset, colorGreen, strings.TrimSpace(string(output)), colorReset)
	} else {
		fmt.Printf("%sCould not fetch IP%s\n", colorRed, colorReset)
	}
}

func printTime() {
	now := time.Now()
	fmt.Printf("\n%s[ Current Time ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("%sLocal:%s    %s\n", colorYellow, colorReset, now.Format("2006-01-02 15:04:05"))
	fmt.Printf("%sUTC:%s      %s\n", colorYellow, colorReset, now.UTC().Format("2006-01-02 15:04:05"))
	fmt.Printf("%sUnix:%s     %d\n", colorYellow, colorReset, now.Unix())
	fmt.Println()
}

func printEnv() {
	fmt.Printf("\n%s[ Environment ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("-", 40))
	for _, key := range []string{"PATH", "HOME", "USER", "SHELL", "TERM"} {
		if v := os.Getenv(key); v != "" {
			if len(v) > 50 {
				v = v[:47] + "..."
			}
			fmt.Printf("%s%s:%s %s\n", colorYellow, key, colorReset, v)
		}
	}
	fmt.Println()
}

func printDisk() {
	fmt.Printf("\n%s[ Disk Usage ]%s\n", colorCyan, colorReset)
	cmd := exec.Command("df", "-h")
	output, _ := cmd.Output()
	fmt.Println(string(output))
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

func cmdEdit(path string, scanner *bufio.Scanner) string {
	if path == "" {
		return "Usage: /edit <file>"
	}
	fullPath := resolvePath(path)
	
	// Read existing content
	var content string
	if data, err := os.ReadFile(fullPath); err == nil {
		content = string(data)
		fmt.Printf("%sFile exists. Current content:%s\n", colorYellow, colorReset)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if i >= 20 {
				fmt.Printf("%s... (%d more lines)%s\n", colorGray, len(lines)-20, colorReset)
				break
			}
			fmt.Printf("%s%3d:%s %s\n", colorGray, i+1, colorReset, line)
		}
	} else {
		fmt.Printf("%sCreating new file: %s%s\n", colorYellow, fullPath, colorReset)
	}
	
	fmt.Printf("\n%sEnter new content (type '/save' on new line to save, '/cancel' to cancel):%s\n", colorYellow, colorReset)
	
	var newContent strings.Builder
	for {
		fmt.Printf("%s|%s ", colorGray, colorReset)
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if line == "/save" {
			if err := os.WriteFile(fullPath, []byte(newContent.String()), 0644); err != nil {
				return fmt.Sprintf("Error saving: %s", err)
			}
			return fmt.Sprintf("%sFile saved: %s%s", colorGreen, fullPath, colorReset)
		}
		if line == "/cancel" {
			return "Edit cancelled"
		}
		newContent.WriteString(line + "\n")
	}
	return "Edit cancelled"
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
	return fmt.Sprintf(`Kamu adalah mytool, AI assistant yang berjalan di terminal dengan akses ke sistem.

Info Sistem:
- Hostname: %s
- OS: %s/%s
- User: %s
- Working Directory: %s

Kamu bisa membantu user dengan:
- Menjawab pertanyaan programming dan teknis
- Menjelaskan kode dan konsep
- Membantu debugging
- Memberikan saran untuk file dan direktori

User bisa menggunakan command seperti /read, /ls, /edit, /run untuk operasi file.
Berikan respons yang ringkas dan helpful dalam Bahasa Indonesia jika user berbicara Indonesia.`, 
		hostname, runtime.GOOS, runtime.GOARCH, os.Getenv("USER"), currentDir)
}

func runChat(args []string) {
	apiKey := getAPIKey()
	if apiKey == "" {
		fmt.Printf("\n%smytool%s - Setup\n\n", colorGreen, colorReset)
		fmt.Println("API key belum di-set. Dapatkan di: https://platform.minimax.io/")
		fmt.Printf("\nMasukkan API Key: ")
		
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				saveAPIKey(apiKey)
				fmt.Printf("%sAPI key tersimpan!%s\n\n", colorGreen, colorReset)
			}
		}
		if apiKey == "" {
			fmt.Printf("%sAPI key diperlukan%s\n", colorRed, colorReset)
			os.Exit(1)
		}
	}

	// Single message mode
	if len(args) > 0 {
		message := strings.Join(args, " ")
		messages := []ChatMessage{
			{Role: "system", Content: getSystemPrompt()},
			{Role: "user", Content: message},
		}
		fmt.Printf("%s", colorGreen)
		sendMessageStream(apiKey, messages)
		fmt.Printf("%s\n", colorReset)
		return
	}

	// Interactive mode
	fmt.Printf("\n%smytool%s - AI Assistant\n", colorGreen, colorReset)
	fmt.Printf("%sKetik /help untuk bantuan, 'exit' untuk keluar%s\n", colorGray, colorReset)
	fmt.Printf("%sDir: %s%s\n\n", colorGray, currentDir, colorReset)

	history := []ChatMessage{
		{Role: "system", Content: getSystemPrompt()},
	}
	scanner := bufio.NewScanner(os.Stdin)
	// Increase scanner buffer for large inputs
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for {
		fmt.Printf("%s> %s", colorYellow, colorReset)
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Exit commands
		if input == "exit" || input == "quit" || input == "/exit" {
			fmt.Printf("%sBye!%s\n", colorCyan, colorReset)
			break
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			result := handleCommand(input, scanner)
			if result != "" {
				fmt.Println(result)
				// Add command result to context for AI
				if !strings.HasPrefix(input, "/help") && !strings.HasPrefix(input, "/clear") {
					history = append(history, ChatMessage{
						Role:    "user",
						Content: fmt.Sprintf("[User executed: %s]\n%s", input, result),
					})
				}
			}
			fmt.Println()
			continue
		}

		// Send to AI
		history = append(history, ChatMessage{Role: "user", Content: input})
		
		fmt.Printf("%s", colorGreen)
		response, err := sendMessageStream(apiKey, history)
		fmt.Printf("%s", colorReset)
		
		if err != nil {
			fmt.Printf("\n%sError: %s%s\n", colorRed, err, colorReset)
			history = history[:len(history)-1]
		} else {
			fmt.Printf("\n\n")
			history = append(history, ChatMessage{Role: "assistant", Content: response})
		}
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
  /read <file>   - Baca file
  /ls [dir]      - List direktori
  /edit <file>   - Edit file
  /run <cmd>     - Jalankan command
  /cd <dir>      - Pindah direktori
  /pwd           - Direktori saat ini
  /clear         - Hapus history
  exit           - Keluar`
	case "/read", "/cat":
		return cmdRead(arg)
	case "/ls", "/dir":
		return cmdList(arg)
	case "/run", "/exec", "/$":
		return cmdRun(arg)
	case "/cd":
		return cmdCd(arg)
	case "/pwd":
		return fmt.Sprintf("Current directory: %s", currentDir)
	case "/edit", "/nano", "/vi":
		return cmdEdit(arg, scanner)
	case "/clear":
		return "History cleared"
	default:
		return fmt.Sprintf("Unknown command: %s (type /help)", cmd)
	}
}

func sendMessageStream(apiKey string, messages []ChatMessage) (string, error) {
	reqBody := ChatRequest{
		Model:     "MiniMax-Text-01",
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
