# playground

This subrepository holds the source for various packages and tools that support
the Go playground: https://play.golang.org/

## building

```
# build the sandbox image
docker build -t sandbox sandbox/
```

## running

```
# run the sandbox
docker run -d -p 8080:8080 sandbox
# get docker host ip, try boot2docker fallback on localhost.
DOCKER_HOST_IP=$(boot2docker ip || echo localhost)
# run go some code
cat /path/to/code.go | go run ./sandbox/client.go | curl --data @- $DOCKER_HOST_IP:8080/compile
```

To submit changes to this repository, see http://golang.org/doc/contribute.html.

# deployment

## managed-vms

```
gcloud preview app run app/app.yaml
gcloud preview app run sandbox/app.yaml

gcloud config set project golang-org
gcloud preview app deploy app/app.yaml --version play
gcloud preview app deploy sandbox/app.yaml --set-default
```

## kubernetes

```
# sandbox
docker push golang/playground-sandbox
gcloud preview container replicationcontrollers create --config sandbox/kubernetes/controller.yaml
gcloud preview container services create --config sandbox/kubernetes/service.yaml
```

## container-vm

```
# sandbox
docker push golang/playground-sandbox
gcloud compute instances create playground-sandbox-vm --image container-vm --metadata-from-file google-container-manifest=sandbox/container-vm.yaml
```
