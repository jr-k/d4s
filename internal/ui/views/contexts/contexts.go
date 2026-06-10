package contexts

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/secrets"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"NAME", "CURRENT", "DESCRIPTION", "ENDPOINT"}

type Context struct {
	Name        string
	Current     bool
	Description string
	Endpoint    string
}

func (c Context) GetID() string { return c.Name }
func (c Context) GetCells() []string {
	current := "False"
	if c.Current {
		current = "True"
	}
	return []string{c.Name, current, c.Description, c.Endpoint}
}

func (c Context) GetStatusColor() (tcell.Color, tcell.Color) {
	if c.Current {
		return styles.ColorIdle, styles.ColorBlack
	}
	return styles.ColorStatusGray, styles.ColorBlack
}

func (c Context) GetColumnValue(column string) string {
	switch strings.ToLower(column) {
	case "name":
		return c.Name
	case "current":
		if c.Current {
			return "True"
		}
		return "False"
	case "description":
		return c.Description
	case "endpoint":
		return c.Endpoint
	}
	return ""
}

func (c Context) GetDefaultColumn() string {
	return "Name"
}

func (c Context) GetDefaultSortColumn() string {
	return "Name"
}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	ctxList, err := app.GetDocker().ListContexts()
	if err != nil {
		return nil, err
	}

	active := strings.TrimSpace(app.GetDocker().ContextName)

	var res []dao.Resource
	for _, c := range ctxList {
		res = append(res, Context{
			Name:        c.Name,
			Current:     c.Name == active,
			Description: c.Description,
			Endpoint:    c.DockerEndpoint,
		})
	}
	return res, nil
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("enter", "Use"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("e", "Edit"),
		common.FormatSCHeader("a", "Add"),
		common.FormatSCHeader("shift-a", "Add Remote"),
		common.FormatSCHeader("ctrl-d", "Delete"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App

	if event.Key() == tcell.KeyCtrlD {
		DeleteAction(app, v)
		return nil
	}
	if event.Key() == tcell.KeyEnter {
		UseAction(app, v)
		return nil
	}

	switch event.Rune() {
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'e':
		Edit(app, v)
		return nil
	case 'a':
		Create(app)
		return nil
	case 'A':
		CreateSSH(app)
		return nil
	}

	return event
}

func UseAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	app.SetDefaultContext(id)
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil {
		return
	}

	label := fmt.Sprintf("[yellow]%s[-]", ids[0])
	if len(ids) > 1 {
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
	wasCurrent := app.GetDocker().ContextName == id

	if err := app.GetDocker().RemoveContext(id, force); err != nil {
		return err
	}
	secrets.Delete(id)

	if wasCurrent {
		app.GetTviewApp().QueueUpdateDraw(func() {
			app.SetDefaultContext("default")
		})
	}
	return nil
}

func Create(app common.AppController) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput},
		{Name: "description", Label: "Description", Type: dialogs.FieldTypeInput},
		{Name: "host", Label: "Docker Host", Type: dialogs.FieldTypeInput},
	}

	dialogs.ShowFormWithDescription(app, "Create Context", "Enter context name and docker endpoint", fields, func(result dialogs.FormResult) {
		name := result["name"]
		description := result["description"]
		host := result["host"]

		if name == "" {
			app.SetFlashError("name is required")
			return
		}

		app.AppendFlashPending(fmt.Sprintf("creating context %s...", name))

		app.RunInBackground(func() {
			err := app.GetDocker().CreateContext(name, description, host)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to create context: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("context %s created", name))
				}
				app.RefreshCurrentView()
			})
		})
	})
}

