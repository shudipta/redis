package controller

import "strconv"

func ClusterInfoCmd() []string {
	return []string{"redis-cli", "-c", "cluster", "info"}
}

func ClusterNodesCmd() []string {
	return []string{"redis-cli", "-c", "cluster", "nodes"}
}

func ClusterCreateCmd(replicas int32, nodes ...string) []string {
	command := []string{"redis-trib", "create"}
	if replicas > 0 {
		replica := strconv.Itoa(int(replicas))
		command = append(command, "--replicas", replica)
	}
	return append(command, nodes...)
}

func ReplicationInfoCmd() []string {
	return []string{"redis-cli", "-c", "info", "replication"}
}

func DebugSegfaultCmd() []string {
	return []string{"redis-cli", "-c", "debug", "segfault"}
}
