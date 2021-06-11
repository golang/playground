# playground

This subrepository holds the source for the Go playground:
https://play.golang.org/

## Building

```bash
# build the image
docker build -t playground .
```

## Running

```bash
docker run --name=play --rm -p 8080:8080 golang/playground &
# run some Go code
cat /path/to/code.go | go run client.go | curl -s --upload-file - localhost:8080/compile
```

## Deployment

### Deployment Triggers

Playground releases automatically triggered when new Go repository tags are pushed to GitHub, or when master is pushed
on the playground repository.

For details, see [deploy/go_trigger.yaml](deploy/go_trigger.yaml),
[deploy/playground_trigger.yaml](deploy/playground_trigger.yaml),
and [deploy/deploy.json](deploy/deploy.json).

Changes to the trigger configuration can be made to the YAML files, or in the GCP UI, which should be kept in sync
using the `push-cloudbuild-trigger` and `pull-cloudbuild-trigger` make targets.

### Deploy via Cloud Build

The Cloud Build configuration will always build and deploy with the latest supported release of Go.

```bash
gcloud builds submit --config deploy/deploy.json .
```

### Deploy via gcloud app deploy

Building the playground Docker container takes more than the default 10 minute time limit of cloud build, so increase
its timeout first (note, `app/cloud_build_timeout` is a global configuration value):

```bash
gcloud config set app/cloud_build_timeout 1200  # 20 mins
```

Alternatively, to avoid Cloud Build and build locally:

```bash
make docker
docker tag golang/playground-go2go:latest gcr.io/golang-org/playground-go2go:latest
docker push gcr.io/golang-org/playground-go2go:latest
gcloud --project=golang-org --account=you@google.com app deploy app.yaml --image-url=gcr.io/golang-org/playground-go2go:latest
```

Then:

```bash
gcloud --project=golang-org --account=you@google.com app deploy app.yaml
```

## Contributing

To submit changes to this repository, see
https://golang.org/doc/contribute.html.
