package main

import (
	"context"
	"fmt"
	stdlog "log"

	"xiaoifei.top/log"
	"xiaoifei.top/registry"
	"xiaoifei.top/separation"
	"xiaoifei.top/service"
)

func main() {
	host, port := "localhost", "6000"
	serviceAddress := fmt.Sprintf("http://%s:%s", host, port)
	ctx, err := service.Start(
		context.Background(),
		host,
		port,
		registry.Registration{
			ServiceName:      registry.SeperationService,
			ServiceURL:       serviceAddress,
			RequiredServices: []registry.ServiceName{registry.LogService},
			ServiceHookURL:   serviceAddress + "/services",
			HeartbeatURL:     serviceAddress + "/heartbeat",
		},
		separation.RegisterHandlers,
	)
	if err != nil {
		stdlog.Fatalln(err)
	}

	registry.OnProviderChange(registry.LogService, func() {
		logProvider, err := registry.GetProvider(registry.LogService)
		if err != nil {
			log.SetDefaultLogger()
			return
		}
		fmt.Printf("Logging service found at: %s\n", logProvider)
		log.SetClientLogger(logProvider, registry.SeperationService)
	})

	<-ctx.Done()
	fmt.Println("Shutdown Service")
}
