---
title: BackupBucket
---

# Contract: `BackupBucket` Resource

The Gardener project features a sub-project called [etcd-backup-restore](https://github.com/gardener/etcd-backup-restore) to take periodic backups of etcd backing Shoot clusters. It demands the bucket (or its equivalent in different object store providers) to be created and configured externally with appropriate credentials. The `BackupBucket` resource takes this responsibility in Gardener.

Before introducing the `BackupBucket` extension resource, Gardener was using Terraform in order to create and manage these provider-specific resources (e.g., see [AWS Backup](https://github.com/gardener/gardener/tree/0.27.0/charts/seed-terraformer/charts/aws-backup)).
Now, Gardener commissions an external, provider-specific controller to take over this task. You can also refer to [backupInfra proposal documentation](https://github.com/gardener/enhancements/tree/main/geps/0002-backup-infrastructure) to get an idea about how the transition was done and understand the resource in a broader scope.

## What Is the Scope of a Bucket?

A bucket will be provisioned per `Seed`. So, a backup of every `Shoot` created on that `Seed` will be stored under a different shoot specific prefix under the bucket.
A bucket is identified by its name, which is derived from the `Seed`'s UID by default, but can also be set explicitly via `Seed.spec.backup.bucketName`. See [Relocating a Backup Bucket](#relocating-a-backup-bucket) for how the bucket a `Seed` uses can be changed.

## What Is the Lifespan of a `BackupBucket`?

The bucket associated with `BackupBucket` will be created at the creation of the `Seed`. And as per current implementation, it will also be deleted on deletion of the `Seed`, if there isn't any `BackupEntry` resource associated with it.

In the future, we plan to introduce a schedule for `BackupBucket` - the deletion logic for the `BackupBucket` resource, which will reschedule it on different available `Seed`s on deletion or failure of a health check for the currently associated `seed`. In that case, the `BackupBucket` will be deleted only if there isn't any schedulable `Seed` available and there isn't any associated `BackupEntry` resource.

## Relocating a Backup Bucket

An operator may need to relocate the etcd backup target of a `Seed` to a new cloud provider or a different region.
However, the provider and region of an existing `BackupBucket` resource are immutable. Further, the `provider`, `region`, and `bucketName` fields of the `Seed`'s `.spec.backup` spec are protected against accidental changes.

To change the backup target, an operator needs to trigger the creation of a new `BackupBucket` by changing the `bucketName` field and optionally the `provider` and `region` fields on the `Seed` resource.
This change must be confirmed by annotating the `Seed` (or, for the virtual garden, the `Garden`) resource:
```
confirmation.gardener.cloud/change-backup=true
```
Without this annotation, `gardener-apiserver` denies any update to these fields.

> [!NOTE]
> This spec-field mutability is unrelated to [Immutable Backup Buckets](../../operations/immutable-backup-buckets.md), which is an opt-in WORM/retention feature protecting the stored snapshot *objects*. If the old bucket is under a **locked** retention policy, it (and its objects) cannot be deleted until the retention period expires, so cleanup after relocation may not be immediate.

### Propagation of the Change
Once confirmed, the new `BackupBucket` will be reconciled by the gardenlet.
The change will take effect in etcd on the next `Shoot` reconciliation (or `Garden` reconciliation for the virtual garden).

### Cleaning up Leftover Buckets
Deleting the old bucket is an operator-driven decision and is done only when the operator is confident it is no longer needed.

### Risks and Recovery

> [!WARNING]
> Relocation triggers a roll of the etcd pod. If the etcd data volume gets corrupted while detaching from the old node or attaching to a new one, the backup sidecar may detect the corruption and restore from the new bucket - which is still empty - resulting in an empty etcd and an unusable shoot cluster.
>
> To recover, revert `bucketName` to the old bucket, delete and recreate the etcd data volume to force a restore from the old bucket, and retry the relocation.
>
> This corruption scenario is rare, but the risk is highest for standalone (non-HA) etcd. Running etcd in an HA setup makes the operation considerably safer, since a single member's volume corruption does not brick the cluster.

## What Needs to Be Implemented to Support a New Infrastructure Provider?

As part of the seed flow, Gardener will create a special CRD in the seed cluster that needs to be reconciled by an extension controller, for example:

```yaml
---
apiVersion: extensions.gardener.cloud/v1alpha1
kind: BackupBucket
metadata:
  name: foo
spec:
  type: azure
  providerConfig:
    <some-optional-provider-specific-backupbucket-configuration>
  region: eu-west-1
  secretRef:
    name: backupprovider
    namespace: shoot--foo--bar
```

The `.spec.secretRef` contains a reference to the provider secret pointing to the account that shall be used to create the needed resources. This provider secret will be configured by the Gardener operator in the `Seed` resource and propagated over there by the seed controller.

After your controller has created the required bucket, if required, it generates the secret to access the objects in the bucket and put a reference to it in `status.generatedSecretRef`.
The secret should be created in the namespace specified in the `backupbucket.extensions.gardener.cloud/generated-secret-namespace` annotation.
In case the annotation is not present, the `garden` namespace should be used.
This secret is supposed to be used by Gardener, or eventually a `BackupEntry` resource and etcd-backup-restore component, for backing up the etcd.

In order to support a new infrastructure provider, you need to write a controller that watches all `BackupBucket`s with `.spec.type=<my-provider-name>`. You can take a look at the below referenced example implementation for the Azure provider.

## References and Additional Resources

* [`BackupBucket` API Reference](../../api-reference/extensions.md#backupbucket)
* [Exemplary Implementation for the Azure Provider](https://github.com/gardener/gardener-extension-provider-azure/tree/master/pkg/controller/backupbucket)
* [`BackupEntry` Resource Documentation](./backupentry.md)
* [Shared Bucket Proposal](https://github.com/gardener/enhancements/tree/main/geps/0002-backup-infrastructure)
