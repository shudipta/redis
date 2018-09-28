package redis_helper

import (
	"github.com/appscode/go/log"
	"fmt"
	"regexp"
	"strings"
	"strconv"
	"github.com/tamalsaha/go-oneliners"
	"github.com/appscode/go/sets"
)

func RunRedisHelper() {
	config := getConfigFromEnv()

	config.waitUntillRedisServersToBeReady()
	config.configureClusterState()

	select {}
}

func getIPByHostName(host string) string {
	cmd := NewCmdWithDefaultOptions()

	out, err := cmd.Run("ping", []string{"-c", "1", host}...)
	if err != nil {
		panic(err)
	}
	r, _ := regexp.Compile("([0-9]+).([0-9]+).([0-9]+).([0-9]+)")

	return r.FindString(out)
}

func ping(ip string) string {
	cmd := NewCmdWithDefaultOptions()
	pong, err := cmd.Run("redis-cli", []string{"-h", ip, "ping"}...)
	if err != nil {
		panic(err)
	}

	return strings.TrimSpace(pong)
}

func getClusterNodes(ip string) string {
	cmd := NewCmdWithDefaultOptions()
	out, err := cmd.Run("redis-cli", []string{"-c", "-h", ip, "cluster", "nodes"}...)
	if err != nil {
		panic(err)
	}

	return strings.TrimSpace(out)
}

func createCluster(addrs ...string) {
	cmd := NewCmdWithInputOptions("yes")
	args := []string{"create"}
	args = append(args, addrs...)
	out, err := cmd.Run("redis-trib", args...)
	if err != nil {
		panic(err)
	}

	fmt.Println(out)
}

func addNode(newAddr, existingAddr, masterId string) {
	cmd := NewCmdWithDefaultOptions()
	var (
		out string
		err error
	)

	if masterId == "" {
		out, err = cmd.Run("redis-trib",
			[]string{"add-node", newAddr, existingAddr}...)
	} else {
		out, err = cmd.Run("redis-trib",
			[]string{"add-node", "--slave", "--master-id", masterId, newAddr, existingAddr}...)
	}

	if err != nil {
		panic(err)
	}

	fmt.Println(out)
}

func deleteNode(existingAddr, nodeId string) {
	cmd := NewCmdWithDefaultOptions()
	out, err := cmd.Run("redis-trib",
		[]string{"del-node", existingAddr, nodeId}...)
	if err != nil {
		panic(err)
	}

	fmt.Println(out)
}

func reshard(from, to, slots string) {
	cmd := NewCmdWithDefaultOptions()
	out, err := cmd.Run("redis-trib",
		[]string{"reshard", "--from", from, "--to", to, "--slots", slots, "--yes"}...)
	if err != nil {
		panic(err)
	}

	fmt.Println(out)
}

func getNodeId(nodesConf string) (nodeId string) {
	nodes := strings.Split(nodesConf, "\n")
	for _, node := range nodes {
		if strings.Contains(node, "myself") {
			nodeId = strings.Split(strings.TrimSpace(node), " ")[0]
		}
	}

	return nodeId
}

func (c Config) waitUntillRedisServersToBeReady() {
	for i := 0; i < c.Cluster.MasterCnt; i++ {
		for j := 0; j <= c.Cluster.Replicas; j++ {
			ip := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local", c.BaseName, i, j, c.Namespace))
			for {
				if ping(ip) == "PONG" {
					log.Infof("%s is ready", ip)
					break
				}
			}
		}
	}
}

