package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/gorilla/schema"
)

const (
	httpProtocolHdr = "X-Forwarded-Proto"
	tmplUsage       = "usage.html.gtpl"
	tmpBootstrap    = "bootstrap.sh.gtpl"
)

var (
	// tmpls is a set of template for this WebSVC (all /*.gtpl files)
	tmpls *template.Template
)

// Params contains all parameters passed to us via HTTP.
type Params struct {
	Origin       string
	IP1          string `schema:"i1" `
	IP2          string `schema:"i2" `
	IP3          string `schema:"i3" `
	Encryption   string `schema:"e" `
	InitialToken string `schema:"t" `
	Prefix       string `schema:"r" `
	ClientPort   string `schema:"c" `
	PeerPort     string `schema:"p" `
	Directory    string `schema:"d" `
	Username     string `schema:"u" `
	Version      string `schema:"v" `
}

// computeOrigin recreates the request URL, so it can be added into the bootstrap.
func computeOrigin(r *http.Request, p *Params) {
	p.Origin = "unknown"
	if r.Host != "" && r.URL != nil {
		proto := r.Header.Get(httpProtocolHdr)
		if proto == "" {
			proto = "http"
		}
		p.Origin = fmt.Sprintf("%s://%s%s", proto, r.Host, r.URL)
	}
	log.Printf("Client %q - REQ %s from Referer %q", r.RemoteAddr, p.Origin, r.Referer())
	p.Origin = strings.Replace(p.Origin, "%", "%%", -1)
}

// parseRequest uses Gorilla schema to process parameters (see http://www.gorillatoolkit.org/pkg/schema)
func parseRequest(r *http.Request) (*Params, error) {
	err := r.ParseForm()
	if err != nil {
		return nil, fmt.Errorf("Could not parse form: %s", err)
	}

	p := new(Params)
	decoder := schema.NewDecoder()
	//decoder.IgnoreUnknownKeys(true)

	err = decoder.Decode(p, r.Form)
	if err != nil {
		return nil, fmt.Errorf("Could not decode form: %s", err)
	}

	log.Printf("FROM %v PARSED %+v\n", r.RemoteAddr, p)

	return p, nil
}

func sendTemplate(template string, data interface{}, w http.ResponseWriter) error {
	var result bytes.Buffer
	var err error

	err = tmpls.ExecuteTemplate(&result, template, data)
	if err != nil {
		return fmt.Errorf("Could not render %s: %s", template, err)
	}

	content, n := result.Bytes(), 0
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))

	n, err = w.Write(content)
	if err != nil {
		return fmt.Errorf("Could not send %s: %s", template, err)
	} else if n != len(content) {
		return fmt.Errorf("Short write while sending %s", template)
	}
	return nil
}

// sendError sends back the "400 BAD REQUEST" to the client
func sendError(code int, err error, w http.ResponseWriter) {
	e := "Unspecified error"
	if err != nil {
		e = err.Error()
	}
	if code <= 0 {
		code = http.StatusBadRequest
	}
	log.Printf("ERROR: %s", e)
	w.WriteHeader(code)
	w.Write([]byte(e))
}

func main() {
	var err error

	tmpls, err = template.ParseGlob("*.gtpl")
	if err != nil {
		log.Printf("ERROR: Could not parse templates: %s", err)
		os.Exit(1)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var p *Params

		if r.ContentLength == 0 && len(r.URL.RawQuery) == 0 {
			// If nothing was submitted, we display the HTML usage
			w.Header().Set("Content-Type", "text/html")
			err = sendTemplate(tmplUsage, nil, w)
		} else if p, err = parseRequest(r); err == nil {
			computeOrigin(r, p)
			err = sendTemplate(tmpBootstrap, p, w)
		}

		if err != nil {
			sendError(http.StatusBadRequest, err, w)
		}
	})

	log.Printf("Serving at 0.0.0.0:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
