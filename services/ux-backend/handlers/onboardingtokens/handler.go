package onboardingtokens

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/red-hat-storage/ocs-operator/v4/controllers/util"
	"github.com/red-hat-storage/ocs-operator/v4/services/ux-backend/handlers"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

const (
	onboardingPrivateKeyFilePath = "/home/ritesh/ticket/key.rsa"
)

func HandleMessage(w http.ResponseWriter, r *http.Request, tokenLifetimeInHours int) {

	switch r.Method {
	case "POST":
		handlePost(w, tokenLifetimeInHours, r)
	default:
		handleUnsupportedMethod(w, r)
	}
}

func handlePost(w http.ResponseWriter, tokenLifetimeInHours int, r *http.Request) {
	quantity, err := getQuantity(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if onboardingToken, err := util.GenerateOnboardingToken(tokenLifetimeInHours, onboardingPrivateKeyFilePath, quantity); err != nil {
		klog.Errorf("failed to get onboardig token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", handlers.ContentTypeTextPlain)

		if _, err := w.Write([]byte("Failed to generate token")); err != nil {
			klog.Errorf("failed write data to response writer, %v", err)
		}
	} else {
		klog.Info("onboarding token generated successfully")
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", handlers.ContentTypeTextPlain)

		if _, err = w.Write([]byte(onboardingToken)); err != nil {
			klog.Errorf("failed write data to response writer: %v", err)
		}
	}
}

func handleUnsupportedMethod(w http.ResponseWriter, r *http.Request) {
	klog.Info("Only POST method should be used to send data to this endpoint /onboarding-tokens")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Header().Set("Content-Type", handlers.ContentTypeTextPlain)
	w.Header().Set("Allow", "POST")

	if _, err := w.Write([]byte(fmt.Sprintf("Unsupported method : %s", r.Method))); err != nil {
		klog.Errorf("failed write data to response writer: %v", err)
	}
}

func getQuantity(r *http.Request) (*resource.Quantity, error) {
	var quota = struct {
		Quota string `json:"quota"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&quota)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}

	// When length is 0 that means either request body is empty or
	// quota parameter is not passed in that case quota is considerred
	// to be unlimited
	if len(quota.Quota) == 0 {
		return nil, nil
	}
	quantity, err := resource.ParseQuantity(quota.Quota)
	if err != nil {
		return nil, fmt.Errorf("invalid quota value sent in request body: %v", err)
	}
	return &quantity, nil
}
