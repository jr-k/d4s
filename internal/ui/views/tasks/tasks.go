package tasks

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jr-k/d4s/internal/dao"
	"github.com/jr-k/d4s/internal/ui/common"
	"github.com/jr-k/d4s/internal/ui/components/inspect"
	"github.com/jr-k/d4s/internal/ui/components/view"
	"github.com/jr-k/d4s/internal/ui/styles"
)

var Headers = []string{"ID", "NAME", "IMAGE", "NODE", "DESIRED STATE", "CURRENT STATE", "ERROR", "CONTAINER"}

func Fetch(app common.AppController, v *view.ResourceView) ([]dao.Resource, error) {
	scope := app.GetActiveScope()

	if scope != nil && scope.Type == "service" {
		if nodeScope := findScope(scope.Parent, "node"); nodeScope != nil {
			return app.GetDocker().ListTasksForServiceAndNodeResource(scope.Value, nodeScope.Value)
		}
		return app.GetDocker().ListTasksForServiceResource(scope.Value)
	}

	if scope != nil && scope.Type == "node" {
		return app.GetDocker().ListTasksForNodeResource(scope.Value)
	}

	return app.GetDocker().ListTasks()
}

func findScope(scope *common.Scope, scopeType string) *common.Scope {
	for scope != nil {
		if scope.Type == scopeType {
			return scope
		}
		scope = scope.Parent
	}
	return nil
}

func GetShortcuts() []string {
	return []string{
		common.FormatSCHeader("enter", "Containers"),
		common.FormatSCHeader("d", "Describe"),
		common.FormatSCHeader("l", "Logs"),
	}
}

func InputHandler(v *view.ResourceView, event *tcell.EventKey) *tcell.EventKey {
	app := v.App

	switch event.Rune() {
	case 'd':
		app.InspectCurrentSelection()
		return nil
	case 'l':
		Logs(app, v)
		return nil
	}

	if event.Key() == tcell.KeyEnter {
		NavigateToContainers(app, v)
		return nil
	}

	return event
}

func NavigateToContainers(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row <= 0 || row > len(v.Data) {
		return
	}

	t, ok := v.Data[row-1].(dao.Task)
	if !ok {
		return
	}

	if t.ContainerID == "" {
		app.SetFlashError("no container for this task")
		return
	}

	app.SetActiveScope(&common.Scope{
		Type:       "task",
		Value:      t.ContainerID,
		Label:      t.Name,
		OriginView: styles.TitleTasks,
	})

	app.SwitchTo(styles.TitleContainers)
}

func Logs(app common.AppController, v *view.ResourceView) {
	row, _ := v.Table.GetSelection()
	if row <= 0 || row > len(v.Data) {
		return
	}

	t, ok := v.Data[row-1].(dao.Task)
	if !ok {
		return
	}

	if t.ContainerID == "" {
		app.SetFlashError("no container for this task")
		return
	}

	subject := t.Name
	app.OpenInspector(inspect.NewLogInspectorWithConfig(t.ContainerID, subject, "container", app.GetConfig().D4S.Logger))
}

func Inspect(app common.AppController, id string) {
	subject := id
	if len(id) > 12 {
		subject = id[:12]
	}
	inspector := inspect.NewTextInspector("Describe task", subject, fmt.Sprintf(" [%s]Loading task...\n", styles.TagAccent), "json")
	app.OpenInspector(inspector)

	app.RunInBackground(func() {
		content, err := app.GetDocker().Inspect("task", id)
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
