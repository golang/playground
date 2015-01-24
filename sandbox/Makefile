docker: Dockerfile
	docker build -t playground/sandbox .

test: docker
	go test
	docker run --rm playground/sandbox test
