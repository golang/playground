CLOUDBUILD_TRIGGER_ID := $(shell jq -r .id cloudbuild_trigger.json)
CLOUDBUILD_TRIGGER_JSON := cloudbuild_trigger.json
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
		-d @$(CLOUDBUILD_TRIGGER_JSON) \
		-X PATCH https://cloudbuild.googleapis.com/v1/projects/golang-org/triggers/$(CLOUDBUILD_TRIGGER_ID)
