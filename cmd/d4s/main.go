package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jr-k/d4s/internal/buildinfo"
	"github.com/jr-k/d4s/internal/ui"
	"github.com/jr-k/d4s/internal/ui/common"
)

func printColored(format string, a ...interface{}) {
	// Mapping from [orange] etc to their respective truecolor ANSI sequences
	colorMap := map[string]string{
		"[#ffb86c]": "\x1b[38;2;255;184;108m",
		"[#ff8c00]": "\x1b[38;2;255;140;3m",
		"[orange]":  "\x1b[38;2;255;184;108m",
		"[cyan]":    "\x1b[38;2;57;166;255m",
		"[white]":   "\x1b[38;2;255;255;255m",
	}
	// Replace all [color] tags with color codes in the format string
	s := fmt.Sprintf(format, a...)
	for k, v := range colorMap {
		s = strings.ReplaceAll(s, k, v)
	}
	if strings.Contains(s, "\x1b[38;2;") {
		s += "\x1b[0m"
	}
	fmt.Print(s)
}

func main() {
	// Version flags
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.BoolVar(showVersion, "v", false, "Print version and exit (shorthand)")

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
		// Print logo & build info à la k9s style
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

	app := ui.NewApp()

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
