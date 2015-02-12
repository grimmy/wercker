package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

// ServiceBox wraps a box as a service
type ServiceBox struct {
	*Box
	murder *LogEntry
}

// ToServiceBox turns a box into a ServiceBox
func (b *RawBox) ToServiceBox(options *PipelineOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	return NewServiceBox(string(*b), options, boxOptions)
}

// NewServiceBox from a name and other references
func NewServiceBox(name string, options *PipelineOptions, boxOptions *BoxOptions) (*ServiceBox, error) {
	box, err := NewBox(name, options, boxOptions)
	murder := rootLogger.WithField("Logger", "Service")
	return &ServiceBox{Box: box, murder: murder}, err
}

// Run executes the service
func (b *ServiceBox) Run() (*docker.Container, error) {
	containerName := fmt.Sprintf("wercker-service-%s-%s", strings.Replace(b.Name, "/", "-", -1), b.options.PipelineID)

	container, err := b.client.CreateContainer(
		docker.CreateContainerOptions{
			Name: containerName,
			Config: &docker.Config{
				Image:           b.Name,
				NetworkDisabled: b.networkDisabled,
			},
		})

	if err != nil {
		return nil, err
	}

	b.client.StartContainer(container.ID, &docker.HostConfig{})
	b.container = container

	go func() {
		status, err := b.client.WaitContainer(container.ID)
		if err != nil {
			b.murder.Errorln("Error waiting", err)
		}
		b.murder.Debugln("Service container finished with status code:", status, container.ID)

		if status != 0 {
			// recv := make(chan string)
			// outputStream := NewReceiver(recv)
			opts := docker.LogsOptions{
				Container:    container.ID,
				Stdout:       true,
				Stderr:       true,
				ErrorStream:  os.Stderr,
				OutputStream: os.Stdout,
				RawTerminal:  false,
			}
			err = b.client.Logs(opts)
			if err != nil {
				b.murder.Panicln(err)
			}
		}
	}()

	return container, nil
}
