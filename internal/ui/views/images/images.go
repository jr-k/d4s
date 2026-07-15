package images

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"ID", "TAGS", "SIZE", "CONTAINERS", "CREATED"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	images, err := app.GetDocker().ListImages()
	if err != nil {
		return nil, err
	}

	scope := app.GetActiveScope()
	if scope != nil && scope.Type == "image" {
		var filtered []dao.Resource
		for _, r := range images {
			if img, ok := r.(dao.Image); ok {
				// Match by ID (full or prefix)
				// Image IDs often start with sha256:, handle that too if needed, but usually dao handles it.
				// dao.Image usually has raw ID. scope.Value from container might be short or long.
				if img.ID == scope.Value || strings.HasPrefix(img.ID, scope.Value) {
					filtered = append(filtered, r)
				}
			}
		}
		return filtered, nil
	}
	return images, nil
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("enter", "Containers"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("v", "Dive"),
		common.FormatSCHeader("r", "Pull"),
		common.FormatSCHeader("shift-p", "Prune"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App
	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	switch event.Rune() {
	case 'v':
		DiveAction(app, v)
		return nil
	case 'r':
		PullAction(app, v)
		return nil
	case 'P':
		PruneAction(app)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	}
	
	if event.Key() == tcell.KeyEnter {
		EnterAction(app, v)
		return nil
	}

	return event
}

func EnterAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	// Find the full image object just to be sure what we have
	// Or just use the ID. The container view needs to know how to filter.
	// We'll pass the Image ID as scope.
	
	// Get Name if possible for nicer display label
	label := id
	for _, it := range v.Data {
		if it.GetID() == id {
			if im, ok := it.(dao.Image); ok {
				if im.Tags != "<none>" && im.Tags != "" {
					label = im.Tags
				}
			}
			break
		}
	}

	// We'll use a special scope type 'image'
	// But we need to make sure containers view supports it (already added beforehand)
	scope := &common.Scope{
		Type:       "image",
		Value:      id, // Use ID for robust filtering
		Label:      fmt.Sprintf("Image: %s", label),
		OriginView: "Images",
		Parent:     app.GetActiveScope(),
	}
	app.SetActiveScope(scope)
	app.SwitchTo(styles.TitleContainers)
}

func PruneAction(app common.AppController) {
	dialogs.ShowConfirmation(app, "PRUNE", "Images", func(force bool) {
		app.SetFlashPending("pruning images...")
		app.RunInBackground(func() {
			err := Prune(app)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess("pruned images")
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func PullAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil || len(ids) == 0 {
		return
	}

	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	count := 0
	for _, item := range v.Data {
		if idMap[item.GetID()] {
			if img, ok := item.(dao.Image); ok {
				if img.RepoTag != "" && img.RepoTag != "<none>" {
					count++
					tag := img.RepoTag

					app.RunInBackground(func() {
						err := app.GetDocker().PullImage(tag)
						app.GetTviewApp().QueueUpdateDraw(func() {
							if err != nil {
								app.SetFlashError(fmt.Sprintf("Pull failed: %v", err))
							}
							app.RefreshCurrentView()
						})
					})
				}
			}
		}
	}

	if count > 0 {
		app.SetFlashPending(fmt.Sprintf("Pulling %d image(s)...", count))
		// Force refresh to show status
		go func() {
			time.Sleep(100 * time.Millisecond)
			app.GetTviewApp().QueueUpdateDraw(func() {
				app.RefreshCurrentView()
			})
		}()
	}
}

func Inspect(app common.AppController, id string) {
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	inspector := inspect.NewTextInspector("Describe image", subject, fmt.Sprintf(" [%s]Loading image...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("image", id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		// Resolve Tags from List
		resolvedSubject := subject
		images, err := app.GetDocker().ListImages()
		if err == nil {
			for _, item := range images {
				if item.GetID() == id {
					if img, ok := item.(dao.Image); ok {
						if img.Tags != "" && img.Tags != "<none>" {
							resolvedSubject = fmt.Sprintf("%s@%s", img.Tags, resolvedSubject)
						}
					}
					break
				}
			}
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			inspector.Subject = resolvedSubject
			inspector.Viewer.Update(content, "json")
			inspector.Viewer.View.SetTitle(inspector.GetTitle())
		})
	})
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }

	label := ids[0]
	if len(ids) == 1 {
		row, _ := v.Table.GetSelection()
		if row > 0 && row <= len(v.Data) {
			item := v.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
					// Inside Confirmation Modal
					label = fmt.Sprintf("%s ([%s]%s[yellow])", label, styles.TagCyan, cells[1])
				}
			}
		}
	} else {
		label = fmt.Sprintf("%d items", len(ids))
	}

	dialogs.ShowConfirmation(app, "DELETE", label, func(force bool) {
		simpleAction := func(id string) error {
			return Remove(id, force, app)
		}
		app.PerformAction(simpleAction, "deleting", styles.ColorStatusRed)
	})
}

