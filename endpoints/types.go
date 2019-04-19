package endpoints

import (
	"log"

	eventsrcclientset "github.com/knative/eventing-sources/pkg/client/clientset/versioned"
	tektoncdclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Resource stores all types here that are reused throughout files
type Resource struct {
	EventSrcClient eventsrcclientset.Interface
	TektonClient   tektoncdclientset.Interface
	K8sClient      k8sclientset.Interface
}

// NewResource returns a new Resource instantiated with its clientsets
func NewResource() (Resource, error) {
	// Get cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error getting in cluster config: %s", err.Error())
		return Resource{}, err
	}

	// Setup event source client
	eventSrcClient, err := eventsrcclientset.NewForConfig(config)
	if err != nil {
		log.Printf("Error building event source client: %s", err.Error())
		return Resource{}, err
	}

	// Setup tektoncd client
	tektonClient, err := tektoncdclientset.NewForConfig(config)
	if err != nil {
		log.Printf("Error building tekton clientset: %s", err.Error())
		return Resource{}, err
	}

	// Setup k8s client
	k8sClient, err := k8sclientset.NewForConfig(config)
	if err != nil {
		log.Printf("Error building k8s clientset: %s", err.Error())
		return Resource{}, err
	}

	r := Resource{
		K8sClient:      k8sClient,
		TektonClient:   tektonClient,
		EventSrcClient: eventSrcClient,
	}
	return r, nil
}
