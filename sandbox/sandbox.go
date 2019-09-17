// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The sandbox program is an HTTP server that receives untrusted
// linux/amd64 in a POST request and then executes them in a gvisor
// sandbox using Docker, returning the output as a response to the
// POST.
//
// It's part of the Go playground (https://play.golang.org/).
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/playground/sandbox/sandboxtypes"
)

var (
	listenAddr = flag.String("listen", ":80", "HTTP server listen address. Only applicable when --mode=server")
	mode       = flag.String("mode", "server", "Whether to run in \"server\" mode or \"contained\" mode. The contained mode is used internally by the server mode.")
	dev        = flag.Bool("dev", false, "run in dev mode (show help messages)")
	numWorkers = flag.Int("workers", runtime.NumCPU(), "number of parallel gvisor containers to pre-spin up & let run concurrently")
)

const (
	maxBinarySize    = 100 << 20
	runTimeout       = 5 * time.Second
	maxOutputSize    = 100 << 20
	memoryLimitBytes = 100 << 20
)

var errTooMuchOutput = errors.New("Output too large")

// containedStartMessage is the first thing written to stdout by the
// contained process when it starts up. This lets the parent HTTP
// server know that a particular container is ready to run a binary.
const containedStartMessage = "started\n"

var (
	readyContainer chan *Container
	runSem         chan struct{}
)

type Container struct {
	name   string
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	cmd    *exec.Cmd

	waitOnce sync.Once
	waitVal  error
}

func (c *Container) Close() {
	setContainerWanted(c.name, false)
	c.stdin.Close()
	c.stdout.Close()
	c.stderr.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.Wait() // just in case
	}
}

func (c *Container) Wait() error {
	c.waitOnce.Do(c.wait)
	return c.waitVal
}

func (c *Container) wait() {
	c.waitVal = c.cmd.Wait()
}

func main() {
	flag.Parse()
	if *mode == "contained" {
		runInGvisor()
		panic("runInGvisor didn't exit")
	}
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}
	log.Printf("Go playground sandbox starting.")

	readyContainer = make(chan *Container, *numWorkers)
	runSem = make(chan struct{}, *numWorkers)
	go makeWorkers()
	go handleSignals()

	if out, err := exec.Command("docker", "version").CombinedOutput(); err != nil {
		log.Fatalf("failed to connect to docker: %v, %s", err, out)
	}
	if *dev {
		log.Printf("Running in dev mode; container published to host at: http://localhost:8080/")
		// TODO: XXXX FIXME: this is no longer the protocol since the addition of the processMeta JSON header,
		// so write a client program to do this instead?
		log.Printf("Run a binary with: curl -v --data-binary @/home/bradfitz/hello http://localhost:8080/run\n")
	} else {
		log.Printf("Listening on %s", *listenAddr)
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/healthz", healthHandler)
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/run", runHandler)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	s := <-c
	log.Fatalf("closing on signal %d: %v", s, s)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK\n")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	io.WriteString(w, "Hi from sandbox\n")
}

// processMeta is the JSON sent to the gvisor container before the untrusted binary.
// It currently contains only the arguments to pass to the binary.
// It might contain environment or other things later.
type processMeta struct {
	Args []string `json:"args"`
}

