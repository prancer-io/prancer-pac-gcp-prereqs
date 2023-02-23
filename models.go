package prancer

var (
	CREATE_CLUSTER  = "create_cluster"
	CREATE_WORKLOAD = "create_workload"
	DELETE_WORKLOAD = "delete_workload"
)

type RequestParameter struct {
	Action string `json:"action" valid:"required"`
}

type CreateClusterConfig struct {
	ClusterName          string `json:"clusterName" valid:"required"`
	Location             string `json:"location" valid:"required"`
	ServiceAccountClient string `json:"serviceAccountClient" valid:"required"`
	Parent               string `json:"parent" valid:"required"`
}

type CreateWorkloadConfig struct {
	ClusterPath     string `json:"clusterPath" valid:"required"`
	ApplicationName string `json:"applicationName" valid:"required"`
	ScannerImage    string `json:"scannerImage" valid:"required"`
	ConfigId        string `json:"configId" valid:"required"`
	Token           string `json:"token" valid:"required"`
	TokenId         string `json:"tokenId" valid:"required"`
	Domain          string `json:"domain" valid:"required"`
	CusId           string `json:"cusId" valid:"required"`
}

type DeleteWorkloadConfig struct {
	Namespace     string `json:"namespace" valid:"required"`
	ConfigMap     string `json:"configmap" valid:"required"`
	Deployment    string `json:"deployment" valid:"required"`
	Parent        string `json:"parent" valid:"required"`
	DeleteCluster bool   `json:"deleteCluster" valid:"required"`
}

type Response struct {
	Message    string            `json:"message"`
	StatusCode int               `json:"statusCode"`
	Status     string            `json:"status"`
	Data       map[string]string `json:"data"`
}
