package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"xiaoifei.top/registry"
)

func Start(ctx context.Context, host, port string, reg registry.Registration, registerHandlerFunc func()) (context.Context, error) {
	// 注册handlers
	registerHandlerFunc()
	// 启动服务器
	ctx = startService(ctx, reg.ServiceName, host, port)
	// 向注册中心发送自己的信息
	err := registry.RegisterService(reg)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}

func startService(ctx context.Context, serviceName registry.ServiceName, host, port string) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	var server http.Server
	server.Addr = ":" + port

	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("服务异常: %v", err)
			err := registry.ShutdownService(fmt.Sprintf("http://%s:%s", host, port))
			if err != nil {
				log.Println(err)
			}
			cancel()
		}
	}()

	go func() {
		fmt.Printf("%v started. Press Enter to stop.\n", serviceName)
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

		err := registry.ShutdownService(fmt.Sprintf("http://%s:%s", host, port))
		if err != nil {
			log.Println(err)
		}
		cancel()
	}()

	return ctx
}
