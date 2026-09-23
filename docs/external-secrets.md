# External Secrets integration and rotation — AWCP-25

AWCP integrates with an already-installed External Secrets Operator (ESO). ESO
owns provider access, `SecretStore`, `ExternalSecret` and its target `Secret`.
AWCP owns only the current-UID workload children and never writes ESO resources.

## API contract

```yaml
spec:
  externalSecrets:
    - externalSecret: payments-settings-sync
      targetSecret: payments-settings
```

Both names are DNS-subdomain names in the `AIWorkload` namespace. The referenced
`ExternalSecret` must select `kind: SecretStore`, its target name must equal
`targetSecret`, and both the store and ExternalSecret must expose `Ready=True`.
`ClusterSecretStore`, cross-namespace references, provider credentials, remote
keys and Secret payload are intentionally absent from AWCP's API.

When valid, AWCP appends `targetSecret` after `spec.secretRefs` as a non-optional
`envFrom.secretRef`. It adds a SHA-256 digest of target Secret name and metadata
resourceVersion to its owned Pod-template annotation. Thus an ESO target update
causes a standard Deployment rollout without reading or logging the Secret data.
This is not a guarantee that a provider value is valid, that an application can
reload credentials in place, or that direct `secretRefs` updates rotate Pods.

## Failure model

`ExternalSecretsReady=False` is written alongside the normal lifecycle
conditions. Stable reasons are `ExternalSecretsUnavailable`, `ExternalSecretNotFound`,
`ExternalSecretStoreScopeInvalid`, `SecretStoreNotReady`, `ExternalSecretNotReady`,
and `ExternalSecretTargetNotFound`. Messages are actionable but intentionally omit
provider paths, Secret names and values. Dependency/API-read failures that cannot
be classified are retried through the existing controller backoff.

## Platform and GitOps boundary

Install ESO separately; AWCP does not bundle its CRDs or controller in
`config/default`. A platform GitOps source may declare namespace-local
`SecretStore` and `ExternalSecret` objects after its ESO installation is healthy.
The AWCP manager has only namespaced `get` on those two ESO resources plus its
existing Secret metadata watch. It has no ESO create/update/delete permission and
no provider or cloud identity.

The kind E2E installs the checksum-verified official ESO `v2.11.0` release only
inside its disposable cluster, configures the official fake provider, changes a
synthetic remote version and verifies the target metadata and AWCP rollout. It
never prints the target Secret data.

References: [ESO ExternalSecret API](https://external-secrets.io/main/api/externalsecret/),
[SecretStore API](https://external-secrets.io/main/api/secretstore/), and
[ESO fake provider](https://external-secrets.io/main/provider/fake/).
