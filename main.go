package main

import (
	"github.com/jawher/mow.cli"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/gorilla/mux"
	"github.com/rcrowley/go-metrics"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	log "github.com/Financial-Times/go-logger"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"time"
)

const appDescription = "Service that reports whether the annotations publishing flow works as expected."

func main() {
	ticker := time.NewTicker(1 * time.Minute)
	app := initApp(ticker)
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("App could not start, error=[%s]\n", err)
		return
	}
}

func initApp(ticker *time.Ticker) *cli.Cli {
	app := cli.App("annotations-publish-healthchecker", appDescription)

	appSystemCode := app.String(cli.StringOpt{
		Name:   "app-system-code",
		Value:  "annotations-publish-healthchecker",
		Desc:   "System Code of the application",
		EnvVar: "APP_SYSTEM_CODE",
	})

	appName := app.String(cli.StringOpt{
		Name:   "app-name",
		Value:  "Annotations Publish Healthchecker",
		Desc:   "Application name",
		EnvVar: "APP_NAME",
	})

	eventReader := app.String(cli.StringOpt{
		Name:   "event-reader",
		Value:  "http://localhost:8083/__splunk-event-reader",
		Desc:   "Splunk Event Reader Address",
		EnvVar: "SPLUNK_EVENT_READER",
	})

	slaWindow := app.Int(cli.IntOpt{
		Name:   "sla-window",
		Value:  2,
		Desc:   "Time period to ignore, when we have no information about annotations publishes. Given in minutes.",
		EnvVar: "SLA_WINDOW",
	})

	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8083",
		Desc:   "Port to listen on",
		EnvVar: "APP_PORT",
	})

	log.InitLogger(*appSystemCode, "info")
	log.Infof("[Startup] annotations-publish-healthchecker is starting ")

	app.Action = func() {
		log.Infof("System code: %s, App Name: %s, Port: %s", *appSystemCode, *appName, *port)

		s := healthcheckerService{
			eventReaderAddress: *eventReader,
			healthStatus:       healthStatus{},
			slaWindow:          *slaWindow,
		}
		s.monitorPublishHealth(ticker)

		go func() {
			routeRequests(*appSystemCode, *appName, *port, &s)
		}()

		waitForSignal()
	}

	return app
}

func routeRequests(appSystemCode string, appName string, port string, healthchecker *healthcheckerService) {
	healthService := newHealthService(&healthConfig{appSystemCode: appSystemCode, appName: appName, port: port}, healthchecker)

	serveMux := http.NewServeMux()

	hc := health.TimedHealthCheck{
		HealthCheck: health.HealthCheck{
			SystemCode:  appSystemCode,
			Name:        appName,
			Description: appDescription,
			Checks:      healthService.checks,
		},
		Timeout: 10 * time.Second,
	}

	serveMux.HandleFunc(healthPath, health.Handler(hc))
	serveMux.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.gtgCheck))
	serveMux.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	handler := requestHandler{healthchecker}
	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/__details", handler.getHealthDetails).Methods("GET")

	var monitoringRouter http.Handler = servicesRouter
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.Logger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	serveMux.Handle("/", monitoringRouter)

	server := &http.Server{Addr: ":" + port, Handler: serveMux}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Infof("HTTP server closing with message: %v", err)
		}
		wg.Done()
	}()

	waitForSignal()
	log.Infof("[Shutdown] annotations-publish-healthchecker is shutting down")

	if err := server.Close(); err != nil {
		log.Errorf("Unable to stop http server: %v", err)
	}

	wg.Wait()
}

func waitForSignal() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
