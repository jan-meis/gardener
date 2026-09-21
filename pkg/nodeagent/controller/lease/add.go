// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ControllerName is the name of the controller.
const ControllerName = "lease"

// AddToManager adds the lease Runnable with the default options to the manager.
func (r *Runnable) AddToManager(mgr manager.Manager, nodeName string) error {
	if r.RESTConfig == nil {
		r.RESTConfig = mgr.GetConfig()
	}
	if r.NodeName == "" {
		r.NodeName = nodeName
	}
	if r.LeaseDurationSeconds == 0 {
		r.LeaseDurationSeconds = 40
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Namespace == "" {
		r.Namespace = metav1.NamespaceSystem
	}

	return mgr.Add(r)
}
