# webhook-extension

## Start extension
```
docker build -t ncskier/extension:latest -f cmd/webhook/Dockerfile .
docker push ncskier/extension:latest

docker build -t ncskier/extension-listener:latest -f cmd/listener/Dockerfile .
docker push ncskier/extension-listener:latest
```

```
kubectl apply -f install/ -n $namespace
```

## Start dashboard
```
kubectl apply -f install/ -n $namespace
kubectl port-forward $(kubectl get pod -l app=tekton-dashboard -o name -n $namespace) 9097:9097 -n $namespace
```

## Create GitHub source
Here's a Knative Eventing tutorial about GitHub sources: https://knative.dev/docs/eventing/samples/github-source/
```
data='{
  "name": "go-hello-world",
  "namespace": "'${namespace}'",
  "gitrepositoryurl": "https://github.com/ncskier/go-hello-world",
  "accesstoken": "github-secret",
  "pipeline": "simple-pipeline"
}'
curl -d "${data}" -H "Content-Type: application/json" -X POST http://localhost:9097/webhook
```
Need secret for accesstoken (in this example the secret is called `github-secret`)