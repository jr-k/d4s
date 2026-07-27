package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/jr-k/d4s/internal/config"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

func (a *App) ShowContextPicker() {
	docker := a.GetDocker()
	contexts, err := docker.ListContexts()
	if err != nil {
		a.AppendFlashError(fmt.Sprintf("failed to load docker contexts: %v", err))
		return
	}

	active := strings.TrimSpace(docker.ContextName)
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
			details = append(details, ctx.DockerEndpoint)
		} else if ctx.Description != "" {
			details = append(details, ctx.Description)
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
	}, func() {
		a.UpdateShortcuts()
	})
}

func (a *App) SetDefaultContext(contextName string) {
	contextName = strings.TrimSpace(contextName)
	if contextName == "" {
		a.AppendFlashError("context name cannot be empty")
		return
	}

	if docker := a.GetDocker(); docker != nil && docker.ContextName == contextName {
		// Re-selecting the active context cancels any older switch that may
		// still be preparing another client in the background.
		switchGen := a.contextSwitchGen.Add(1)
		a.saveDefaultContext(contextName, switchGen)
		a.updateHeader()
		return
	}

	a.ReloadContext(contextName)
}

// ReloadContext rebuilds the Docker client for contextName, even when it
// is already the active context. Used after editing a context (endpoint
// or credentials change) so changes apply without restarting d4s.
func (a *App) ReloadContext(contextName string) {
	contextName = strings.TrimSpace(contextName)
	if contextName == "" {
		return
	}

	switchGen := a.contextSwitchGen.Add(1)
	a.SetFlashPending(fmt.Sprintf("switching context to %s...", contextName))

	a.RunInBackground(func() {
		newDocker, err := dao.NewDockerClient(contextName, a.Cfg.D4S.GetAPIServerTimeout(), contextName)
		if err != nil {
			a.TviewApp.QueueUpdateDraw(func() {
				if a.contextSwitchGen.Load() != switchGen {
					return
				}
				a.AppendFlashError(fmt.Sprintf("failed to switch context: %v", err))
				a.updateHeader()
			})
			return
		}

		a.TviewApp.QueueUpdateDraw(func() {
			if a.contextSwitchGen.Load() != switchGen {
				a.RunInBackground(func() {
					_ = newDocker.Close()
				})
				return
			}

			if a.ActiveInspector != nil {
				a.ActiveInspector.OnUnmount()
				a.ActiveInspector = nil
			}
			if a.Pages.HasPage("inspect") {
				a.Pages.RemovePage("inspect")
			}

			a.SafeSetScope(nil)
			a.ActiveFilter = ""
			currentPage, _ := a.Pages.GetFrontPage()
			for title, v := range a.Views {
				// Context metadata remains usable while resources from the
				// newly selected Docker endpoint are loading.
				if title == currentPage && title != styles.TitleContexts {
					v.SetLoading(true)
				} else if title != styles.TitleContexts {
					v.InvalidateData()
				}
				// Cancel requests against the previous endpoint and reject
				// any results that arrive after the client swap.
				v.InvalidateFetch()
			}

			oldDocker := a.swapDocker(newDocker)
			if oldDocker != nil {
				a.RunInBackground(func() {
					_ = oldDocker.Close()
				})
			}

			a.saveDefaultContext(contextName, switchGen)

			a.RestoreFocus()
			a.UpdateShortcuts()
			a.updateHeader()
			a.RefreshCurrentView()
		})
	})
}

func (a *App) saveDefaultContext(contextName string, switchGen uint64) {
	a.Cfg.D4S.DefaultContext = contextName
	cfg := *a.Cfg

	a.RunInBackground(func() {
		// Serialize writes so an older context switch can never overwrite a
		// newer selection after a slow filesystem operation.
		a.contextSaveMx.Lock()
		defer a.contextSaveMx.Unlock()

		if a.contextSwitchGen.Load() != switchGen {
			return
		}
		err := config.Save(&cfg)

		a.TviewApp.QueueUpdateDraw(func() {
			if a.contextSwitchGen.Load() != switchGen {
				return
			}
			if err != nil {
				a.AppendFlashError(fmt.Sprintf("failed to save default context: %v", err))
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
