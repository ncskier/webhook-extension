package main

import (
	"log"
	"net/http"

	restful "github.com/emicklei/go-restful"
	eventsrcclient "github.com/knative/eventing-sources/pkg/client/clientset/versioned/typed/sources/v1alpha1"
	endpoints "github.com/ncskier/webhook-extension/endpoints"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// Get cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("error getting in cluster config: %s", err.Error())
	}

	// Setup event source client
	eventSrcClient, err := eventsrcclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("error building event source client: %s", err.Error())
	}

	// Setup k8s client
	k8sClient, err := k8sclientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("error building k8s clientset: %s", err.Error())
	}

	r := endpoints.Resource{
		K8sClient:      k8sClient,
		EventSrcClient: eventSrcClient,
	}
	// r := endpoints.Resource{}

	// Set up routes
	wsContainer := restful.NewContainer()

	r.RegisterTo(wsContainer)

	// Add liveness/readiness
	wsContainer.Add(endpoints.LivenessWebService())
	wsContainer.Add(endpoints.ReadinessWebService())

	// Serve
	log.Print("Creating server and entering wait loop")
	server := &http.Server{Addr: ":8080", Handler: wsContainer}
	log.Fatal(server.ListenAndServe())
}
