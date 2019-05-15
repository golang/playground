.PHONY: docker test

docker:
	docker build -t golang/playground .

test:
	# Run fast tests first: (and tests whether, say, things compile)
	GO111MODULE=on go test -v
	# Then run the slower tests, which happen as one of the
	# Dockerfile RUN steps:
	docker build -t golang/playground .
