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

// Streaming response structures
type StreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type StreamResponse struct {
	Choices []StreamChoice `json:"choices"`
}

func main() {
	args := os.Args[1:]

	// Termux/Android bug: os.Args may contain duplicate executable path
	if len(args) > 0 && (strings.HasSuffix(args[0], "/mytool") || strings.HasSuffix(args[0], "\\mytool.exe")) {
		args = args[1:]
	}

	if len(args) < 1 {
		printHelp()
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
		fmt.Printf("%sUnknown command: %s%s\n", colorRed, command, colorReset)
		fmt.Println("Run 'mytool help' for usage.")
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println()
	fmt.Printf("%s╔═══════════════════════════════════════╗%s\n", colorCyan, colorReset)
	fmt.Printf("%s║           mytool CLI v%-16s║%s\n", colorCyan, version, colorReset)
	fmt.Printf("%s╚═══════════════════════════════════════╝%s\n", colorCyan, colorReset)
	fmt.Println()
	fmt.Println("Usage: mytool <command> [arguments]")
	fmt.Println()
	fmt.Printf("%sAvailable Commands:%s\n", colorYellow, colorReset)
	fmt.Println("  version       Show version information")
	fmt.Println("  info          Display system information")
	fmt.Println("  ip            Show public IP address")
	fmt.Println("  time          Display current time in multiple formats")
	fmt.Println("  env           List environment variables")
	fmt.Println("  disk          Show disk usage")
	fmt.Println("  chat [msg]    Chat with Minimax AI")
	fmt.Println("  help          Show this help message")
	fmt.Println()
	fmt.Printf("%sChat Examples:%s\n", colorYellow, colorReset)
	fmt.Println("  mytool chat                     # Interactive mode")
	fmt.Println("  mytool chat \"Hello!\"            # Single message")
	fmt.Println()
	fmt.Printf("%sEnvironment:%s\n", colorYellow, colorReset)
	fmt.Println("  MINIMAX_API_KEY    Required for chat command")
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
	fmt.Println()
	fmt.Printf("%s[ System Information ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("─", 40))

	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()

	info := map[string]string{
		"Hostname":     hostname,
		"OS":           runtime.GOOS,
		"Architecture": runtime.GOARCH,
		"CPUs":         fmt.Sprintf("%d", runtime.NumCPU()),
		"Go Version":   runtime.Version(),
		"Working Dir":  wd,
		"User":         os.Getenv("USER"),
		"Home":         os.Getenv("HOME"),
		"Shell":        os.Getenv("SHELL"),
	}

	for key, value := range info {
		if value != "" {
			fmt.Printf("%s%-14s%s %s\n", colorYellow, key+":", colorReset, value)
		}
	}
	fmt.Println()
}

func printIP() {
	fmt.Printf("%sFetching public IP...%s\n", colorYellow, colorReset)

	services := []string{
		"https://ifconfig.me",
		"https://api.ipify.org",
		"https://icanhazip.com",
	}

	for _, service := range services {
		cmd := exec.Command("curl", "-s", "-m", "5", service)
		output, err := cmd.Output()
		if err == nil {
			ip := strings.TrimSpace(string(output))
			if ip != "" {
				fmt.Printf("%sPublic IP:%s %s%s%s\n", colorCyan, colorReset, colorGreen, ip, colorReset)
				return
			}
		}
	}

	fmt.Printf("%sCould not fetch public IP%s\n", colorRed, colorReset)
}

func printTime() {
	now := time.Now()

	fmt.Println()
	fmt.Printf("%s[ Current Time ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("─", 40))

	year, week := now.ISOWeek()
	formats := map[string]string{
		"Local":       now.Format("2006-01-02 15:04:05"),
		"UTC":         now.UTC().Format("2006-01-02 15:04:05 MST"),
		"RFC3339":     now.Format(time.RFC3339),
		"Unix":        fmt.Sprintf("%d", now.Unix()),
		"Unix Milli":  fmt.Sprintf("%d", now.UnixMilli()),
		"ISO Week":    fmt.Sprintf("%d-W%02d", year, week),
		"Day of Year": fmt.Sprintf("%d", now.YearDay()),
	}

	for key, value := range formats {
		fmt.Printf("%s%-12s%s %s\n", colorYellow, key+":", colorReset, value)
	}
	fmt.Println()
}

func printEnv() {
	fmt.Println()
	fmt.Printf("%s[ Environment Variables ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("─", 40))

	important := []string{"PATH", "HOME", "USER", "SHELL", "LANG", "TERM", "EDITOR", "GOPATH", "GOROOT"}

	for _, key := range important {
		value := os.Getenv(key)
		if value != "" {
			if len(value) > 60 {
				value = value[:57] + "..."
			}
			fmt.Printf("%s%-10s%s %s\n", colorYellow, key+":", colorReset, value)
		}
	}

	fmt.Printf("\n%sTotal: %d environment variables%s\n", colorCyan, len(os.Environ()), colorReset)
	fmt.Println()
}

func printDisk() {
	fmt.Println()
	fmt.Printf("%s[ Disk Usage ]%s\n", colorCyan, colorReset)
	fmt.Println(strings.Repeat("─", 40))

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("wmic", "logicaldisk", "get", "size,freespace,caption")
	} else {
		cmd = exec.Command("df", "-h")
	}

	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("%sError getting disk info: %s%s\n", colorRed, err, colorReset)
		return
	}

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if i == 0 {
			fmt.Printf("%s%s%s\n", colorYellow, line, colorReset)
		} else {
			fmt.Println(line)
		}
	}
	fmt.Println()
}

// ==================== MINIMAX CHAT ====================

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

func runChat(args []string) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		fmt.Printf("%sError: MINIMAX_API_KEY environment variable not set%s\n", colorRed, colorReset)
		fmt.Println()
		fmt.Println("To get an API key:")
		fmt.Println("  1. Go to https://platform.minimax.io/")
		fmt.Println("  2. Create an account or sign in")
		fmt.Println("  3. Go to API Keys and create a new key")
		fmt.Println()
		fmt.Println("Then set the key:")
		fmt.Printf("  export MINIMAX_API_KEY=\"your-api-key\"\n")
		fmt.Println()
		os.Exit(1)
	}

	// If message provided as argument, send single message with streaming
	if len(args) > 0 {
		message := strings.Join(args, " ")
		fmt.Printf("%s", colorGreen)
		_, err := sendMessageStream(apiKey, []ChatMessage{{Role: "user", Content: message}})
		fmt.Printf("%s\n", colorReset)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		return
	}

	// Interactive mode
	fmt.Println()
	fmt.Printf("%s╔═══════════════════════════════════════╗%s\n", colorCyan, colorReset)
	fmt.Printf("%s║         Minimax AI Chat               ║%s\n", colorCyan, colorReset)
	fmt.Printf("%s╚═══════════════════════════════════════╝%s\n", colorCyan, colorReset)
	fmt.Printf("%sType 'exit' or 'quit' to end the conversation%s\n", colorGray, colorReset)
	fmt.Println()

	var history []ChatMessage
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("%sYou: %s", colorYellow, colorReset)
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" || input == "/exit" || input == "/quit" {
			fmt.Printf("%sGoodbye!%s\n", colorCyan, colorReset)
			break
		}

		if input == "/clear" {
			history = nil
			fmt.Printf("%sConversation cleared.%s\n", colorGray, colorReset)
			continue
		}

		if input == "/help" {
			fmt.Println()
			fmt.Printf("%sCommands:%s\n", colorYellow, colorReset)
			fmt.Println("  /clear  - Clear conversation history")
			fmt.Println("  /help   - Show this help")
			fmt.Println("  /exit   - Exit chat")
			fmt.Println()
			continue
		}

		history = append(history, ChatMessage{Role: "user", Content: input})

		fmt.Printf("%sAI: %s%s", colorCyan, colorReset, colorGreen)
		response, err := sendMessageStream(apiKey, history)
		fmt.Printf("%s", colorReset)
		if err != nil {
			fmt.Printf("\n%sError: %s%s\n", colorRed, err, colorReset)
			history = history[:len(history)-1]
			continue
		}

		fmt.Printf("\n\n")
		history = append(history, ChatMessage{Role: "assistant", Content: response})
	}
}

func sendMessageStream(apiKey string, messages []ChatMessage) (string, error) {
	reqBody := ChatRequest{
		Model:     "MiniMax-Text-01",
		MaxTokens: 4096,
		Messages:  messages,
		Stream:    true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", minimaxAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var fullResponse strings.Builder
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fullResponse.String(), nil
		}

		line = strings.TrimSpace(line)
		if line == "" || line == "data: [DONE]" {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			var streamResp StreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
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

func sendMessage(apiKey string, messages []ChatMessage) (string, error) {
	reqBody := ChatRequest{
		Model:     "MiniMax-Text-01",
		MaxTokens: 4096,
		Messages:  messages,
		Stream:    false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", minimaxAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return chatResp.Choices[0].Message.Content, nil
}
