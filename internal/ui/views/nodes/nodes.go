package nodes

import (
	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
)

var Headers = []string{"ID", "HOSTNAME", "STATUS", "AVAIL", "ROLE", "VERSION"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListNodes()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveNode(id, force)
}

