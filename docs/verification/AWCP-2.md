# AWCP-2 — Technical baseline and architecture evidence

Date: 2026-09-12. Milestone type: architecture and source verification; no operator implementation.

## Delivered scope

README entry point; scope/use-case/non-goal document; architecture and resource/field responsibility matrices; naming and ownership contract; API/status truth table; exact version/asset lock; eight ADRs; all-16-story traceability. Original backlog exports remain a historical planning snapshot.

## Upstream observations

| Check | Actual observation | Interpretation |
| --- | --- | --- |
| Kubebuilder v4.15.0 tagged go.mod | Go 1.26.0, controller-runtime v0.24.1, core clients v0.36.0 | Compatible minor for the selected design |
| Kubebuilder v4.15.0 tagged Makefile | controller-tools v0.21.0, Kustomize v5.8.1, golangci-lint v2.12.2 | Exact scaffold tool pins retained |
| Kubebuilder v4.16.0 tagged go.mod | controller-runtime v0.25.0, core clients v0.37.0 | Newer scaffold rejected for this V0 baseline |
| controller-runtime v0.24.1 tag | Commit 3be3f1bf2b2fcc6b5c9510d55c6a9972294653d0 | Immutable setup-envtest source selection |
| Nested setup-envtest go.mod | Module exists at that commit; minimum Go 1.26.0 | Source pin can be used during bootstrap |
| Kubernetes Go proxy version endpoints | All seven selected release-family modules return v0.36.4 metadata | Published versions exist; complete MVS/build still to run |
| kind v0.33.0 release | Explicit v1.36.4 node image with digest and amd64/arm64 support | Pin image explicitly; disregard contradictory introductory default text |
| envtest-v1.36.2 release | darwin-arm64 and linux-amd64 tarballs and asset digests exist | Published availability only |
| Go official release JSON | go1.26.8 archives and SHA-256 for both target hosts | Exact toolchain pin |
| kubectl v1.36.4 official SHA endpoints | Both target host digests returned | Exact client artifact pin |
| Local PATH check | Go/Kubebuilder/Kustomize/golangci-lint absent; Docker/kind/kubectl present | Bootstrap prerequisites identified; no global installation performed |

Source locations and all binary SHA-256 values are in [compatibility](../compatibility.md) and the [lock](../../toolchain.lock.json). GitHub API read commands inspected tagged files and release metadata; no external application code was executed.

## Reproducible source checks

```sh
gh api 'repos/kubernetes-sigs/kubebuilder/contents/testdata/project-v4/go.mod?ref=v4.15.0' --jq '.content | @base64d'
gh api 'repos/kubernetes-sigs/kubebuilder/contents/testdata/project-v4/Makefile?ref=v4.15.0' --jq '.content | @base64d'
gh api repos/kubernetes-sigs/controller-runtime/git/ref/tags/v0.24.1 --jq '{ref,object}'
gh api repos/kubernetes-sigs/kind/releases/tags/v0.33.0 --jq '{tag_name,body}'
gh api repos/kubernetes-sigs/controller-tools/releases/tags/envtest-v1.36.2 --jq '[.assets[] | {name,digest}]'
```

Module availability was checked at `https://proxy.golang.org/k8s.io/<module>/@v/v0.36.4.info` for api, apimachinery, client-go, apiextensions-apiserver, apiserver, component-base and streaming. All returned the selected version.

## Local document checks

Executed a local Node standard-library read-only assertion pass over this repository, followed by `git diff --cached --check`.

| Check | Result |
| --- | --- |
| Markdown documents / balanced fenced blocks | PASS — 17 documents |
| ADR structure and required sections | PASS — 8 records |
| Local Markdown link targets | PASS — 53 existing targets |
| Lock JSON parsing and exact core pins | PASS — 7 client modules at v0.36.4 |
| Immutable revisions / node digest / asset checksum shape | PASS — 2 platform sets, 10 published binary asset records |
| Original backlog coverage / forward-only dependencies | PASS — 16 stories, 31 dependency edges |
| Staged whitespace/errors | PASS — `git diff --cached --check` returned exit code 0 |

The assertion pass parsed JSON with `JSON.parse`, checked required ADR sections and internal targets with filesystem reads, balanced code fences, compared exact selected pin values, checked SHA/revision formats and confirmed every dependency points to an earlier source step. It did not fetch URLs, execute binaries or pretend to test controller behavior. The final staged file review includes the pre-existing backlog exports created for this project.

## Runtime evidence deliberately not claimed

No repository go.mod/scaffold exists yet. No Go build, local binary checksum download, envtest process, Docker image build, Kubernetes cluster, hosted CI or release has run. Source comparison uses the selected upstream scaffold; AWCP-3 must generate the project's scaffold and apply the documented exact-patch adjustments before its own build gate.

This satisfies AWCP-2's design/source/availability scope. AWCP-3 owns actual tool installation and generated-project compatibility; AWCP-14 owns real kind behavior. The [traceability matrix](../traceability.md) makes these boundaries explicit.

## Next step

AWCP-3: bootstrap from Kubebuilder v4.15.0 in a scratch directory, preserve these documents, install project-local pinned tools, resolve the selected module graph, generate AIWorkload API/controller scaffolds, and run the actual build/test/lint gates. Carry the relevant envtest teardown and safe cleanup corrections noted in compatibility.md. No webhook/cert-manager requirement is introduced.
