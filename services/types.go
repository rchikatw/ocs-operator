package services

import "k8s.io/apimachinery/pkg/api/resource"

type OnboardingTicket struct {
	ID             string             `json:"id"`
	ExpirationDate int64              `json:"expirationDate,string"`
	StorageQuota   *resource.Quantity `json:"storageQuota,omitempty"`
}
