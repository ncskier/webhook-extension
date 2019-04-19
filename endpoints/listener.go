package endpoints

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	restful "github.com/emicklei/go-restful"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	gh "gopkg.in/go-playground/webhooks.v3/github"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const gitServerLabel = "gitServer"
const gitOrgLabel = "gitOrg"
const gitRepoLabel = "gitRepo"
const githubEventParameter = "Ce-Github-Event"

// BuildInformation - information required to build a particular commit from a Git repository.
type BuildInformation struct {
	REPOURL   string
	SHORTID   string
	COMMITID  string
	REPONAME  string
	TIMESTAMP string
}

func handleWebhook(request *restful.Request, response *restful.Response) {
	log.Printf("Handle webhook request: %+v", request)
	response.Write([]byte("Handle Webhook"))
}

// handleWebhook should be called when we hit the / endpoint with webhook data. Todo provide proper responses e.g. 503, server errors, 200 if good
func (r Resource) handleWebhook(request *restful.Request, response *restful.Response) {
	log.Print("In HandleWebhook code with error handling for a GitHub event...")
	buildInformation := BuildInformation{}
	log.Printf("Github event name to look for is: %s", githubEventParameter)
	gitHubEventType := request.HeaderParameter(githubEventParameter)

	if len(gitHubEventType) < 1 {
		log.Printf("found header (%s) exists but has no value! Request is: %+v", githubEventParameter, request)
		return
	}

	gitHubEventTypeString := strings.Replace(gitHubEventType, "\"", "", -1)

	log.Printf("GitHub event type is %s", gitHubEventTypeString)

	timestamp := getDateTimeAsString()

	if gitHubEventTypeString == "push" {
		log.Print("Handling a push event...")

		webhookData := gh.PushPayload{}

		if err := request.ReadEntity(&webhookData); err != nil {
			log.Printf("an error occurred decoding webhook data: %s", err)
			return
		}

		buildInformation.REPOURL = webhookData.Repository.URL
		buildInformation.SHORTID = webhookData.HeadCommit.ID[0:7]
		buildInformation.COMMITID = webhookData.HeadCommit.ID
		buildInformation.REPONAME = webhookData.Repository.Name
		buildInformation.TIMESTAMP = timestamp

		createPipelineRunFromWebhookData(buildInformation, r)
		log.Printf("Build information for repository %s:%s %s", buildInformation.REPOURL, buildInformation.SHORTID, buildInformation)

	} else if gitHubEventTypeString == "pull_request" {
		log.Print("Handling a pull request event...")

		webhookData := gh.PullRequestPayload{}

		if err := request.ReadEntity(&webhookData); err != nil {
			log.Printf("an error occurred decoding webhook data: %s", err)
			return
		}

		buildInformation.REPOURL = webhookData.Repository.HTMLURL
		buildInformation.SHORTID = webhookData.PullRequest.Head.Sha[0:7]
		buildInformation.COMMITID = webhookData.PullRequest.Head.Sha
		buildInformation.REPONAME = webhookData.Repository.Name
		buildInformation.TIMESTAMP = timestamp

		createPipelineRunFromWebhookData(buildInformation, r)
		log.Printf("Build information for repository %s:%s %s", buildInformation.REPOURL, buildInformation.SHORTID, buildInformation)

	} else {
		log.Print("event wasn't a push or pull event, no action will be taken")
	}
}

