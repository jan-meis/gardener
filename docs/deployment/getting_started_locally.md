# Deploying Gardener Locally

This document will walk you through deploying Gardener on your local machine.
If you encounter difficulties, please open an issue so that we can make this process easier.

## Overview

Gardener runs in any Kubernetes cluster.
In this guide, we will start a [KinD](https://kind.sigs.k8s.io/) cluster which is used as both garden and seed cluster (please refer to the [architecture overview](../concepts/architecture.md)) for simplicity.

Based on [Skaffold](https://skaffold.dev/), the container images for all required components will be built and deployed into the cluster (via their [Helm charts](https://helm.sh/)).

![Architecture Diagram](content/getting_started_locally.png)

## Alternatives

When deploying Gardener on your local machine you might face several limitations:

- Your machine doesn't have enough compute resources (see [prerequisites](#prerequisites)) for hosting a second seed cluster or multiple shoot clusters.
- Testing Gardener's [IPv6 features](../development/ipv6.md) requires a Linux machine and native IPv6 connectivity to the internet, but you're on macOS or don't have IPv6 connectivity in your office environment or via your home ISP.

In these cases, you might want to check out one of the following options that run the setup described in this guide elsewhere for circumventing these limitations:

- [remote local setup](#remote-local-setup): deploy on a remote pod for more compute resources
- [dev box on Google Cloud](https://github.com/gardener-community/dev-box-gcp): deploy on a Google Cloud machine for more compute resource and/or simple IPv4/IPv6 dual-stack networking

## Prerequisites

- Make sure that you have followed the [Local Setup guide](../development/local_setup.md) up until the [Get the sources](../development/local_setup.md#get-the-sources) step.
- Make sure your Docker daemon is up-to-date, up and running and has enough resources (at least `8` CPUs and `16Gi` memory; see [here](https://docs.docker.com/desktop/mac/#resources) how to configure the resources for Docker for Mac).
  > Please note that 8 CPU / 16Gi memory might not be enough for more than two `Shoot` clusters, i.e., you might need to increase these values if you want to run additional `Shoot`s.
  > If you plan on following the optional steps to [create a second seed cluster](#optional-setting-up-a-second-seed-cluster), the required resources will be more - at least `10` CPUs and `32Gi` memory.
  Additionally, please configure at least `120Gi` of disk size for the Docker daemon.
  > Tip: You can clean up unused data with `docker system df` and `docker system prune -a`.

## Setting Up the KinD Cluster (Garden and Seed)

```bash
make kind-up
```

> If you want to set up an IPv6 KinD cluster, use `make kind-up IPFAMILY=ipv6` instead.

This command sets up a new KinD cluster named `gardener-local` and stores the kubeconfig in the `./dev-setup/kubeconfigs/runtime/kubeconfig` file.

> It might be helpful to copy this file to `$HOME/.kube/config`, since you will need to target this KinD cluster multiple times.
Alternatively, make sure to set your `KUBECONFIG` environment variable to `./dev-setup/kubeconfigs/runtime/kubeconfig` for all future steps via `export KUBECONFIG=$PWD/dev-setup/kubeconfigs/runtime/kubeconfig`.

After [setting up Gardener](#setting-up-gardener), the kubeconfig for the virtual garden cluster will be available at `./dev-setup/kubeconfigs/virtual-garden/kubeconfig`.

All following steps assume that you are using the runtime cluster kubeconfig.

Additionally, this command also deploys a local container registry to the cluster, as well as a few registry mirrors, that are set up as a pull-through cache for all upstream registries Gardener uses by default.
This is done to speed up image pulls across local clusters.

The local registry can now be accessed either via `localhost:5001` or `registry.local.gardener.cloud:5001` for pushing and pulling.
The storage directories of the registries are mounted to the host machine under `dev/local-registry`.
With this, mirrored images don't have to be pulled again after recreating the cluster.

It may also be necessary to mark the registry in Docker as an insecure registry to ensure that Docker establishes the connection via HTTP. This can be achieved by adding the registry accordingly in the `/etc/docker/daemon.json` file:
```json
{ "insecure-registries":["registry.local.gardener.cloud:5001"] }
``` 

The command also deploys a default [calico](https://github.com/projectcalico/calico) installation as the cluster's CNI implementation with `NetworkPolicy` support (the default `kindnet` CNI doesn't provide `NetworkPolicy` support).
Furthermore, it deploys the [metrics-server](https://github.com/kubernetes-sigs/metrics-server) in order to support HPA and VPA on the seed cluster.

After a restart of your host, the loopback IP addresses that were created for the kind cluster could be removed.
To avoid recreating the cluster, there is a script that can be used to easily bring back the addresses.
```bash
./dev-setup/infra.sh setup-loopback-devices
```

> Depending on the cluster you are using for your local dev setup you might have to:
>  - Replace the value for `--cluster-name` with the name of the kind cluster you are using for your local dev setup.
>  - Add the `--ip-family <ipv4|ipv6|dual>` flag if you want to set up `ipv6` or `dual` stack IPs (`ipv4` is the default value).
>  - Add the `--multi-zonal` flag if you want to set up IPs for a multi-zonal cluster.

## Setting Up IPv6 Single-Stack Networking (optional)

We need to configure NAT for outgoing traffic from the kind network to the internet.
After executing `make kind-up IPFAMILY=ipv6`, execute the following command to set up the corresponding iptables rules:

```bash
ip6tables -t nat -A POSTROUTING -o $(ip route | grep '^default') -s fd00:10::/64 -j MASQUERADE
```

## Configure Feature Gates (Optional)

You can optionally configure [feature gates](./feature_gates.md) on different components of your local Gardener deployment. The feature gates can be set by editing respective resource manifests before [setting up Gardener](#setting-up-gardener). They can also be configured dynamically on the running deployment by editing respective resources on the appropriate clusters, or by running relevant `make` targets after updating the `featureGates` configurations in respective files. The files, `make` targets and the resources to edit for configuring feature gates on respective Gardener components are detailed in the table below.

| Gardener components                                    | Resource manifests                                                                                                | Respective `make` targets      | Respective resources                               |
|--------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|--------------------------------|----------------------------------------------------|
| `gardener-operator`                                    | [`charts/gardener/operator/values.yaml` ]( ../../charts/gardener/operator/values.yaml ) ( `config.featureGates` ) | `operator-up` / `operator-dev` |  N/A                                               |
| `gardener-apiserver` and `gardener-controller-manager` | [`dev-setup/garden/base/garden.yaml`](../../dev-setup/garden/base/garden.yaml)                                    | `garden-up`                    | `garden` resource on the runtime cluster           |
| `gardenlet`                                            | [`dev-setup/gardenlet/base/gardenlet.yaml`](../../dev-setup/gardenlet/base/gardenlet.yaml)                        | `seed-up` / `seed-dev`         | `gardenlet` resource on the virtual garden cluster |

## Setting Up Gardener

```bash
make gardener-up
```

> If you want to set up an IPv6 ready Gardener, use `make gardener-up IPFAMILY=ipv6` instead.

This will first build the base images (which might take a bit if you do it for the first time).
Afterwards, the Gardener resources will be deployed into the cluster.

## Developing Gardener

```bash
make gardener-dev
```

This is similar to `make gardener-up` but additionally starts a [skaffold dev loop](https://skaffold.dev/docs/workflows/dev/).
After the initial deployment, skaffold starts watching source files.
Once it has detected changes, press any key to trigger a new build and deployment of the changed components.

Tip: you can set the `SKAFFOLD_MODULE` environment variable to select specific modules of the skaffold configuration (see [`skaffold.yaml`](../../skaffold.yaml)) that skaffold should watch, build, and deploy.
This significantly reduces turnaround times during development.

For example, if you want to develop changes to gardenlet:

```bash
# initial deployment of all components
make gardener-up
# start iterating on gardenlet without deploying other components
make gardener-dev SKAFFOLD_MODULE=gardenlet
```

## Debugging Gardener

Debugging a Gardener component involves three steps: deploy it with debugging settings, attach [Delve](https://github.com/go-delve/delve) to its running process, and connect your IDE to Delve.
The local cloud-controller-manager uses a different workflow, described [below](#debugging-the-local-cloud-controller-manager).

> [!WARNING]
> The debug setup disables health probes and leader election and uses a single replica so that components can remain paused at breakpoints.
> Use it only in development environments, never in production.

### Deploying components for debugging

The following commands assume that you have [set up the KinD cluster](#setting-up-the-kind-cluster-garden-and-seed) and are running them from the repository root.
The full debug setup supports only the default IPv4 `single-node` scenario created by `make kind-up`, not the IPv6, dual-stack, multi-node, or multi-zone variants.

To deploy the full Gardener stack with debugging settings:

```bash
export KUBECONFIG="$PWD/dev-setup/kubeconfigs/runtime/kubeconfig"
make gardener-debug
```

The debug profiles build the supported components on the Skaffold debug support image, which includes Delve, without changing their entrypoints.
They disable Go optimizations and inlining and preserve local source paths for stepping through code in your IDE.

`make gardener-debug` combines `operator-debug`, `garden-debug`, and `seed-debug`.
These targets return after deployment; they do not start Delve, forward ports, or watch for source changes.
Wait for deployment to finish before attaching a debugger.

If you already have a running Gardener setup and want to debug only part of it, use the targets below instead:

| Component | Deployment commands |
|-----------|---------------------|
| `gardener-operator` | `make operator-debug` |
| `gardener-apiserver`, `gardener-controller-manager`, `gardener-scheduler`, `gardener-admission-controller` | `make operator-debug`, then `make garden-debug` |
| `gardenlet` | `make seed-debug` |

`operator-debug` builds the gardener-operator and the images of the four Gardener components it deploys.
`garden-debug` applies the [`Garden` debug overlay](../../dev-setup/garden/overlays/debug/kustomization.yaml): it disables health probes, disables leader election for the gardener-controller-manager and gardener-scheduler, and sets all four components to one replica.
These settings are configured through `spec.virtualCluster.gardener.<component>.deployment` in the `Garden` resource (see the [`Garden` API reference](../api-reference/operator.md)).
Both targets are required to debug an operator-managed component: `garden-debug` alone does not build debug images, and `operator-debug` alone does not change these components' probe or leader-election settings.

`seed-debug` deploys the gardenlet with the [`Gardenlet` debug overlay](../../dev-setup/gardenlet/overlays/debug/kustomization.yaml), which disables its health probes and leader election.

### Attaching the debugger

Use the [`attach-debugger-gardener-component.sh`](../../hack/attach-debugger-gardener-component.sh) helper to start Delve in the component's container and forward its port to your machine.
The helper uses your current `KUBECONFIG` and looks for pods in the runtime cluster's `garden` namespace, not in the virtual garden cluster.
If you open a new terminal, set the runtime kubeconfig there as well.

For example, to debug the gardener-controller-manager:

```bash
export KUBECONFIG="$PWD/dev-setup/kubeconfigs/runtime/kubeconfig"
./hack/attach-debugger-gardener-component.sh gardener-controller-manager
```

Replace `gardener-controller-manager` with any component listed in the table above.
The deployment targets also print the applicable helper commands.

Keep the helper running while you debug, and connect your IDE to `127.0.0.1:2345` as described below for [GoLand](#debugging-in-goland) or [VS Code](#debugging-in-vs-code).
To debug another component at the same time, run a second helper in a separate terminal with a different port:

```bash
./hack/attach-debugger-gardener-component.sh -p 2346 gardenlet
```

Connect the second IDE configuration to `127.0.0.1:2346`.
Ports are not assigned automatically by the helper.

The component is already running when Delve attaches, so breakpoints in startup code will not catch code that has already executed.
After changing source code, rerun the relevant deployment targets and attach to the new pod again.

### Behavior at breakpoints

Pausing at a breakpoint pauses the whole Go process, not just the selected goroutine (see Go issues [25578](https://github.com/golang/go/issues/25578) and [31132](https://github.com/golang/go/issues/31132)).
The component cannot respond to health probes or renew its leader-election lease while paused, which is why the debug setup disables them.
Other components can still log errors or time out while waiting for the paused component.

> [!NOTE]
> The gardener-apiserver is an aggregated API server.
> While it is paused, requests to the API groups it serves (e.g. `core.gardener.cloud`) through the virtual garden's kube-apiserver cannot complete.
> This also affects `kubectl` commands and controllers accessing these APIs.

### Debugging in GoLand

1. Open the Gardener repository in GoLand and edit your **Run/Debug Configurations**.
2. Add a **Go Remote** configuration.
3. Set **Host** to `127.0.0.1` and **Port** to the port forwarded by the helper (`2345` by default).
4. _Recommended:_ Change the behavior of **On disconnect** to **Leave it running**.
5. Start the debug configuration, set breakpoints, and trigger the operation you want to inspect.

### Debugging in VS Code

Open the Gardener repository in VS Code with the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.Go) installed.
Create `.vscode/launch.json` with the following contents, or add the configuration to its existing `configurations` array:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Gardener remote",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "debugAdapter": "dlv-dap",
      "host": "127.0.0.1",
      "port": 2345
    }
  ]
}
```

Set `port` to the value passed to the helper if you are not using `2345`.
Select **Gardener remote** in **Run and Debug**, start debugging, and trigger the operation you want to inspect.

The [ko builder](https://skaffold.dev/docs/builders/builder-types/ko/) compiles the binaries locally, and the debug profiles preserve these source paths.
No source-path mapping is needed when your IDE uses the same checkout as the build.
If you build from a different path or machine, configure `substitutePath` to map your workspace to the source location used during compilation (see the [VS Code Go remote debugging documentation](https://github.com/golang/vscode-go/blob/master/docs/debugging.md#remote-debugging)).

### Debugging the local cloud-controller-manager

The `cloud-provider-local-debug` target debugs the local cloud-controller-manager, not the gardener-extension-provider-local controller.
Unlike the core debug targets, it uses `skaffold debug`, which starts the process under Delve and forwards its debug port automatically:

```bash
export KUBECONFIG="$PWD/dev-setup/kubeconfigs/runtime/kubeconfig"
make cloud-provider-local-debug
```

Keep this command running and connect your IDE to the local port printed by Skaffold (normally `56268`).
Do not run the attach helper for this component.
For VS Code, use the configuration above with that port.
See the [Skaffold debugging documentation](https://skaffold.dev/docs/workflows/debug/) for more details.

### Returning to the normal setup

Resume any paused processes, disconnect your IDE, and stop the attach helpers before redeploying.
To restore the regular images, health probes, and leader-election settings, run:

```bash
make gardener-up
```

If you also used `cloud-provider-local-debug`, restore the local cloud-controller-manager with `make cloud-provider-local-up`.

## Creating a `Shoot` Cluster

> [!NOTE]
> The following steps assume that you are using the kubeconfig that points to the virtual garden cluster: `export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig`.

You can wait for the `Seed` to be ready by running:

```bash
./hack/usage/wait-for.sh seed local GardenletReady SeedSystemComponentsHealthy ExtensionsReady
```

Alternatively, you can run `kubectl get seed local` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local   Ready    local      local    4m42s   vX.Y.Z-dev    v1.28.1
```

In order to create a first shoot cluster, just run:

```bash
kubectl apply -f example/provider-local/shoot.yaml
```

You can wait for the `Shoot` to be ready by running:

```bash
NAMESPACE=garden-local ./hack/usage/wait-for.sh shoot local APIServerAvailable ControlPlaneHealthy ObservabilityComponentsHealthy EveryNodeReady SystemComponentsHealthy
```

Alternatively, you can run `kubectl -n garden-local get shoot local` and wait for the `LAST OPERATION` to reach `100%`:

```bash
NAME    CLOUDPROFILE   PROVIDER   REGION   K8S VERSION   HIBERNATION   LAST OPERATION            STATUS    AGE
local   local          local      local   1.28.1        Awake         Create Processing (43%)   healthy   94s
```

If you don't need any worker pools, you can create a workerless `Shoot` by running:

```bash
kubectl apply -f example/provider-local/shoot-workerless.yaml
```

(Optional): You could also execute a simple e2e test (creating and deleting a shoot) by running:

```shell
make test-e2e-local-simple
```

### Accessing the `Shoot` Cluster

To access the `Shoot`, you can acquire a `kubeconfig` by using the [`shoots/adminkubeconfig` subresource](../usage/shoot/shoot_access.md#shootsadminkubeconfig-subresource).

For convenience a [helper script](../../hack/usage/generate-kubeconfig.sh) is provided in the `hack` directory. By default, the script will generate an admin kubeconfig for a `Shoot` named "local" in the `garden-local` namespace valid for one hour.

```bash
./hack/usage/generate-kubeconfig.sh > admin-kubeconf.yaml
```

To generate a viewer kubeconfig instead of an admin kubeconfig, use the `--viewer` flag:

```bash
./hack/usage/generate-kubeconfig.sh --viewer > viewer-kubeconf.yaml
```

> [!NOTE]
> Keep in mind that using a VPN on your local machine could cause problems with the setup, and the shoot's kubeconfig could fail with connection issues.
> If you experience connection problems using the shoot's kubeconfig, try disabling the VPN first.

If you want to change the default namespace or shoot name, you can do so by passing different values as arguments.

```bash
./hack/usage/generate-kubeconfig.sh --namespace <namespace> --shoot-name <shootname> > admin-kubeconf.yaml
```

To access an Ingress resource from the `Seed`, use the Ingress host with port `8448` (`https://<ingress-host>:8448`, for example `https://gu-local--local.ingress.local.seed.local.gardener.cloud:8448`).

## (Optional): Setting Up a Second Seed Cluster

There are cases where you would want to create a second seed cluster in your local setup.
For example, if you want to test the [control plane migration](../operations/control_plane_migration.md) feature.
The following steps describe how to do that.

Start by setting up the second KinD cluster:

```bash
make kind2-up
```

This command sets up a new KinD cluster named `gardener-local2` and stores its kubeconfig in the `./dev-setup/kubeconfigs/seed2/kubeconfig` file.

In order to deploy required resources in the KinD cluster that you just created, run:

```bash
make seed2-up
```

The following steps assume that you are using the kubeconfig that points to the virtual garden cluster: `export KUBECONFIG=$PWD/dev-setup/kubeconfigs/virtual-garden/kubeconfig`.

You can wait for the `local2` `Seed` to be ready by running:

```bash
./hack/usage/wait-for.sh seed local2 GardenletReady SeedSystemComponentsHealthy ExtensionsReady
```

Alternatively, you can run `kubectl get seed local2` and wait for the `STATUS` to indicate readiness:

```bash
NAME    STATUS   PROVIDER   REGION   AGE     VERSION       K8S VERSION
local2  Ready    local      local    4m42s   vX.Y.Z-dev    v1.25.1
```

If you want to perform control plane migration, you can follow the steps outlined in [Control Plane Migration](../operations/control_plane_migration.md) to migrate the shoot cluster to the second seed you just created.

## Deleting the `Shoot` Cluster

```shell
./hack/usage/delete shoot local garden-local
```

## (Optional): Tear Down the Second Seed Cluster

``` shell
make kind2-down
```

## Tear Down the Gardener Environment

```shell
make kind-down
```

## Remote Local Setup

Just like Prow is executing the KinD-based e2e tests in a K8s pod, it is
possible to interactively run this KinD based Gardener development environment,
aka "local setup", in a "remote" K8s pod.

```shell
k apply -f docs/deployment/content/remote-local-setup.yaml
k exec -it remote-local-setup-0 -- sh

tmux a
```

### Caveats

Please refer to the [TMUX documentation](https://github.com/tmux/tmux/wiki) for
working effectively inside the remote-local-setup pod.

To access Plutono, Prometheus or other components in a browser, two port forwards are needed:

The port forward from the laptop to the pod:

```shell
k port-forward remote-local-setup-0 3000
```

The port forward in the remote-local-setup pod to the respective component:

```shell
k port-forward -n shoot--local--local deployment/plutono 3000
```

## Related Links

- [Local Provider Extension](../extensions/provider-local.md)
