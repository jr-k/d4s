package images

import (
	"github.com/jessym/d4s/internal/dao"
	"github.com/jessym/d4s/internal/ui/common"
)

var Headers = []string{"ID", "TAGS", "SIZE", "CREATED"}

func Fetch(app common.AppController) ([]dao.Resource, error) {
	return app.GetDocker().ListImages()
}

func Prune(app common.AppController) error {
	return app.GetDocker().PruneImages()
}

func Remove(id string, force bool, app common.AppController) error {
	return app.GetDocker().RemoveImage(id, force)
}

