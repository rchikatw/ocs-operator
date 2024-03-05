package deploymentcapablity

import (
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

const (
	onboardingPrivateKeyFilePath = "/etc/private-key/key"
	ContentTypeTextPlain         = "text/plain"
)

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		handlePost(w)
	default:
		handleUnsupportedMethod(w, r)
	}
}

func handlePost(w http.ResponseWriter) {

	if deploymentCapablityJson, err := checkDeploymentCapablity(); err != nil {
		klog.Errorf("failed to get api's available: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		if _, err := w.Write([]byte("Failed to get api's available")); err != nil {
			klog.Errorf("failed write data to response writer, %v", err)
		}
	} else {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		if _, err = w.Write(deploymentCapablityJson); err != nil {
			klog.Errorf("failed write data to response writer: %v", err)
		}
	}
}

func handleUnsupportedMethod(w http.ResponseWriter, r *http.Request) {

	klog.Info("Only POST method should be used to send data to this endpoint /deployment-tokens")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Header().Set("Content-Type", ContentTypeTextPlain)
	w.Header().Set("Allow", "POST")

	if _, err := w.Write([]byte(fmt.Sprintf("Unsupported method : %s", r.Method))); err != nil {
		klog.Errorf("failed write data to response writer: %v", err)
	}
}

func checkDeploymentCapablity() ([]byte, error) {

	deploymentCapablity := make(map[string]bool)
	// actualSecret := &corev1.Secret{}
	// Check for private secret
	// err = client.Get(context.Background(), types.NamespacedName{Name: "onboardingValidationPublicKeySecretName",
	// 	Namespace: "instance.Namespace"}, actualSecret)

	// if err != nil {
	// 	deploymentCapablity["onboardingTokenFeature"] = true
	// }
	deploymentCapablity["onboardingTokenEnabled"] = true
	deploymentCapablity["rotateKeysEnabled"] = true

	// Check for error
	deploymentCapablityJson, err := json.Marshal(deploymentCapablity)
	if err != nil {
		klog.Exitf("failed to get deployment capablity for the backend services")
		return nil, err
	}
	return deploymentCapablityJson, nil

}