func Edit(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	if id == "default" {
		app.AppendFlashError("cannot edit the default context")
		return
	}

	// Get current values from the table data
	var currentDesc, currentEndpoint string
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		if c, ok := v.Data[row-1].(Context); ok {
			currentDesc = c.Description
			currentEndpoint = c.Endpoint
		}
	}

	fields := []dialogs.FormField{
		{Name: "description", Label: "Description", Type: dialogs.FieldTypeInput, Default: currentDesc},
		{Name: "host", Label: "Docker Host", Type: dialogs.FieldTypeInput, Default: currentEndpoint},
	}

	dialogs.ShowFormWithDescription(app, fmt.Sprintf("Edit Context: %s", id), "", fields, func(result dialogs.FormResult) {
		description := result["description"]
		host := result["host"]

		app.AppendFlashPending(fmt.Sprintf("updating context %s...", id))

		app.RunInBackground(func() {
			err := app.GetDocker().UpdateContext(id, description, host)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to update context: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("context %s updated", id))
				}
				app.RefreshCurrentView()
			})
		})
	})
}

func CreateSSH(app common.AppController) {
	items := []dialogs.PickerItem{
		{Label: "SSH Key", Description: "authenticate with a private key (recommended)", Value: secrets.AuthTypeKey},
		{Label: "Password", Description: "authenticate with a password", Value: secrets.AuthTypePassword},
	}

	dialogs.ShowPicker(app, "Authentication Method", items, func(authType string) {
		showSSHForm(app, authType)
	})
}

func showSSHForm(app common.AppController, authType string) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput, Placeholder: "prod-server"},
		{Name: "host", Label: "Host (user@ip)", Type: dialogs.FieldTypeInput, Placeholder: "root@192.168.1.100"},
	}

	if authType == secrets.AuthTypeKey {
		fields = append(fields,
			dialogs.FormField{Name: "key", Label: "SSH Key", Type: dialogs.FieldTypeInput, Placeholder: "~/.ssh/id_ed25519"},
			dialogs.FormField{Name: "passphrase", Label: "Passphrase (optional)", Type: dialogs.FieldTypeInput, Secret: true},
		)
	} else {
		fields = append(fields,
			dialogs.FormField{Name: "password", Label: "Password", Type: dialogs.FieldTypeInput, Secret: true},
		)
	}

	fields = append(fields,
		dialogs.FormField{Name: "socket", Label: "Docker Socket", Type: dialogs.FieldTypeInput, Default: "/var/run/docker.sock"},
	)

	dialogs.ShowFormWithDescription(app, "Add Remote Context (SSH)", "Creates a Docker context using SSH tunnel", fields, func(result dialogs.FormResult) {
		name := strings.TrimSpace(result["name"])
		host := strings.TrimSpace(result["host"])
		socket := strings.TrimSpace(result["socket"])

		if name == "" {
			app.SetFlashError("name is required")
			return
		}
		if host == "" {
			app.SetFlashError("host is required")
			return
		}
		if authType == secrets.AuthTypePassword && result["password"] == "" {
			app.SetFlashError("password is required")
			return
		}

		creds := secrets.SSHCredentials{
			AuthType:   authType,
			KeyPath:    expandHome(strings.TrimSpace(result["key"])),
			Passphrase: result["passphrase"],
			Password:   result["password"],
		}

		sshURL := fmt.Sprintf("ssh://%s", host)
		if socket != "" && socket != "/var/run/docker.sock" {
			sshURL = fmt.Sprintf("ssh://%s%s", host, socket)
		}

		app.AppendFlashPending(fmt.Sprintf("creating SSH context %s (%s)...", name, sshURL))

		app.RunInBackground(func() {
			err := app.GetDocker().CreateContext(name, fmt.Sprintf("SSH remote: %s", host), sshURL)
			if err == nil {
				if kerr := secrets.Save(name, creds); kerr != nil {
					app.GetTviewApp().QueueUpdateDraw(func() {
						app.AppendFlashError(fmt.Sprintf("context created but credentials not saved: %v", kerr))
					})
				}
			}
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to create context: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("SSH context '%s' created", name))
				}
				app.RefreshCurrentView()
			})
		})
	})
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}

func Inspect(app common.AppController, id string) {
	inspector := inspect.NewTextInspector("Describe context", id, fmt.Sprintf(" [%s]Loading context...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().InspectContext(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			inspector.Viewer.Update(content, "json")
		})
	})
}
