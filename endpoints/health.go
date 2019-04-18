package endpoints

import (
	"net/http"

	restful "github.com/emicklei/go-restful"
)

func checkHealth(request *restful.Request, response *restful.Response) {
	response.WriteHeader(http.StatusNoContent)
}

// LivenessWebService returns the liveness web service
func LivenessWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/liveness")
	ws.Route(ws.GET("/").To(checkHealth))

	return ws
}

// ReadinessWebService returns the readiness web service
func ReadinessWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/readiness")
	ws.Route(ws.GET("/").To(checkHealth))

	return ws
}
