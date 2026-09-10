// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"context"
	"fmt"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	leasecontroller "k8s.io/component-helpers/apimachinery/lease"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Runnable renews the gardener-node-agent's heartbeat Lease in the kube-system namespace of
// the shoot. It uses the upstream k8s.io/component-helpers lease controller - the same
// mechanism the kubelet uses for its node heartbeat Lease - which reads and renews the Lease
// via a direct (uncached) client-go client and re-reads live on conflict. This avoids the
// failure mode of a controller-runtime cached client, whose informer cache can freeze on a
// half-open watch stream and then drive a stale-resourceVersion 409 backoff storm.
type Runnable struct {
	RESTConfig *rest.Config
	// ClientSet is the client-go clientset used to read and renew the Lease. It is a direct
	// (uncached) client. If nil, it is built from RESTConfig in Start. It is primarily exposed
	// for testing with a fake clientset.
	ClientSet            kubernetes.Interface
	NodeName             string
	Namespace            string
	LeaseDurationSeconds int32
	Clock                clock.Clock
}

// Start builds a live client-go clientset, wires up the component-helpers lease controller and
// runs it until the context is cancelled.
func (r *Runnable) Start(ctx context.Context) error {
	log := logf.FromContext(ctx).WithName(ControllerName)

	clientSet := r.ClientSet
	if clientSet == nil {
		var err error
		clientSet, err = kubernetes.NewForConfig(r.RESTConfig)
		if err != nil {
			return fmt.Errorf("failed creating clientset for lease controller: %w", err)
		}
	}

	// Fetch the Node once via a live read to obtain its UID for the owner reference. The Lease is
	// owned by the Node so that it is garbage-collected when the Node is deleted.
	node, err := clientSet.CoreV1().Nodes().Get(ctx, r.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed getting node %q for lease controller: %w", r.NodeName, err)
	}

	leaseName := gardenerutils.NodeAgentLeaseName(r.NodeName)

	controller := leasecontroller.NewController(
		r.Clock,
		clientSet,
		leaseName,
		r.LeaseDurationSeconds,
		nil,
		time.Duration(r.LeaseDurationSeconds)*time.Second/4,
		leaseName,
		r.Namespace,
		newLeasePostProcessFunc(node),
	)

	log.Info("Starting lease controller", "leaseName", leaseName, "namespace", r.Namespace, "leaseDurationSeconds", r.LeaseDurationSeconds)
	controller.Run(ctx)
	return nil
}

// newLeasePostProcessFunc returns a ProcessLeaseFunc that sets the given Node as the controlling
// owner of the Lease. This mirrors the behavior of the previous reconciler which used
// controllerutil.SetControllerReference(node, lease, ...).
func newLeasePostProcessFunc(node *corev1.Node) leasecontroller.ProcessLeaseFunc {
	return func(lease *coordinationv1.Lease) error {
		lease.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         corev1.SchemeGroupVersion.String(),
			Kind:               "Node",
			Name:               node.GetName(),
			UID:                node.GetUID(),
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		}}
		return nil
	}
}
