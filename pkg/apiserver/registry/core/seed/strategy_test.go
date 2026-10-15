// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apiserver/registry/core/seed"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForCreate", func() {
		It("should default an omitted backup bucket name to the assigned UID", func() {
			seed := &core.Seed{
				ObjectMeta: metav1.ObjectMeta{UID: "seed-uid"},
				Spec:       core.SeedSpec{Backup: &core.Backup{}},
			}

			strategy.PrepareForCreate(ctx, seed)

			Expect(seed.Spec.Backup.BucketName).To(Equal(ptr.To("seed-uid")))
			Expect(seed.Generation).To(Equal(int64(1)))
		})

		It("should preserve an explicit backup bucket name", func() {
			seed := &core.Seed{
				ObjectMeta: metav1.ObjectMeta{UID: "seed-uid"},
				Spec:       core.SeedSpec{Backup: &core.Backup{BucketName: ptr.To("custom-bucket")}},
			}

			strategy.PrepareForCreate(ctx, seed)

			Expect(seed.Spec.Backup.BucketName).To(Equal(ptr.To("custom-bucket")))
		})

		It("should keep backup disabled when no backup configuration is supplied", func() {
			seed := &core.Seed{}

			strategy.PrepareForCreate(ctx, seed)

			Expect(seed.Spec.Backup).To(BeNil())
		})
	})

	Describe("#PrepareForUpdate", func() {
		var oldSeed, newSeed *core.Seed

		BeforeEach(func() {
			oldSeed = &core.Seed{}
			newSeed = &core.Seed{}
		})

		It("should preserve the status", func() {
			newSeed.Status = core.SeedStatus{KubernetesVersion: new("1.2.3")}
			strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
			Expect(newSeed.Status).To(Equal(oldSeed.Status))
		})

		DescribeTable("backup bucket name defaulting",
			func(oldBackup *core.Backup, bucketName *string, expectedName string) {
				oldSeed.UID = "seed-uid"
				oldSeed.Spec.Backup = oldBackup
				newSeed = oldSeed.DeepCopy()
				newSeed.Spec.Backup = &core.Backup{BucketName: bucketName}

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Spec.Backup.BucketName).To(Equal(ptr.To(expectedName)))
				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
				Expect(oldSeed.Spec.Backup).To(Equal(oldBackup))
			},
			Entry("should populate the default for an older Seed", &core.Backup{}, nil, "seed-uid"),
			Entry("should populate the default when enabling backups", nil, nil, "seed-uid"),
			Entry("should preserve an explicit name", &core.Backup{}, ptr.To("custom-bucket"), "custom-bucket"),
		)

		Context("generation increment", func() {
			It("should not bump the generation if nothing changed", func() {
				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump the generation if the spec changed", func() {
				newSeed.Spec.Provider.Type = "foo"

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the deletionTimestamp was set", func() {
				now := metav1.Now()
				newSeed.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should not bump the generation if the deletionTimestamp was already set", func() {
				now := metav1.Now()
				oldSeed.DeletionTimestamp = &now
				newSeed.DeletionTimestamp = &now

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})

			It("should bump the generation if the operation annotation was set to renew-garden-access-secrets", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the operation annotation was set to renew-kubeconfig", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-kubeconfig")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation if the operation annotation was set to renew-workload-identity-tokens", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-workload-identity-tokens")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
			})

			It("should bump the generation and remove the annotation if the operation annotation was set to reconcile", func() {
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "reconcile")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
				Expect(newSeed.Annotations).NotTo(ContainElement("gardener.cloud/operation"))
			})

			It("should not bump the generation if the operation annotation didn't change", func() {
				metav1.SetMetaDataAnnotation(&oldSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")

				strategy.PrepareForUpdate(ctx, newSeed, oldSeed)

				Expect(newSeed.Generation).To(Equal(oldSeed.Generation))
			})
		})
	})

	Describe("#Canonicalize", func() {
		var seed *core.Seed

		BeforeEach(func() {
			seed = &core.Seed{}
		})

		It("should add the labels for the seed provider and region", func() {
			seed.Spec = core.SeedSpec{Provider: core.SeedProvider{Type: "provider-type", Region: "provider-region"}}
			strategy.Canonicalize(seed)

			Expect(seed.Labels).To(And(
				HaveKeyWithValue("seed.gardener.cloud/provider", "provider-type"),
				HaveKeyWithValue("seed.gardener.cloud/region", "provider-region"),
			))
		})
	})
})
