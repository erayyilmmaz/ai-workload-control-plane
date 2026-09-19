# AWCP-24 — Policy profile ve ValidatingAdmissionPolicy

## Teslim edilen sınır

- İsteğe bağlı `spec.policy` alanı yalnızca `baseline` veya `restricted` kabul
  eder; kullanıcı CEL’i, policy URL’si veya controller bypass seçeneği yoktur.
- GitOps’un kurduğu `ValidatingAdmissionPolicy` yalnızca AWCP AIWorkload CREATE/
  UPDATE isteklerini hedefler. Binding, `restricted` etiketli tenant
  namespace’lerine `Deny` uygular.
- Restricted profil; `policy: restricted`, tenant kimliği, CPU/memory request ve
  limitleri ile `network.enabled: true` gerektirir.
- Manager’a admissionregistration yetkisi, ClusterRole veya policy reconcile
  sorumluluğu verilmedi. Policy/binding GitOps’un cluster-scoped paketidir.
- Paket kaldırılırken policy/binding de kaldırılır; CRD ve Namespace’ler önceki
  güvenli uninstall sözleşmesiyle korunmaya devam eder.

## Yerel kanıt

| Komut | Sonuç | Kapsam |
| --- | --- | --- |
| `make generate manifests fmt test-unit` | geçti | API enum, generated CRD ve temel regresyon |
| `make test-envtest` | geçti | Gerçek API server’da policy/binding kurulumunun deny, compliant restricted allow ve V0 namespace allow davranışı |
| `make verify` | geçti | Build, vet, lint, unit, generated-artifact, doküman, GitOps ve Terraform statik doğrulamaları |
| `make e2e` | çalıştırıldı; ortam engeli | Docker daemon socket’i (`~/.docker/run/docker.sock`) erişilebilir olmadığından kind cluster oluşturulamadı; policy E2E sonucu iddia edilmez |

Hosted CI sonucu bu story’nin commit/push adımından sonra ayrıca kaydedilmelidir.
