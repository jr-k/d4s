package compose

import (
	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
)

var Headers = []string{"PROJECT", "STATUS", "CONFIG FILES"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListCompose()
}

func Restart(app common.AppController, id string) error {
	return app.GetDocker().RestartComposeProject(id)
}

func Stop(app common.AppController, id string) error {
	return app.GetDocker().StopComposeProject(id)
}

