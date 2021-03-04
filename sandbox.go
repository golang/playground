// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(andybons): add logging
// TODO(andybons): restrict memory use

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"cloud.google.com/go/compute/metadata"
	"github.com/bradfitz/gomemcache/memcache"
	"golang.org/x/playground/internal"
	"golang.org/x/playground/internal/gcpdial"
	"golang.org/x/playground/sandbox/sandboxtypes"
)

const (
	maxCompileTime = 5 * time.Second
	maxRunTime     = 5 * time.Second

	// progName is the implicit program name written to the temp
	// dir and used in compiler and vet errors.
	progName = "prog.go"
)

const (
	go2goTranslateTimeoutError = "timeout running go2go translate"
	goBuildTimeoutError        = "timeout running go build"
	runTimeoutError            = "timeout running program"
)

// internalErrors are strings found in responses that will not be cached
// due to their non-deterministic nature.
var internalErrors = []string{
	"out of memory",
	"cannot allocate memory",
}

type request struct {
	Body    string
	WithVet bool // whether client supports vet response in a /compile request (Issue 31970)
}

type response struct {
	Errors      string
	Events      []Event
	Status      int
	IsTest      bool
	TestsFailed int

	// VetErrors, if non-empty, contains any vet errors. It is
	// only populated if request.WithVet was true.
	VetErrors string `json:",omitempty"`
	// VetOK reports whether vet ran & passsed. It is only
	// populated if request.WithVet was true. Only one of
	// VetErrors or VetOK can be non-zero.
	VetOK bool `json:",omitempty"`
}