// This is the main flow that handles building and deploying: given everything we need to kick off a build, do so
func createPipelineRunFromWebhookData(buildInformation BuildInformation, r Resource) {
	log.Printf("In createPipelineRunFromWebhookData, build information: %s", buildInformation)

	// TODO: Use the dashboard endpoint to create the PipelineRun
	// Track PR: https://github.com/tektoncd/dashboard/pull/33
	// and issue: https://github.com/tektoncd/dashboard/issues/47

	// These can be set either when creating the event handler/github source manually through yml or when installing the Helm chart.
	// For the chart, PIPELINE_RUN_NAMESPACE picks up the specified namespace. If this is set it will be used.

	// Otherwise, we use the namespace where this has been installed. This allows us to have PipelineRuns in namespaces other than the installed to namespace
	// but may require additional RBAC configuration depending on your cluster configuration and service account permissions.

	// The specified service account name is also exposed through the chart's values.yaml: defaulting to "tekton-pipelines".

	pipelineNs := os.Getenv("PIPELINE_RUN_NAMESPACE")
	if pipelineNs == "" {
		pipelineNs = "default"
	}
	saName := "default"

	log.Printf("PipelineRuns will be created in the namespace %s", pipelineNs)
	log.Printf("PipelineRuns will be created with the service account %s", saName)

	startTime := getDateTimeAsString()

	// Assumes you've already applied the yml: so the pipeline definition and its tasks must exist upfront.
	generatedPipelineRunName := fmt.Sprintf("devops-pipeline-run-%s", startTime)

	// get information from related githubsource instance
	registrySecret, helmSecret, pipelineTemplateName := r.getGitHubSourceInfo(buildInformation.REPOURL, pipelineNs)

	// Unique names are required so timestamp them.
	imageResourceName := fmt.Sprintf("docker-image-%s", startTime)
	gitResourceName := fmt.Sprintf("git-source-%s", startTime)

	pipeline, err := r.getPipelineImpl(pipelineTemplateName, pipelineNs)
	if err != nil {
		log.Printf("could not find the pipeline template %s in namespace %s", pipelineTemplateName, pipelineNs)
		return
	}
	log.Printf("Found the pipeline template %s OK", pipelineTemplateName)

	log.Print("Creating PipelineResources next...")

	registryURL := os.Getenv("DOCKER_REGISTRY_LOCATION")
	urlToUse := fmt.Sprintf("%s/%s:%s", registryURL, strings.ToLower(buildInformation.REPONAME), buildInformation.SHORTID)
	log.Printf("Pushing the image to %s", urlToUse)

	paramsForImageResource := []v1alpha1.Param{{Name: "url", Value: urlToUse}}
	pipelineImageResource := definePipelineResource(imageResourceName, pipelineNs, paramsForImageResource, "image")
	createdPipelineImageResource, err := r.TektonClient.TektonV1alpha1().PipelineResources(pipelineNs).Create(pipelineImageResource)
	if err != nil {
		log.Printf("could not create pipeline image resource to be used in the pipeline, error: %s", err)
	} else {
		log.Printf("Created pipeline image resource %s successfully", createdPipelineImageResource.Name)
	}

	paramsForGitResource := []v1alpha1.Param{{Name: "revision", Value: buildInformation.COMMITID}, {Name: "url", Value: buildInformation.REPOURL}}
	pipelineGitResource := definePipelineResource(gitResourceName, pipelineNs, paramsForGitResource, "git")
	createdPipelineGitResource, err := r.TektonClient.TektonV1alpha1().PipelineResources(pipelineNs).Create(pipelineGitResource)

	if err != nil {
		log.Printf("could not create pipeline git resource to be used in the pipeline, error: %s", err)
	} else {
		log.Printf("Created pipeline git resource %s successfully", createdPipelineGitResource.Name)
	}

	gitResourceRef := v1alpha1.PipelineResourceRef{Name: gitResourceName}
	imageResourceRef := v1alpha1.PipelineResourceRef{Name: imageResourceName}

	resources := []v1alpha1.PipelineResourceBinding{{Name: "docker-image", ResourceRef: imageResourceRef}, {Name: "git-source", ResourceRef: gitResourceRef}}

	imageTag := buildInformation.SHORTID
	imageName := fmt.Sprintf("%s/%s", registryURL, strings.ToLower(buildInformation.REPONAME))
	releaseName := fmt.Sprintf("%s-%s", strings.ToLower(buildInformation.REPONAME), buildInformation.SHORTID)
	repositoryName := strings.ToLower(buildInformation.REPONAME)
	params := []v1alpha1.Param{{Name: "image-tag", Value: imageTag},
		{Name: "image-name", Value: imageName},
		{Name: "release-name", Value: releaseName},
		{Name: "repository-name", Value: repositoryName},
		{Name: "target-namespace", Value: pipelineNs}}

	if registrySecret != "" {
		params = append(params, v1alpha1.Param{Name: "registry-secret", Value: registrySecret})
	}
	if helmSecret != "" {
		params = append(params, v1alpha1.Param{Name: "helm-secret", Value: helmSecret})
	}

	// PipelineRun yml defines the references to the above named resources.
	pipelineRunData, err := definePipelineRun(generatedPipelineRunName, pipelineNs, saName, buildInformation.REPOURL,
		pipeline, v1alpha1.PipelineTriggerTypeManual, resources, params)

	log.Printf("Creating a new PipelineRun named %s in the namespace %s using the service account %s", generatedPipelineRunName, pipelineNs, saName)

	pipelineRun, err := r.TektonClient.TektonV1alpha1().PipelineRuns(pipelineNs).Create(pipelineRunData)
	if err != nil {
		log.Printf("error creating the PipelineRun: %s", err)
		return
	}
	log.Printf("PipelineRun created: %+v", pipelineRun)
}

