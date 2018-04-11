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
