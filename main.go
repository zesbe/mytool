package main

import (
	"encoding/json"
	"fmt"
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
)

func main() {
	args := os.Args[1:]

	// Termux/Android bug: os.Args may contain duplicate executable path
	// Detect and skip if args[0] looks like a path to this executable
	if len(args) > 0 && (strings.HasSuffix(args[0], "/mytool") || strings.HasSuffix(args[0], "\\mytool.exe")) {
		args = args[1:]
	}

	if len(args) < 1 {
		printHelp()
		return
	}

	command := args[0]

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
	fmt.Println("Usage: mytool <command>")
	fmt.Println()
	fmt.Printf("%sAvailable Commands:%s\n", colorYellow, colorReset)
	fmt.Println("  version    Show version information")
	fmt.Println("  info       Display system information")
	fmt.Println("  ip         Show public IP address")
	fmt.Println("  time       Display current time in multiple formats")
	fmt.Println("  env        List environment variables")
	fmt.Println("  disk       Show disk usage")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Printf("%sExamples:%s\n", colorYellow, colorReset)
	fmt.Println("  mytool version")
	fmt.Println("  mytool info")
	fmt.Println("  mytool ip")
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

	// Try multiple services
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

	// Show important env vars
	important := []string{"PATH", "HOME", "USER", "SHELL", "LANG", "TERM", "EDITOR", "GOPATH", "GOROOT"}

	for _, key := range important {
		value := os.Getenv(key)
		if value != "" {
			// Truncate long values
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

// Helper for JSON output (can be extended)
func toJSON(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
