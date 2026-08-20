// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizerrestriction

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// Handler protects finalizers on system resources from being removed by users other than the gardenlet,
// system service accounts, or system administrators.
type Handler struct {
	Decoder               admission.Decoder
	ProtectedFinalizers   sets.Set[string]
	SubjectAccessReviewer clientauthorizationv1.SubjectAccessReviewInterface
}

// Handle allows the request unless it removes a protected finalizer. Removal of a protected finalizer is only
// permitted for the gardenlet, dedicated system service accounts, and system administrators.
func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {

	// TODO: Discuss: we could do the user checks first, which would probably be more performant, since we dont need to unmarshal the object for most valid requests.
	// However, then it would be harder to write proper admission messages, because we don't even know if finalizers are being modified.
	oldObj := &metav1.PartialObjectMetadata{}
	if err := json.Unmarshal(req.OldObject.Raw, oldObj); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	oldFinalizers := sets.New(oldObj.GetFinalizers()...)
	newObj := &metav1.PartialObjectMetadata{}
	if err := json.Unmarshal(req.Object.Raw, newObj); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	newFinalizers := sets.New(newObj.GetFinalizers()...)
	changedFinalizers := h.ProtectedFinalizers.Intersection(oldFinalizers).Difference(newFinalizers)
	if changedFinalizers.Len() == 0 {
		return admission.Allowed("No protected finalizers were modified")
	}

	//TODO: discuss / find out if we can reuse gardenletidentity (see e.g. pkg/admissioncontroller/webhook/admission/shootrestriction/handler.go:78)
	// to simplify / consolidate some of the logic here
	// ------------------

	// Apply logic from stolen from updaterestriction handler (pkg/admissioncontroller/webhook/admission/updaterestriction/handler.go:25)
	// TODO: do we even need than and can we solve it via the TODO suggestion above?
	if req.UserInfo.Username == "system:serviceaccount:kube-system:gardener-internal" ||
		req.UserInfo.Username == "system:serviceaccount:kube-system:generic-garbage-collector" {
		return admission.Allowed(fmt.Sprintf("technical user %q is allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
	}
	if sets.New(req.UserInfo.Groups...).HasAny(v1beta1constants.SeedsGroup, v1beta1constants.ShootsGroup) {
		return admission.Allowed(fmt.Sprintf("gardenlet is allowed to modify protected finalizer(s) %v", changedFinalizers.UnsortedList()))
	}

	// Apply the logic stolen from getAdminUserGroups (pkg/apiserver/registry/core/shoot/storage/admin_kubeconfig.go:54) to check if user is system admin (aka ops)
	// vs just normal project admin.
	// TODO: Discuss / find out if this overlaps with the previous check for technical users.
	extra := make(map[string]authorizationv1.ExtraValue, len(req.UserInfo.Extra))
	for k, v := range req.UserInfo.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}
	systemAccessReview := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    "",
				Resource: "secrets",
				Verb:     "list",
			},
			User:   req.UserInfo.Username,
			Groups: req.UserInfo.Groups,
			UID:    req.UserInfo.UID,
			Extra:  extra,
		},
	}
	result, err := h.SubjectAccessReviewer.Create(ctx, systemAccessReview, metav1.CreateOptions{})
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if result.Status.Allowed {
		return admission.Allowed(fmt.Sprintf("system administrator user %q is allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
	}

	return admission.Denied(fmt.Sprintf("user %q is not allowed to modify protected finalizer(s) %v", req.UserInfo.Username, changedFinalizers.UnsortedList()))
}
