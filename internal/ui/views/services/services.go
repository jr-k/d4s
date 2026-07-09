package services

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/portforward"
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

	// Filter by Config Scope
	if scope != nil && scope.Type == "config" {
		return app.GetDocker().ListServicesForConfig(scope.Value)
	}

	services, err := app.GetDocker().ListServices()
	if err != nil {
		return nil, err
	}

	// Filter by Stack Scope
	if scope != nil && scope.Type == "stack" {
		var filtered []dao.Resource
		for _, s := range services {
			if svc, ok := s.(dao.Service); ok {
				if svc.Stack == scope.Value {
					filtered = append(filtered, s)
				}
			}
		}
		return filtered, nil
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
		common.FormatSCHeader("t", "Tasks"),
		common.FormatSCHeader("f", "Show PortForward"),
		common.FormatSCHeader("x", "Secrets"),
		common.FormatSCHeader("m", "ConfigMaps"),
		common.FormatSCHeader("l", "Logs"),
		common.FormatSCHeader("p", "Ps"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("s", "Scale"),
		common.FormatSCHeader("r", "Restart"),
		common.FormatSCHeader("z", "No Replica"),
		common.FormatSCHeader("shift-f", "Port-Forward"),
		common.FormatSCHeader("shift-e", "Edit Env"),
		common.FormatSCHeader("shift-x", "Attach Secrets"),
		common.FormatSCHeader("shift-m", "Edit Mounts"),
		common.FormatSCHeader("shift-n", "Attach Networks"),
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
	case 'f':
		ShowPortForwards(app, v)
		return nil
	case 'F':
		PortForwardAction(app, v)
		return nil
	case 'x':
		Secrets(app, v)
		return nil
	case 'X':
		SecretsPicker(app, v)
		return nil
	case 'm':
		Configs(app, v)
		return nil
	case 'M':
		MountsPicker(app, v)
		return nil
	case 'E':
		EnvPicker(app, v)
		return nil
	case 'N':
		NetworksPicker(app, v)
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
	case 'p':
		Ps(app, v)
		return nil
	case 'r':
		RestartAction(app, v)
		return nil
	case 'z':
		ScaleZero(app, v)
		return nil
	case 't':
		TasksAction(app, v)
		return nil
	case 'd':
		app.InspectCurrentSelection()
		return nil
	}

	return event
}

func ViewAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	r, _ := v.Table.GetSelection()
	name := id
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

func TasksAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	r, _ := v.Table.GetSelection()
	name := id
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

	app.SwitchTo(styles.TitleTasks)
}

func Logs(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}
	subject := resolveServiceSubject(v, id)

	app.OpenInspector(inspect.NewLogInspectorWithConfig(id, subject, "service", app.GetConfig().D4S.Logger))
}

