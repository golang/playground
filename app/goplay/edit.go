// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goplay

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"appengine"
	"appengine/datastore"
)

const hostname = "play.golang.org"

func init() {
	http.Handle("/", hstsHandler(edit))
}

var editTemplate = template.Must(template.ParseFiles("goplay/edit.html"))

type editData struct {
	Snippet *Snippet
	Share   bool
}

func edit(w http.ResponseWriter, r *http.Request) {
	// Redirect foo.play.golang.org to play.golang.org.
	if strings.HasSuffix(r.Host, "."+hostname) {
		http.Redirect(w, r, "https://"+hostname, http.StatusFound)
		return
	}

	snip := &Snippet{Body: []byte(hello)}
	if strings.HasPrefix(r.URL.Path, "/p/") {
		if !allowShare(r) {
			msg := `<h1>Unavailable For Legal Reasons</h1><p>If you believe this is in error, please <a href="https://golang.org/issue">file an issue</a>.</p>`
			http.Error(w, msg, http.StatusUnavailableForLegalReasons)
			return
		}
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
			if r.FormValue("download") == "true" {
				w.Header().Set(
					"Content-Disposition", fmt.Sprintf(`attachment; filename="%s.go"`, id),
				)
			}
			w.Header().Set("Content-type", "text/plain")
			w.Write(snip.Body)
			return
		}
	}
	editTemplate.Execute(w, &editData{snip, allowShare(r)})
}

const hello = `package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello, playground")
}
`
