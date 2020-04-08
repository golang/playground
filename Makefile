CLOUDBUILD_PLAYGROUND_TRIGGER_JSON := deploy/playground_trigger.json
CLOUDBUILD_PLAYGROUND_TRIGGER_ID := $(shell jq -r .id ${CLOUDBUILD_PLAYGROUND_TRIGGER_JSON})
CLOUDBUILD_GO_TRIGGER_JSON := deploy/go_trigger.json
CLOUDBUILD_GO_TRIGGER_ID := $(shell jq -r .id ${CLOUDBUILD_GO_TRIGGER_JSON})
GCLOUD_ACCESS_TOKEN := $(shell gcloud auth print-access-token)

.PHONY: docker test update-cloudbuild-trigger

docker:
	docker build -t golang/playground .

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

update-cloudbuild-trigger:
	# The gcloud CLI doesn't yet support updating a trigger.
	curl -H "Authorization: Bearer $(GCLOUD_ACCESS_TOKEN)" -H "Content-Type: application/json" \
		-d @$(CLOUDBUILD_GO_TRIGGER_JSON) \
		-X PATCH https://cloudbuild.googleapis.com/v1/projects/golang-org/triggers/$(CLOUDBUILD_GO_TRIGGER_ID)
	curl -H "Authorization: Bearer $(GCLOUD_ACCESS_TOKEN)" -H "Content-Type: application/json" \
		-d @$(CLOUDBUILD_PLAYGROUND_TRIGGER_JSON) \
		-X PATCH https://cloudbuild.googleapis.com/v1/projects/golang-org/triggers/$(CLOUDBUILD_PLAYGROUND_TRIGGER_ID)
