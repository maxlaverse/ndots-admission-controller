package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func dummyAdmissionReview(podRaw string) *v1.AdmissionReview {
	return &v1.AdmissionReview{
		Request: &v1.AdmissionRequest{
			UID:       "74b4c8e6-5003-485d-ac50-18b943a88b24",
			Namespace: "default",
			Name:      "ndots-admission-controller-7649d9b499-vfwgb",
			Operation: "CREATE",
			Object:    runtime.RawExtension{Raw: []byte(podRaw)},
			Kind: metav1.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Pod",
			},
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kube-system:job-controller",
			},
		},
	}
}

func TestPatchPodsWithoutNdots(t *testing.T) {
	admissionResponse := ReviewPodAdmission(dummyAdmissionReview(`{}`))

	assert.Nil(t, admissionResponse.Result)
	assert.True(t, admissionResponse.Allowed)
	assert.Equal(t, v1.PatchTypeJSONPatch, *admissionResponse.PatchType)
	assert.Equal(t, "[{\"op\":\"add\",\"path\":\"/spec/dnsConfig\",\"value\":{\"options\":[{\"name\":\"ndots\",\"value\":\"1\"}]}}]", string(admissionResponse.Patch))
}

func TestDontPatchPodsWithExistingNdots(t *testing.T) {
	admissionResponse := ReviewPodAdmission(dummyAdmissionReview(`{ "spec": { "dnsConfig":{ "options": [ { "name": "ndots", "value":"5" } ] }}}`))

	assert.Nil(t, admissionResponse.Result)
	assert.True(t, admissionResponse.Allowed)
	assert.Nil(t, admissionResponse.PatchType)
}

func TestRejectUnserializablePods(t *testing.T) {
	admissionResponse := ReviewPodAdmission(dummyAdmissionReview(""))

	assert.False(t, admissionResponse.Allowed)
	assert.NotNil(t, admissionResponse.Result)
	assert.Equal(t, "unexpected end of JSON input", admissionResponse.Result.Message)
}
