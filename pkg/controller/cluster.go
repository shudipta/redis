package controller

import (
	"strings"

	"github.com/appscode/go/log"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/redis/pkg/exec"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

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
