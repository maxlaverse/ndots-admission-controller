package pkg

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/fsnotify/fsnotify"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

var (
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	runtimeScheme = runtime.NewScheme()
)

type admissionHandler func(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// KeyPair contains the certificate and private key required to listen
// for TLS connections.
type KeyPair struct {
	TLSCertFilepath string
	TLSKeyFilepath  string
}

// WebhookServer is a webserver answering to AdmissionRequests sent by
// the Kubernetes API server.
type WebhookServer struct {
	keyopts KeyPair
}

// NewWebhookServer returns a new web server that can process
// AdmissionReviews.
func NewWebhookServer(keyopts KeyPair) *WebhookServer {
	return &WebhookServer{
		keyopts: keyopts,
	}
}

// Run loads the key pair, listens to incoming admission reviews
// and answers them until the context is cancelled.
func (srv *WebhookServer) Run(ctx context.Context) error {
	for {
		keyContext, err := srv.cancelledContextOnKeyChanges(ctx)
		if err != nil {
			return err
		}

		err = srv.runServer(keyContext)
		if err != nil || ctx.Err() != nil {
			return err
		}

		klog.V(0).Infof("Restarting server")
		time.Sleep(time.Second) // waiting a bit for second key to be propagated
	}
}

func (srv *WebhookServer) cancelledContextOnKeyChanges(ctx context.Context) (context.Context, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = watcher.Add(srv.keyopts.TLSCertFilepath)
	if err != nil {
		return nil, err
	}

	err = watcher.Add(srv.keyopts.TLSKeyFilepath)
	if err != nil {
		return nil, err
	}

	newCtx, cancel := context.WithCancel(ctx)
	go func() {
		klog.V(0).Infof("Watching certificates: %v", watcher.WatchList())
		defer cancel()
		defer watcher.Close()

		select {
		case event, ok := <-watcher.Events:
			if ok {
				klog.V(0).Infof("event: %v", event)
			}
		case err, ok := <-watcher.Errors:
			if ok {
				klog.Errorf("error:", err)
			}
		case <-ctx.Done():
		}
	}()

	return newCtx, nil
}

func (srv *WebhookServer) runServer(ctx context.Context) error {
	keyPair, err := tls.LoadX509KeyPair(srv.keyopts.TLSCertFilepath, srv.keyopts.TLSKeyFilepath)
	if err != nil {
		return fmt.Errorf("error loading key pair: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/webhook", httpWrap(ReviewPodAdmission))

	server := &http.Server{
		Addr:      ":8443",
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{keyPair}},
		ErrorLog:  newHttpServerErrorLogger(),
	}

	go func() {
		<-ctx.Done()
		klog.V(0).Infof("Server shutting down")
		server.Shutdown(ctx)
	}()

	klog.V(0).Infof("Server started")
	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to listen and serve: %w", err)
	}
	return nil
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(204)
}

// httpWrap wraps an 'admissionHandler' function into a webserver handler.
func httpWrap(m admissionHandler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.Body != nil {
			if data, err := ioutil.ReadAll(r.Body); err == nil {
				body = data
			}
		}
		if len(body) == 0 {
			klog.Error("Client sent an empty body")
			http.Error(w, "Client sent an empty body", http.StatusBadRequest)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			klog.Errorf("Content-Type=%s, expect application/json", contentType)
			http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
			return
		}

		var admissionResponse *admissionv1.AdmissionResponse
		ar := admissionv1.AdmissionReview{}
		if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
			klog.Errorf("Can't decode body: %v", err)
			admissionResponse = &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		} else {
			admissionResponse = m(&ar)
		}

		admissionReview := admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				Kind:       ar.Kind,
				APIVersion: ar.APIVersion,
			},
		}
		if admissionResponse != nil {
			admissionReview.Response = admissionResponse
			if ar.Request != nil {
				admissionReview.Response.UID = ar.Request.UID
			}
		}

		resp, err := json.Marshal(admissionReview)
		if err != nil {
			klog.Errorf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}

		if _, err := w.Write(resp); err != nil {
			klog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	}
}
