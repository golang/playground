LATEST_GO := $(shell go run ./cmd/latestgo)

.PHONY: docker test update-cloudbuild-trigger

docker:
	docker build --build-arg GO_VERSION=$(LATEST_GO) -t golang/playground .

runlocal:
	docker network create sandnet || true
	docker kill play_dev || true
	docker run --name=play_dev --rm --network=sandnet -ti -p 127.0.0.1:8081:8080/tcp golang/playground --backend-url="http://sandbox_dev.sandnet/run"

test_go:
	# Run fast tests first: (and tests whether, say, things compile)
	GO111MODULE=on go test -v ./...

test_gvisor: docker
	docker kill sandbox_front_test || true
	docker run --rm --name=sandbox_front_test --network=sandnet -t golang/playground --runtests

# Note: test_gvisor is not included in "test" yet, because it requires
# running a separate server first ("make runlocal" in the sandbox
# directory)
test: test_go

define GOTIP_MESSAGE
Note: deploy/gotip_schedule.yaml must be manually managed with gcloud.
Example:
	gcloud scheduler jobs update http \
	projects/golang-org/locations/us-central1/jobs/playground-deploy-gotip-playground-schedule \
	--schedule="0 10 * * *" --time-zone="America/New_York"
endef
export GOTIP_MESSAGE

push-cloudbuild-triggers:
	gcloud beta builds triggers import --project golang-org --source deploy/go_trigger.yaml
	gcloud beta builds triggers import --project golang-org --source deploy/go_goprev_trigger.yaml
	gcloud beta builds triggers import --project golang-org --source deploy/playground_trigger.yaml
	gcloud beta builds triggers import --project golang-org --source deploy/playground_goprev_trigger.yaml
	gcloud alpha builds triggers import --project golang-org --source deploy/gotip_scheduled_trigger.yaml
	@echo "$$GOTIP_MESSAGE"

pull-cloudbuild-triggers:
	gcloud beta builds triggers export --project golang-org playground-redeploy-go-release --destination deploy/go_trigger.yaml
	gcloud beta builds triggers export --project golang-org playground-redeploy-goprev-go-release --destination deploy/go_goprev_trigger.yaml
	gcloud beta builds triggers export --project golang-org playground-redeploy-playground --destination deploy/playground_trigger.yaml
	gcloud beta builds triggers export --project golang-org playground-redeploy-goprev-playground --destination deploy/playground_goprev_trigger.yaml
	gcloud alpha builds triggers export --project golang-org playground-deploy-gotip-playground --destination deploy/gotip_scheduled_trigger.yaml
	gcloud scheduler --project=golang-org jobs describe --format=yaml playground-deploy-gotip-playground-schedule > deploy/gotip_schedule.yaml
