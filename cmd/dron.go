package main

import (
	"os"
	"os/signal"

	"github.com/bububa/cron"
	log "github.com/sirupsen/logrus"
	"github.com/stepanstipl/dron/events"
	"sync"
)

var (
	version string = "DEV"
)

func main() {
	// Configure logging
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.InfoLevel)
	log.SetOutput(os.Stdout)

	log.Infof("Starting dcron version %s.", version)
	// We want to cleanup on interrupt
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	wg := sync.WaitGroup{}

	// Create and start cron
	log.Infof("Starting cron scheduler.")
	c := cron.New()
	go c.Start()

	// Be nice
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for _ = range signalChan {
			log.Infof("Received interrupt, cleaning up.")
			c.Stop()
			wg.Wait()
			cleanupDone <- true
		}
	}()

	// Start central logic
	go events.NewRouter(c)

	// Wait till we're done
	<-cleanupDone
	log.Infof("Bye bye!")
}
