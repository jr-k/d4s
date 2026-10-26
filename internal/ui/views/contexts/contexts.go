package contexts

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/secrets"
	"github.com/jr-k/d4s/internal/sshutil"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
	"golang.org/x/crypto/ssh"
)

var Headers = []string{"NAME", "CURRENT", "DESCRIPTION", "TYPE", "ENDPOINT"}

type Context struct {
	Name        string
	Current     bool
	Description string
	Endpoint    string
}

func (c Context) GetID() string { return c.Name }

// Type derives the context kind from its endpoint scheme.
func (c Context) Type() string {
	switch {
	case strings.HasPrefix(c.Endpoint, "ssh://"):
		return "ssh"
	case strings.HasPrefix(c.Endpoint, "tcp://"):
		return "tcp"
	case strings.HasPrefix(c.Endpoint, "unix://"), strings.HasPrefix(c.Endpoint, "npipe://"):
		return "local"
	case c.Endpoint == "":
		return "unknown"
	default:
		return strings.SplitN(c.Endpoint, "://", 2)[0]
	}
}

func (c Context) GetCells() []string {
	current := "False"
	if c.Current {
		current = "True"
	}
	return []string{c.Name, current, c.Description, c.Type(), c.Endpoint}
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
	case "type":
		return c.Type()
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
	items := []dialogs.PickerItem{
		{
			Label:       "Local Context",
			Description: "create a context from a Docker endpoint",
			Value:       "local",
			Shortcut:    'l',
		},
		{
			Label:       "Remote Context (SSH Key)",
			Description: "connect over SSH with a private key",
			Value:       secrets.AuthTypeKey,
			Shortcut:    'k',
		},
		{
			Label:       "Remote Context (Password)",
			Description: "connect over SSH with a password",
			Value:       secrets.AuthTypePassword,
			Shortcut:    'p',
		},
	}

	dialogs.ShowPicker(app, "Add Context", items, func(contextType string) {
		switch contextType {
		case "local":
			showLocalForm(app)
		case secrets.AuthTypePassword, secrets.AuthTypeKey:
			showSSHForm(app, contextType)
		}
	})
}

func showLocalForm(app common.AppController) {
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

	if strings.HasPrefix(currentEndpoint, "ssh://") {
		editSSH(app, id, currentDesc, currentEndpoint)
		return
	}

	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput, Default: id},
		{Name: "description", Label: "Description", Type: dialogs.FieldTypeInput, Default: currentDesc},
		{Name: "host", Label: "Docker Host", Type: dialogs.FieldTypeInput, Default: currentEndpoint},
	}

	dialogs.ShowFormWithDescription(app, fmt.Sprintf("Edit Context: %s", id), "", fields, func(result dialogs.FormResult) {
		name := strings.TrimSpace(result["name"])
		description := result["description"]
		host := result["host"]

		if name == "" {
			app.SetFlashError("name is required")
			return
		}

		app.AppendFlashPending(fmt.Sprintf("updating context %s...", id))

		app.RunInBackground(func() {
			isCurrent := app.GetDocker().ContextName == id
			err := app.GetDocker().UpdateAndRenameContext(id, name, description, host)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to update context: %v", err))
					app.RefreshCurrentView()
					return
				}
				app.AppendFlashSuccess(fmt.Sprintf("context %s updated", name))
				if isCurrent {
					// Rebuild the client so the new endpoint applies immediately
					app.ReloadContext(name)
				} else {
					app.RefreshCurrentView()
				}
			})
		})
	})
}

// parseSSHURL splits "ssh://user@host[:port][/socket/path]" into its form fields.
func parseSSHURL(endpoint string) (user, host, port, socket string) {
	parsed, err := url.Parse(endpoint)
	if err == nil {
		if parsed.User != nil {
			user = parsed.User.Username()
		}
		host = parsed.Hostname()
		port = parsed.Port()
		socket = parsed.Path
	}
	if port == "" {
		port = "22"
	}
	return user, host, port, socket
}

func buildSSHURL(user, host, port, socket string) string {
	sshURL := fmt.Sprintf("ssh://%s@%s", user, net.JoinHostPort(host, port))
	if socket != "" {
		sshURL += socket
	}
	return sshURL
}

