package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/1024casts/snake/handler"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	routers "github.com/1024casts/snake/router"

	"github.com/1024casts/snake/model"
	"github.com/1024casts/snake/pkg/redis"

	// http pprof
	_ "net/http/pprof"
	"os"

	"github.com/1024casts/snake/config"
	v "github.com/1024casts/snake/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/lexkong/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	cfg     = pflag.StringP("config", "c", "", "snake config file path.")
	version = pflag.BoolP("version", "v", false, "show version info.")
)

// @title snake docs api
// @version 1.0
// @description snake demo

// @contact.name 1024casts/snake
// @contact.url http://www.swagger.io/support
// @contact.email

// @host localhost:8080
// @BasePath /v1
func main() {
	pflag.Parse()
	if *version {
		v := v.Get()
		marshaled, err := json.MarshalIndent(&v, "", "  ")
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(marshaled))
		return
	}

	// init config
	if err := config.Init(*cfg); err != nil {
		panic(err)
	}

	// init db
	model.DB.Init()
	defer model.DB.Close()

	// init redis
	redis.Init()

	// Set gin mode.
	gin.SetMode(viper.GetString("run_mode"))

	// Create the Gin engine.
	router := gin.Default()

	// HealthCheck 健康检查路由
	router.GET("/health", handler.HealthCheck)
	// metrics router 可以在 prometheus 中进行监控
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API Routes.
	routers.Load(router)

	log.Infof("Start to listening the incoming requests on http address: %s", viper.GetString("addr"))
	srv := &http.Server{
		Addr:    viper.GetString("addr"),
		Handler: router,
	}
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf(err, "listen: %s", err.Error())
		}
		log.Info(srv.ListenAndServe().Error())
	}()

	gracefulStop(srv)

}

// gracefulStop 优雅退出
// 等待中断信号以超时 5 秒正常关闭服务器
// 官方说明：https://github.com/gin-gonic/gin#graceful-restart-or-stop
func gracefulStop(srv *http.Server) {
	quit := make(chan os.Signal)
	// kill 命令发送信号 syscall.SIGTERM
	// kill -2 命令发送信号 syscall.SIGINT
	// kill -9 命令发送信号 syscall.SIGKILL
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	// 5 秒后捕获 ctx.Done() 信号
	select {
	case <-ctx.Done():
		log.Info("timeout of 5 seconds.")
	default:
	}
	log.Info("Server exiting")
}
