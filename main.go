package main

import (
	"log"

	logs "github.com/appscode/log/golog"
	"github.com/k8sdb/redis/pkg/cmds"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	if err := cmds.NewRootCmd(Version).Execute(); err != nil {
		log.Fatal(err)
	}
}
