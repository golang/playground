# playground

This subrepository holds the source for the Go playground:
https://play.golang.org/

To submit changes to this repository, see http://golang.org/doc/contribute.html.

## Building

```
# build the image
docker build -t playground .
```

## Running

```
docker run --rm -d -p 8080:8080 playground
# run go some code
cat /path/to/code.go | go run client.go | curl --data @- localhost:8080/compile
```

# Deployment

```
gcloud --project=golang-org --account=person@example.com app deploy app.yaml
```

# Contributing

To submit changes to this repository, see http://golang.org/doc/contribute.html.