/* Get all pipelines in a given namespace: the caller needs to handle any errors,
an empty v1alpha1.Pipeline{} is returned if no pipeline is found */
func (r Resource) getPipelineImpl(name, namespace string) (v1alpha1.Pipeline, error) {
	log.Printf("in getPipelineImpl, name %s, namespace %s", name, namespace)

	pipelines := r.TektonClient.TektonV1alpha1().Pipelines(namespace)
	pipeline, err := pipelines.Get(name, metav1.GetOptions{})
	if err != nil {
		log.Printf("could not retrieve the pipeline called %s in namespace %s", name, namespace)
		return v1alpha1.Pipeline{}, err
	}
	log.Print("Found the pipeline definition OK")
	return *pipeline, nil
}

/* Create a new PipelineResource: this should be of type git or image */
func definePipelineResource(name, namespace string, params []v1alpha1.Param, resourceType v1alpha1.PipelineResourceType) *v1alpha1.PipelineResource {
	pipelineResource := v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.PipelineResourceSpec{
			Type:   resourceType,
			Params: params,
		},
	}
	resourcePointer := &pipelineResource
	return resourcePointer
}

/* Create a new PipelineRun - repoUrl, resourceBinding and params can be nill depending on the Pipeline
each PipelineRun has a 1 hour timeout: */
func definePipelineRun(pipelineRunName, namespace, saName, repoURL string,
	pipeline v1alpha1.Pipeline,
	triggerType v1alpha1.PipelineTriggerType,
	resourceBinding []v1alpha1.PipelineResourceBinding,
	params []v1alpha1.Param) (*v1alpha1.PipelineRun, error) {

	gitServer, gitOrg, gitRepo := "", "", ""
	err := errors.New("")
	if repoURL != "" {
		gitServer, gitOrg, gitRepo, err = getGitValues(repoURL)
		if err != nil {
			log.Printf("there was an error getting the Git values: %s", err)
			return &v1alpha1.PipelineRun{}, err
		}
	}

	pipelineRunData := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineRunName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":          "devops-knative",
				gitServerLabel: gitServer,
				gitOrgLabel:    gitOrg,
				gitRepoLabel:   gitRepo,
			},
		},

		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{Name: pipeline.Name},
			// E.g. v1alpha1.PipelineTriggerTypeManual
			Trigger:        v1alpha1.PipelineTrigger{Type: triggerType},
			ServiceAccount: saName,
			Timeout:        &metav1.Duration{Duration: 1 * time.Hour},
			Resources:      resourceBinding,
			Params:         params,
		},
	}
	pipelineRunPointer := &pipelineRunData
	return pipelineRunPointer, nil
}

// Returns the git server excluding transport, org and repo
func getGitValues(url string) (gitServer, gitOrg, gitRepo string, err error) {
	repoURL := ""
	if url != "" {
		url = strings.ToLower(url)
		if strings.Contains(url, "https://") {
			repoURL = strings.TrimPrefix(url, "https://")
		} else {
			repoURL = strings.TrimPrefix(url, "http://")
		}
	}

	// example at this point: github.com/tektoncd/pipeline
	numSlashes := strings.Count(repoURL, "/")
	if numSlashes < 2 {
		return "", "", "", errors.New("Url didn't match the requirements (at least two slashes)")
	}
	repoURL = strings.TrimSuffix(repoURL, "/")

	gitServer = repoURL[0:strings.Index(repoURL, "/")]
	gitOrg = repoURL[strings.Index(repoURL, "/")+1 : strings.LastIndex(repoURL, "/")]
	gitRepo = repoURL[strings.LastIndex(repoURL, "/")+1:]

	return gitServer, gitOrg, gitRepo, nil
}

func getDateTimeAsString() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// ListenerWebService returns the liveness web service
func ListenerWebService(r Resource) *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/")
	ws.Route(ws.POST("").To(r.handleWebhook))

	return ws
}