// runInGvisor is run when we're now inside gvisor. We have no network
// at this point. We can read our binary in from stdin and then run
// it.
func runInGvisor() {
	const binPath = "/tmpfs/play"
	if _, err := io.WriteString(os.Stdout, containedStartMessage); err != nil {
		log.Fatalf("writing to stdout: %v", err)
	}
	slurp, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading stdin in contained mode: %v", err)
	}
	nl := bytes.IndexByte(slurp, '\n')
	if nl == -1 {
		log.Fatalf("no newline found in input")
	}
	metaJSON, bin := slurp[:nl], slurp[nl+1:]

	if err := ioutil.WriteFile(binPath, bin, 0755); err != nil {
		log.Fatalf("writing contained binary: %v", err)
	}

	var meta processMeta
	if err := json.NewDecoder(bytes.NewReader(metaJSON)).Decode(&meta); err != nil {
		log.Fatalf("error decoding JSON meta: %v", err)
	}

	// As part of a temporary transition plan, we also support
	// running nacl binaries in this sandbox. The point isn't to
	// double sandbox things as much as it is to let us transition
	// things in steps: first to split the sandbox into two parts
	// (frontend & backend), and then to change the type of binary
	// (nacl to linux/amd64). This means we can do step 1 of the
	// migration during the Go 1.13 dev cycle and have less
	// risk/rush during the Go 1.14 release, which should just be
	// a flag flip.
	// This isn't a perfect heuristic, but it works and it's cheap:
	isNacl := bytes.Contains(slurp, []byte("_rt0_amd64p32_nacl"))

	cmd := exec.Command(binPath)
	if isNacl {
		cmd = exec.Command("/usr/local/bin/sel_ldr_x86_64", "-l", "/dev/null", "-S", "-e", binPath)
	}
	cmd.Args = append(cmd.Args, meta.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	err = cmd.Wait()
	os.Remove(binPath) // not that it matters much, this container will be nuked
	os.Exit(errExitCode(err))
	return

}

