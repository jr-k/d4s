package dao

import (
	"io"

	"github.com/docker/docker/pkg/stdcopy"
)

func (d *DockerClient) DumpContainerLogs(id string, w io.Writer, timestamps bool) error {
	reader, err := d.Container.LogsSnapshot(id, timestamps)
	if err != nil {
		return err
	}
	defer reader.Close()

	hasTTY, err := d.HasTTY(id)
	if err != nil {
		return err
	}

	if hasTTY {
		_, err = io.Copy(w, reader)
		return err
	}

	_, err = stdcopy.StdCopy(w, w, reader)
	return err
}
