// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0


// TODO: check tests

package finalizerrestriction_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientauthorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/finalizerrestriction"
)

var _ = Describe("handler", func() {
	var (
		ctx      context.Context
		reviewer *fakeSubjectAccessReviewer
		handler  *finalizerrestriction.Handler
		req      admission.Request

		// rawWithFinalizers marshals an object carrying the given finalizers into its metadata.
		rawWithFinalizers = func(finalizers ...string) []byte {
			obj := &metav1.PartialObjectMetadata{}
			obj.Finalizers = finalizers
			raw, err := json.Marshal(obj)
			Expect(err).NotTo(HaveOccurred())
			return raw
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		reviewer = &fakeSubjectAccessReviewer{allowed: false}
		handler = &finalizerrestriction.Handler{
			ProtectedFinalizers:   sets.New("gardener"),
			SubjectAccessReviewer: reviewer,
		}

		req = admission.Request{}
		req.UserInfo = authenticationv1.UserInfo{Username: "some-project-admin"}
		req.Resource = metav1.GroupVersionResource{Resource: "shoots"}
		req.Operation = admissionv1.Update
		// By default the request removes the protected "gardener" finalizer.
		req.OldObject.Raw = rawWithFinalizers("gardener", "some-other-finalizer")
		req.Object.Raw = rawWithFinalizers("some-other-finalizer")
	})

	Describe("#Handle", func() {
		It("should allow the request if no protected finalizer is modified", func() {
			// The protected finalizer is retained; only an unprotected one is removed.
			req.OldObject.Raw = rawWithFinalizers("gardener", "some-other-finalizer")
			req.Object.Raw = rawWithFinalizers("gardener")

			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
		})

		It("should allow the request if no finalizers change at all", func() {
			req.OldObject.Raw = rawWithFinalizers("gardener")
			req.Object.Raw = rawWithFinalizers("gardener")

			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Result.Message).To(Equal("No protected finalizers were modified"))
		})

		DescribeTable("should allow technical users to remove a protected finalizer",
			func(username string) {
				req.UserInfo = authenticationv1.UserInfo{Username: username}

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(ContainSubstring("technical user"))
				Expect(resp.Result.Message).To(ContainSubstring("gardener"))
			},

			Entry("gardener-internal", "system:serviceaccount:kube-system:gardener-internal"),
			Entry("generic-garbage-collector", "system:serviceaccount:kube-system:generic-garbage-collector"),
		)

		DescribeTable("should allow the gardenlet to remove a protected finalizer",
			func(group string) {
				req.UserInfo.Groups = []string{group}

				resp := handler.Handle(ctx, req)
				Expect(resp.Allowed).To(BeTrue())
				Expect(resp.Result.Message).To(ContainSubstring("gardenlet is allowed to modify protected finalizer"))
			},

			Entry("seed gardenlet", "gardener.cloud:system:seeds"),
			Entry("self-hosted shoot gardenlet", "gardener.cloud:system:shoots"),
		)

		It("should allow a system administrator to remove a protected finalizer", func() {
			reviewer.allowed = true

			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Result.Message).To(ContainSubstring("system administrator user"))
			Expect(resp.Result.Message).To(ContainSubstring("gardener"))
		})

		It("should deny a project administrator removing a protected finalizer", func() {
			reviewer.allowed = false

			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Code).To(Equal(int32(403)))
			Expect(resp.Result.Message).To(ContainSubstring(`user "some-project-admin" is not allowed to modify protected finalizer`))
			Expect(resp.Result.Message).To(ContainSubstring("gardener"))
		})

		It("should error if the SubjectAccessReview fails", func() {
			reviewer.evaluationError = "review failed"

			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Code).To(Equal(int32(500)))
		})

		It("should error if the new object cannot be decoded", func() {
			req.Object.Raw = []byte("not-json")

			resp := handler.Handle(ctx, req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Code).To(Equal(int32(500)))
		})
	})
})

type fakeSubjectAccessReviewer struct {
	// allowed is true when the user has gardener system-wide permissions.
	allowed         bool
	reason          string
	evaluationError string
}

var _ clientauthorizationv1.SubjectAccessReviewInterface = (*fakeSubjectAccessReviewer)(nil)

func (f *fakeSubjectAccessReviewer) Create(_ context.Context, subjectAccessReview *authorizationv1.SubjectAccessReview, _ metav1.CreateOptions) (*authorizationv1.SubjectAccessReview, error) {
	if f.evaluationError != "" {
		return nil, errors.New(f.evaluationError)
	}

	ret := subjectAccessReview.DeepCopy()
	ret.Status = authorizationv1.SubjectAccessReviewStatus{
		Allowed: f.allowed,
		Denied:  !f.allowed,
		Reason:  f.reason,
	}
	return ret, nil
}
