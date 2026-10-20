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
		},
		separation.RegisterHandlers,
	)
	if err != nil {
		stdlog.Fatalln(err)
	}

	if logProvider, err := registry.GetProvider(registry.LogService); err == nil {
		fmt.Printf("Logging service found at: %s\n", logProvider)
		log.SetClientLogger(logProvider, registry.SeperationService)
	}

	<-ctx.Done()
	fmt.Println("Shutdown Service")
}
