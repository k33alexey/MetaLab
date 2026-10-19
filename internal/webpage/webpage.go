// Package webpage holds how every ML server answers the browser: its page, the
// policy under which the browser runs it, and the data the page asks for.
// Studio, Portal, Manager and the prototype serve one kind of page — markup
// with its own styles and scripts that asks nothing of another origin and is
// never framed — so they answer it alike.
package webpage

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
)

// pagePath is where every server keeps its page among its embedded files.
const pagePath = "ui/index.html"

// SetPolicy sets the security headers of an ML page on an answer: scripts,
// styles and requests of its own origin only (inline ones allowed, as the pages
// are written so), no plugins, no other base, no framing, no guessing of the
// content type and no referrer.
func SetPolicy(header http.Header) {
	header.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
}

// Secured answers every request of next under the policy of an ML page.
func Secured(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		SetPolicy(response.Header())
		next.ServeHTTP(response, request)
	})
}

// Page answers the page of a server from its embedded files, never cached, so
// that a new build is the page the browser shows; product names the server in
// the refusal when the page is missing from the build.
func Page(files fs.ReadFileFS, product string) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		page, err := files.ReadFile(pagePath)
		if err != nil {
			http.Error(response, product+" UI unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		_, _ = response.Write(page)
	}
}

// JSON answers value as JSON with status, never cached, as the data of a page
// is the state of the moment. The value is encoded before anything is written,
// so a value that does not encode is refused whole rather than cut short
// after its status.
func JSON(response http.ResponseWriter, status int, value any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		http.Error(response, "Unable to encode response", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_, _ = response.Write(body.Bytes())
}

// Answer answers what a route of a server did: a failure the way the route
// refuses it, and otherwise the result as JSON with status - or nothing but
// the status for 204 No Content, which has no body. Every route of Studio,
// Manager and Portal that does one thing and answers it ends through here.
func Answer(response http.ResponseWriter, status int, result any, err error, refuse func(http.ResponseWriter, error)) {
	if err != nil {
		refuse(response, err)
		return
	}
	if status == http.StatusNoContent {
		response.WriteHeader(status)
		return
	}
	JSON(response, status, result)
}

// Refusal is the refusal of a route that answers every failure the same way:
// with status and text, or with the failure's own words where text is empty.
// A route that tells failures apart writes its refusal itself.
func Refusal(status int, text string) func(http.ResponseWriter, error) {
	return func(response http.ResponseWriter, err error) {
		message := text
		if message == "" {
			message = err.Error()
		}
		http.Error(response, message, status)
	}
}

// PolicyMissing names the headers of the policy an answer lacks or carries
// with another value; the tests of every server ask it of their answers, so
// that the policy is spelt out in one place and its values in one test.
func PolicyMissing(header http.Header) []string {
	want := http.Header{}
	SetPolicy(want)
	var missing []string
	for name := range want {
		if header.Get(name) != want.Get(name) {
			missing = append(missing, name)
		}
	}
	return missing
}