func Ps(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	tasks, err := app.GetDocker().ListTasksForService(id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	subject := resolveServiceSubject(v, id)

	// Build node name map
	nodeNames := make(map[string]string)
	nodes, err := app.GetDocker().ListNodes()
	if err == nil {
		for _, n := range nodes {
			if node, ok := n.(dao.Node); ok {
				nodeNames[node.ID] = node.Hostname
			}
		}
	}

	var lines []string
	if len(tasks) == 0 {
		lines = append(lines, "# No tasks for this service")
	} else {
		// Header
		lines = append(lines, fmt.Sprintf("%-14s %-20s %-20s %-15s %-15s %s", "ID", "NAME", "NODE", "DESIRED STATE", "CURRENT STATE", "ERROR"))
		lines = append(lines, strings.Repeat("-", 120))

		for _, t := range tasks {
			taskID := t.ID
			if len(taskID) > 12 {
				taskID = taskID[:12]
			}

			taskName := t.Spec.ContainerSpec.Image
			if t.Slot > 0 {
				// For replicated services, use service name + slot
				taskName = fmt.Sprintf("%s.%d", t.ServiceID[:12], t.Slot)
			}

			nodeName := t.NodeID
			if name, ok := nodeNames[t.NodeID]; ok {
				nodeName = name
			} else if len(nodeName) > 12 {
				nodeName = nodeName[:12]
			}

			desiredState := string(t.DesiredState)
			currentState := string(t.Status.State)
			errorMsg := t.Status.Err

			lines = append(lines, fmt.Sprintf("%-14s %-20s %-20s %-15s %-15s %s", taskID, taskName, nodeName, desiredState, currentState, errorMsg))
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Service ps", subject, strings.Join(lines, "\n"), "text"))
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

func ScaleAction(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

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

func RestartAction(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil {
		return
	}

	dialogs.ShowConfirmation(app, "RESTART", fmt.Sprintf("%d services", len(ids)), func(force bool) {
		restartAction := func(id string) error {
			return app.GetDocker().RestartService(id)
		}
		app.PerformAction(restartAction, "restarting", styles.ColorStatusOrange)
	})
}

func ScaleZero(app common.AppController, v *view.ResourceView) {
	ids, err := v.GetSelectedIDs()
	if err != nil {
		return
	}

	msg := fmt.Sprintf("You are about to scale %d services to 0 replicas.\nThis will make them unavailable.\nAre you sure?", len(ids))

	dialogs.ShowConfirmation(app, "NO REPLICA", msg, func(force bool) {
		scaleAction := func(id string) error {
			return app.GetDocker().ScaleService(id, 0)
		}
		app.PerformAction(scaleAction, "scaling to zero", styles.ColorStatusOrange)
	})
}

func Inspect(app common.AppController, id string) {
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	inspector := inspect.NewTextInspector("Describe service", subject, fmt.Sprintf(" [%s]Loading service...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("service", id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				inspector.Viewer.Update(fmt.Sprintf("Error: %v", err), "text")
			})
			return
		}

		// Resolve Name
		resolvedSubject := subject
		services, err := app.GetDocker().ListServices()
		if err == nil {
			for _, item := range services {
				if item.GetID() == id {
					if s, ok := item.(dao.Service); ok {
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

	subject := resolveServiceSubject(v, id)
	app.SetFlashPending("loading secrets...")

	app.RunInBackground(func() {
		allSecrets, err := app.GetDocker().ListSecrets()
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		if len(allSecrets) == 0 {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError("no secrets available") })
			return
		}

		currentSecrets, err := app.GetDocker().GetServiceSecrets(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		attachedSecrets := make(map[string]*swarm.SecretReference)
		currentTargets := make(map[string]string)
		for _, s := range currentSecrets {
			attachedSecrets[s.SecretID] = s
			if s.File != nil && s.File.Name != "" {
				currentTargets[s.SecretID] = s.File.Name
			}
		}

		var items []dialogs.SecretAttachItem
		for _, sec := range allSecrets {
			s := sec.(dao.Secret)
			target := currentTargets[s.ID]
			if target == "" {
				target = s.Name
			}
			_, selected := attachedSecrets[s.ID]
			items = append(items, dialogs.SecretAttachItem{
				ID:         s.ID,
				SecretName: s.Name,
				Target:     target,
				Selected:   selected,
			})
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			app.SetFlashText("")
			dialogs.ShowSecretEditor(app, subject, items, func(selected []dialogs.SecretAttachItem) {
				var newSecretRefs []*swarm.SecretReference
				for _, s := range selected {
					target := strings.TrimSpace(s.Target)
					if target == "" {
						target = s.SecretName
					}
					newSecretRefs = append(newSecretRefs, &swarm.SecretReference{
						SecretID:   s.ID,
						SecretName: s.SecretName,
						File: &swarm.SecretReferenceFileTarget{
							Name: target,
							UID:  "0",
							GID:  "0",
							Mode: 0444,
						},
					})
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
		})
	})
}

func Configs(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	cfgs, err := app.GetDocker().GetServiceConfigs(id)
	if err != nil {
		app.SetFlashError(fmt.Sprintf("%v", err))
		return
	}

	subject := resolveServiceSubject(v, id)

	var lines []string
	if len(cfgs) == 0 {
		lines = append(lines, "# No configs attached to this service")
	} else {
		lines = append(lines, "# Configs attached to this service")
		lines = append(lines, "")
		for _, c := range cfgs {
			configID := c.ConfigID
			if len(configID) > 12 {
				configID = configID[:12]
			}
			line := fmt.Sprintf("- %s (ID: %s)", c.ConfigName, configID)
			if c.File != nil {
				line += fmt.Sprintf(" -> %s", c.File.Name)
			}
			lines = append(lines, line)
		}
	}

	app.OpenInspector(inspect.NewTextInspector("Configmap service", subject, strings.Join(lines, "\n"), "text"))
}

func ConfigsPicker(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	subject := resolveServiceSubject(v, id)
	app.SetFlashPending("loading configs...")

	app.RunInBackground(func() {
		allConfigs, err := app.GetDocker().ListConfigs()
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		if len(allConfigs) == 0 {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError("no configs available") })
			return
		}

		currentConfigs, err := app.GetDocker().GetServiceConfigs(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		attachedIDs := make(map[string]bool)
		for _, c := range currentConfigs {
			attachedIDs[c.ConfigID] = true
		}

		var items []dialogs.MultiPickerItem
		for _, cfg := range allConfigs {
			c := cfg.(dao.Config)
			items = append(items, dialogs.MultiPickerItem{
				ID:       c.ID,
				Label:    c.Name,
				Selected: attachedIDs[c.ID],
			})
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			app.SetFlashText("")
			dialogs.ShowMultiPicker(app, "Attach ConfigMaps", subject, items, func(selected []string) {
				selectedMap := make(map[string]bool)
				for _, id := range selected {
					selectedMap[id] = true
				}

				var newConfigRefs []*swarm.ConfigReference
				for _, cfg := range allConfigs {
					c := cfg.(dao.Config)
					if selectedMap[c.ID] {
						newConfigRefs = append(newConfigRefs, &swarm.ConfigReference{
							ConfigID:   c.ID,
							ConfigName: c.Name,
							File: &swarm.ConfigReferenceFileTarget{
								Name: c.Name,
								UID:  "0",
								GID:  "0",
								Mode: 0444,
							},
						})
					}
				}

				app.SetFlashPending("updating service configs...")
				app.RunInBackground(func() {
					err := app.GetDocker().SetServiceConfigs(id, newConfigRefs)
					app.GetTviewApp().QueueUpdateDraw(func() {
						if err != nil {
							app.SetFlashError(fmt.Sprintf("%v", err))
						} else {
							app.SetFlashSuccess("service configs updated")
							app.RefreshCurrentView()
						}
					})
				})
			})
		})
	})
}

func EnvPicker(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	subject := resolveServiceSubject(v, id)
	app.SetFlashPending("loading env...")

	app.RunInBackground(func() {
		env, err := app.GetDocker().GetServiceEnv(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		var items []dialogs.EnvItem
		for _, line := range env {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			value := ""
			if len(parts) == 2 {
				value = parts[1]
			}
			items = append(items, dialogs.EnvItem{
				Key:      key,
				Value:    value,
				Selected: true,
			})
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			app.SetFlashText("")
			dialogs.ShowEnvEditor(app, subject, items, func(envVars []string) {
				app.SetFlashPending("updating service env...")
				app.RunInBackground(func() {
					err := app.GetDocker().SetServiceEnv(id, envVars)
					app.GetTviewApp().QueueUpdateDraw(func() {
						if err != nil {
							app.SetFlashError(fmt.Sprintf("%v", err))
						} else {
							app.SetFlashSuccess("service env updated")
							app.RefreshCurrentView()
						}
					})
				})
			})
		})
	})
}

func NetworksPicker(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	subject := resolveServiceSubject(v, id)
	app.SetFlashPending("loading networks...")

	app.RunInBackground(func() {
		allNetworks, err := app.GetDocker().ListNetworks()
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		if len(allNetworks) == 0 {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError("no networks available") })
			return
		}

		currentNetworks, err := app.GetDocker().GetServiceNetworks(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		attachedIDs := make(map[string]bool)
		for _, n := range currentNetworks {
			attachedIDs[n.Target] = true
		}

		var items []dialogs.MultiPickerItem
		for _, res := range allNetworks {
			if n, ok := res.(dao.Network); ok {
				if n.Scope != "swarm" {
					continue
				}
				items = append(items, dialogs.MultiPickerItem{
					ID:       n.ID,
					Label:    n.Name,
					Selected: attachedIDs[n.ID],
				})
			}
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			app.SetFlashText("")
			dialogs.ShowMultiPicker(app, "Attach Networks", subject, items, func(selected []string) {
				var newNetworks []swarm.NetworkAttachmentConfig
				for _, netID := range selected {
					newNetworks = append(newNetworks, swarm.NetworkAttachmentConfig{
						Target: netID,
					})
				}

				app.SetFlashPending("updating service networks...")
				app.RunInBackground(func() {
					err := app.GetDocker().SetServiceNetworks(id, newNetworks)
					app.GetTviewApp().QueueUpdateDraw(func() {
						if err != nil {
							app.SetFlashError(fmt.Sprintf("%v", err))
						} else {
							app.SetFlashSuccess("service networks updated")
							app.RefreshCurrentView()
						}
					})
				})
			})
		})
	})
}

func MountsPicker(app common.AppController, v *view.ResourceView) {
	id, err := v.GetSelectedID()
	if err != nil {
		return
	}

	subject := resolveServiceSubject(v, id)
	app.SetFlashPending("loading mounts...")

	app.RunInBackground(func() {
		allVolumes, err := app.GetDocker().ListVolumes()
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		availableVolumes := make(map[string]bool)
		for _, res := range allVolumes {
			if vol, ok := res.(dao.Volume); ok {
				availableVolumes[vol.Name] = true
			}
		}

		currentMounts, err := app.GetDocker().GetServiceMounts(id)
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() { app.SetFlashError(fmt.Sprintf("%v", err)) })
			return
		}

		var items []dialogs.MountAttachItem
		for _, current := range currentMounts {
			source := current.Source
			mountType := string(current.Type)
			if mountType == "" {
				mountType = string(mount.TypeVolume)
			}

			target := current.Target
			if target == "" && source != "" {
				target = "/" + source
			}

			items = append(items, dialogs.MountAttachItem{
				ID:        mountKey(mountType, source, target),
				Source:    source,
				MountType: mountType,
				Target:    target,
				Selected:  true,
			})
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			app.SetFlashText("")
			dialogs.ShowMountEditor(app, subject, items, availableVolumes, func(selected []dialogs.MountAttachItem) {
				var newMounts []mount.Mount
				for _, item := range selected {
					mountType := strings.ToLower(strings.TrimSpace(item.MountType))
					if mountType == "" {
						mountType = string(mount.TypeVolume)
					}

					source := strings.TrimSpace(item.Source)
					target := strings.TrimSpace(item.Target)
					if target == "" && source != "" {
						target = "/" + source
					}

					newMounts = append(newMounts, mount.Mount{
						Type:   mount.Type(mountType),
						Source: source,
						Target: target,
					})
				}

				app.SetFlashPending("updating service mounts...")
				app.RunInBackground(func() {
					err := app.GetDocker().SetServiceMounts(id, newMounts)
					app.GetTviewApp().QueueUpdateDraw(func() {
						if err != nil {
							app.SetFlashError(fmt.Sprintf("%v", err))
						} else {
							app.SetFlashSuccess("service mounts updated")
							app.RefreshCurrentView()
						}
					})
				})
			})
		})
	})
}

func mountKey(mountType, source, target string) string {
	return fmt.Sprintf("%s:%s:%s", mountType, source, target)
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

func ShowPortForwards(app common.AppController, v *view.ResourceView) {
	if !app.GetDocker().IsSSHContext() {
		app.AppendFlashError("port-forward is only available on SSH contexts")
		return
	}

	_, err := v.GetSelectedID()
	if err != nil {
		return
	}

	serviceName := getSelectedServiceName(v)
	if serviceName == "" {
		return
	}

	app.SetActiveScope(&common.Scope{
		Type:       "service",
		Value:      serviceName,
		Label:      serviceName,
		OriginView: styles.TitleServices,
	})
	app.SwitchTo(styles.TitlePortForwards)
}

func PortForwardAction(app common.AppController, v *view.ResourceView) {
	if !app.GetDocker().IsSSHContext() {
		app.AppendFlashError("port-forward is only available on SSH contexts")
		return
	}

	_, err := v.GetSelectedID()
	if err != nil {
		return
	}

	serviceName := getSelectedServiceName(v)
	if serviceName == "" {
		return
	}

	pickerTitle := fmt.Sprintf("Port-Forward: %s", serviceName)
	dialogs.ShowPickerLoading(app, pickerTitle)

	app.RunInBackground(func() {
		containers, err := app.GetDocker().ListContainers()
		if err != nil {
			app.GetTviewApp().QueueUpdateDraw(func() {
				app.GetPages().RemovePage("picker")
				app.AppendFlashError(fmt.Sprintf("failed to list containers: %v", err))
			})
			return
		}

		var items []dialogs.PickerItem
		for _, r := range containers {
			if c, ok := r.(dao.Container); ok {
				if c.ServiceName == serviceName && c.Ports != "" {
					items = append(items, dialogs.PickerItem{
						Label:       c.Names,
						Description: c.Ports,
						Value:       c.ID,
					})
				}
			}
		}

		app.GetTviewApp().QueueUpdateDraw(func() {
			if len(items) == 0 {
				app.GetPages().RemovePage("picker")
				app.AppendFlashError("no containers with exposed ports for this service")
				return
			}

			if len(items) == 1 {
				app.GetPages().RemovePage("picker")
				portForwardContainer(app, containers, items[0].Value)
				return
			}

			dialogs.ShowPicker(app, pickerTitle, items, func(containerID string) {
				portForwardContainer(app, containers, containerID)
			})
		})
	})
}

func getSelectedServiceName(v *view.ResourceView) string {
	row, _ := v.Table.GetSelection()
	if row > 0 {
		index := row - 1
		if index >= 0 && index < len(v.Data) {
			if s, ok := v.Data[index].(dao.Service); ok {
				return s.Name
			}
		}
	}
	return ""
}

func portForwardContainer(app common.AppController, containers []dao.Resource, containerID string) {
	var name, portsStr string
	for _, r := range containers {
		if c, ok := r.(dao.Container); ok {
			if c.ID == containerID {
				name = c.Names
				portsStr = c.Ports
				break
			}
		}
	}

	portInfos := parsePortsString(portsStr)
	if len(portInfos) == 0 {
		app.AppendFlashError("no ports found")
		return
	}

	dialogs.ShowPortForwardDialog(app, containerID, name, portInfos, func(result dialogs.PortForwardResult) {
		app.SetFlashPending(fmt.Sprintf("forwarding %s:%d...", result.Address, result.LocalPort))
		app.RunInBackground(func() {
			containerIP, err := app.GetDocker().GetContainerIP(containerID)
			if err != nil {
				app.GetTviewApp().QueueUpdateDraw(func() {
					app.AppendFlashError(fmt.Sprintf("failed to get container IP: %v", err))
				})
				return
			}

			pf := &portforward.PortForward{
				ContextName:   app.GetDocker().ContextName,
				SSHHost:       app.GetDocker().GetSSHHost(),
				ContainerID:   containerID,
				ContainerName: name,
				ContainerIP:   containerIP,
				ContainerPort: result.ContainerPort,
				HostPort:      result.HostPort,
				LocalPort:     result.LocalPort,
			}

			err = app.GetPortForwardManager().Add(pf)
			app.GetTviewApp().QueueUpdateDraw(func() {
				if err != nil {
					app.AppendFlashError(fmt.Sprintf("port-forward failed: %v", err))
				} else {
					app.AppendFlashSuccess(fmt.Sprintf("forwarding %s:%d -> container:%d", result.Address, result.LocalPort, result.ContainerPort))
					app.RefreshCurrentView()
				}
			})
		})
	})
}

func parsePortsString(ports string) []dialogs.PortInfo {
	var result []dialogs.PortInfo
	seen := make(map[uint16]bool)
	for _, part := range strings.Split(ports, ", ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts := strings.SplitN(part, "->", 2)
		if len(parts) != 2 {
			continue
		}
		var cp int
		fmt.Sscanf(parts[1], "%d", &cp)
		if cp <= 0 || seen[uint16(cp)] {
			continue
		}
		seen[uint16(cp)] = true
		// Host part is either "8080" or "0.0.0.0:8080"
		var hp int
		hostPart := parts[0]
		if idx := strings.LastIndex(hostPart, ":"); idx >= 0 {
			hostPart = hostPart[idx+1:]
		}
		fmt.Sscanf(hostPart, "%d", &hp)
		result = append(result, dialogs.PortInfo{
			ContainerPort: uint16(cp),
			HostPort:      uint16(hp),
			Protocol:      "tcp",
		})
	}
	return result
}
