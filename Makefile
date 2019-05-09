.PHONY: update-deps docker test

docker: Dockerfile
	docker build -t playground .

test: docker
	go test
	docker run --rm playground test
