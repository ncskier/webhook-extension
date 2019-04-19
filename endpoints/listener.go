package endpoints

import (
	"log"

	restful "github.com/emicklei/go-restful"
)

func handleWebhook(request *restful.Request, response *restful.Response) {
	log.Printf("Handle webhook request: %v", request)
	response.Write([]byte("Handle Webhook"))
}

// ListenerWebService returns the liveness web service
func ListenerWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/")
	ws.Route(ws.POST("").To(handleWebhook))

	return ws
}
