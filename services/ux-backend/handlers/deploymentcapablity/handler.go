package deploymentcapablity

import (
	"fmt"
	"net/http"

	"k8s.io/klog"
)

var ContentTypeTextPlain = "text/plain"

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		handleGet(w)
	default:
		handleUnsupportedMethod(w, r)
	}
}

func handleGet(w http.ResponseWriter) {

}

func handleUnsupportedMethod(w http.ResponseWriter, r *http.Request) {
	klog.Info("Only GET method should be used to send data to this endpoint /deployment-capablity")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Header().Set("Content-Type", ContentTypeTextPlain)
	w.Header().Set("Allow", "GET")

	if _, err := w.Write([]byte(fmt.Sprintf("Unsupported method : %s", r.Method))); err != nil {
		klog.Errorf("failed write data to response writer: %v", err)
	}
}
