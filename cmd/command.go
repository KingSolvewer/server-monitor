package cmd

import (
	"fmt"
	"github.com/solvewer/server-monitor/configuration"
	"github.com/solvewer/server-monitor/monitor"
	"github.com/solvewer/server-monitor/util"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var (
	file    string
	rootCmd = &cobra.Command{
		Use: "app",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting application...")
		},
	}
)

func init() {
	exePath, err := os.Executable()
	if err != nil {
		configuration.GetLogger(configuration.GlobalLogName).Error("程序执行目录错误", zap.Error(err))
		panic(err)
	}

	dir := filepath.Dir(exePath)
	configuration.GetLogger(configuration.GlobalLogName).Info("程序执行", zap.String("程序执行目录", dir))

	monitorCmd := startMonitor()
	mysqlMonitorCmd := mysqlMonitor()

	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(mysqlMonitorCmd)
}

func Exec() {
	var (
		cmd *cobra.Command
		err error
	)
	if cmd, err = rootCmd.ExecuteC(); err != nil {
		if cmd != nil {
			configuration.GetLogger(configuration.GlobalLogName).Error("程序执行发生错误：", zap.String("监控项：", cmd.Name()), zap.Error(err))
		} else {
			configuration.GetLogger(configuration.GlobalLogName).Error("程序执行发生错误：", zap.Error(err))
		}
	} else {
		configuration.GetLogger(configuration.GlobalLogName).Info("程序执行成功", zap.String("监控项", cmd.Name()))
	}

	signalChan := make(chan os.Signal, 1)
	ch := make(chan struct{})
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	if cmd != nil {
		go func() {
			for range signalChan {
				logger := configuration.GetLumberjackLogger(cmd.Name())
				if err = logger.Rotate(); err == nil {
					configuration.GetLogger(configuration.GlobalLogName).Info("日志备份成功", zap.String("监控项", cmd.Name()))
				} else {
					configuration.GetLogger(configuration.GlobalLogName).Info("日志备份失败", zap.String("监控项", cmd.Name()), zap.Error(err))
				}
				ch <- struct{}{}
			}
		}()
		<-ch
	}
}

func startMonitor() *cobra.Command {
	monitorCmd := &cobra.Command{
		Use: configuration.WebLogName,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// web服务器监控日志
			configuration.InitLogger(configuration.WebLogName, util.LogPath(configuration.WebLogName))
		},
		Run: func(cmd *cobra.Command, args []string) {
			configuration.GetLogger(configuration.WebLogName).Info("开始监控服务器", zap.String("配置文件", file))

			// 开启监控
			monitor.Start()
		},
	}

	validateArgs(monitorCmd)

	return monitorCmd
}

func mysqlMonitor() *cobra.Command {
	mysqlCmd := &cobra.Command{
		Use: configuration.MysqlLogName,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// web服务器监控日志
			configuration.InitLogger(configuration.MysqlLogName, util.LogPath(configuration.MysqlLogName))
		},
		Run: func(cmd *cobra.Command, args []string) {
			configuration.GetLogger(configuration.MysqlLogName).Info("开始监控Mysql数据库", zap.String("配置文件", file))

			monitor.StartMysql()
		},
	}

	validateArgs(mysqlCmd)

	return mysqlCmd
}

func validateArgs(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&file, "file", "f", ".env", "The file to run the server-monitor")
	_ = cmd.MarkFlagRequired("file")
}
