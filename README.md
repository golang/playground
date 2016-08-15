# playground

This subrepository holds the source for various packages and tools that support
the Go playground: https://play.golang.org/

To submit changes to this repository, see http://golang.org/doc/contribute.html.

## Building

```
# build the sandbox image
docker build -t sandbox sandbox/
```

## Running

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
gcloud --project golang-org app deploy sandbox/app.yaml --no-promote --version=17rc6
```

Use the Cloud Console's to set the new version as the default:
	https://cloud.google.com/console/appengine/versions?project=golang-org&moduleId=sandbox
Then test that play.golang.org and tour.golang.org are working before deleting
the old version.
