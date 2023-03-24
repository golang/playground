# playground

[![Go Reference](https://pkg.go.dev/badge/golang.org/x/playground.svg)](https://pkg.go.dev/golang.org/x/playground)

This subrepository holds the source for the Go playground:
https://go.dev/play/

## Building

```bash
# build the image
docker build -t golang/playground .
```

## Running

```bash
docker run --name=play --rm -p 8080:8080 golang/playground &
# run some Go code
cat /path/to/code.go | go run client.go | curl -s --upload-file - localhost:8080/compile
```

To run the "gotip" version of the playground, set `GOTIP=true`
in your environment (via `-e GOTIP=true` if using `docker run`).

## Deployment

### Deployment Triggers

Playground releases automatically triggered when new Go repository tags are pushed to GitHub, or when master is pushed
on the playground repository.

For details, see [deploy/go_trigger.yaml](deploy/go_trigger.yaml),
[deploy/playground_trigger.yaml](deploy/playground_trigger.yaml),
and [deploy/deploy.json](deploy/deploy.json).

Changes to the trigger configuration can be made to the YAML files, or in the GCP UI, which should be kept in sync
using the `push-cloudbuild-triggers` and `pull-cloudbuild-triggers` make targets.

### Deploy via Cloud Build

The Cloud Build configuration will always build and deploy with the latest supported release of Go.

```bash
gcloud --project=golang-org builds submit --config deploy/deploy.json .
```

To deploy the "Go tip" version of the playground, which uses the latest
development build, use `deploy_gotip.json` instead:

```bash
gcloud --project=golang-org builds submit --config deploy/deploy_gotip.json .
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
docker tag golang/playground:latest gcr.io/golang-org/playground:latest
docker push gcr.io/golang-org/playground:latest
gcloud --project=golang-org --account=you@google.com app deploy app.yaml --image-url=gcr.io/golang-org/playground:latest
```

Then:

```bash
gcloud --project=golang-org --account=you@google.com app deploy app.yaml
```

## Contributing

To submit changes to this repository, see
https://golang.org/doc/contribute.html.
