CLOUDBUILD_PLAYGROUND_TRIGGER_JSON := deploy/playground_trigger.json
CLOUDBUILD_PLAYGROUND_TRIGGER_ID := $(shell jq -r .id ${CLOUDBUILD_PLAYGROUND_TRIGGER_JSON})
CLOUDBUILD_GO_TRIGGER_JSON := deploy/go_trigger.json
CLOUDBUILD_GO_TRIGGER_ID := $(shell jq -r .id ${CLOUDBUILD_GO_TRIGGER_JSON})
GCLOUD_ACCESS_TOKEN := $(shell gcloud auth print-access-token)

.PHONY: docker test update-cloudbuild-trigger

docker:
	docker build -t golang/playground .

test:
	# Run fast tests first: (and tests whether, say, things compile)
	GO111MODULE=on go test -v
	# Then run the slower tests, which happen as one of the
	# Dockerfile RUN steps:
	docker build -t golang/playground .

update-cloudbuild-trigger:
	# The gcloud CLI doesn't yet support updating a trigger.
	curl -H "Authorization: Bearer $(GCLOUD_ACCESS_TOKEN)" -H "Content-Type: application/json" \
		-d @$(CLOUDBUILD_GO_TRIGGER_JSON) \
		-X PATCH https://cloudbuild.googleapis.com/v1/projects/golang-org/triggers/$(CLOUDBUILD_GO_TRIGGER_ID)
	curl -H "Authorization: Bearer $(GCLOUD_ACCESS_TOKEN)" -H "Content-Type: application/json" \
		-d @$(CLOUDBUILD_PLAYGROUND_TRIGGER_JSON) \
		-X PATCH https://cloudbuild.googleapis.com/v1/projects/golang-org/triggers/$(CLOUDBUILD_PLAYGROUND_TRIGGER_ID)
