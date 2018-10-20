# rerost/playground
This Playground is a Playground that can specify the package of the third party. In addition, GCP-independent deployment is also possible as a data store.

1. You can also use third party things like this
1. It does not require GCP

I support redis. But, If you want to use other storage, You need implement
1. https://github.com/rerost/playground/blob/master/infra/cache/client.go 
1. https://github.com/rerost/playground/blob/master/infra/store/client.go
1. https://github.com/rerost/playground/blob/master/middleware/middleware.go

# playground

This subrepository holds the source for the Go playground:
https://play.golang.org/

## Building

```
# build the image
docker build -t playground .
```

## Running

```
docker run --name=play --rm -d -p 8080:8080 playground
# run some Go code
cat /path/to/code.go | go run client.go | curl -s --upload-file - localhost:8080/compile
```

# Deployment

```
gcloud --project=golang-org --account=person@example.com app deploy app.yaml
```

# Contributing

To submit changes to this repository, see
https://golang.org/doc/contribute.html.
