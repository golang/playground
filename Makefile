.PHONY: docker test update-cloudbuild-trigger

docker:
	docker build -t golang/playground-go2go .

runlocal: docker
	docker network create sandnet || true
	docker kill play_dev || true
	docker run --name=play_dev --rm --network=sandnet -ti -p 127.0.0.1:8081:8080/tcp golang/playground-go2go --backend-url="http://sandbox_dev.sandnet/run"

test_go:
	# Run fast tests first: (and tests whether, say, things compile)
	GO111MODULE=on go test -v ./...

test_gvisor: docker
	docker kill sandbox_front_test || true
	docker run --rm --name=sandbox_front_test --network=sandnet -t golang/playground-go2go --runtests

# Note: test_gvisor is not included in "test" yet, because it requires
# running a separate server first ("make runlocal" in the sandbox
# directory)
test: test_go

push-cloudbuild-triggers:
	gcloud beta builds triggers import --project golang-org --source deploy/go2goplay_trigger.yaml

pull-cloudbuild-triggers:
	gcloud beta builds triggers export --project golang-org go2go-redeploy-playground --destination deploy/go2goplay_trigger.yaml