func validateSSHPort(port string) (string, error) {
	port = strings.TrimSpace(port)
	value, err := strconv.ParseUint(port, 10, 16)
	if err != nil || value == 0 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

func editSSH(app common.AppController, id, currentDesc, currentEndpoint string) {
	existing, _ := secrets.Load(id)

	authType := secrets.AuthTypeKey
	if existing != nil && existing.AuthType != "" {
		authType = existing.AuthType
	}

	showSSHEditForm(app, id, currentDesc, currentEndpoint, authType, existing)
}

func showSSHEditForm(app common.AppController, id, currentDesc, currentEndpoint, authType string, existing *secrets.SSHCredentials) {
	currentUser, currentHost, currentPort, currentSocket := parseSSHURL(currentEndpoint)
	if currentSocket == "" {
		currentSocket = "/var/run/docker.sock"
	}

	// Pre-fill credentials only when the auth method is unchanged
	var defaultKey, defaultPassphrase, defaultPassword string
	if existing != nil && existing.AuthType == authType {
		defaultKey = existing.KeyPath
		defaultPassphrase = existing.Passphrase
		defaultPassword = existing.Password
	}

	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput, Default: id},
		{Name: "description", Label: "Description", Type: dialogs.FieldTypeInput, Default: currentDesc},
		{Name: "user", Label: "SSH User", Type: dialogs.FieldTypeInput, Default: currentUser, Placeholder: "root"},
		{Name: "host", Label: "Host", Type: dialogs.FieldTypeInput, Default: currentHost, Placeholder: "192.168.1.100"},
		{Name: "port", Label: "SSH Port", Type: dialogs.FieldTypeInput, Default: currentPort, Placeholder: "22"},
	}

	if authType == secrets.AuthTypeKey {
		fields = append(fields,
			dialogs.FormField{
				Name:        "key",
				Label:       "SSH Key",
				Type:        dialogs.FieldTypeInput,
				Default:     defaultKey,
				Placeholder: "~/.ssh/id_ed25519",
				Suggestions: discoverSSHKeys(),
			},
			dialogs.FormField{Name: "passphrase", Label: "Passphrase (optional)", Type: dialogs.FieldTypeInput, Default: defaultPassphrase, Secret: true},
		)
	} else {
		fields = append(fields,
			dialogs.FormField{Name: "password", Label: "Password", Type: dialogs.FieldTypeInput, Default: defaultPassword, Secret: true},
		)
	}

	fields = append(fields,
		dialogs.FormField{Name: "socket", Label: "Docker Socket", Type: dialogs.FieldTypeInput, Default: currentSocket},
	)

	dialogs.ShowFormWithDescription(app, fmt.Sprintf("Edit Context: %s", id), "Updates the SSH context and its credentials", fields, func(result dialogs.FormResult) {
		name := strings.TrimSpace(result["name"])
		description := result["description"]
		user := strings.TrimSpace(result["user"])
		host := strings.TrimSpace(result["host"])
		port, portErr := validateSSHPort(result["port"])
		socket := strings.TrimSpace(result["socket"])

		if name == "" {
			app.SetFlashError("name is required")
			return
		}
		if user == "" {
			app.SetFlashError("SSH user is required")
			return
		}
		if host == "" {
			app.SetFlashError("host is required")
			return
		}
		if portErr != nil {
			app.SetFlashError(portErr.Error())
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

		sshURL := buildSSHURL(user, host, port, socket)

		app.AppendFlashPending(fmt.Sprintf("updating context %s...", id))

		app.RunInBackground(func() {
			isCurrent := app.GetDocker().ContextName == id
			err := app.GetDocker().UpdateAndRenameContext(id, name, description, sshURL)
			if err == nil {
				if kerr := secrets.Save(name, creds); kerr != nil {
					app.GetTviewApp().QueueUpdateDraw(func() {
						app.AppendFlashError(fmt.Sprintf("context updated but credentials not saved: %v", kerr))
					})
				} else if name != id {
					secrets.Delete(id)
				}
				// Drop the multiplexed ssh session so the next connection
				// re-authenticates and picks up server-side changes.
				sshutil.CloseControlMaster(strings.TrimPrefix(currentEndpoint, "ssh://"))
				sshutil.CloseControlMaster(strings.TrimPrefix(sshURL, "ssh://"))
			}
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("failed to update context: %v", err))
					app.RefreshCurrentView()
					return
				}
				app.AppendFlashSuccess(fmt.Sprintf("SSH context '%s' updated", name))
				if isCurrent {
					// Rebuild the client so the new endpoint/credentials apply immediately
					app.ReloadContext(name)
				} else {
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func showSSHForm(app common.AppController, authType string) {
	fields := []dialogs.FormField{
		{Name: "name", Label: "Name", Type: dialogs.FieldTypeInput, Placeholder: "prod-server"},
		{Name: "description", Label: "Description", Type: dialogs.FieldTypeInput},
		{Name: "user", Label: "SSH User", Type: dialogs.FieldTypeInput, Placeholder: "root"},
		{Name: "host", Label: "Host", Type: dialogs.FieldTypeInput, Placeholder: "192.168.1.100"},
		{Name: "port", Label: "SSH Port", Type: dialogs.FieldTypeInput, Default: "22", Placeholder: "22"},
	}

	if authType == secrets.AuthTypeKey {
		fields = append(fields,
			dialogs.FormField{
				Name:        "key",
				Label:       "SSH Key",
				Type:        dialogs.FieldTypeInput,
				Placeholder: "~/.ssh/id_ed25519",
				Suggestions: discoverSSHKeys(),
			},
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
		description := result["description"]
		user := strings.TrimSpace(result["user"])
		host := strings.TrimSpace(result["host"])
		port, portErr := validateSSHPort(result["port"])
		socket := strings.TrimSpace(result["socket"])

		if name == "" {
			app.SetFlashError("name is required")
			return
		}
		if user == "" {
			app.SetFlashError("SSH user is required")
			return
		}
		if host == "" {
			app.SetFlashError("host is required")
			return
		}
		if portErr != nil {
			app.SetFlashError(portErr.Error())
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

		sshURL := buildSSHURL(user, host, port, socket)

		app.AppendFlashPending(fmt.Sprintf("creating SSH context %s (%s)...", name, sshURL))

		app.RunInBackground(func() {
			err := app.GetDocker().CreateContext(name, description, sshURL)
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

func discoverSSHKeys() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sshDir := filepath.Join(home, ".ssh")
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sshDir, entry.Name()))
		if err != nil {
			continue
		}

		_, err = ssh.ParseRawPrivateKey(data)
		var passphraseMissing *ssh.PassphraseMissingError
		if err == nil || errors.As(err, &passphraseMissing) {
			keys = append(keys, filepath.Join("~", ".ssh", entry.Name()))
		}
	}
	return keys
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
