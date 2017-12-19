# playground

This subrepository holds the source for various packages and tools that support
the Go playground: https://play.golang.org/

To submit changes to this repository, see http://golang.org/doc/contribute.html.

## Frontend

### Building

```
# build the frontend image
docker build -t frontend frontend/
```

### Dev Setup

```
gcloud components install cloud-datastore-emulator
```

### Running

```
# run the datastore emulator
gcloud --project=golang-org beta emulators datastore start
# set env vars
$(gcloud beta emulators datastore env-init)
# run the frontend
cd frontend && go install && frontend
```

Now visit localhost:8080 to ensure it worked.

## Sandbox

### Building

```
# build the sandbox image
docker build -t sandbox sandbox/
```

### Running

```
# run the sandbox
docker run -d -p 8080:8080 sandbox
# get docker host ip, try boot2docker fallback on localhost.
DOCKER_HOST_IP=$(boot2docker ip || echo localhost)
# run go some code
cat /path/to/code.go | go run ./sandbox/client.go | curl --data @- $DOCKER_HOST_IP:8080/compile
```

To submit changes to this repository, see http://golang.org/doc/contribute.html.

# Deployment

(Googlers only) To deploy the front-end, use `play/deploy.sh`.

```
gcloud --project golang-org app deploy sandbox/app-flex.yaml --no-promote
```

Use the Cloud Console's to set the new version as the default:
https://cloud.google.com/console/appengine/versions?project=golang-org&moduleId=sandbox-flex
Then test that play.golang.org and tour.golang.org are working before deleting
the old version.