func processNodesConf(nodesConf string) map[string]*RedisNode {
	var (
		slotRange []string
		start, end int
		nds map[string]*RedisNode
	)

	nodes := strings.Split(nodesConf, "\n")

	for _, node := range nodes {
		node = strings.TrimSpace(node)
		parts := strings.Split(strings.TrimSpace(node), " ")

		if strings.Contains(parts[2], "master") {
			nd := RedisNode{
				Id: parts[0],
				Ip: strings.Split(parts[1], ":")[0],
				Port: 6379,
				Role: "master",
				Down: false,
			}
			if strings.Contains(parts[2], "fail") {
				nd.Down = true
			}
			nd.SlotsCnt = 0
			for j := 8; j < len(parts); j++ {
				slotRange = strings.Split(parts[j], "-")
				start, _ = strconv.Atoi(slotRange[0])
				end, _ = strconv.Atoi(slotRange[1])

				nd.SlotStart = append(nd.SlotStart, start)
				nd.SlotEnd = append(nd.SlotEnd, end)
				nd.SlotsCnt += (end - start)
			}
			nd.Slaves = []*RedisNode{}

			nds[nd.Id] = &nd
		}
	}

	for _, node := range nodes {
		node = strings.TrimSpace(node)
		parts := strings.Split(strings.TrimSpace(node), " ")

		if strings.Contains(parts[2], "slave") {
			nd := RedisNode{
				Id: parts[0],
				Ip: strings.Split(parts[1], ":")[0],
				Port: 6379,
				Role: "slave",
				Down: false,
			}
			if strings.Contains(parts[2], "fail") {
				nd.Down = true
			}
			nd.Master = nds[parts[3]]
			nds[parts[3]].Slaves = append(nds[parts[3]].Slaves, &nd)
		}
	}

	for masterId, master := range nds {
		fmt.Println(">>>>>>>> masterId =", masterId)
		fmt.Println("=============================================================")
		oneliners.PrettyJson(*master)
		for _, slave := range master.Slaves {
			oneliners.PrettyJson(*slave)
			fmt.Println(">>>>>>>> my masterId =", slave.Master.Id)
		}
	}

	return nds
}

