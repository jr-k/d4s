package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/jr-k/d4s/internal/config"
	daocommon "github.com/jr-k/d4s/internal/dao/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
)

func (a *App) DumpActiveContainerLogs() {
	logInspector, ok := a.ActiveInspector.(*inspect.LogInspector)
	if !ok || logInspector == nil {
		a.AppendFlashError("logdump is only available in a container log view")
		return
	}
	if logInspector.ResourceType != "container" {
		a.AppendFlashError("logdump is only available in a container log view")
		return
	}

	id := logInspector.ResourceID
	fileID := id
	if len(fileID) > 12 {
		fileID = fileID[:12]
	}
	filePrefix := fileID
	if subject := strings.TrimSpace(logInspector.Subject); subject != "" {
		filePrefix = sanitizeLogFilenameSubject(subject, fileID)
	}

	logsDir := config.LogsDir()
	if logsDir == "" {
		a.AppendFlashError("unable to determine d4s logs directory")
		return
	}

	a.SetFlashPending(fmt.Sprintf("dumping logs for %s...", id))

	a.RunInBackground(func() {
		if err := os.MkdirAll(logsDir, 0o755); err != nil {
			a.SafeQueueUpdateDraw(func() {
				a.AppendFlashError(fmt.Sprintf("failed to create logs dir: %v", err))
			})
			return
		}

		ts := time.Now().Format("20060102-150405")
		path := filepath.Join(logsDir, fmt.Sprintf("%s.%s.log", filePrefix, ts))

		f, err := os.Create(path)
		if err != nil {
			a.SafeQueueUpdateDraw(func() {
				a.AppendFlashError(fmt.Sprintf("failed to create log file: %v", err))
			})
			return
		}

		dumpErr := a.Docker.DumpContainerLogs(id, f, true)
		closeErr := f.Close()
		if dumpErr == nil {
			dumpErr = closeErr
		}

		a.SafeQueueUpdateDraw(func() {
			if dumpErr != nil {
				a.AppendFlashError(fmt.Sprintf("failed to dump logs: %v", dumpErr))
				return
			}
			a.AppendFlashSuccess(fmt.Sprintf("logs dumped to %s", daocommon.ShortenPath(path)))
		})
	})
}

func sanitizeLogFilenameSubject(subject string, fallbackID string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fallbackID
	}

	subject = strings.ReplaceAll(subject, "@", ".")

	var b strings.Builder
	lastDot := false
	for _, r := range subject {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			b.WriteRune(r)
			lastDot = r == '.'
		case !lastDot:
			b.WriteByte('.')
			lastDot = true
		}
	}

	name := strings.Trim(b.String(), ".")
	if name == "" {
		return fallbackID
	}
	return name
}
