package cmd

import (
	"errors"
	"fmt"
	"github.com/solvewer/server-monitor/monitor"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
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
	monitorCmd := startMonitor()
	mysqlMonitorCmd := mysqlMonitor()

	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(mysqlMonitorCmd)
}

func Exec() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func startMonitor() *cobra.Command {
	monitorCmd := &cobra.Command{
		Use: "start-monitor",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting server-monitor")
			fmt.Println(file)
			exePath, err := os.Executable()
			if err != nil {
				panic(err)
			}

			dir := filepath.Dir(exePath)
			fmt.Println("程序自身所在目录:", dir)

			// 开启监控
			monitor.Start()
		},
	}
	//monitorCmd.Flags().StringVarP(&file, "file", "f", ".env", "The file to run the server-monitor")
	//err := monitorCmd.MarkFlagRequired("file")
	//if err != nil {
	//	err = errors.New("缺少必要的文件名参数:" + err.Error())
	//	panic(err)
	//}

	validateArgs(monitorCmd)

	return monitorCmd
}

func mysqlMonitor() *cobra.Command {
	mysqlCmd := &cobra.Command{
		Use: "mysql-monitor",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting mysql monitor...")

			exePath, err := os.Executable()
			if err != nil {
				panic(err)
			}

			dir := filepath.Dir(exePath)
			fmt.Println("程序自身所在目录:", dir)

			monitor.StartMysql()
		},
	}

	validateArgs(mysqlCmd)

	return mysqlCmd
}

func validateArgs(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&file, "file", "f", ".env", "The file to run the server-monitor")
	err := cmd.MarkFlagRequired("file")
	if err != nil {
		err = errors.New("缺少必要的文件名参数:" + err.Error())
		panic(err)
	}
}
