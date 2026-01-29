package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/dialogs"
	"github.com/jr-k/d4s/internal/ui/styles"
)

func resolveServiceSubject(v *view.ResourceView, id string) string {
	name := ""
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if s, ok := v.Data[index].(dao.Service); ok {
				name = s.Name
			}
		}
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	if name != "" {
		return fmt.Sprintf("%s@%s", name, subject)
	}
	return subject
}

func Env(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	env, err := app.GetDocker().GetServiceEnv(id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	subject := resolveServiceSubject(v, id)

	var lines []string
	for _, line := range env {
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		lines = append(lines, "# No environment variables defined")
	}

	app.OpenInspector(inspect.NewTextInspector("Environment service", subject, strings.Join(lines, "\n"), "env"))
}

func Secrets(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	secrets, err := app.GetDocker().GetServiceSecrets(id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	subject := resolveServiceSubject(v, id)

	var lines []string
	if len(secrets) == 0 {
		lines = append(lines, "# No secrets attached to this service")
	} else {
		lines = append(lines, "# Secrets attached to this service")
		lines = append(lines, "# (Secret values are not accessible for security reasons)")
		lines = append(lines, "")
		for _, s := range secrets {
			secretID := s.SecretID
			if len(secretID) > 12 {
				secretID = secretID[:12]
			}
			line := fmt.Sprintf("- %s (ID: %s)", s.SecretName, secretID)
			if s.File != nil {
				line += fmt.Sprintf(" -> %s", s.File.Name)
			}
			lines = append(lines, line)
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Secrets service", subject, strings.Join(lines, "\n"), "text"))
}

var Headers = []string{"ID", "NAME", "IMAGE", "MODE", "REPLICAS", "PORTS", "CREATED", "UPDATED"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	scope := app.GetActiveScope()

	// Filter by Secret Scope
	if scope != nil && scope.Type == "secret" {
		return app.GetDocker().ListServicesForSecret(scope.Value)
	}

	services, err := app.GetDocker().ListServices()
	if err != nil {
		return nil, err
	}

	// Filter by Node Scope
	if scope != nil && scope.Type == "node" {
		nodeID := scope.Value
		var filtered []dao.Resource

		// We need to check which services have tasks on this node
		// This requires an extra call to list tasks for this node
		tasks, err := app.GetDocker().ListTasksForNode(nodeID)
		if err == nil {
			serviceIDs := make(map[string]bool)
			for _, task := range tasks {
				serviceIDs[task.ServiceID] = true
			}

			for _, s := range services {
				if serviceIDs[s.GetID()] {
					filtered = append(filtered, s)
				}
			}
			return filtered, nil
		}
	}

	return services, nil
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("e", "Env"),
		common.FormatSCHeader("x", "Secrets"),
		common.FormatSCHeader("l", "Logs"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("s", "Scale"),
		common.FormatSCHeader("z", "No Replica"),
		common.FormatSCHeader("shift-x", "Attach Secrets"),
		common.FormatSCHeader("shift-i", "Edit Image"),
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
		ViewAction(app, v)
		return nil
	}
	
	switch event.Rune() {
	case 'e':
		Env(app, v)
		return nil
	case 'x':
		Secrets(app, v)
		return nil
	case 'X':
		SecretsPicker(app, v)
		return nil
	case 'I':
		EditAction(app, v)
		return nil
	case 's':
		ScaleAction(app, v)
		return nil
	case 'l':
		Logs(app, v)
		return nil
	case 'z':
		ScaleZero(app, v)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	}
	
	return event
}

func ViewAction(app common.AppController, v *view.ResourceView) {
	// Filter containers by this service
	id, err := v.GetSelectedID()
	if err != nil { return }
	
	r, _ := v.Table.GetSelection()
	// Headers: "ID", "NAME", ...
	// Name is column 1
	name := id // Fallback
	nameCell := v.Table.GetCell(r, 1)
	if nameCell != nil {
		name = nameCell.Text
	}

	trimSpaceLeftRightName := strings.TrimSpace(name)
	
	app.SetActiveScope(&common.Scope{
		Type:       "service",
		Value:      trimSpaceLeftRightName,
		Label:      trimSpaceLeftRightName,
		OriginView: styles.TitleServices,
	})
	
	app.SwitchTo(styles.TitleContainers)
}

func Logs(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	
	// Open Logs view
	app.OpenInspector(inspect.NewLogInspector(id, id, "service"))
}

func DeleteAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }

	dialogs.ShowConfirmation(app, "DELETE", fmt.Sprintf("%d services", len(ids)), func(force bool) {
		simpleAction := func(id string) error {
			return Remove(id, force, app)
		}
		app.PerformAction(simpleAction, "deleting", styles.ColorStatusRed)
	})
}

func ScaleAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil { return }
	
	currentReplicas := ""
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		item := v.Data[row-1]
		cells := item.GetCells()
		if len(cells) > 4 {
			currentReplicas = strings.TrimSpace(cells[4])
		}
	}
	Scale(app, id, currentReplicas)
}

