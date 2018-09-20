package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/appscode/go/types"
	core_util "github.com/appscode/kutil/core/v1"
	"github.com/appscode/kutil/tools/portforward"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func WaitUntilFailedNodeAvailable(kubeClient kubernetes.Interface, statefulSet *apps.StatefulSet) error {
	return core_util.WaitUntilPodRunningBySelector(
		kubeClient,
		statefulSet.Namespace,
		statefulSet.Spec.Selector,
		int(types.Int32(statefulSet.Spec.Replicas)),
	)
}

func ForwardPort(
	kubeClient kubernetes.Interface,
	restConfig *rest.Config,
	meta metav1.ObjectMeta, clientPodName string, port int) (*portforward.Tunnel, error) {
	tunnel := portforward.NewTunnel(
		kubeClient.CoreV1().RESTClient(),
		restConfig,
		meta.Namespace,
		clientPodName,
		port,
	)

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func GetPods(
	kubeClient kubernetes.Interface,
	meta metav1.ObjectMeta, selector labels.Set) (*core.PodList, error) {
	return kubeClient.CoreV1().Pods(meta.Namespace).List(metav1.ListOptions{
		LabelSelector: selector.String(),
	})
}

func FowardedPodsIPWithTunnel(
	kubeClient kubernetes.Interface,
	restConfig *rest.Config,
	meta metav1.ObjectMeta, podSelector labels.Set) ([]string, []*portforward.Tunnel, error) {

	pods, err := GetPods(kubeClient, meta, podSelector)
	if err != nil {
		return nil, nil, err
	}

	rdAddresses := make([]string, len(pods.Items))
	tunnels := make([]*portforward.Tunnel, len(pods.Items))
	for i, po := range pods.Items {
		parts := strings.Split(po.Name, "-")
		idx, _ := strconv.Atoi(parts[len(parts)-1])
		tunnels[idx], err = ForwardPort(kubeClient, restConfig, meta, po.Name, 6379)
		address := fmt.Sprintf("%d", tunnels[i].Local)
		rdAddresses[idx] = address
	}

	return rdAddresses, tunnels, nil
}
