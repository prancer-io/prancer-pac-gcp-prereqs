package prancer

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/container/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetGKEClientset(cluster *container.Cluster, ts oauth2.TokenSource) (kubernetes.Interface, error) {
	capem, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cluster CA cert: %s", err)
	}

	config := &rest.Config{
		Host: cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: capem,
		},
	}
	config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return &oauth2.Transport{
			Source: ts,
			Base:   rt,
		}
	})

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise clientset from config: %s", err)
	}

	return clientset, nil
}

func CreateNamespace(namespace string, clientset kubernetes.Interface, ctx context.Context) (*v1.Namespace, error) {
	var (
		nameSpace *v1.Namespace
		err       error
	)

	nameSpace, err = clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"name": namespace,
			},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		fmt.Println("Failed to create namespace: ", err)
		return &v1.Namespace{}, err
	}

	return nameSpace, nil
}

func CreateConfigMap(configmap, namespace string, clientset kubernetes.Interface, ctx context.Context, createWorkloadConfig CreateWorkloadConfig) (*v1.ConfigMap, error) {
	var (
		err       error
		configMap *v1.ConfigMap
	)

	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmap,
			Namespace: namespace,
		},
		Data: map[string]string{
			"PAC_CONFIG_ID":          createWorkloadConfig.ConfigId,
			"PAC_CONFIG_TOKEN":       createWorkloadConfig.Token,
			"PAC_CONFIG_ID_TOKEN":    createWorkloadConfig.TokenId,
			"PAC_CONFIG_DOMAIN":      createWorkloadConfig.Domain,
			"PAC_CONFIG_CUSTOMER_ID": createWorkloadConfig.CusId,
		},
	}

	configMap, err = clientset.CoreV1().ConfigMaps(namespace).Create(ctx, &cm, metav1.CreateOptions{})
	if err != nil {
		fmt.Println("Failed to create configmap: ", err)
		return &v1.ConfigMap{}, fmt.Errorf(fmt.Sprintf("Failed to create cofigmap: %s", err))
	}

	return configMap, nil
}

func BuildDeployment(configMap, deploymentName string, createWorkloadConfig CreateWorkloadConfig) *appsv1.Deployment {
	allowPrivileged := true

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "pac-cli",
					"version": "v1",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "pac-cli",
						"version": "v1",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "prancer-pac-cli",
							Image:           createWorkloadConfig.ScannerImage,
							ImagePullPolicy: "Always",
							EnvFrom: []v1.EnvFromSource{
								{
									ConfigMapRef: &v1.ConfigMapEnvSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: configMap,
										},
									},
								},
							},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: &allowPrivileged,
								Capabilities: &v1.Capabilities{
									Add: []v1.Capability{"NET_RAW"},
								},
							},
						},
					},
					DNSPolicy:     "ClusterFirst",
					RestartPolicy: "Always",
				},
			},
		},
	}
}

func DeleteConfigMap(namespace, configmap string, clientset kubernetes.Interface, ctx context.Context) error {
	return clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configmap, metav1.DeleteOptions{})
}

// Waiter Code to wait Until Conatiner to stop
func WaitWorkloadToDelete(ctx context.Context, clientset kubernetes.Interface, namespace, workloadName string) error {
	var (
		count int64 // To count minutes
	)
	ticker := time.NewTicker(time.Second * 30)
	timeoutchan := make(chan bool)

	go func() {
		for range ticker.C {
			count++
			deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, workloadName, metav1.GetOptions{})
			fmt.Println("Deployment... ", deployment)
			if err != nil {
				timeoutchan <- true
			} else if count == 10 {
				fmt.Println("Workload is not deleted and it exeed 5 minutes")
				timeoutchan <- false
			}
		}
	}()
	if <-timeoutchan {
		fmt.Println("Workload is Deleted :")
		ticker.Stop()
		return nil
	} else {
		fmt.Println("Workload is not able to stop")
		ticker.Stop()
		return errors.New("workload is not stopped yet")
	}
}

func DeleteNamespace(namespace string, clientset kubernetes.Interface, ctx context.Context) error {
	return clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
}