func ScaleZero(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil { return }
	
	msg := fmt.Sprintf("You are about to scale %d services to 0 replicas.\nThis will make them unavailable.\nAre you sure?", len(ids))
	
	dialogs.ShowConfirmation(app, "NO REPLICA", msg, func(force bool) {
		scaleAction := func(id string) error {
			return app.GetDocker().ScaleService(id, 0)
		}
		app.PerformAction(scaleAction, "scaling to zero", styles.ColorStatusOrange)
	})
}

func Inspect(app common.AppController, id string) {
	content, err := app.GetDocker().Inspect("service", id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}

	// Resolve Name
	services, err := app.GetDocker().ListServices()
	if err == nil {
		for _, item := range services {
			if item.GetID() == id {
				if s, ok := item.(dao.Service); ok {
					subject = fmt.Sprintf("%s@%s", s.Name, subject)
				}
				break
			}
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Describe service", subject, content, "json"))
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveService(id)
}

func Scale(app common.AppController, id string, currentReplicas string) {
	if parts := strings.Split(currentReplicas, "/"); len(parts) == 2 {
		currentReplicas = parts[1]
	}

	dialogs.ShowInput(app, "Scale Service", "Replicas:", currentReplicas, func(text string) {
		replicas, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			app.SetFlashError("invalid number")
			return
		}
		
		app.SetFlashPending(fmt.Sprintf("scaling %s to %d...", id, replicas))
		
		app.RunInBackground(func() {
			err := app.GetDocker().ScaleService(id, replicas)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess(fmt.Sprintf("service scaled to %d", replicas))
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func SecretsPicker(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	// Get all secrets
	allSecrets, err := app.GetDocker().ListSecrets()
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	if len(allSecrets) == 0 {
		app.SetFlashError("no secrets available")
		return
	}

	// Get current service secrets
	currentSecrets, err := app.GetDocker().GetServiceSecrets(id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	// Build map of attached secret IDs
	attachedIDs := make(map[string]bool)
	for _, s := range currentSecrets {
		attachedIDs[s.SecretID] = true
	}

	// Build picker items
	var items []dialogs.MultiPickerItem
	for _, sec := range allSecrets {
		s := sec.(dao.Secret)
		items = append(items, dialogs.MultiPickerItem{
			ID:       s.ID,
			Label:    s.Name,
			Selected: attachedIDs[s.ID],
		})
	}

	subject := resolveServiceSubject(v, id)

	dialogs.ShowMultiPicker(app, fmt.Sprintf("Secrets for %s", subject), items, func(selected []string) {
		// Build new secret references
		selectedMap := make(map[string]bool)
		for _, id := range selected {
			selectedMap[id] = true
		}

		// Build secret refs from selected IDs
		var newSecretRefs []*swarm.SecretReference
		for _, sec := range allSecrets {
			s := sec.(dao.Secret)
			if selectedMap[s.ID] {
				newSecretRefs = append(newSecretRefs, &swarm.SecretReference{
					SecretID:   s.ID,
					SecretName: s.Name,
					File: &swarm.SecretReferenceFileTarget{
						Name: s.Name,
						UID:  "0",
						GID:  "0",
						Mode: 0444,
					},
				})
			}
		}

		app.SetFlashPending("updating service secrets...")
		app.RunInBackground(func() {
			err := app.GetDocker().SetServiceSecrets(id, newSecretRefs)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess("service secrets updated")
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func EditAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	currentImage := ""
	row, _ := v.Table.GetSelection()
	if row > 0 && row <= len(v.Data) {
		item := v.Data[row-1]
		cells := item.GetCells()
		if len(cells) > 2 {
			currentImage = strings.TrimSpace(cells[2])
		}
	}

	fields := []dialogs.FormField{
		{Name: "image", Label: "Image", Type: dialogs.FieldTypeInput, Default: currentImage},
	}

	dialogs.ShowForm(app, "Edit Service Image", fields, func(result dialogs.FormResult) {
		image := result["image"]
		if image == "" {
			return
		}

		app.SetFlashPending(fmt.Sprintf("updating service %s...", id))

		app.RunInBackground(func() {
			err := app.GetDocker().UpdateServiceImage(id, image)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.SetFlashError(fmt.Sprintf("%v", err))
				} else {
					app.SetFlashSuccess("service updated")
					app.RefreshCurrentView()
				}
			})
		})
	})
}
