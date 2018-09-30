.PHONY: update-deps docker test

update-deps:
	go install golang.org/x/build/cmd/gitlock
	gitlock --update=Dockerfile golang.org/x/playground

docker: Dockerfile
	docker build -t playground .

test: docker
	go test
	docker run --rm playground test

.PHONY:run
run:
	docker stop play
	docker rm play
	docker run --name=play -d -p 1234:8080 playground
