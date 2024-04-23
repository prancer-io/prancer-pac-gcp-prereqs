package prancer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"

	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/asaskevich/govalidator"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	functions.HTTP("RunPAC", RunPAC)
}

func RunPAC(w http.ResponseWriter, r *http.Request) {
	var (
		requestParameter RequestParameter
		response         Response
		err              error
	)

	bodyBytes, _ := ioutil.ReadAll(r.Body)
	r.Body.Close()

	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	if err := json.NewDecoder(r.Body).Decode(&requestParameter); err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to create a cluster, %s", err),
		}
		w.Header().Set("Content-Type", "application/json")
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
		return
	}

	switch requestParameter.Action {
	case CREATE_CLUSTER:
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		form := new(CreateClusterConfig)
		if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
			response = Response{
				Status:     "Fail",
				StatusCode: 400,
				Message:    fmt.Sprintf("Invalid request parameter! %s", err),
			}
			w.Header().Set("Content-Type", "application/json")
			responseBytes, _ := json.Marshal(response)
			w.Write(responseBytes)
			return
		}

		if _, err = govalidator.ValidateStruct(form); err != nil {
			response = Response{
				Status:     "Fail",
				StatusCode: 400,
				Message:    fmt.Sprintf("Invalid request parameter! %s", err),
			}
			w.Header().Set("Content-Type", "application/json")
			responseBytes, _ := json.Marshal(response)
			w.Write(responseBytes)
			return
		}

		response = CreateCluster(*form)
		w.Header().Set("Content-Type", "application/json")
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
		return

	case CREATE_WORKLOAD:

		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		form := new(CreateWorkloadConfig)
		if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
			response = Response{
				Status:     "Fail",
				StatusCode: 400,
				Message:    fmt.Sprintf("Invalid request parameter! %s", err),
			}
			w.Header().Set("Content-Type", "application/json")
			responseBytes, _ := json.Marshal(response)
			w.Write(responseBytes)
			return
		}

		if _, err = govalidator.ValidateStruct(form); err != nil {
			response = Response{
				Status:     "Fail",
				StatusCode: 400,
				Message:    fmt.Sprintf("Invalid request parameter! %s", err),
			}
			w.Header().Set("Content-Type", "application/json")
			responseBytes, _ := json.Marshal(response)
			w.Write(responseBytes)
			return
		}

		response = CreateWorkloadDeployment(*form)
		w.Header().Set("Content-Type", "application/json")
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
		return

	case DELETE_WORKLOAD:
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		form := new(DeleteWorkloadConfig)
		if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
			response = Response{
				Status:     "Fail",
				StatusCode: 400,
				Message:    fmt.Sprintf("Invalid request parameter! %s", err),
			}
			w.Header().Set("Content-Type", "application/json")
			responseBytes, _ := json.Marshal(response)
			w.Write(responseBytes)
			return
		}

		if _, err = govalidator.ValidateStruct(form); err != nil {
			response = Response{
				Status:     "Fail",
				StatusCode: 400,
				Message:    fmt.Sprintf("Invalid request parameter! %s", err),
			}
			w.Header().Set("Content-Type", "application/json")
			responseBytes, _ := json.Marshal(response)
			w.Write(responseBytes)
			return
		}

		response = DeleteWorkloadAndCluster(*form)
		w.Header().Set("Content-Type", "application/json")
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
		return
	default:
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Invalid request parameter! Provide the valid value for `action` parameter.",
		}
		w.Header().Set("Content-Type", "application/json")
		responseBytes, _ := json.Marshal(response)
		w.Write(responseBytes)
		return
	}
}

func CreateCluster(createClusterConfig CreateClusterConfig) Response {
	var (
		response           Response
		defaultCredentials *google.Credentials
		containerService   *container.Service
		err                error
	)

	ctx := context.Background()
	containerService, err = container.NewService(ctx)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to create container service. %s", err),
		}
		return response
	}

	rb := &container.CreateClusterRequest{
		Cluster: &container.Cluster{
			Name:             createClusterConfig.ClusterName,
			InitialNodeCount: 1,
			Autoscaling: &container.ClusterAutoscaling{
				EnableNodeAutoprovisioning: true,
				AutoprovisioningNodePoolDefaults: &container.AutoprovisioningNodePoolDefaults{
					ServiceAccount: createClusterConfig.ServiceAccountClient,
				},
			},
			BinaryAuthorization: &container.BinaryAuthorization{
				EvaluationMode: "DISABLED",
			},
			Autopilot: &container.Autopilot{
				Enabled: true,
			},
			Location: createClusterConfig.Location,
		},
	}

	_, err = containerService.Projects.Locations.Clusters.Create(createClusterConfig.Parent, rb).Context(ctx).Do()
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create cluster",
		}
		return response
	}

	defaultCredentials, err = google.FindDefaultCredentials(ctx, compute.ComputeScope)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create cluster",
		}
		return response
	}

	clusterPath := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", defaultCredentials.ProjectID, createClusterConfig.Location, createClusterConfig.ClusterName)

	response = Response{
		Status:     "Success",
		StatusCode: 200,
		Message:    "Cluster created successfully",
		Data: map[string]string{
			"clusterPath": clusterPath,
		},
	}
	return response
}

