// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package lease_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/nodeagent/controller/lease"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Lease", func() {
	Describe("#ObjectName", func() {
		It("should return the expected name", func() {
			Expect(gardenerutils.NodeAgentLeaseName("foo")).To(Equal("gardener-node-agent-foo"))
		})
	})

	Describe("#Start", func() {
		const nodeName = "foo"

		var (
			ctx       context.Context
			cancel    context.CancelFunc
			node      *corev1.Node
			clientSet *fake.Clientset
			runnable  *lease.Runnable
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
			DeferCleanup(cancel)

			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: types.UID("node-uid")}}
			clientSet = fake.NewSimpleClientset(node)

			runnable = &lease.Runnable{
				ClientSet:            clientSet,
				NodeName:             nodeName,
				Namespace:            metav1.NamespaceSystem,
				LeaseDurationSeconds: 40,
				Clock:                testclock.NewFakeClock(metav1.Now().Time),
			}
		})

		It("should create the heartbeat lease with the expected fields and owner reference", func() {
			leaseName := gardenerutils.NodeAgentLeaseName(nodeName)

			go func() {
				defer GinkgoRecover()
				Expect(runnable.Start(ctx)).To(Succeed())
			}()

			Eventually(func(g Gomega) {
				l, err := clientSet.CoordinationV1().Leases(metav1.NamespaceSystem).Get(ctx, leaseName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(l.Spec.HolderIdentity).To(PointTo(Equal(leaseName)))
				g.Expect(l.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(40))))
				g.Expect(l.Spec.RenewTime).NotTo(BeNil())
				g.Expect(l.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Node",
					Name:               nodeName,
					UID:                types.UID("node-uid"),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				}))
			}).Should(Succeed())

			cancel()
		})
	})
})
