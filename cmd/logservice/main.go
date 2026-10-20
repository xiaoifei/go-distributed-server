package main

import (
	"context"
	"fmt"
	stdlog "log"

	"xiaoifei.top/log"
	"xiaoifei.top/registry"
	"xiaoifei.top/service"
)

func main() {
	log.Run("./distributed.log")
	host, port := "localhost", "4000"
	serviceAddress := fmt.Sprintf("http://%s:%s", host, port)
	ctx, err := service.Start(
		context.Background(),
		host,
		port,
		registry.Registration{
			ServiceName:      registry.LogService,
			ServiceURL:       serviceAddress,
			RequiredServices: make([]registry.ServiceName, 0),
			ServiceHookURL:   serviceAddress + "/services",
		},
		log.RegisterHandelers,
	)
	if err != nil {
		stdlog.Fatalln(err)
	}

	<-ctx.Done()
	fmt.Println("Shutdown Service")
}