func CreateWorkloadDeployment(createWorkloadConfig CreateWorkloadConfig) Response {
	var (
		response                                      Response
		cluster                                       *container.Cluster
		containerService                              *container.Service
		err                                           error
		appName, nameSpace, configMap, deploymentName string
	)

	ctx := context.Background()
	containerService, err = container.NewService(ctx)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create container service",
		}
		return response
	}

	cluster, err = container.NewProjectsLocationsClustersService(containerService).Get(createWorkloadConfig.ClusterPath).Do()
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create container service",
		}
		return response
	}

	creds, err := google.FindDefaultCredentials(ctx, container.CloudPlatformScope)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Error in get google credentials",
		}
		return response
	}

	clientset, err := GetGKEClientset(cluster, creds.TokenSource)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create kubernets client",
		}
		return response
	}

	reg := regexp.MustCompile(`[^\w\d-]`)
	appName = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(createWorkloadConfig.ApplicationName, " ", "-"), "_", "-"))
	appName = reg.ReplaceAllString(appName, "")

	nameSpace = fmt.Sprintf("%s-%s", appName, "namespace")
	configMap = fmt.Sprintf("%s-%s", appName, "configmap")
	deploymentName = fmt.Sprintf("%s-%s", appName, "deployment")

	namespace, _ := CreateNamespace(nameSpace, clientset, ctx)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create namespace",
		}
		return response
	}

	configmap, err := CreateConfigMap(configMap, namespace.Name, clientset, ctx, createWorkloadConfig)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create confgmap",
		}
		return response
	}

	clientset.AppsV1().Deployments(namespace.Name).Delete(ctx, deploymentName, metav1.DeleteOptions{})

	_, err = clientset.AppsV1().Deployments(namespace.Name).Create(ctx, BuildDeployment(configmap.Name, deploymentName, createWorkloadConfig), metav1.CreateOptions{})
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create pod",
		}
		return response
	}

	response = Response{
		Status:     "Success",
		StatusCode: 200,
		Message:    "Successfully created deployment",
	}
	return response
}

func DeleteWorkloadAndCluster(deleteWorkloadCluster DeleteWorkloadConfig) Response {
	var (
		containerService *container.Service
		response         Response
		err              error
	)

	ctx := context.Background()
	containerService, err = container.NewService(ctx)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    "Failed to create container service",
		}
		return response
	}

	cluster, err := container.NewProjectsLocationsClustersService(containerService).Get(deleteWorkloadCluster.Parent).Do()
	if err != nil {
		fmt.Printf("Failed to load a cluster %s", err)

		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to load a cluster %s", err),
		}
		return response
	}

	creds, err := google.FindDefaultCredentials(ctx, container.CloudPlatformScope)
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Error in get google credentials %s", err),
		}
		return response
	}

	clientset, err := GetGKEClientset(cluster, creds.TokenSource)
	if err != nil {
		fmt.Printf("Failed to create kubernets client, %s", err)
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("failed to create kubernets client, %s", err),
		}
		return response
	}

	fmt.Printf("Deleting Config Map")

	err = DeleteConfigMap(deleteWorkloadCluster.Namespace, deleteWorkloadCluster.ConfigMap, clientset, ctx)
	if err != nil {
		fmt.Printf("Failed to delete configmap: %s", err)
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to delete configmap: %s", err),
		}
		return response
	}

	err = clientset.AppsV1().Deployments(deleteWorkloadCluster.Namespace).Delete(ctx, deleteWorkloadCluster.Deployment, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("Failed to delete deployment, %s", err)
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to delete deployment, %s", err),
		}
		return response
	}

	if err = WaitWorkloadToDelete(ctx, clientset, deleteWorkloadCluster.Deployment, deleteWorkloadCluster.Namespace); err != nil {
		fmt.Printf("Error while deleting workload: %s", err)
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Error while deleting workload: %s", err),
		}
		return response
	}

	fmt.Printf("deleting namespace")
	err = DeleteNamespace(deleteWorkloadCluster.Namespace, clientset, ctx)
	if err != nil {
		fmt.Printf("Failed to delete namespace, %s", err)
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to delete namespace, %s", err),
		}
		return response
	}

	if strings.ToLower(deleteWorkloadCluster.DeleteCluster) != "true" {
		response = Response{
			Status:     "Success",
			StatusCode: 200,
			Message:    "Workload deleted successfully",
		}
		return response
	}

	_, err = containerService.Projects.Locations.Clusters.Delete(deleteWorkloadCluster.Parent).Context(ctx).Do()
	if err != nil {
		response = Response{
			Status:     "Fail",
			StatusCode: 400,
			Message:    fmt.Sprintf("Failed to delete cluster, %s", err),
		}
		return response
	}

	response = Response{
		Status:     "Success",
		StatusCode: 200,
		Message:    "Cluster deleted successfully",
	}
	return response
}
