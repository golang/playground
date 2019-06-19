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
docker run --name=play --rm -d -p 8080:8080 playground
# run some Go code
cat /path/to/code.go | go run client.go | curl -s --upload-file - localhost:8080/compile
```

## Deployment

Building the playground Docker container takes more than the default 10 minute time limit of cloud build, so increase
its timeout first (note, `app/cloud_build_timeout` is a global configuration value):

```bash
gcloud config set app/cloud_build_timeout 1200  # 20 mins
```

Alternatively, to avoid Cloud Build and build locally:

```bash
make docker
docker tag playground:latest gcr.io/golang-org/playground:latest
docker push gcr.io/golang-org/playground:latest
gcloud --project=golang-org --account=you@google.com app deploy app.yaml --image-url=gcr.io/golang-org/playground:latest
```

Then:

```bash
gcloud --project=golang-org --account=you@google.com app deploy app.yaml
```

### Deployment Triggers

Playground releases are also triggered when new tags are pushed to Github. The Cloud Build trigger configuration is
defined in [cloudbuild_trigger.json](cloudbuild_trigger.json).

Triggers can be updated by running the following Make target:

```bash
make update-cloudbuild-trigger
```

## Contributing

To submit changes to this repository, see
https://golang.org/doc/contribute.html.
