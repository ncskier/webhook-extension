package main

import (
	"log"
	"net/http"

	restful "github.com/emicklei/go-restful"
	"github.com/ncskier/webhook-extension/endpoints"
)

func main() {
	// Create/setup resource
	r, err := endpoints.NewResource()
	if err != nil {
		log.Fatalf("Fatal error creating resource: %s", err.Error())
	}

	// Set up routes
	wsContainer := restful.NewContainer()
	// Add listener
	wsContainer.Add(endpoints.ListenerWebService(r))
	// Add liveness/readiness
	wsContainer.Add(endpoints.LivenessWebService())
	wsContainer.Add(endpoints.ReadinessWebService())

	// Serve
	log.Print("Creating server and entering wait loop")
	server := &http.Server{Addr: ":8080", Handler: wsContainer}
	log.Fatal(server.ListenAndServe())
}
