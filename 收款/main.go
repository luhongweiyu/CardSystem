package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "中文 YAML 配置文件路径")
	checkOnly := flag.Bool("check-config", false, "校验配置及内置密钥后退出，不访问支付宝")
	flag.Parse()
	if err := run(*configPath, *checkOnly); err != nil {
		slog.Error("服务未能正常运行", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, checkOnly bool) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	credentials, err := loadCredentials()
	if err != nil {
		return err
	}
	client, err := newAlipayClient(credentials, cfg.Timeout)
	if err != nil {
		return err
	}
	if checkOnly {
		slog.Info("配置及内置 RSA 密钥校验通过；未请求支付宝")
		return nil
	}
	service := newService(cfg, client, time.Now)
	server := &http.Server{
		Addr: cfg.Listen, Handler: newRouter(cfg, service),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: cfg.Timeout + 15*time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- server.ListenAndServe() }()
	slog.Info("收款核对服务启动", "listen", cfg.Listen, "path", cfg.Path)
	select {
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.Timeout+time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
	return nil
}
