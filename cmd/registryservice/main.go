package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"xiaoifei.top/registry"
)

func main() {
	registry.SetupRegistryService()
	http.Handle("/services", &registry.RegistryService{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var server http.Server
	server.Addr = registry.ServerPort
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("服务异常：%v", err)
			cancel()
		}
	}()

	go func() {
		fmt.Printf("Registry service started. Press Enter to stop.\n")
		fmt.Scanln()

		shutdownCtx, shutdwonCancle := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer shutdwonCancle()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("优雅关闭失败，强制关闭: %v", err)
			if err := server.Close(); err != nil {
				log.Printf("强制关闭失败: %v", err)
			}
		}
		cancel()
	}()

	<-ctx.Done()
	fmt.Println("Shutting down registry service")
}
