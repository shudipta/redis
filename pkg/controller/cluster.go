package controller

import (
	"fmt"
	"strings"

	"github.com/appscode/go/log"
	"github.com/appscode/go/sets"
	"github.com/appscode/kutil"
	rd "github.com/go-redis/redis"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/redis/pkg/exec"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (c *Controller) waitUntilRedisClusterConfigured(redis *api.Redis) error {
	return wait.PollImmediate(kutil.RetryInterval, kutil.ReadinessTimeout, func() (bool, error) {
		podName := fmt.Sprintf("%s-shard%d-%d", redis.Name, 0, 0)

		pod, err := c.Client.CoreV1().Pods(redis.Namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		rdClient := rd.NewClient(&rd.Options{
			//Addr: fmt.Sprintf("%s:6379", pod.Status.PodIP),
			Addr: fmt.Sprintf("127.0.0.1:8000", pod.Status.PodIP),
		})

		slots, err := rdClient.ClusterSlots().Result()
		if err != nil {
			return false, nil
		}

		//total := 0
		masterIds := sets.NewString()
		checkReplcas := true
		for _, slot := range slots {
			//total += slot.End - slot.Start + 1
			masterIds.Insert(slot.Nodes[0].Id)
			checkReplcas = checkReplcas && (len(slot.Nodes)-1 == int(*redis.Spec.Cluster.Replicas))
		}

		// TODO: need to check whether total slots number is equal to 16384 or not
		//if total != 16384 || masterIds.Len() != int(*redis.Spec.Cluster.Master) || !checkReplcas {
		if masterIds.Len() != int(*redis.Spec.Cluster.Master) || !checkReplcas {
			return false, nil
		}

		return true, nil
	})
}

func (c *Controller) configureCluster(redis *api.Redis, statefulset *apps.StatefulSet) error {
	pods, err := c.Client.CoreV1().Pods(redis.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set{
			api.LabelDatabaseKind: api.ResourceKindRedis,
			api.LabelDatabaseName: redis.Name,
		}.String(),
	})
	if err != nil {
		return err
	}

	nodes := []string{}
	for _, po := range pods.Items {
		node := po.Status.PodIP
		nodes = append(nodes, node+":6379")
	}

	log.Infof("Checking whether a cluster is already configured or not using nodes (%v)", nodes)
	yes, err := c.isClusterCreated(&pods.Items[0], int(*statefulset.Spec.Replicas))
	if err != nil {
		return err
	} else if yes {
		log.Infof("A cluster is already configured using nodes (%v)", nodes)
		return nil
	}

	log.Infof("Creating cluster using nodes(%v)", nodes)
	err = c.createCluster(pods.Items, *redis.Spec.Cluster.Replicas, nodes...)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) isClusterCreated(pod *core.Pod, expectedNodes int) (bool, error) {
	e := exec.NewExecWithDefaultOptions(c.restConfig, c.Client)
	out, err := e.Run(pod, ClusterNodesCmd()...)
	if err != nil {
		return false, err
	}

	info := strings.Split(strings.TrimSpace(out), "\n")
	return len(info) == expectedNodes, nil
}

func (c *Controller) createCluster(pods []core.Pod, replicas int32, nodes ...string) error {
	input := "yes"
	e := exec.NewExecWithInputOptions(c.restConfig, c.Client, input)

	_, err := e.Run(&pods[0], ClusterCreateCmd(replicas, nodes...)...)
	if err != nil {
		return errors.Wrapf(err, "Failed to configure redis instances (%v) as a cluster", nodes)
	}

	return nil
}
