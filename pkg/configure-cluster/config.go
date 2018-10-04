package configure_cluster

import (
	"os"
	"strconv"
	"io/ioutil"
	"strings"
)

type Config struct {
	BaseName  string
	Namespace string

	Cluster RedisCluster
}

type RedisCluster struct {
	MasterCnt int
	Replicas  int
}

type RedisNode struct {
	SlotStart []int
	SlotEnd   []int
	SlotsCnt  int

	Id   string
	Ip   string
	Port int
	Role string
	Down bool

	Master *RedisNode
	Slaves []*RedisNode
}

func getConfigFromEnv() Config {
	baseName := os.Getenv("BASE_NAME")
	namespace := ""
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		namespace = strings.TrimSpace(string(data))
		if len(namespace) > 0 {
			namespace = "default"
		}
	}
	//namespace := os.Getenv("NAMESPACE")
	//if namespace == "" {
	//	namespace = "default"
	//}
	masterCnt, _ := strconv.Atoi(os.Getenv("MASTER_COUNT"))
	replicas, _ := strconv.Atoi(os.Getenv("REPLICAS"))

	return Config{
		BaseName:  baseName,
		Namespace: namespace,
		Cluster: RedisCluster{
			MasterCnt: masterCnt,
			Replicas:  replicas,
		},
	}
}
