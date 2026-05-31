package secrets

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"ID", "NAME", "SERVICES", "CREATED", "UPDATED", "LABELS"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	return app.GetDocker().ListSecrets()
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("s", "Services"),
		common.FormatSCHeader("x", "Decode"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("a", "Add"),
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
	case 's':
		ViewServices(app, v)
		return nil
	case 'x':
		Decode(app, v)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'a':
		Create(app)
		return nil
	}

	return event
}

func ViewServices(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	// Get secret name for label
	name := id
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if s, ok := v.Data[index].(dao.Secret); ok {
				name = s.Name
			}
		}
	}

	app.SetActiveScope(&common.Scope{
		Type:       "secret",
		Value:      id,
		Label:      name,
		OriginView: styles.TitleSecrets,
	})

	app.SwitchTo(styles.TitleServices)
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil {
		return
	}

	label := ids[0]
	if len(ids) == 1 {
		row, _ := v.Table.GetSelection()
		if row > 0 && row <= len(v.Data) {
			item := v.Data[row-1]
			if item.GetID() == ids[0] {
				cells := item.GetCells()
				if len(cells) > 1 {
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

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveSecret(id)
}

func Create(app common.AppController) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput},
		{Name: "value", Label: "Value", Type: dialogs.FieldTypeInput},
	}

	dialogs.ShowFormWithDescription(app, "Create Secret", "Enter secret name and value", fields, func(result dialogs.FormResult) {
		name := result["name"]
		value := result["value"]

		if name == "" || value == "" {
			app.SetFlashError("name and value are required")
			return
		}

		app.AppendFlashPending(fmt.Sprintf("creating secret %s...", name), 30*time.Second)

		app.RunInBackground(func() {
			err := app.GetDocker().CreateSecret(name, []byte(value))
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to create secret: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("secret %s created", name))
				}
				app.RefreshCurrentView()
			})
		})
	})
}

func Decode(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	name := id
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if s, ok := v.Data[index].(dao.Secret); ok {
				name = s.Name
			}
		}
	}

	inspector := inspect.NewTextInspector("Decode secret", name, fmt.Sprintf(" [%s]Loading value...\n", styles.TagAccent), "text")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		serviceName := fmt.Sprintf("d4s-secret-decode-%d", time.Now().UnixNano())

		createCmd := exec.Command("docker", "service", "create",
			"--name", serviceName,
			"--secret", name,
			"--restart-condition", "none",
			"--detach",
			app.GetConfig().D4S.ShellPod.Image,
			"cat", fmt.Sprintf("/run/secrets/%s", name),
		)

		if out, err := createCmd.CombinedOutput(); err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: failed to create decode service: %v — %s", err, strings.TrimSpace(string(out))), "text")
			})
			return
		}

		defer func() {
			exec.Command("docker", "service", "rm", serviceName).Run()
		}()

		var content string
		var decodeErr error

		for i := 0; i < 30; i++ {
			time.Sleep(1 * time.Second)

			psCmd := exec.Command("docker", "service", "ps", "--format", "{{.CurrentState}}", "--no-trunc", serviceName)
			psOut, err := psCmd.Output()
			if err != nil {
				continue
			}

			state := strings.TrimSpace(string(psOut))
			if strings.Contains(state, "Complete") {
				logsCmd := exec.Command("docker", "service", "logs", "--raw", serviceName)
				logOut, err := logsCmd.Output()
				if err != nil {
					decodeErr = fmt.Errorf("failed to read logs: %v", err)
				} else {
					content = string(logOut)
				}
				break
			}

			if strings.Contains(state, "Failed") || strings.Contains(state, "Rejected") {
				decodeErr = fmt.Errorf("service task failed: %s", state)
				break
			}
		}

		if decodeErr == nil && content == "" {
			decodeErr = fmt.Errorf("timed out waiting for secret decode")
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			if decodeErr != nil {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", decodeErr), "text")
				return
			}

			inspector.Viewer.Update(fmt.Sprintf("%s=%s", name, content), "env")
		})
	})
}

func Inspect(app common.AppController, id string) {
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	inspector := inspect.NewTextInspector("Describe secret", subject, fmt.Sprintf(" [%s]Loading secret...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("secret", id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		// Resolve Name
		resolvedSubject := subject
		secrets, err := app.GetDocker().ListSecrets()
		if err == nil {
			for _, item := range secrets {
				if item.GetID() == id {
					if s, ok := item.(dao.Secret); ok {
						resolvedSubject = fmt.Sprintf("%s@%s", s.Name, resolvedSubject)
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
