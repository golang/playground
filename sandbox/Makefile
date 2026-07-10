ZONE := us-central1-f
TEST_VM := gvisor-cos-test-vm
PROJ := golang-org
NETWORK := golang
TAG ?= test-cos

# Docker environment for the sandbox server itself (containing docker CLI, etc), running
# in a privileged container.
docker:
	docker build -f Dockerfile --tag=golang/playground-sandbox ..
	docker tag golang/playground-sandbox golang/playground-sandbox:$(TAG)
	docker tag golang/playground-sandbox gcr.io/$(PROJ)/playground-sandbox:$(TAG)

# dockergvisor builds the golang/playground-sandbox-gvisor docker
# image, which is the environment that the untrusted programs run in
# (a busybox:glibc world with this directory's sandbox binary which
# runs in --mode=contained)
dockergvisor:
	docker build -f Dockerfile.gvisor --tag=golang/playground-sandbox-gvisor ..
	docker tag golang/playground-sandbox-gvisor golang/playground-sandbox-gvisor:$(TAG)
	docker tag golang/playground-sandbox-gvisor gcr.io/$(PROJ)/playground-sandbox-gvisor:$(TAG)

push: docker dockergvisor
	docker push gcr.io/$(PROJ)/playground-sandbox:$(TAG)
	docker push gcr.io/$(PROJ)/playground-sandbox-gvisor:$(TAG)

# runlocal runs the sandbox server locally, for use with the frontend
# parent directory's "test_nacl" or "test_gvisor" test targets.
runlocal: docker dockergvisor
	docker network create sandnet || true
	docker kill sandbox_dev || true
	docker run --name=sandbox_dev --rm --network=sandnet -ti -p 127.0.0.1:8080:80/tcp -v /var/run/docker.sock:/var/run/docker.sock golang/playground-sandbox:$(TAG) --dev --container=golang/playground-sandbox-gvisor:$(TAG)

cloud-init.yaml.expanded: cloud-init.yaml
	sed -e "s/PROJECT_NAME/$(PROJ)/g" -e "s/TAG_NAME/$(TAG)/g" cloud-init.yaml > cloud-init.yaml.expanded

# create_test_vm creates a test VM for interactive debugging.
create_test_vm: cloud-init.yaml.expanded
	gcloud --project=$(PROJ) compute instances create $(TEST_VM) \
	--zone $(ZONE) \
	--network $(NETWORK) \
	--no-address \
	--image-project cos-cloud \
	--image-family cos-stable \
	--metadata-from-file user-data=cloud-init.yaml.expanded

# delete_test_vm deletes the test VM from create_test_vm.
delete_test_vm:
	gcloud --project=$(PROJ) compute instances delete $(TEST_VM) --quiet --zone $(ZONE)

# ssh connects to the create_test_vm VM. It must be run from the same network.
ssh:
	gcloud --project=$(PROJ) compute ssh $(TEST_VM) --internal-ip --zone $(ZONE)

.PHONY: docker dockergvisor push cloud-init.yaml.expanded create_test_vm delete_test_vm ssh