// commandHandler returns an http.HandlerFunc.
// This handler creates a *request, assigning the "Body" field a value
// from the "body" form parameter or from the HTTP request body.
// If there is no cached *response for the combination of cachePrefix and request.Body,
// handler calls cmdFunc and in case of a nil error, stores the value of *response in the cache.
// The handler returned supports Cross-Origin Resource Sharing (CORS) from any domain.
func (s *server) commandHandler(cachePrefix string, cmdFunc func(context.Context, *request) (*response, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cachePrefix := cachePrefix // so we can modify it below
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == "OPTIONS" {
			// This is likely a pre-flight CORS request.
			return
		}

		var req request
		// Until programs that depend on golang.org/x/tools/godoc/static/playground.js
		// are updated to always send JSON, this check is in place.
		if b := r.FormValue("body"); b != "" {
			req.Body = b
			req.WithVet, _ = strconv.ParseBool(r.FormValue("withVet"))
		} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.log.Errorf("error decoding request: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if req.WithVet {
			cachePrefix += "_vet" // "prog" -> "prog_vet"
		}

		resp := &response{}
		key := cacheKey(cachePrefix, req.Body)
		if err := s.cache.Get(key, resp); err != nil {
			if !errors.Is(err, memcache.ErrCacheMiss) {
				s.log.Errorf("s.cache.Get(%q, &response): %v", key, err)
			}
			resp, err = cmdFunc(r.Context(), &req)
			if err != nil {
				s.log.Errorf("cmdFunc error: %v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if strings.Contains(resp.Errors, goBuildTimeoutError) || strings.Contains(resp.Errors, runTimeoutError) {
				// TODO(golang.org/issue/38576) - This should be a http.StatusBadRequest,
				// but the UI requires a 200 to parse the response. It's difficult to know
				// if we've timed out because of an error in the code snippet, or instability
				// on the playground itself. Either way, we should try to show the user the
				// partial output of their program.
				s.writeResponse(w, resp, http.StatusOK)
				return
			}
			for _, e := range internalErrors {
				if strings.Contains(resp.Errors, e) {
					s.log.Errorf("cmdFunc compilation error: %q", resp.Errors)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			for _, el := range resp.Events {
				if el.Kind != "stderr" {
					continue
				}
				for _, e := range internalErrors {
					if strings.Contains(el.Message, e) {
						s.log.Errorf("cmdFunc runtime error: %q", el.Message)
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
				}
			}
			if err := s.cache.Set(key, resp); err != nil {
				s.log.Errorf("cache.Set(%q, resp): %v", key, err)
			}
		}

		s.writeResponse(w, resp, http.StatusOK)
	}
}

func (s *server) writeResponse(w http.ResponseWriter, resp *response, status int) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		s.log.Errorf("error encoding response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	if _, err := io.Copy(w, &buf); err != nil {
		s.log.Errorf("io.Copy(w, &buf): %v", err)
		return
	}
}

func cacheKey(prefix, body string) string {
	h := sha256.New()
	io.WriteString(h, body)
	return fmt.Sprintf("%s-%s-%x", prefix, runtime.Version(), h.Sum(nil))
}

// isTest tells whether name looks like a test (or benchmark, according to prefix).
// It is a Test (say) if there is a character after Test that is not a lower-case letter.
// We don't want mistaken Testimony or erroneous Benchmarking.
func isTest(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) { // "Test" is ok
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

var failedTestPattern = "--- FAIL"

// compileAndRun tries to build and run a user program.
// The output of successfully ran program is returned in *response.Events.
// If a program cannot be built or has timed out,
// *response.Errors contains an explanation for a user.
func compileAndRun(ctx context.Context, req *request) (*response, error) {
	// TODO(andybons): Add semaphore to limit number of running programs at once.
	tmpDir, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	br, err := sandboxBuild(ctx, tmpDir, []byte(req.Body), req.WithVet)
	if err != nil {
		return nil, err
	}
	if br.errorMessage != "" {
		return &response{Errors: br.errorMessage}, nil
	}

	execRes, err := sandboxRun(ctx, br.exePath, br.testParam)
	if err != nil {
		return nil, err
	}
	if execRes.Error != "" {
		return &response{Errors: execRes.Error}, nil
	}

	rec := new(Recorder)
	rec.Stdout().Write(execRes.Stdout)
	rec.Stderr().Write(execRes.Stderr)
	events, err := rec.Events()
	if err != nil {
		log.Printf("error decoding events: %v", err)
		return nil, fmt.Errorf("error decoding events: %v", err)
	}
	var fails int
	if br.testParam != "" {
		// In case of testing the TestsFailed field contains how many tests have failed.
		for _, e := range events {
			fails += strings.Count(e.Message, failedTestPattern)
		}
	}
	return &response{
		Events:      events,
		Status:      execRes.ExitCode,
		IsTest:      br.testParam != "",
		TestsFailed: fails,
		VetErrors:   br.vetOut,
		VetOK:       req.WithVet && br.vetOut == "",
	}, nil
}

// buildResult is the output of a sandbox build attempt.
type buildResult struct {
	// goPath is a temporary directory if the binary was built with module support.
	// TODO(golang.org/issue/25224) - Why is the module mode built so differently?
	goPath string
	// exePath is the path to the built binary.
	exePath string
	// useModules is true if the binary was built with module support.
	useModules bool
	// testParam is set if tests should be run when running the binary.
	testParam string
	// errorMessage is an error message string to be returned to the user.
	errorMessage string
	// vetOut is the output of go vet, if requested.
	vetOut string
}

// cleanup cleans up the temporary goPath created when building with module support.
func (b *buildResult) cleanup() error {
	if b.useModules && b.goPath != "" {
		return os.RemoveAll(b.goPath)
	}
	return nil
}

// sandboxBuild builds a Go program and returns a build result that includes the build context.
//
// An error is returned if a non-user-correctable error has occurred.
//
// TODO(golang.org/issue/39675): for initial simplicity, the go2goplay instance currently supports
// only single-file, main-package programs.
func sandboxBuild(ctx context.Context, tmpDir string, in []byte, vet bool) (*buildResult, error) {
	files, err := splitFiles(in)
	if err != nil {
		return &buildResult{errorMessage: err.Error()}, nil
	}

	br := new(buildResult)
	defer br.cleanup()
	var buildPkgArg = "."

	br.useModules = allowModuleDownloads(files)
	if !files.Contains("go.mod") && br.useModules {
		files.AddFile("go.mod", []byte("module play\n"))
	}

	for f, src := range files.m {
		fset := token.NewFileSet()
		pf, err := parser.ParseFile(fset, f, src, parser.PackageClauseOnly)
		if err == nil && pf.Name.Name != "main" {
			return &buildResult{errorMessage: "package name must be main"}, nil
		}

		if filepath.Ext(f) == ".go" {
			f = f + "2" // .go --> .go2 since go2go requires the extension to be .go2
		}
		in := filepath.Join(tmpDir, f)
		if err := ioutil.WriteFile(in, src, 0644); err != nil {
			return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
		}

		if filepath.Ext(in) != ".go2" {
			continue
		}
		cmd := exec.Command("/usr/local/go-faketime/pkg/tool/linux_amd64/go2go", "translate", in)
		cmd.Dir = filepath.Dir(in)
		out := &bytes.Buffer{}
		cmd.Stderr, cmd.Stdout = out, out
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("error starting go2go translate: %v", err)
		}
		ctx, cancel := context.WithTimeout(ctx, maxCompileTime)
		defer cancel()
		if err := internal.WaitOrStop(ctx, cmd, os.Interrupt, 250*time.Millisecond); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				br.errorMessage = fmt.Sprintln(go2goTranslateTimeoutError)
			} else if ee := (*exec.ExitError)(nil); !errors.As(err, &ee) {
				log.Printf("error go2go translating program: %v", err)
				return nil, fmt.Errorf("error translating go2go source: %v", err)
			}
			br.errorMessage = br.errorMessage + strings.Replace(out.String(), tmpDir+"/", "", -1)
			return br, nil
		}
	}

	br.exePath = filepath.Join(tmpDir, "a.out")
	goCache := filepath.Join(tmpDir, "gocache")

	cmd := exec.Command("/usr/local/go-faketime/bin/go", "build", "-o", br.exePath, "-tags=faketime")
	cmd.Dir = tmpDir
	cmd.Env = []string{"GOOS=linux", "GOARCH=amd64", "GOROOT=/usr/local/go-faketime"}
	cmd.Env = append(cmd.Env, "GOCACHE="+goCache)
	if br.useModules {
		// Create a GOPATH just for modules to be downloaded
		// into GOPATH/pkg/mod.
		cmd.Args = append(cmd.Args, "-modcacherw")
		br.goPath, err = ioutil.TempDir("", "gopath")
		if err != nil {
			log.Printf("error creating temp directory: %v", err)
			return nil, fmt.Errorf("error creating temp directory: %v", err)
		}
		cmd.Env = append(cmd.Env, "GO111MODULE=on", "GOPROXY="+playgroundGoproxy())
	} else {
		br.goPath = os.Getenv("GOPATH")              // contains old code.google.com/p/go-tour, etc
		cmd.Env = append(cmd.Env, "GO111MODULE=off") // in case it becomes on by default later
	}
	cmd.Args = append(cmd.Args, buildPkgArg)
	cmd.Env = append(cmd.Env, "GOPATH="+br.goPath)
	out := &bytes.Buffer{}
	cmd.Stderr, cmd.Stdout = out, out

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting go build: %v", err)
	}
	ctx, cancel := context.WithTimeout(ctx, maxCompileTime)
	defer cancel()
	if err := internal.WaitOrStop(ctx, cmd, os.Interrupt, 250*time.Millisecond); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			br.errorMessage = fmt.Sprintln(goBuildTimeoutError)
		} else if ee := (*exec.ExitError)(nil); !errors.As(err, &ee) {
			log.Printf("error building program: %v", err)
			return nil, fmt.Errorf("error building go source: %v", err)
		}
		// Return compile errors to the user.
		// Rewrite compiler errors to strip the tmpDir name.
		br.errorMessage = br.errorMessage + strings.Replace(out.String(), tmpDir+"/", "", -1)

		// "go build", invoked with a file name, puts this odd
		// message before any compile errors; strip it.
		br.errorMessage = strings.Replace(br.errorMessage, "# command-line-arguments\n", "", 1)

		return br, nil
	}
	const maxBinarySize = 100 << 20 // copied from sandbox backend; TODO: unify?
	if fi, err := os.Stat(br.exePath); err != nil || fi.Size() == 0 || fi.Size() > maxBinarySize {
		if err != nil {
			return nil, fmt.Errorf("failed to stat binary: %v", err)
		}
		return nil, fmt.Errorf("invalid binary size %d", fi.Size())
	}
	if vet {
		// TODO: do this concurrently with the execution to reduce latency.
		br.vetOut, err = vetCheckInDir(tmpDir, br.goPath, br.useModules)
		if err != nil {
			return nil, fmt.Errorf("running vet: %v", err)
		}
	}
	return br, nil
}