func (c Config) configureClusterState() {
	ip := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local", c.BaseName, 0, 0, c.Namespace))
	nodesConf := getClusterNodes(ip)

	nds := processNodesConf(nodesConf)
	masterCnt := len(nds)

	// remove slave(s)
	for _, master := range nds {
		if c.Cluster.Replicas < len(master.Slaves) {
			var (
				runningSlavesIps sets.String
				found bool
			)

			// find slaves' ips of this master those need to be keep alive
			for i := 0; i < c.Cluster.MasterCnt; i++ {
				runningSlavesIps = sets.NewString()
				found = false

				for j := 0; j <= c.Cluster.Replicas; j++ {
					curIp := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local",
						c.BaseName, i, j, c.Namespace))
					if curIp == master.Ip {
						found = true
						continue
					}
					runningSlavesIps.Insert(curIp)
				}

				if found {
					break
				}
			}

			// delete the slaves those aren't in the set 'runningSlavesIps'
			for _, slave := range master.Slaves {
				if !runningSlavesIps.Has(slave.Ip) {
					deleteNode(master.Ip+":6379", slave.Id)
				}
			}
		}
	}

	// remove failed master(s)
	//if masterCnt > c.Cluster.MasterCnt {
	//	var (
	//		fallenMastersId, runningMastersId []string
	//		slotsPerMaster, slotsRequired, allocatedSlotsCnt int
	//	)
	//	for masterId, master := range nds {
	//		if master.Down {
	//			fallenMastersId = append(fallenMastersId, masterId)
	//		} else {
	//			runningMastersId = append(runningMastersId, masterId)
	//		}
	//	}
	//
	//	slotsPerMaster = 16384 / c.Cluster.MasterCnt
	//	for i, runningMasterId := range runningMastersId {
	//		slotsRequired = slotsPerMaster
	//		if i == len(runningMastersId) - 1 {
	//			// this change is only for last master that is in running master(s) list
	//			slotsRequired = 16384 - (slotsPerMaster * i)
	//		}
	//
	//		allocatedSlotsCnt = nds[runningMasterId].SlotsCnt
	//		for _, fallenMasterId := range
	//	}
	//}

	if masterCnt > 1 {
		// add new master(s)
		if masterCnt < c.Cluster.MasterCnt {
			for i := masterCnt; i < c.Cluster.MasterCnt; i++ {
				newMasterIp := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local", c.BaseName, i, 0, c.Namespace))
				addNode(newMasterIp+":6379", ip+"6379", "")
			}
			nds = processNodesConf(getClusterNodes(ip))
		}

		// add slots to empty master(s)
		var (
			nonEmptyMastersId, emptyMastersId []string
			slotsPerMaster, slotsRequired, allocatedSlotsCnt int
		)

		for masterId, master := range nds {
			if master.SlotsCnt == 0 {
				emptyMastersId = append(emptyMastersId, masterId)
			} else {
				nonEmptyMastersId = append(nonEmptyMastersId, masterId)
			}
		}

		slotsPerMaster = 16384 / c.Cluster.MasterCnt
		for i, emptyMasterId := range emptyMastersId {
			slotsRequired = slotsPerMaster
			if i == len(emptyMastersId) - 1 {
				// this change is only for last master that is being added
				slotsRequired = 16384 - (slotsPerMaster * i)
			}

			allocatedSlotsCnt = nds[emptyMasterId].SlotsCnt
			for _, masterId := range nonEmptyMastersId {
				// compare with slotsRequired
				if allocatedSlotsCnt < slotsRequired {
					// But compare with slotsPerMaster. Existing masters always need slots equal to
					// slotsPerMaster not slotsRequired since slotsRequired may change for last master
					// that is being added.
					if nds[masterId].SlotsCnt > slotsPerMaster {
						slots := nds[masterId].SlotsCnt - slotsPerMaster
						if slots > slotsRequired - allocatedSlotsCnt {
							slots = slotsRequired - allocatedSlotsCnt
						}

						reshard(masterId, emptyMasterId, strconv.Itoa(slots))
						allocatedSlotsCnt += slots
						nds[masterId].SlotsCnt -= slots
					}
				} else {
					break
				}
			}
		}
		nds = processNodesConf(getClusterNodes(ip))

		// add new slave(s)
		for i := 0; i < c.Cluster.MasterCnt; i++ {
			curMasterId := ""
			curMasterIp := ""
FindMaster:
			for j := 0; j <= c.Cluster.Replicas; j++ {
				curIp := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local",
					c.BaseName, i, j, c.Namespace))
				for masterId, _ := range nds {
					if nds[masterId].Ip == curIp {
						curMasterIp = curIp
						curMasterId = nds[masterId].Id
						break FindMaster
					}
				}
			}

			for j := 0; j <= c.Cluster.Replicas; j++ {
				curIp := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local",
					c.BaseName, i, j, c.Namespace))
				exists := false
FindSlave:
				for _, master := range nds {
					for _, slave := range master.Slaves {
						if slave.Ip == curIp {
							exists = true
							break FindSlave
						}
					}
				}
				if !exists {
					addNode(curIp+":6379", curMasterIp+":6379", curMasterId)
				}
			}
		}
	} else {
		c.createNewCluster()
	}

}

func (c Config) createNewCluster() {
	masterAddrs := make([]string, c.Cluster.MasterCnt)
	masterNodeIds := make([]string, c.Cluster.MasterCnt)

	for i := 0; i < c.Cluster.MasterCnt; i++ {
		ip := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local",
			c.BaseName, i, 0, c.Namespace))
		masterAddrs[i] = ip + ":6379"
		masterNodeIds[i] = getNodeId(getClusterNodes(ip))
	}
	createCluster(masterAddrs...)

	for i := 0; i < c.Cluster.MasterCnt; i++ {
		for j := 1; j <= c.Cluster.Replicas; j++ {
			ip := getIPByHostName(fmt.Sprintf("%s-shard%d-%d.%s.svc.cluster.local", c.BaseName, i, j, c.Namespace))
			addNode(ip+":6379", masterAddrs[i], masterNodeIds[i])
		}
	}
}