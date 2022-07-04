package pkg

import (
	"encoding/json"

	"github.com/mattbaird/jsonpatch"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const (
	ndotsValue      = "1"
	ndotsOptionName = "ndots"
)

func ReviewPodAdmission(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request

	klog.V(1).Infof("AdmissionReview UID=%v for Kind=%v/%v Namespace=%v Operation=%v UserInfo=%v", req.UID, req.Kind.Kind, req.Kind.Version, req.Namespace, req.Operation, req.UserInfo.Username)

	pod, err := extractPodFromReview(req)
	if err != nil {
		klog.Errorf("AdmissionResponse UID=%v for Kind=%v/%v Namespace=%v Result=UnmarshallingFailed Error=%v", req.UID, req.Kind.Kind, req.Kind.Version, req.Namespace, err)
		return rejectWithError(err)
	}

	if !podMutationRequired(pod) {
		klog.V(1).Infof("AdmissionResponse UID=%v for Kind=%v/%v Namespace=%v PodName=%v Result=MutationNotRequired", req.UID, req.Kind.Kind, req.Kind.Version, req.Namespace, pod.Name)
		return admitWithoutChange()
	}

	patchBytes, err := createPatch(pod)
	if err != nil {
		klog.Errorf("AdmissionResponse UID=%v for Kind=%v/%v Namespace=%v PodName=%v Result=PatchCreationFailed Error=%v", req.UID, req.Kind.Kind, req.Kind.Version, req.Namespace, pod.Name, err.Error())
		return rejectWithError(err)
	}

	klog.V(1).Infof("AdmissionResponse UID=%v for Kind=%v/%v Namespace=%v PodName=%v Result=MutationSuccessful Patch=%v", req.UID, req.Kind.Kind, req.Kind.Version, req.Namespace, pod.Name, string(patchBytes))
	return admitWithPatch(patchBytes)
}

func createPatch(pod *corev1.Pod) ([]byte, error) {
	incomingPod, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	if pod.Spec.DNSConfig == nil {
		pod.Spec.DNSConfig = &corev1.PodDNSConfig{}
	}

	value := ndotsValue
	newOptions := corev1.PodDNSConfigOption{
		Name:  ndotsOptionName,
		Value: &value,
	}
	pod.Spec.DNSConfig.Options = append(pod.Spec.DNSConfig.Options, newOptions)

	newPod, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(incomingPod, newPod)
	if err != nil {
		return nil, err
	}
	patchRaw, err := json.Marshal(patch)
	return patchRaw, err
}

func podMutationRequired(pod *corev1.Pod) bool {
	if pod.Spec.DNSConfig == nil {
		return true
	}
	for _, o := range pod.Spec.DNSConfig.Options {
		if o.Name == ndotsOptionName {
			return false
		}
	}
	return true
}

func extractPodFromReview(req *admissionv1.AdmissionRequest) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	if err := json.Unmarshal(req.Object.Raw, pod); err != nil {
		klog.Errorf("Could not unmarshal raw object: %v", err)
		return nil, err
	}
	return pod, nil
}

func rejectWithError(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func admitWithoutChange() *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func admitWithPatch(patchBytes []byte) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}
