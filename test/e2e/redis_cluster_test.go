package e2e_test

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/appscode/kutil/tools/portforward"
	rd "github.com/go-redis/redis"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/redis/test/e2e/framework"
	"github.com/kubedb/redis/test/e2e/matcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var createAndWaitForRunning = func() {
	By("Create RedisVersion: " + cl.redisVersion.Name)
	err := cl.f.CreateRedisVersion(cl.redisVersion)
	Expect(err).NotTo(HaveOccurred())

	By("Create Redis: " + cl.redis.Name)
	err = cl.f.CreateRedis(cl.redis)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Running redis")
	cl.f.EventuallyRedisRunning(cl.redis.ObjectMeta).Should(BeTrue())
}

var deleteTestResource = func() {
	By("Delete redis")
	err := cl.f.DeleteRedis(cl.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for redis to be paused")
	cl.f.EventuallyDormantDatabaseStatus(cl.redis.ObjectMeta).Should(matcher.HavePaused())

	By("WipeOut redis")
	_, err = cl.f.PatchDormantDatabase(cl.redis.ObjectMeta, func(in *api.DormantDatabase) *api.DormantDatabase {
		in.Spec.WipeOut = true
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Delete Dormant Database")
	err = cl.f.DeleteDormantDatabase(cl.redis.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for redis resources to be wipedOut")
	cl.f.EventuallyWipedOut(cl.redis.ObjectMeta).Should(Succeed())

	By("Delete RedisVersion")
	err = cl.f.DeleteRedisVersion(cl.redisVersion.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
}

type clusterScenario struct {
	ports   []string
	ips     []string
	nodeIds []string
	clients map[string]*rd.Client
	role    map[string]string

	slotStart map[string]int
	slotEnd   map[string]int
}

func (s *clusterScenario) masters() []*rd.Client {
	result := make([]*rd.Client, 3)
	for pos, port := range s.ports[:3] {
		result[pos] = s.clients[port]
	}
	return result
}

func (s *clusterScenario) slaves() []*rd.Client {
	result := make([]*rd.Client, 3)
	for pos, port := range s.ports[3:] {
		result[pos] = s.clients[port]
	}
	return result
}

func (s *clusterScenario) addrs() []string {
	addrs := make([]string, len(s.ports))
	for i, port := range s.ports {
		addrs[i] = net.JoinHostPort("127.0.0.1", port)
	}

	return addrs
}

func (s *clusterScenario) nodeAddr(slotStart, slotEnd int, role string) string {
	for i, port := range s.ports {
		if s.slotStart[port] == slotStart && s.slotEnd[port] == slotEnd && s.role[port] == role {
			return s.ips[i] + ":6379"
		}
	}

	return ""
}

func (s *clusterScenario) clusterClient(opt *rd.ClusterOptions) *rd.ClusterClient {
	var errBadState = fmt.Errorf("cluster state is not consistent")
	opt.Addrs = s.addrs()
	client := rd.NewClusterClient(opt)

	Eventually(func() error {
		if opt.ClusterSlots != nil {
			fmt.Println("clusterslots exists")
			return nil
		}

		err := client.ForEachMaster(func(master *rd.Client) error {
			_, errp := master.Ping().Result()
			if errp != nil {
				return fmt.Errorf("%v: master(%s) ping error <-> %v", errBadState, master.String(), errp)
			}
			s := master.Info("replication").Val()
			if !strings.Contains(s, "role:master") {
				return fmt.Errorf("%v: %s is not master in role", errBadState, master.String())
			}
			return nil
		})
		if err != nil {
			return err
		}

		err = client.ForEachSlave(func(slave *rd.Client) error {
			_, errp := slave.Ping().Result()
			if errp != nil {
				return fmt.Errorf("%v: slave(%s) ping error <-> %v", errBadState, slave.String(), errp)
			}
			s := slave.Info("replication").Val()
			if !strings.Contains(s, "role:slave") {
				return fmt.Errorf("%v: %s is not slave in role", errBadState, slave.String())
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	}, 5*time.Minute, 5*time.Second).Should(BeNil())

	return client
}

func assertSlotsEqual(slots, wanted []rd.ClusterSlot) error {
	for _, s2 := range wanted {
		ok := false
		for _, s1 := range slots {
			if slotEqual(s1, s2) {
				ok = true
				break
			}
		}
		if ok {
			continue
		}
		return fmt.Errorf("%v not found in %v", s2, slots)
	}
	return nil
}

func slotEqual(s1, s2 rd.ClusterSlot) bool {
	if s1.Start != s2.Start {
		return false
	}
	if s1.End != s2.End {
		return false
	}
	if len(s1.Nodes) != len(s2.Nodes) {
		return false
	}
	for i, n1 := range s1.Nodes {
		if n1.Addr != s2.Nodes[i].Addr {
			return false
		}
	}
	return true
}

var _ = Describe("Redis Cluster", func() {
	var (
		err         error
		skipMessage string
		//redisInstanceNumber int
		//selector            labels.Set
		failover bool
		opt      *rd.ClusterOptions
		client   *rd.ClusterClient
		cluster  *clusterScenario
		addrs    []string
		tunnels  []*portforward.Tunnel
		nodes    []framework.Node
	)

	var clusterSlots = func() ([]rd.ClusterSlot, error) {
		slots := []rd.ClusterSlot{
			// First node with 1 master and 1 slave.
			{
				Start: 0,
				End:   5460,
				Nodes: []rd.ClusterNode{{
					Addr: ":" + addrs[0], // master
				}, {
					Addr: ":" + addrs[3], // 1st slave
				}},
			},
			// Second node with 1 master and 1 slave.
			{
				Start: 5461,
				End:   10922,
				Nodes: []rd.ClusterNode{{
					Addr: ":" + addrs[1], // master
				}, {
					Addr: ":" + addrs[4], // 1st slave
				}},
			},
			// Third node with 1 master and 1 slave.
			{
				Start: 10923,
				End:   16383,
				Nodes: []rd.ClusterNode{{
					Addr: ":" + addrs[2], // master
				}, {
					Addr: ":" + addrs[5], // 1st slave
				}},
			},
		}
		return slots, nil
	}

	BeforeEach(func() {
		//redisInstanceNumber = int(redis.Spec.Cluster.Master * (redis.Spec.Cluster.ReplicationFactor + 1))
		//selector = labels.Set{
		//	api.LabelDatabaseKind: api.ResourceKindRedis,
		//	api.LabelDatabaseName: redis.Name,
		//}
		skipMessage = ""

		addrs, tunnels, err = cl.f.GetPodsIPWithTunnel(cl.redis.ObjectMeta, cl.selector)
		Expect(err).NotTo(HaveOccurred())

		nodes = cl.f.Sync(addrs, cl.redis.ObjectMeta, cl.selector)

		By(fmt.Sprintf("creating client with ports %v", addrs))
		cluster = &clusterScenario{
			ports:     []string{nodes[0].Port, nodes[1].Port, nodes[2].Port, nodes[3].Port, nodes[4].Port, nodes[5].Port},
			nodeIds:   make([]string, 6),
			ips:       make([]string, 6),
			clients:   make(map[string]*rd.Client, 6),
			role:      make(map[string]string, 6),
			slotStart: make(map[string]int, 6),
			slotEnd:   make(map[string]int, 6),
		}
		for i, port := range cluster.ports {
			cluster.ips[i] = nodes[i].Ip
			cluster.nodeIds[i] = nodes[i].Id
			cluster.clients[port] = nodes[i].Client
			cluster.role[port] = nodes[i].Role
			cluster.slotStart[port] = nodes[i].SlotStart
			cluster.slotEnd[port] = nodes[i].SlotEnd
		}
	})

	AfterEach(func() {
		for i := range tunnels {
			defer tunnels[i].Close()
			By(fmt.Sprintf("closing tunnels[%d]", i))
		}
	})

	var assertSimple = func() {
		It("should GET/SET/DEL", func() {
			res := client.Get("A").Val()
			if failover {
				Expect(res).To(Equal("VALUE"))
			} else {
				Expect(res).To(Equal(""))
			}

			err = client.Set("A", "VALUE", 0).Err()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				return client.Get("A").Val()
			}, 30*time.Second).Should(Equal("VALUE"))

			cnt, err := client.Del("A").Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(cnt).To(Equal(int64(1)))
		})
	}

	var assertPubSub = func() {
		It("supports PubSub", func() {
			pubsub := client.Subscribe("mychannel")
			defer pubsub.Close()

			Eventually(func() error {
				_, err := client.Publish("mychannel", "hello").Result()
				if err != nil {
					return err
				}

				msg, err := pubsub.ReceiveTimeout(time.Second)
				if err != nil {
					return err
				}

				_, ok := msg.(*rd.Message)
				if !ok {
					return fmt.Errorf("got %T, wanted *redis.Message", msg)
				}

				return nil
			}, 30*time.Second).ShouldNot(HaveOccurred())
		})
	}

	Context("Cluster Commands", func() {
		BeforeEach(func() {
			opt = &rd.ClusterOptions{
				ClusterSlots:  clusterSlots,
				RouteRandomly: true,
			}
			client = cluster.clusterClient(opt)

			err := client.ForEachMaster(func(master *rd.Client) error {
				return master.FlushDB().Err()
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = client.ForEachMaster(func(master *rd.Client) error {
				return master.FlushDB().Err()
			})
			Expect(client.Close()).NotTo(HaveOccurred())
		})

		It("should CLUSTER INFO", func() {
			res, err := client.ClusterInfo().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("cluster_known_nodes:6"))
		})

		It("calls fn for every master node", func() {
			for i := 0; i < 10; i++ {
				Expect(client.Set(strconv.Itoa(i), "", 0).Err()).NotTo(HaveOccurred())
			}

			err := client.ForEachMaster(func(master *rd.Client) error {
				return master.FlushDB().Err()
			})
			Expect(err).NotTo(HaveOccurred())

			size, err := client.DBSize().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(0)))
		})

		It("should CLUSTER SLOTS", func() {
			res, err := client.ClusterSlots().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(HaveLen(3))

			wanted := []rd.ClusterSlot{
				{
					Start: 0,
					End:   5460,
					Nodes: []rd.ClusterNode{{
						Id:   "",
						Addr: cluster.nodeAddr(0, 5460, "master"),
					}, {
						Id:   "",
						Addr: cluster.nodeAddr(0, 5460, "slave"),
					}},
				}, {
					Start: 5461,
					End:   10922,
					Nodes: []rd.ClusterNode{{
						Id:   "",
						Addr: cluster.nodeAddr(5461, 10922, "master"),
					}, {
						Id:   "",
						Addr: cluster.nodeAddr(5461, 10922, "slave"),
					}},
				}, {
					Start: 10923,
					End:   16383,
					Nodes: []rd.ClusterNode{{
						Id:   "",
						Addr: cluster.nodeAddr(10923, 16383, "master"),
					}, {
						Id:   "",
						Addr: cluster.nodeAddr(10923, 16383, "slave"),
					}},
				},
			}

			Expect(assertSlotsEqual(res, wanted)).NotTo(HaveOccurred())
		})

		It("should CLUSTER NODES", func() {
			res, err := client.ClusterNodes().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(res)).To(BeNumerically(">", 400))
		})

		It("should CLUSTER COUNT-FAILURE-REPORTS", func() {
			n, err := client.ClusterCountFailureReports(cluster.nodeIds[0]).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(int64(0)))
		})

		It("should CLUSTER COUNTKEYSINSLOT", func() {
			n, err := client.ClusterCountKeysInSlot(10).Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(int64(0)))
		})

		It("should CLUSTER SAVECONFIG", func() {
			res, err := client.ClusterSaveConfig().Result()
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal("OK"))
		})

		It("should CLUSTER SLAVES", func() {
			for i, node := range nodes {
				if node.Role == "master" {
					nodesList, err := client.ClusterSlaves(cluster.nodeIds[i]).Result()
					Expect(err).NotTo(HaveOccurred())
					Expect(nodesList).Should(ContainElement(ContainSubstring("slave")))
					Expect(nodesList).Should(HaveLen(1))
					break
				}
			}

		})

		It("should RANDOMKEY", func() {
			const nkeys = 100

			for i := 0; i < nkeys; i++ {
				err := client.Set(fmt.Sprintf("key%d", i), "value", 0).Err()
				Expect(err).NotTo(HaveOccurred())
			}

			var keys []string
			addKey := func(key string) {
				for _, k := range keys {
					if k == key {
						return
					}
				}
				keys = append(keys, key)
			}

			for i := 0; i < nkeys*10; i++ {
				key := client.RandomKey().Val()
				addKey(key)
			}

			Expect(len(keys)).To(BeNumerically("~", nkeys, nkeys/10))
		})

		assertSimple()
		assertPubSub()
	})

	Context("Cluster failover", func() {
		BeforeEach(func() {
			failover = true

			opt = &rd.ClusterOptions{
				ClusterSlots:  clusterSlots,
				RouteRandomly: true,
			}
			client = cluster.clusterClient(opt)

			err := client.ForEachMaster(func(master *rd.Client) error {
				return master.FlushDB().Err()
			})
			Expect(err).NotTo(HaveOccurred())

			err = client.ForEachSlave(func(slave *rd.Client) error {
				defer GinkgoRecover()

				Eventually(func() int64 {
					return slave.DBSize().Val()
				}, "30s").Should(Equal(int64(0)))

				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			err = client.Set("A", "VALUE", 0).Err()
			Expect(err).NotTo(HaveOccurred())

			err = client.ReloadState()
			Eventually(func() bool {
				err = client.ReloadState()
				if err != nil {
					return false
				}
				return true
			}, "30s").Should(BeTrue())

			client.ForEachSlave(func(slave *rd.Client) error {
				err = slave.ClusterFailover().Err()
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					err := client.ReloadState()
					if err != nil {
						return false
					}
					return true
				}, "30s").Should(BeTrue())
				return nil
			})
		})

		AfterEach(func() {
			failover = false
			Expect(client.Close()).NotTo(HaveOccurred())
		})

		assertSimple()
	})
})
