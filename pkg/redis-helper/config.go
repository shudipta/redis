package redis_helper

import (
	"strconv"
	"os"
)

type Config struct {
	BaseName string
	Namespace string

	Cluster RedisCluster
}

type RedisCluster struct {
	MasterCnt int
	Replicas int
}

type RedisNode struct {
	SlotStart []int
	SlotEnd []int
	SlotsCnt int

	Id string
	Ip string
	Port int
	Role string
	Down bool


	Master *RedisNode
	Slaves []*RedisNode
}

func getConfigFromEnv() Config {
	baseName := os.Getenv("BASE_NAME")
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	masterCnt, _ := strconv.Atoi(os.Getenv("MASTER_COUNT"))
	replicas, _ := strconv.Atoi(os.Getenv("REPLICAS"))

	return Config{
		BaseName: baseName,
		Namespace: namespace,
		Cluster: RedisCluster{
			MasterCnt: masterCnt,
			Replicas: replicas,
		},
	}
}