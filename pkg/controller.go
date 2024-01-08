package pkg

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
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

const (
	watcherShutdownDelay = 1 * time.Second
	tlsCertFilename      = "tls.crt"
	tlsKeyFilename       = "tls.key"
)

type admissionHandler func(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

// WebhookServer is a webserver answering to AdmissionRequests sent by
// the Kubernetes API server.
type WebhookServer struct {
	tlsCertificateDirectory string
}

// NewWebhookServer returns a new web server that can process
// AdmissionReviews.
func NewWebhookServer(tlsCertificateDirectory string) *WebhookServer {
	return &WebhookServer{
		tlsCertificateDirectory: tlsCertificateDirectory,
	}
}

// Run starts the webserver, and restarts it if the key pair is modified
func (srv *WebhookServer) Run(ctx context.Context) error {
	for {
		watcherContext, err := directoryWatcherContext(ctx, srv.tlsCertificateDirectory)
		if err != nil {
			return err
		}

		err = srv.runServer(watcherContext)
		if err != nil || ctx.Err() != nil {
			return err
		}

		klog.V(0).Infof("Restarting server")
	}
}

// runServer loads the key pair, listens to incoming admission reviews
// and answers them until the context is cancelled.
func (srv *WebhookServer) runServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/webhook", httpWrap(ReviewPodAdmission))

	certificates, err := srv.loadCertificates()
	if err != nil {
		return fmt.Errorf("error loading certificate: %w", err)
	}

	server := &http.Server{
		Addr:      ":8443",
		Handler:   mux,
		TLSConfig: &tls.Config{Certificates: certificates},
		ErrorLog:  newHttpServerErrorLogger(),
	}

	go func() {
		<-ctx.Done()
		klog.V(0).Infof("Server shutting down")
		server.Shutdown(ctx)
	}()

	listenConfig := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}

	listener, err := listenConfig.Listen(context.Background(), "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	klog.V(0).Infof("Server started")
	if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (srv *WebhookServer) loadCertificates() ([]tls.Certificate, error) {
	keyPair, err := tls.LoadX509KeyPair(filepath.Join(srv.tlsCertificateDirectory, tlsCertFilename), filepath.Join(srv.tlsCertificateDirectory, tlsKeyFilename))
	if err != nil {
		return nil, fmt.Errorf("error loading key pair: %w", err)
	}

	if len(keyPair.Certificate) == 0 {
		return nil, fmt.Errorf("no certificate found")
	}

	foundValidCertificate := false
	for _, certBytes := range keyPair.Certificate {
		cert, err := x509.ParseCertificate(certBytes)
		if err != nil {
			return nil, fmt.Errorf("error parsing certificate: %w", err)
		}

		klog.V(0).Infof("Certificate for '%v' is valid from '%s' to '%s'", cert.Subject, cert.NotBefore, cert.NotAfter)

		if cert.NotBefore.Before(time.Now()) && cert.NotAfter.After(time.Now()) {
			foundValidCertificate = true
		}
	}
	if !foundValidCertificate {
		return nil, fmt.Errorf("no valid certificate found")
	}

	return []tls.Certificate{keyPair}, nil
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(204)
}

// httpWrap wraps an 'admissionHandler' function into a webserver handler.
func httpWrap(m admissionHandler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.Body != nil {
			if data, err := io.ReadAll(r.Body); err == nil {
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
