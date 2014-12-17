# playground

This subrepository holds the source for various packages and tools that support
the Go playground: https://play.golang.org/

## building

```
# build the golang:nacl base image
docker build -t golang:nacl gonacl/
# build the sandbox server image
docker build -t sandbox sandbox/
```

## running

```
# run the sandbox
docker run -d -p 8080:8080 sandbox
# get docker host ip, try boot2docker fallback on localhost.
DOCKER_HOST_IP=$(boot2docker ip || echo localhost)
# run go some code
cat /path/to/code.go | go run ./sandbox/client.go | curl --data @- $DOCKER_HOST_IP:8080/compile
```

To submit changes to this repository, see http://golang.org/doc/contribute.html.
