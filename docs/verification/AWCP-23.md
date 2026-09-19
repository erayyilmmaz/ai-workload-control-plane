# AWCP-23 — Tenant modeli, namespace’ler, ResourceQuota ve LimitRange

## Teslim edilen sınır

- `spec.tenant` isteğe bağlı ve DNS-label doğrulamalı bir API alanıdır; alan
  yoksa V0 davranışı aynen sürer.
- `WATCH_NAMESPACES` açık bir listeyle `awcp-workloads` ile üç referans tenant
  namespace’ini sınırlar. Eski tekil `WATCH_NAMESPACE` ayarı uyumluluk için
  desteklenir, fakat ikisi birlikte kabul edilmez.
- Tenant kullanan bir AIWorkload, yalnızca kendi namespace’inde GitOps’un
  yönettiği `awcp-tenant-profile` ConfigMap’iyle eşleşir. Eşleşmezse çocuk
  kaynak oluşturulmadan `TenantReady=False` ve güvenli bir neden üretilir.
- `small`, `medium`, `large` profilleri, ayrı namespace ResourceQuota,
  LimitRange, object-count quota, default-deny ingress ve tenant-developer
  Role/RoleBinding’leriyle teslim edildi.
- Manager ve workload ServiceAccount’ları için cluster-wide workload/Secret/
  quota izni eklenmedi. Manager’ın ConfigMap izni yalnızca sabit profile adı
  için `get`tir; workload ServiceAccount’larına RoleBinding yoktur.

## Yerel kanıt

| Komut | Sonuç | Kapsam |
| --- | --- | --- |
| `make generate manifests fmt test-unit` | geçti | API, controller tenant/profile mismatch, quota hata sınıflaması, çoklu scope, manifest ve kaynak etiketleri |
| `make test-envtest` | geçti | gerçek API sunucusunda CRD strict validation ve mevcut reconciliation regresyonları |
| `make lint fmt-check verify-generated render gitops-verify verify-docs` | geçti | format/lint, generated drift, Kustomize/GitOps render ve doküman sözleşmesi |
| `git diff --check` ve `bash -n test/e2e/lifecycle-e2e.sh` | geçti | boşluk ve E2E shell sözleşmesi |
| `make e2e` | yerelde çalıştırılamadı | Yerel Docker daemon socket’i erişilemediği için image build aşamasında durdu; kind cluster oluşturulmadı |

Kind E2E betiği tenant Role sınırını, tenant-bravo Secret’ının
tenant-alpha’da görünmemesini, workload ServiceAccount’ının Secret okuyamamasını,
LimitRange reddini ve ResourceQuota reddini doğrular. Bu canlı kabul kanıtı Docker
erişimi olan GitHub Actions `e2e` job’ında veya Docker başlatılmış yerelde ayrıca
gözlemlenmelidir.

## Açık sınırlar

Bu referans tek cluster modelidir. Namespace ve RBAC izolasyonunu kanıtlar; gerçek
paket ağ trafiği izolasyonu CNI’nin NetworkPolicy uygulamasına bağlıdır ve kind
varsayılan ağında üretim CNI kanıtı sayılmaz. Tenant onboarding/değişiklikleri
GitOps platform değişikliğidir; bir AIWorkload tenant namespace’i ya da yetkisi
seçemez.
