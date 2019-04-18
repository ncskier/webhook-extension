package endpoints

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	restful "github.com/emicklei/go-restful"
	eventapi "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	eventsrcclient "github.com/knative/eventing-sources/pkg/client/clientset/versioned/typed/sources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclientset "k8s.io/client-go/kubernetes"
)

// Resource stores all types here that are reused throughout files
type Resource struct {
	K8sClient      k8sclientset.Interface
	EventSrcClient eventsrcclient.SourcesV1alpha1Interface
}

// Webhook stores the webhook information
type Webhook struct {
	Name                 string `json:"name"`
	Namespace            string `json:"namespace"`
	GitRepositoryURL     string `json:"gitrepositoryurl"`
	AccessTokenRef       string `json:"accesstoken"`
	Pipeline             string `json:"pipeline"`
	RegistrySecret       string `json:"registrysecret,omitempty"`
	HelmSecret           string `json:"helmsecret,omitempty"`
	RepositorySecretName string `json:"repositorysecretname,omitempty"`
}

// ConfigMapName ... the name of the ConfigMap to create
const ConfigMapName = "githubsource"

// RegisterTo registers the webhook routes to the container
func (r Resource) RegisterTo(container *restful.Container) {
	ws := new(restful.WebService)
	ws.
		Path("/webhook").
		Consumes(restful.MIME_JSON, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_JSON)

	ws.Route(ws.POST("/").To(r.createWebhook))
	// ws.Route(ws.GET("/").To(r.getAllWebhooks))
	// ws.Route(ws.GET("/{webhook-id}").To(r.getWebhook))
	// ws.Route(ws.PUT("/{webhook-id}").To(r.updateWebhook))
	// ws.Route(ws.DELETE("/{webhook-id}").To(r.deleteWebhook))

	container.Add(ws)
}

func (r Resource) createWebhook(request *restful.Request, response *restful.Response) {
	log.Printf("create webhook %v", request)

	source := Webhook{}
	if err := request.ReadEntity(&source); err != nil {
		log.Printf("Got an error trying to create a githubsource: %s", err)
		RespondError(response, err, http.StatusBadRequest)
		return
	}
	namespace := source.Namespace
	if namespace == "" {
		log.Printf("Error: namespace is required, but none was given")
		return
	}
	log.Printf("createGitHubSource: namespace: %s, entry: %v", namespace, source)
	pieces := strings.Split(source.GitRepositoryURL, "/")
	if len(pieces) < 4 {
		log.Printf("error createGitHubSource: GitRepositoryURL format: %+v", source.GitRepositoryURL)
		RespondError(response, errors.New("GitRepositoryURL format error"), http.StatusBadRequest)
		return
	}
	log.Printf("createGitHubSource: URL: %s, Owner-repo: %s",
		strings.TrimSuffix(source.GitRepositoryURL, pieces[len(pieces)-2]+"/"+pieces[len(pieces)-1]),
		pieces[len(pieces)-2]+"/"+strings.TrimSuffix(pieces[len(pieces)-1], ".git"))
	entry := eventapi.GitHubSource{
		ObjectMeta: metav1.ObjectMeta{Name: source.Name},
		Spec: eventapi.GitHubSourceSpec{
			OwnerAndRepository: pieces[len(pieces)-2] + "/" + strings.TrimSuffix(pieces[len(pieces)-1], ".git"),
			EventTypes:         []string{"push", "pull_request"},
			GitHubAPIURL:       strings.TrimSuffix(source.GitRepositoryURL, pieces[len(pieces)-2]+"/"+pieces[len(pieces)-1]) + "api/v3/",
			AccessToken: eventapi.SecretValueFromSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "accessToken",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: source.AccessTokenRef,
					},
				},
			},
			SecretToken: eventapi.SecretValueFromSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "secretToken",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: source.AccessTokenRef,
					},
				},
			},
			Sink: &corev1.ObjectReference{
				APIVersion: "serving.knative.dev/v1alpha1",
				Kind:       "Service",
				Name:       "tekton-dashboard-service",
			},
		},
	}
	_, err := r.EventSrcClient.GitHubSources(namespace).Create(&entry)
	if err != nil {
		log.Printf("error createGitHubSource: %+v", err)
		RespondError(response, err, http.StatusBadRequest)
		return
	}
	// sources := r.readGitHubSource(namespace)
	// sources[source.Name] = source
	// r.writeGitHubSource(namespace, sources)
	response.WriteHeader(http.StatusNoContent)
}

func (r Resource) getAllWebhooks(request *restful.Request, response *restful.Response) {

}

func (r Resource) getWebhook(request *restful.Request, response *restful.Response) {

}

func (r Resource) updateWebhook(request *restful.Request, response *restful.Response) {

}

func (r Resource) deleteWebhook(request *restful.Request, response *restful.Response) {

}

func (r Resource) readGitHubSource(namespace string) map[string]Webhook {
	log.Printf("readGitHubSource")
	configMapClient := r.K8sClient.CoreV1().ConfigMaps(namespace)
	configMap, err := configMapClient.Get(ConfigMapName, metav1.GetOptions{})
	if err != nil {
		log.Printf("readGitHubSource: %s", err)
		configMap = &corev1.ConfigMap{}
		configMap.BinaryData = make(map[string][]byte)
	}
	raw, ok := configMap.BinaryData["GitHubSource"]
	var result map[string]Webhook
	if ok {
		err = json.Unmarshal(raw, &result)
		if err != nil {
			log.Printf("readGitHubSource: %s", err)
		}
	} else {
		result = make(map[string]Webhook)
	}
	log.Printf("readGitHubSource: %v", result)
	return result
}

func (r Resource) writeGitHubSource(namespace string, source map[string]Webhook) {
	log.Printf("writeGitHubSource: nameSpace: %s, %+v", namespace, source)
	configMapClient := r.K8sClient.CoreV1().ConfigMaps(namespace)
	configMap, err := configMapClient.Get(ConfigMapName, metav1.GetOptions{})
	var create = false
	if err != nil {
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: namespace,
			},
		}
		configMap.BinaryData = make(map[string][]byte)
		create = true
	}
	buf, err := json.Marshal(source)
	if err != nil {
		log.Printf("writeGitHubSource: %s", err)
	}
	configMap.BinaryData["GitHubSource"] = buf
	if create {
		_, err = configMapClient.Create(configMap)
		if err != nil {
			log.Printf("writeGitHubSource: %s", err)
		}
	} else {
		_, err = configMapClient.Update(configMap)
		if err != nil {
			log.Printf("writeGitHubSource: %s", err)
		}
	}

}

// RespondError ...
func RespondError(response *restful.Response, err error, statusCode int) {
	log.Printf("[RespondError] Error: %s", err.Error())
	log.Printf("Response is %v\n", *response)
	response.AddHeader("Content-Type", "text/plain")
	response.WriteError(statusCode, err)
}

// RespondErrorMessage ...
func RespondErrorMessage(response *restful.Response, message string, statusCode int) {
	log.Printf("[RespondErrorMessage] Message: %s", message)
	response.AddHeader("Content-Type", "text/plain")
	response.WriteErrorString(statusCode, message)
}

// RespondErrorAndMessage ...
func RespondErrorAndMessage(response *restful.Response, err error, message string, statusCode int) {
	log.Printf("[RespondErrorAndMessage] Error: %s", err.Error())
	log.Printf("Message is %x\n", message)
	response.AddHeader("Content-Type", "text/plain")
	response.WriteErrorString(statusCode, message)
}
