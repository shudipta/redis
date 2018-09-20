package framework

import (
	"strconv"
	"strings"
	"time"

	"github.com/appscode/kutil/tools/portforward"
	rd "github.com/go-redis/redis"
	"github.com/kubedb/redis/pkg/controller"
	"github.com/kubedb/redis/pkg/exec"
	"github.com/kubedb/redis/test/e2e/util"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (f *Framework) RedisClusterOptions() *rd.ClusterOptions {
	return &rd.ClusterOptions{
		DialTimeout:        10 * time.Second,
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		PoolSize:           10,
		PoolTimeout:        30 * time.Second,
		IdleTimeout:        500 * time.Millisecond,
		IdleCheckFrequency: 500 * time.Millisecond,
	}
}

type Node struct {
	Id        string
	Ip        string
	Port      string
	Role      string
	Master    string
	SlotStart int
	SlotEnd   int

	Client *rd.Client
}

func (f *Framework) GetPodsIPWithTunnel(
	meta metav1.ObjectMeta,
	podSelector labels.Set) ([]string, []*portforward.Tunnel, error) {

	return util.FowardedPodsIPWithTunnel(f.kubeClient, f.restConfig, meta, podSelector)
}

func (f *Framework) Sync(addrs []string, meta metav1.ObjectMeta, podSelector labels.Set) []Node {
	var nodes = make([]Node, 6)

	pods, err := util.GetPods(f.kubeClient, meta, podSelector)
	Expect(err).NotTo(HaveOccurred())

	e := exec.NewExecWithDefaultOptions(f.restConfig, f.kubeClient)
	for _, po := range pods.Items {
		parts := strings.Split(po.Name, "-")
		idx, _ := strconv.Atoi(parts[len(parts)-1])

		out, err := e.Run(&po, controller.ClusterNodesCmd()...)
		Expect(err).NotTo(HaveOccurred())

		out = strings.TrimSpace(out)
		for _, info := range strings.Split(out, "\n") {
			info = strings.TrimSpace(info)
			if strings.Contains(info, "myself") {
				parts := strings.Split(info, " ")

				node := Node{
					Id:     parts[0],
					Ip:     strings.Split(parts[1], ":")[0],
					Port:   addrs[idx],
					Master: parts[0],

					Client: rd.NewClient(&rd.Options{
						Addr: ":" + addrs[idx],
					}),
				}

				if strings.Contains(parts[2], "slave") {
					node.Role = "slave"
					node.Master = parts[3]
				} else {
					slotBounds := strings.Split(parts[len(parts)-1], "-")
					node.Role = "master"

					// slotBounds contains only int. So errors are ignored
					node.SlotStart, _ = strconv.Atoi(slotBounds[0])
					node.SlotEnd, _ = strconv.Atoi(slotBounds[1])
				}
				nodes[idx] = node
				break
			}
		}

		for i := range nodes {
			if nodes[i].Role == "slave" {
				for j := range nodes {
					if nodes[j].Id == nodes[i].Master {
						nodes[i].SlotStart = nodes[j].SlotStart
						nodes[i].SlotEnd = nodes[j].SlotEnd
					}
				}
			}
		}
	}

	return nodes
}

func (f *Framework) CheckStatefulSetPodStatus(meta metav1.ObjectMeta) {
	sts, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	err = util.WaitUntilFailedNodeAvailable(f.kubeClient, sts)
	Expect(err).NotTo(HaveOccurred())
}
