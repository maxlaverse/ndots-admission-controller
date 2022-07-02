# ndots-admission-controller

![Tests](https://github.com/maxlaverse/ndots-admission-controller/actions/workflows/tests.yml/badge.svg?branch=main)
![Go Version](https://img.shields.io/github/go-mod/go-version/maxlaverse/ndots-admission-controller)
![Releases](https://img.shields.io/github/v/release/maxlaverse/ndots-admission-controller?include_prereleases)

A [Kubernetes Admission Controller] that injects `spec.dnsConfig.options.ndots:1` inside Pods upon creation (if not present).
This prevents most of the DNS resolvers from issuing extra DNS requests based on the local search list, which improves performance at the expense of portability.

## Supported Versions

The controller has been tested and built with the following components:
* Kubernetes >= 1.19
* Cert-Manager >= 1.4.0

## Installation

```bash
helm repo add maxlaverse https://maxlaverse.github.io/helm-charts/
helm repo update
helm install ndots-admission-controller maxlaverse/ndots-admission-controller
```

## What is ndots ?

From [man resolv.conf]
> ndots:n
>
> Sets a threshold for the number of dots which must appear in a name [...] before an initial absolute query will be made.
> The default for n is 1, meaning that if there are any dots in a name, the name will be tried first as an absolute name before any search list elements are appended to it.
> The value for this option is silently capped to 15. 

By default, Kubernetes sets [ndots:5] with at least `<namespace>.svc.cluster.local`, `svc.cluster.local`, and `cluster.local` as search items (see [author's explanation on the number 5])

**Example**:
For a Pod running in the `default` Kubernetes namespace, when trying to resolve `google.com` the DNS resolver would make the following requests:
* `google.com.default.svc.cluster.local`
* `google.com.svc.cluster.local`
* `google.com.cluster.local`
* `google.com`

The first 3 requests are unnecessary as we know the exact name of the domain we want to reach.
To prevent this behavior, we have two possibilities:
1. append a final dot to the hostname, making it fully qualified: `google.com => google.com.`.
2. set `ndots` to `1` or `0`.

The first solution is inconvenient:
* it requires appending the final dot in a lot of different places that are not always easily accessible (e.g. libraries of SaaS providers).
* some load-balancers return SSL errors because the Common Name doesn't match anymore.
* monitoring tools would often treat both `google.com` and `google.com.` as separate domains.

The _ndots-admission-controller_ is an implementation of the second solution.

## License

Distributed under the Apache License. See [LICENSE](./LICENSE) for more information.

[Kubernetes Admission Controller]: https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/
[ndots:5]: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/
[author's explanation on the number 5]: https://github.com/kubernetes/kubernetes/issues/33554#issuecomment-266251056
[man resolv.conf]: https://man5.pgdp.sse.in.tum.de/resolver.5.html