func Prune(app common.AppController) error {
	return app.GetDocker().PruneImages()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveImage(id, force)
}

func DiveAction(app common.AppController, v *view.ResourceView) {
	path, err := exec.LookPath("dive")
	if err != nil {
		app.AppendFlashError("dive command not found in PATH")
		return
	}

	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	// Prepare to suspend the UI
	app.StopAutoRefresh()
	app.SetPaused(true)
	defer func() {
		app.SetPaused(false)
		app.StartAutoRefresh()
	}()

	app.GetTviewApp().Suspend(func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Dive panic: %v\n", r)
			}
		}()

		// Clear any existing signal handlers in the app to avoid conflicts
		signal.Reset(os.Interrupt, syscall.SIGTERM)
		
		// Setup local signal monitoring
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(c)

		// IMPORTANT: d4s restores signal handling when we return from Suspend/This function?
		// No, we must Restore them if necessary. But usually tcell re-inits on resume.
		// However, it's safer to not leave d4s without signal handlers if this crashes.
		// Since we cannot easily "restore previous", we rely on app re-init or default default.
		// But actually, we don't know what the global handlers were.
		// Usually a restart of the loop handles it.

		fmt.Printf("Running dive on %s...\n", id)
		
		cmd := exec.Command(path, id)
		// Point dive at the same daemon as d4s (remote over SSH included)
		if docker := app.GetDocker(); docker != nil && docker.ContextName != "" && docker.ContextName != "default" {
			cmd.Env = append(os.Environ(), "DOCKER_CONTEXT="+docker.ContextName)
			if docker.IsSSHContext() {
				cmd.Env = append(cmd.Env, "DOCKER_HOST=ssh://"+docker.GetSSHHost())
			}
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Start the command
		if err := cmd.Start(); err != nil {
			fmt.Printf("Error starting dive: %v\nPress Enter...", err)
			fmt.Scanln()
			return
		}

		// Handle signals
		done := make(chan struct{})
		go func() {
			select {
			case <-c:
				// User interrupt
				fmt.Println("\nStopping dive (Ctrl+C received)...") // Feedback
				
				// Try graceful termination first (SIGINT)
				// Since we are in the same process group, dive already got the SIGINT from the TTY driver!
				// We don't need to send it again usually.
				// But if dive is ignoring it, we might need to Kill.
				
				// Give it a moment to exit gracefully?
				timer := time.NewTimer(500 * time.Millisecond)
				
				select {
				case <-done:
					// Exited naturally
					timer.Stop()
				case <-timer.C:
					// Didn't exit, FORCE KILL
					fmt.Println("Dive did not exit, force killing...")
					_ = cmd.Process.Kill()
				case <-c:
					// Second Ctrl+C? Kill immediately
					fmt.Println("Force killing...")
					_ = cmd.Process.Kill()
				}
			case <-done:
				// Command finished
			}
		}()

		// Wait for command
		err = cmd.Wait()
		close(done)

		// Check errors
		if err != nil {
			// Ignore exit status 130 or kill signals as they are intented interruptions
			s := err.Error()
			if s != "signal: interrupt" && s != "exit status 130" && s != "signal: killed" && s != "signal: terminated" {
				fmt.Printf("\nDive exited with error: %v\nPress Enter to continue...", err)
				fmt.Scanln()
			}
		}
	})

	if app.GetScreen() != nil {
		app.GetScreen().Sync()
	}
}