func makeWorkers() {
	for {
		c, err := startContainer(context.Background())
		if err != nil {
			log.Printf("error starting container: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		readyContainer <- c
	}
}

func randHex(n int) string {
	b := make([]byte, n/2)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}

var (
	wantedMu        sync.Mutex
	containerWanted = map[string]bool{}
)

// setContainerWanted records whether a named container is wanted or
// not. Any unwanted containers are cleaned up asynchronously as a
// sanity check against leaks.
//
// TODO(bradfitz): add leak checker (background docker ps loop)
func setContainerWanted(name string, wanted bool) {
	wantedMu.Lock()
	defer wantedMu.Unlock()
	if wanted {
		containerWanted[name] = true
	} else {
		delete(containerWanted, name)
	}
}

func getContainer(ctx context.Context) (*Container, error) {
	select {
	case c := <-readyContainer:
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func startContainer(ctx context.Context) (c *Container, err error) {
	name := "play_run_" + randHex(8)
	setContainerWanted(name, true)
	var stdin io.WriteCloser
	var stdout io.ReadCloser
	var stderr io.ReadCloser
	defer func() {
		if err == nil {
			return
		}
		setContainerWanted(name, false)
		if stdin != nil {
			stdin.Close()
		}
		if stdout != nil {
			stdout.Close()
		}
		if stderr != nil {
			stderr.Close()
		}
	}()

	cmd := exec.Command("docker", "run",
		"--name="+name,
		"--rm",
		"--tmpfs=/tmpfs",
		"-i", // read stdin

		"--runtime=runsc",
		"--network=none",
		"--memory="+fmt.Sprint(memoryLimitBytes),

		"gcr.io/golang-org/playground-sandbox-gvisor:latest",
		"--mode=contained")
	stdin, err = cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err = cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	errc := make(chan error, 1)
	go func() {
		buf := make([]byte, len(containedStartMessage))
		if _, err := io.ReadFull(stdout, buf); err != nil {
			errc <- fmt.Errorf("error reading header from sandbox container: %v", err)
			return
		}
		if string(buf) != containedStartMessage {
			errc <- fmt.Errorf("sandbox container sent wrong header %q; want %q", buf, containedStartMessage)
			return
		}
		errc <- nil
	}()
	select {
	case <-ctx.Done():
		log.Printf("timeout starting container")
		cmd.Process.Kill()
		return nil, ctx.Err()
	case err := <-errc:
		if err != nil {
			log.Printf("error starting container: %v", err)
			return nil, err
		}
	}
	return &Container{
		name:   name,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		cmd:    cmd,
	}, nil
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	tlast := t0
	var logmu sync.Mutex
	logf := func(format string, args ...interface{}) {
		if !*dev {
			return
		}
		logmu.Lock()
		defer logmu.Unlock()
		t := time.Now()
		d := t.Sub(tlast)
		d0 := t.Sub(t0)
		tlast = t
		log.Print(fmt.Sprintf("+%10v +%10v ", d0, d) + fmt.Sprintf(format, args...))
	}
	logf("/run")

	if r.Method != "POST" {
		http.Error(w, "expected a POST", http.StatusBadRequest)
		return
	}

	// Bound the number of requests being processed at once.
	// (Before we slurp the binary into memory)
	select {
	case runSem <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	defer func() { <-runSem }()

	bin, err := ioutil.ReadAll(http.MaxBytesReader(w, r.Body, maxBinarySize))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logf("read %d bytes", len(bin))

	c, err := getContainer(r.Context())
	if err != nil {
		if cerr := r.Context().Err(); cerr != nil {
			log.Printf("getContainer, client side cancellation: %v", cerr)
			return
		}
		http.Error(w, "failed to get container", http.StatusInternalServerError)
		log.Printf("failed to get container: %v", err)
		return
	}
	logf("got container %s", c.name)
	defer c.Close()
	defer logf("leaving handler; about to close container")

	runTimer := time.NewTimer(runTimeout)
	defer runTimer.Stop()

	errc := make(chan error, 2) // user-visible error
	waitc := make(chan error, 1)

	copyOut := func(which string, dst *[]byte, r io.Reader) {
		buf := make([]byte, 4<<10)
		for {
			n, err := r.Read(buf)
			logf("%s: Read = %v, %v", which, n, err)
			*dst = append(*dst, buf[:n]...)
			if err == io.EOF {
				return
			}
			if len(*dst) > maxOutputSize {
				errc <- errTooMuchOutput
				return
			}
			if err != nil {
				log.Printf("reading %s: %v", which, err)
				errc <- fmt.Errorf("error reading %v", which)
				return
			}
		}
	}

	res := &sandboxtypes.Response{}
	go func() {
		var meta processMeta
		meta.Args = r.Header["X-Argument"]
		metaJSON, _ := json.Marshal(&meta)
		metaJSON = append(metaJSON, '\n')
		if _, err := c.stdin.Write(metaJSON); err != nil {
			log.Printf("stdin write meta: %v", err)
			errc <- errors.New("failed to write meta to child")
			return
		}
		if _, err := c.stdin.Write(bin); err != nil {
			log.Printf("stdin write: %v", err)
			errc <- errors.New("failed to write binary to child")
			return
		}
		c.stdin.Close()
		logf("wrote+closed")
		go copyOut("stdout", &res.Stdout, c.stdout)
		go copyOut("stderr", &res.Stderr, c.stderr)
		waitc <- c.Wait()
	}()
	var waitErr error
	select {
	case waitErr = <-waitc:
		logf("waited: %v", waitErr)
	case err := <-errc:
		logf("got error: %v", err)
		if err == errTooMuchOutput {
			sendError(w, err.Error())
			return
		}
		if err != nil {
			http.Error(w, "failed to read stdout from docker run", http.StatusInternalServerError)
			return
		}
	case <-runTimer.C:
		logf("timeout")
		sendError(w, "timeout running program")
		return
	}

	res.ExitCode = errExitCode(waitErr)
	res.Stderr = cleanStderr(res.Stderr)
	sendResponse(w, res)
}

func errExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

func sendError(w http.ResponseWriter, errMsg string) {
	sendResponse(w, &sandboxtypes.Response{Error: errMsg})
}

func sendResponse(w http.ResponseWriter, r *sandboxtypes.Response) {
	jres, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		http.Error(w, "error encoding JSON", http.StatusInternalServerError)
		log.Printf("json marshal: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprint(len(jres)))
	w.Write(jres)
}

// cleanStderr removes spam stderr lines from the beginning of x
// and returns a slice of x.
func cleanStderr(x []byte) []byte {
	for {
		nl := bytes.IndexByte(x, '\n')
		if nl == -1 || !isSpamStderrLine(x[:nl+1]) {
			return x
		}
		x = x[nl+1:]
	}
}

var warningPrefix = []byte("WARNING: ")

// isSpamStderrLine reports whether line is a spammy line of stderr
// output from Docker. Currently it only matches things starting with
// "WARNING: " like:
//     WARNING: Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.
//
// TODO: remove this and instead just make the child process start by
// writing a known header to stderr, then have parent skip everything
// before that unique header.
func isSpamStderrLine(line []byte) bool {
	return bytes.HasPrefix(line, warningPrefix)
}