// sandboxRun runs a Go binary in a sandbox environment.
func sandboxRun(ctx context.Context, exePath string, testParam string) (sandboxtypes.Response, error) {
	var execRes sandboxtypes.Response
	exeBytes, err := ioutil.ReadFile(exePath)
	if err != nil {
		return execRes, err
	}
	ctx, cancel := context.WithTimeout(ctx, maxRunTime)
	defer cancel()
	sreq, err := http.NewRequestWithContext(ctx, "POST", sandboxBackendURL(), bytes.NewReader(exeBytes))
	if err != nil {
		return execRes, fmt.Errorf("NewRequestWithContext %q: %w", sandboxBackendURL(), err)
	}
	sreq.Header.Add("Idempotency-Key", "1") // lets Transport do retries with a POST
	if testParam != "" {
		sreq.Header.Add("X-Argument", testParam)
	}
	sreq.GetBody = func() (io.ReadCloser, error) { return ioutil.NopCloser(bytes.NewReader(exeBytes)), nil }
	res, err := sandboxBackendClient().Do(sreq)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			execRes.Error = runTimeoutError
			return execRes, nil
		}
		return execRes, fmt.Errorf("POST %q: %w", sandboxBackendURL(), err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Printf("unexpected response from backend: %v", res.Status)
		return execRes, fmt.Errorf("unexpected response from backend: %v", res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(&execRes); err != nil {
		log.Printf("JSON decode error from backend: %v", err)
		return execRes, errors.New("error parsing JSON from backend")
	}
	return execRes, nil
}

// allowModuleDownloads reports whether the code snippet in src should
// be allowed to download modules.
func allowModuleDownloads(_ *fileSet) bool {
	// Disabled for go2go.
	return false
}

// playgroundGoproxy returns the GOPROXY environment config the playground should use.
// It is fetched from the environment variable PLAY_GOPROXY. A missing or empty
// value for PLAY_GOPROXY returns the default value of https://proxy.golang.org.
func playgroundGoproxy() string {
	proxypath := os.Getenv("PLAY_GOPROXY")
	if proxypath != "" {
		return proxypath
	}
	return "https://proxy.golang.org"
}

// healthCheck attempts to build a binary from the source in healthProg.
// It returns any error returned from sandboxBuild, or nil if none is returned.
func (s *server) healthCheck(ctx context.Context) error {
	tmpDir, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	br, err := sandboxBuild(ctx, tmpDir, []byte(healthProg), false)
	if err != nil {
		return err
	}
	if br.errorMessage != "" {
		return errors.New(br.errorMessage)
	}
	return nil
}

// sandboxBackendURL returns the URL of the sandbox backend that
// executes binaries. This backend is required for Go 1.14+ (where it
// executes using gvisor, since Native Client support is removed).
//
// This function either returns a non-empty string or it panics.
func sandboxBackendURL() string {
	if v := os.Getenv("SANDBOX_BACKEND_URL"); v != "" {
		return v
	}
	id, _ := metadata.ProjectID()
	switch id {
	case "golang-org":
		return "http://sandbox.play-sandbox-fwd.il4.us-central1.lb.golang-org.internal/run"
	}
	panic(fmt.Sprintf("no SANDBOX_BACKEND_URL environment and no default defined for project %q", id))
}

var sandboxBackendOnce struct {
	sync.Once
	c *http.Client
}

func sandboxBackendClient() *http.Client {
	sandboxBackendOnce.Do(initSandboxBackendClient)
	return sandboxBackendOnce.c
}

// initSandboxBackendClient runs from a sync.Once and initializes
// sandboxBackendOnce.c with the *http.Client we'll use to contact the
// sandbox execution backend.
func initSandboxBackendClient() {
	id, _ := metadata.ProjectID()
	switch id {
	case "golang-org":
		// For production, use a funky Transport dialer that
		// contacts backend directly, without going through an
		// internal load balancer, due to internal GCP
		// reasons, which we might resolve later. This might
		// be a temporary hack.
		tr := http.DefaultTransport.(*http.Transport).Clone()
		rigd := gcpdial.NewRegionInstanceGroupDialer("golang-org", "us-central1", "play-sandbox-rigm")
		tr.DialContext = func(ctx context.Context, netw, addr string) (net.Conn, error) {
			if addr == "sandbox.play-sandbox-fwd.il4.us-central1.lb.golang-org.internal:80" {
				ip, err := rigd.PickIP(ctx)
				if err != nil {
					return nil, err
				}
				addr = net.JoinHostPort(ip, "80") // and fallthrough
			}
			var d net.Dialer
			return d.DialContext(ctx, netw, addr)
		}
		sandboxBackendOnce.c = &http.Client{Transport: tr}
	default:
		sandboxBackendOnce.c = http.DefaultClient
	}
}

const healthProg = `
package main

import "fmt"

func main() { fmt.Print("ok") }
`
