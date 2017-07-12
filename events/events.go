package events

import (
	"context"

	"github.com/bububa/cron"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/stepanstipl/dron/jobs"
	"time"
)

type Router struct {
	cli     *client.Client
}

type Handler struct {
	cron 	*cron.Cron
	cli     *client.Client
}

func NewHandler(c *cron.Cron, cli *client.Client) *Handler {
	return &Handler{
		cron: c,
		cli: cli,
	}
}

func (h *Handler) Sync() {
	// Add current containers
	containers, _ := h.cli.ContainerList(context.Background(), types.ContainerListOptions{})
	for _, co := range containers {
		js := jobs.BuildFromLabels(co.ID, co.Labels)
		jobs.Add(h.cron, js)
	}
}

func NewRouter(c *cron.Cron) (Router) {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Errorf("Failed to create Docker client.")
		panic(err)
	}

	router := Router{cli: cli}
	handler := NewHandler(c, cli)

	timeout := 1 * time.Second

	for {
		eventChan, errChan := router.Listen()
		log.Infof("Listening for Docker events.")
		handler.Sync()

		select {
		case event := <-eventChan:
			handler.Handle(&event)
			if timeout > 1 * time.Second {
				timeout = 1 * time.Second
			}
		case err := <-errChan:
			log.WithError(err).Errorf("Error on Docker event stream, waiting %s.", timeout)
			time.Sleep(timeout)
			if timeout < 5 * time.Minute {
				timeout *= 2
			}
		}
	}
}

// Handle implements handler interface
func (this Handler) Handle(msg *events.Message) {
		if msg.Action == "start" || msg.Action == "create" {
			log.Infof("Processing %s event for container: %s", msg.Action, msg.ID)

			container, _ := this.cli.ContainerInspect(context.Background(), msg.ID)
			js := jobs.BuildFromLabels(msg.ID, container.Config.Labels)
			jobs.Add(this.cron, js)
		}

		if msg.Action == "destroy" {
			log.Infof("Processing %s event for container: %s", msg.Action, msg.ID)
			jobs.Delete(this.cron, msg.ID)
		}
}

// Listen implements the Router interface
func (router Router) Listen() (<-chan events.Message, <-chan error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Errorf("Failed to create Docker client.")
		panic(err)
	}
	filterArgs := filters.NewArgs()

	// Listen to start, create & destroy
	filterArgs.Add("event", "start")
	filterArgs.Add("event", "create")
	filterArgs.Add("event", "destroy")

	eventOptions := types.EventsOptions{
		Filters: filterArgs,
	}

	return cli.Events(context.Background(), eventOptions)
}