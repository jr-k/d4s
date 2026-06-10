package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jr-k/d4s/internal/buildinfo"
	"github.com/jr-k/d4s/internal/config"
	"github.com/jr-k/d4s/internal/secrets"
	"github.com/jr-k/d4s/internal/ui"
	"github.com/jr-k/d4s/internal/ui/common"
)

var hexColorRe = regexp.MustCompile(`\[#([0-9a-fA-F]{6})\]`)

func printColored(format string, a ...interface{}) {
	colorMap := map[string]string{
		"[orange]": "\x1b[38;2;255;184;108m",
		"[cyan]":   "\x1b[38;2;57;166;255m",
		"[white]":  "\x1b[38;2;255;255;255m",
	}
	s := fmt.Sprintf(format, a...)
	for k, v := range colorMap {
		s = strings.ReplaceAll(s, k, v)
	}
	s = hexColorRe.ReplaceAllStringFunc(s, func(match string) string {
		hex := hexColorRe.FindStringSubmatch(match)[1]
		r, _ := strconv.ParseInt(hex[0:2], 16, 64)
		g, _ := strconv.ParseInt(hex[2:4], 16, 64)
		b, _ := strconv.ParseInt(hex[4:6], 16, 64)
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
	})
	if strings.Contains(s, "\x1b[38;2;") {
		s += "\x1b[0m"
	}
	fmt.Print(s)
}

func main() {
	// SSH_ASKPASS mode: ssh invokes d4s to retrieve stored credentials
	if secrets.RunAskpassIfRequested() {
		return
	}

	// Version flags
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.BoolVar(showVersion, "v", false, "Print version and exit (shorthand)")

	// Context flag
	var contextName string
	flag.StringVar(&contextName, "context", "", "Docker context to use")
	flag.StringVar(&contextName, "c", "", "Docker context to use (shorthand)")

	// Skin flag
	var skinName string
	flag.StringVar(&skinName, "skin", "", "Skin to use (overrides config)")
	flag.StringVar(&skinName, "s", "", "Skin to use (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  -v, --version          Print version and exit\n")
		fmt.Fprintf(os.Stderr, "  -c, --context string   Docker context to use\n")
		fmt.Fprintf(os.Stderr, "  -s, --skin string      Skin to use (overrides config)\n")
	}

	flag.Parse()

	// Accept also: d4s version (as positional arg)
	args := os.Args[1:]
	containsVersionArg := false
	for _, arg := range args {
		a := strings.ToLower(arg)
		if a == "version" || a == "-v" || a == "--version" || a == "-version" {
			containsVersionArg = true
			break
		}
	}

	if *showVersion || containsVersionArg {
		fmt.Println()

		logoStr := "\n" + strings.Join(common.GetLogo(), "\n") + "\n"
		printColored("%s", logoStr)

		fmt.Println()
		printColored("[cyan]Version:[white]    %s\n", buildinfo.Version) // example: v0.50.6
		printColored("[cyan]Commit:[white]     %s\n", buildinfo.Commit)  // SHA, example: 13cb55bb...
		buildDate := buildinfo.Date
		// Optionally, format build date for nice output
		if buildDate != "" {
			t, err := time.Parse(time.RFC3339, buildDate)
			if err == nil {
				buildDate = t.Format("2006-01-02T15:04:05Z")
			}
		}
		printColored("[cyan]Date:[white]       %s\n", buildDate)
		fmt.Println()
		return
	}

	cfg := config.Load()

	// CLI skin flag has max precedence
	if skinName != "" {
		cfg.D4S.UI.Skin = skinName
	}

	app, err := ui.NewApp(contextName, cfg)
	if err != nil {
		fmt.Printf("Startup Error: %v\n", err)
		os.Exit(1)
	}

	// Handle signals for clean shutdown (SIGINT, SIGTERM)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		app.TviewApp.Stop()
	}()

	if err := app.Run(); err != nil {
		fmt.Printf("Error running D4s: %v\n", err)
		os.Exit(1)
	}
}
