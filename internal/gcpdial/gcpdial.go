// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gcpdial monitors VM instance groups to let frontends dial
// them directly without going through an internal load balancer.
package gcpdial

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/compute/v1"
)

type Dialer struct {
	lister instanceLister

	mu            sync.Mutex
	lastInstances []string           // URLs of instances
	prober        map[string]*prober // URL of instance to its prober
	ready         map[string]string  // URL of instance to ready IP
}

type prober struct {
	d       *Dialer
	instURL string
	cancel  func()          // called by Dialer to shut down this dialer
	ctx     context.Context // context that's canceled from above

	pi *parsedInstance

	// owned by the probeLoop goroutine:
	ip      string
	healthy bool
}

func newProber(d *Dialer, instURL string) *prober {
	ctx, cancel := context.WithCancel(context.Background())
	return &prober{
		d:       d,
		instURL: instURL,
		cancel:  cancel,
		ctx:     ctx,
	}
}

func (p *prober) probeLoop() {
	log.Printf("start prober for %s", p.instURL)
	defer log.Printf("end prober for %s", p.instURL)

	pi, err := parseInstance(p.instURL)
	if err != nil {
		log.Printf("gcpdial: prober %s: failed to parse: %v", p.instURL, err)
		return
	}
	p.pi = pi

	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		p.probe()
		select {
		case <-p.ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (p *prober) probe() {
	if p.ip == "" && !p.getIP() {
		return
	}
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequest("GET", "http://"+p.ip+"/healthz", nil)
	if err != nil {
		log.Printf("gcpdial: prober %s: NewRequest: %v", p.instURL, err)
		return
	}
	req = req.WithContext(ctx)
	res, err := http.DefaultClient.Do(req)
	if res != nil {
		defer res.Body.Close()
		defer io.Copy(ioutil.Discard, res.Body)
	}
	healthy := err == nil && res.StatusCode == http.StatusOK
	if healthy == p.healthy {
		// No change.
		return
	}
	p.healthy = healthy

	p.d.mu.Lock()
	defer p.d.mu.Unlock()
	if healthy {
		if p.d.ready == nil {
			p.d.ready = map[string]string{}
		}
		p.d.ready[p.instURL] = p.ip
		// TODO: possible optimization: trigger
		// Dialer.PickIP waiters to wake up rather
		// than them polling once a second.
	} else {
		delete(p.d.ready, p.instURL)
		var why string
		if err != nil {
			why = err.Error()
		} else {
			why = res.Status
		}
		log.Printf("gcpdial: prober %s: no longer healthy; %v", p.instURL, why)
	}
}

// getIP populates p.ip and reports whether it did so.
func (p *prober) getIP() bool {
	if p.ip != "" {
		return true
	}
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()
	svc, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("gcpdial: prober %s: NewService: %v", p.instURL, err)
		return false
	}
	inst, err := svc.Instances.Get(p.pi.Project, p.pi.Zone, p.pi.Name).Context(ctx).Do()
	if err != nil {
		log.Printf("gcpdial: prober %s: Get: %v", p.instURL, err)
		return false
	}
	var ip string
	var other []string
	for _, ni := range inst.NetworkInterfaces {
		if strings.HasPrefix(ni.NetworkIP, "10.") {
			ip = ni.NetworkIP
		} else {
			other = append(other, ni.NetworkIP)
		}
	}
	if ip == "" {
		log.Printf("gcpdial: prober %s: didn't find 10.x.x.x IP; found %q", p.instURL, other)
		return false
	}
	p.ip = ip
	return true
}

// PickIP returns a randomly healthy IP, waiting until one is available, or until ctx expires.
func (d *Dialer) PickIP(ctx context.Context) (ip string, err error) {
	for {
		if ip, ok := d.pickIP(); ok {
			return ip, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (d *Dialer) pickIP() (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.ready) == 0 {
		return "", false
	}
	num := rand.Intn(len(d.ready))
	for _, v := range d.ready {
		if num > 0 {
			num--
			continue
		}
		return v, true
	}
	panic("not reachable")
}

func (d *Dialer) poll() {
	// TODO(golang.org/issue/38315) - Plumb a context in here correctly
	ctx := context.TODO()
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		d.pollOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (d *Dialer) pollOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := d.lister.ListInstances(ctx)
	cancel()
	if err != nil {
		log.Printf("gcpdial: polling %v: %v", d.lister, err)
		return
	}

	want := map[string]bool{} // the res []string turned into a set
	for _, instURL := range res {
		want[instURL] = true
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	// Stop + remove any health check probers that no longer appear in the
	// instance group.
	for instURL, prober := range d.prober {
		if !want[instURL] {
			prober.cancel()
			delete(d.prober, instURL)
		}
	}
	// And start any new health check probers that are newly added
	// (or newly known at least) to the instance group.
	for _, instURL := range res {
		if _, ok := d.prober[instURL]; ok {
			continue
		}
		p := newProber(d, instURL)
		go p.probeLoop()
		if d.prober == nil {
			d.prober = map[string]*prober{}
		}
		d.prober[instURL] = p
	}
	d.lastInstances = res
}

// NewRegionInstanceGroupDialer returns a new dialer that dials named
// regional instance group in the provided project and region.
//
// It begins polling immediately, and there's no way to stop it.
// (Until we need one)
func NewRegionInstanceGroupDialer(project, region, group string) *Dialer {
	d := &Dialer{
		lister: regionInstanceGroupLister{project, region, group},
	}
	go d.poll()
	return d
}

// instanceLister is something that can list the current set of VMs.
//
// The idea is that we'll have both zonal and regional instance group listers,
// but currently we only have regionInstanceGroupLister below.
type instanceLister interface {
	// ListInstances returns a list of instances in their API URL form.
	//
	// The API URL form is parseable by the parseInstance func. See its docs.
	ListInstances(context.Context) ([]string, error)
}

// regionInstanceGroupLister is an instanceLister implementation that watches a regional
// instance group for changes to its set of VMs.
type regionInstanceGroupLister struct {
	project, region, group string
}

func (rig regionInstanceGroupLister) ListInstances(ctx context.Context) (ret []string, err error) {
	svc, err := compute.NewService(ctx)
	if err != nil {
		return nil, err
	}
	rigSvc := svc.RegionInstanceGroups
	insts, err := rigSvc.ListInstances(rig.project, rig.region, rig.group, &compute.RegionInstanceGroupsListInstancesRequest{
		InstanceState: "RUNNING",
		PortName:      "", // all
	}).Context(ctx).MaxResults(500).Do()
	if err != nil {
		return nil, err
	}
	// TODO: pagination for really large sets? Currently we truncate the results
	// to the first 500 VMs, which seems like plenty for now.
	// 500 is the maximum the API supports; see:
	// https://pkg.go.dev/google.golang.org/api/compute/v1?tab=doc#RegionInstanceGroupsListInstancesCall.MaxResults
	for _, it := range insts.Items {
		ret = append(ret, it.Instance)
	}
	return ret, nil
}

// parsedInstance contains the project, zone, and name of a VM.
type parsedInstance struct {
	Project, Zone, Name string
}

// parseInstance parses e.g. "https://www.googleapis.com/compute/v1/projects/golang-org/zones/us-central1-c/instances/playsandbox-7sj8" into its parts.
func parseInstance(u string) (*parsedInstance, error) {
	const pfx = "https://www.googleapis.com/compute/v1/projects/"
	if !strings.HasPrefix(u, pfx) {
		return nil, fmt.Errorf("failed to parse instance %q; doesn't begin with %q", u, pfx)
	}
	u = u[len(pfx):] // "golang-org/zones/us-central1-c/instances/playsandbox-7sj8"
	f := strings.Split(u, "/")
	if len(f) != 5 || f[1] != "zones" || f[3] != "instances" {
		return nil, fmt.Errorf("failed to parse instance %q; unexpected format", u)
	}
	return &parsedInstance{f[0], f[2], f[4]}, nil
}
