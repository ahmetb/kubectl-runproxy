package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var (
	tokenSource oauth2.TokenSource
)

func main() {
	cert, err := generateKeyPair()
	if err != nil {
		log.Fatalf("failed to self-sign tls cert: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", fallbackHandler)
	mux.HandleFunc("/api", apiHandler)
	mux.HandleFunc("/apis", apisHandler)
	mux.HandleFunc("/apis/", routingHandler)
	mux.HandleFunc("/openapi", routingHandler)             // TODO currently only used in "kubectl edit", which doesn't work due to lack of PATCH
	mux.HandleFunc("/swagger-2.0.0.pb-v1", routingHandler) // TODO currently  only used in "kubectl edit", which doesn't work due to lack of PATCH
	log.Printf("starting server...\nCA certificate (PEM in base64 format):\n%s",b64cert(cert))

	addr := "localhost:6443"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on port: %v", err)
	}
	srv := &http.Server{Addr: addr, Handler: mux,}
	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{cert}
	err = srv.Serve(tls.NewListener(lis, tlsConfig))
	log.Fatal(err)
}

func fallbackHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("request method=%s path=%s", req.Method, req.URL.Path)
	w.WriteHeader(http.StatusNotFound)
}

func routingHandler(w http.ResponseWriter, origReq *http.Request) {
	const gkeBackend = `146.148.59.112`
	const runBackend = `us-central1-run.googleapis.com`

	var backend string
	var skipServerVerification bool
	if strings.Contains(origReq.URL.Path, "/namespaces/") {
		backend = runBackend // for actual api requests
	} else {
		backend = gkeBackend // for schema/discovery requests
		skipServerVerification = true
	}

	origReq.URL.Scheme = "https"
	origReq.URL.Host = backend
	origReq.Header.Set("host", backend)

	req, err := http.NewRequest(origReq.Method, origReq.URL.String(), origReq.Body)
	if err != nil {
		log.Printf("WARN: failed to create new request for proxyinh: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	req.Header = origReq.Header
	req.Header.Set("accept-encoding", "identity") // do not GZIP between proxy and apiserver

	log.Printf("proxying request to %s", req.URL)

	client := &http.Client{}
	if skipServerVerification {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WARN: client.Do error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	log.Printf("proxying complete for %s code=%d", req.URL, resp.StatusCode)

	// copy headers
	for hdrKey, hdr := range resp.Header {
		for _, v := range hdr {
			w.Header().Add(hdrKey, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if origReq.Method == http.MethodDelete {
		prepDeleteResponse(w, resp.StatusCode, resp.Body, resp.Header.Get("content-encoding") == "gzip")
		return
	}

	// copy body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("failed to copy proxied resp body: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

}

func prepDeleteResponse(w io.Writer, status int, body io.Reader, isGZIP bool) {
	if status == http.StatusOK {
		fmt.Fprint(w, `{"apiVersion":"v1", "kind":"Status","status":"Success"}`)
		return
	}

	body = io.TeeReader(body, os.Stderr)
	b, err := ioutil.ReadAll(body)
	if err != nil {
		log.Printf("WARN: failed to read original resp body: %v", err)
		return
	}

	// parse error.message from oneplatform response
	var msg string
	v := struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}{}
	if err := json.Unmarshal(b, &v); err == nil && v.Error.Message != "" {
		msg = v.Error.Message
	} else {
		msg = fmt.Sprintf("original response from Cloud Run API: %s", string(b))
	}

	reason := "Unknown" // TODO(ahmetb) find list of enums for reason and make it work with oneplatform
	if v.Error.Status != "" {
		reason = v.Error.Status
	} else if status == http.StatusNotFound {
		reason = "NotFound"
	}

	log.Printf("response:")
	w = io.MultiWriter(w, os.Stderr)

	resp := struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
		Status     string `json:"status"`
		Code       int    `json:"code"`
		Message    string `json:"message"`
		Reason     string `json:"reason"`
	}{
		APIVersion: "v1",
		Kind:       "Status",
		Status:     "Failure",
		Code:       status,
		Reason:     reason,
		Message:    msg,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("WARN: failed to write response: %v", err)
		return
	}
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, `{}`) // TODO(ahmetb) is this an acceptable response?
}

func apisHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, `{
  "kind": "APIGroupList",
  "apiVersion": "v1",
  "groups": [
    {
      "name": "autoscaling.internal.knative.dev",
      "versions": [
        {
          "groupVersion": "autoscaling.internal.knative.dev/v1alpha1",
          "version": "v1alpha1"
        }
      ],
      "preferredVersion": {
        "groupVersion": "autoscaling.internal.knative.dev/v1alpha1",
        "version": "v1alpha1"
      }
    },
    {
      "name": "caching.internal.knative.dev",
      "versions": [
        {
          "groupVersion": "caching.internal.knative.dev/v1alpha1",
          "version": "v1alpha1"
        }
      ],
      "preferredVersion": {
        "groupVersion": "caching.internal.knative.dev/v1alpha1",
        "version": "v1alpha1"
      }
    },
    {
      "name": "domains.cloudrun.com",
      "versions": [
        {
          "groupVersion": "domains.cloudrun.com/v1alpha1",
          "version": "v1alpha1"
        }
      ],
      "preferredVersion": {
        "groupVersion": "domains.cloudrun.com/v1alpha1",
        "version": "v1alpha1"
      }
    },
    {
      "name": "networking.internal.knative.dev",
      "versions": [
        {
          "groupVersion": "networking.internal.knative.dev/v1alpha1",
          "version": "v1alpha1"
        }
      ],
      "preferredVersion": {
        "groupVersion": "networking.internal.knative.dev/v1alpha1",
        "version": "v1alpha1"
      }
    },
    {
      "name": "serving.knative.dev",
      "versions": [
        {
          "groupVersion": "serving.knative.dev/v1alpha1",
          "version": "v1alpha1"
        }
      ],
      "preferredVersion": {
        "groupVersion": "serving.knative.dev/v1alpha1",
        "version": "v1alpha1"
      }
    }
  ]
}`)
}
