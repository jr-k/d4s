package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/jr-k/d4s/internal/config"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/dialogs"
)

func (a *App) ShowContextPicker() {
	contexts, err := a.Docker.ListContexts()
	if err != nil {
		a.AppendFlashError(fmt.Sprintf("failed to load docker contexts: %v", err))
		return
	}

	active := strings.TrimSpace(a.Docker.ContextName)
	saved := strings.TrimSpace(a.Cfg.D4S.DefaultContext)

	items := make([]dialogs.PickerItem, 0, len(contexts))
	for _, ctx := range contexts {
		var markers []string
		if ctx.Name == active {
			markers = append(markers, "active")
		}
		if ctx.Name == saved {
			markers = append(markers, "default")
		}

		var details []string
		if len(markers) > 0 {
			details = append(details, strings.Join(markers, ", "))
		}
		if ctx.DockerEndpoint != "" {
			details = append(details, shortenContextText(ctx.DockerEndpoint, 30))
		} else if ctx.Description != "" {
			details = append(details, shortenContextText(ctx.Description, 30))
		}

		description := "Select as d4s default context"
		if len(details) > 0 {
			description = strings.Join(details, " • ")
		}

		items = append(items, dialogs.PickerItem{
			Label:       ctx.Name,
			Description: description,
			Value:       ctx.Name,
		})
	}

	dialogs.ShowPicker(a, "Docker Contexts", items, func(value string) {
		a.SetDefaultContext(value)
	})
}

func (a *App) SetDefaultContext(contextName string) {
	contextName = strings.TrimSpace(contextName)
	if contextName == "" {
		a.AppendFlashError("context name cannot be empty")
		return
	}

	if a.Docker != nil && a.Docker.ContextName == contextName {
		a.Cfg.D4S.DefaultContext = contextName
		if err := config.Save(a.Cfg); err != nil {
			a.AppendFlashError(fmt.Sprintf("failed to save default context: %v", err))
			return
		}

		a.AppendFlashSuccess(contextSavedMessage(contextName))
		a.updateHeader()
		return
	}

	a.SetFlashPending(fmt.Sprintf("switching context to %s...", contextName))
	a.SetPaused(true)
	a.StopAutoRefresh()

	if a.ActiveInspector != nil {
		a.ActiveInspector.OnUnmount()
		a.ActiveInspector = nil
	}
	if a.Pages.HasPage("inspect") {
		a.Pages.RemovePage("inspect")
	}

	a.SafeSetScope(nil)
	a.ActiveFilter = ""
	for _, v := range a.Views {
		v.SetLoading(true)
	}
	a.RestoreFocus()
	a.UpdateShortcuts()

	a.RunInBackground(func() {
		newDocker, err := dao.NewDockerClient(contextName, a.Cfg.D4S.GetAPIServerTimeout(), contextName)
		if err != nil {
			a.TviewApp.QueueUpdateDraw(func() {
				a.SetPaused(false)
				a.StartAutoRefresh()
				a.AppendFlashError(fmt.Sprintf("failed to switch context: %v", err))
				a.RefreshCurrentView()
				a.updateHeader()
			})
			return
		}

		a.Cfg.D4S.DefaultContext = contextName
		saveErr := config.Save(a.Cfg)

		a.TviewApp.QueueUpdateDraw(func() {
			a.Docker = newDocker
			a.SetPaused(false)
			a.StartAutoRefresh()
			a.RestoreFocus()
			a.UpdateShortcuts()
			a.updateHeader()
			a.RefreshCurrentView()
			a.preloadViews()

			if saveErr != nil {
				a.AppendFlashError(fmt.Sprintf("switched to %s, but failed to save default: %v", contextName, saveErr))
				return
			}

			a.AppendFlashSuccess(contextSavedMessage(contextName))
		})
	})
}

func contextSavedMessage(name string) string {
	msg := fmt.Sprintf("default context set to %s", name)
	if os.Getenv("DOCKER_HOST") != "" || os.Getenv("DOCKER_CONTEXT") != "" {
		msg += " (env vars still override on startup)"
	}
	return msg
}

func shortenContextText(text string, max int) string {
	if max <= 3 || len(text) <= max {
		return text
	}

	keep := (max - 3) / 2
	tail := max - 3 - keep
	return text[:keep] + "..." + text[len(text)-tail:]
}
