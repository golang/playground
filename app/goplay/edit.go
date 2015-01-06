// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goplay

import (
	"html/template"
	"net/http"
	"strings"

	"appengine"
	"appengine/datastore"
)

const hostname = "play.golang.org"

func init() {
	http.HandleFunc("/", edit)
}

var editTemplate = template.Must(template.ParseFiles("goplay/edit.html"))

type editData struct {
	Snippet *Snippet
}

func edit(w http.ResponseWriter, r *http.Request) {
	// Redirect foo.play.golang.org to play.golang.org.
	if strings.HasSuffix(r.Host, "."+hostname) {
		http.Redirect(w, r, "http://"+hostname, http.StatusFound)
		return
	}

	snip := &Snippet{Body: []byte(hello)}
	if strings.HasPrefix(r.URL.Path, "/p/") {
		c := appengine.NewContext(r)
		id := r.URL.Path[3:]
		serveText := false
		if strings.HasSuffix(id, ".go") {
			id = id[:len(id)-3]
			serveText = true
		}
		key := datastore.NewKey(c, "Snippet", id, 0, nil)
		err := datastore.Get(c, key, snip)
		if err != nil {
			if err != datastore.ErrNoSuchEntity {
				c.Errorf("loading Snippet: %v", err)
			}
			http.Error(w, "Snippet not found", http.StatusNotFound)
			return
		}
		if serveText {
			w.Header().Set("Content-type", "text/plain")
			w.Write(snip.Body)
			return
		}
	}
	editTemplate.Execute(w, &editData{snip})
}

const hello = `package main

import "fmt"

func main() {
	fmt.Println("Hello, playground")
}
`
