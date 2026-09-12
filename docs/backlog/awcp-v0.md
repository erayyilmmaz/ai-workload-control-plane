# AWCP V0 — Jira backlog aktarımı

Doğrulama: 12 Eylül 2026. Durum: backlog hazır; geliştirme başlamadı.

[Epic AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1) altında 16 Story oluşturuldu. 31 Blocks ilişkisi geri okunarak yönleri doğrulandı. Her Story'de 8 ortak bölüm, gerçek Jira bağımlılık bağlantıları, kabul kriterleri, 3 adımlı doğrulama planı ve teknik kaynaklar var.

## Geliştirme sırası

| Sıra | Jira | Çalışma | Ön koşullar |
| --- | --- | --- | --- |
| 01 | [AWCP-2](https://erayyilmmaz.atlassian.net/browse/AWCP-2) | Technical baseline, scope ve architecture decisions | Yok |
| 02 | [AWCP-3](https://erayyilmmaz.atlassian.net/browse/AWCP-3) | Repository foundation ve Kubebuilder project bootstrap | AWCP-2 |
| 03 | [AWCP-4](https://erayyilmmaz.atlassian.net/browse/AWCP-4) | AIWorkload Custom Resource API ve validation contract | AWCP-3 |
| 04 | [AWCP-5](https://erayyilmmaz.atlassian.net/browse/AWCP-5) | Reconciliation engine ve resource ownership modeli | AWCP-4 |
| 05 | [AWCP-6](https://erayyilmmaz.atlassian.net/browse/AWCP-6) | Deployment reconciliation ve workload lifecycle | AWCP-5 |
| 06 | [AWCP-7](https://erayyilmmaz.atlassian.net/browse/AWCP-7) | Service discovery ve exposure lifecycle | AWCP-6 |
| 07 | [AWCP-8](https://erayyilmmaz.atlassian.net/browse/AWCP-8) | Workload identity, Secret references ve least-privilege security | AWCP-6 |
| 08 | [AWCP-9](https://erayyilmmaz.atlassian.net/browse/AWCP-9) | Network isolation ve NetworkPolicy reconciliation | AWCP-6 |
| 09 | [AWCP-10](https://erayyilmmaz.atlassian.net/browse/AWCP-10) | Status Conditions, failure model ve Kubernetes Events | AWCP-6, AWCP-7, AWCP-8, AWCP-9 |
| 10 | [AWCP-11](https://erayyilmmaz.atlassian.net/browse/AWCP-11) | Controller observability ve metrics | AWCP-5, AWCP-10 |
| 11 | [AWCP-12](https://erayyilmmaz.atlassian.net/browse/AWCP-12) | Deletion, garbage collection ve lifecycle edge cases | AWCP-6, AWCP-7, AWCP-8, AWCP-9, AWCP-10 |
| 12 | [AWCP-13](https://erayyilmmaz.atlassian.net/browse/AWCP-13) | Unit + envtest integration test suite | AWCP-6, AWCP-7, AWCP-8, AWCP-9, AWCP-10, AWCP-11, AWCP-12 |
| 13 | [AWCP-14](https://erayyilmmaz.atlassian.net/browse/AWCP-14) | kind-based end-to-end test environment | AWCP-13 |
| 14 | [AWCP-15](https://erayyilmmaz.atlassian.net/browse/AWCP-15) | Packaging ve developer installation experience | AWCP-14 |
| 15 | [AWCP-16](https://erayyilmmaz.atlassian.net/browse/AWCP-16) | CI quality gates ve supply-chain hygiene | AWCP-13, AWCP-14 |
| 16 | [AWCP-17](https://erayyilmmaz.atlassian.net/browse/AWCP-17) | Documentation, portfolio demo ve V0 release | AWCP-15, AWCP-16 |

## Kontrol sonucu

- 1 Epic + 16 Story; tümü To Do, doğru parent altında; mevcut Rank sırası 01..16 ile uyumlu.
- 31 bağımlılık, yön ve sayı bakımından beklenen graph ile aynı; döngü yok.
- 17 açıklama yazılan içerikle biçim normalizasyonu sonrası birebir eşleşti.
- 16 Story'nin ADF ve Jira rendered HTML çıktısında 8 ana başlık, sıralı test listesi ve beklenen bütün kod blokları doğrulandı. Tarayıcı/pencere açılmadı; görsel ekran kontrolü yapılmadı.
- Kaynak taslaktaki 394 liste maddesi kontrol edildi. Ağsız test cümlesi ilk bağımlılık hazırlığı ayrımıyla düzeltildi; diğer liste maddeleri korundu.
- envtest GC beklentisi kind E2E'ye taşındı; Secret watch/restore, ownership collision ve status generation sınırları tamamlandı.
- Status YAML örneği required Condition alanları ve ortak endpoint/reason contract'ına uyarlandı.
- Eksik manager deploy adımı quick start'a eklendi; CI sonucu ile branch protection ayrıldı.
- Güncel sürüm gözlemleri kaynaklandırıldı; proje üzerinde henüz çalıştırılmamış uyumluluk kontrolü 01. Story'nin kapanış şartıdır.
- Kaynak tahminlerinin toplamı 29–39 saattir. 18–20 çalışma günü, haftada beş gün ile yaklaşık dört iş haftasıdır.

## Sonraki adım

[AWCP-2 — Teknik baseline ve mimari kararlar](https://erayyilmmaz.atlassian.net/browse/AWCP-2) üzerinden başlanır. İlk Story 01 numarasını taşır; AWCP-1 epic numarasıdır. Geliştirme sırasında Jira açıklamalarındaki kabul kriterleri ve test kanıtları takip edilir. Bu aktarım kod geliştirme, test koşumu, Git işlemi veya release yayını değildir.

## Tam içerik

## 1. Amaç ve mimari

 Developer'ın tek bir namespaced `AIWorkload` kaynağı tanımlamasıyla güvenli ve gözlemlenebilir bir workload'un Kubernetes üzerinde oluşturulmasını, güncellenmesini, izlenmesini, drift durumunda yeniden reconcile edilmesini ve silinmesini sağlayan Go tabanlı Kubernetes operator geliştirmek.

V0 ana akışı:

```text
Developer
   ↓
AIWorkload CR
   ↓
Kubernetes API
   ↓
AIWorkload Controller
   ↓
Reconciliation Loop
   ├─ Deployment
   ├─ Service
   ├─ ServiceAccount
   ├─ NetworkPolicy
   └─ Status / Conditions / Events
         ↓
   Running Workload
         ↓
Controller Metrics
         ↓
Prometheus / Grafana
```

## 2. V0 kapsam sınırı

**Dahil:**

* Go
* Kubebuilder
* controller-runtime
* `AIWorkload` CRD
* Kubernetes reconciliation
* Deployment
* Service
* ServiceAccount
* Secret references
* NetworkPolicy
* readiness/liveness
* status/conditions
* Kubernetes Events
* metrics
* OpenTelemetry/Prometheus/Grafana
* unit tests
* envtest
* kind E2E
* Docker
* GitHub Actions
* Helm/Kustomize tabanlı kurulum
* README/demo

**V0 kapsam dışı:**

* frontend
* REST management API
* SaaS
* organizations/users
* billing
* GPU scheduling
* inference server
* LLM gateway
* model routing
* agent execution runtime
* vector database
* PostgreSQL
* Redis
* RabbitMQ/Kafka
* multi-cluster
* cloud-specific identity
* Terraform
* Argo CD
* HPA
* admission webhook
* full multi-tenancy


## 3. Teknik baseline ve karar durumu

12 Eylül 2026 kaynak kontrolü: [Kubernetes sürüm sayfası](https://kubernetes.io/releases/) 1.37.0 güncel minor ve 1.36.4 bakım sürümünü listeliyor. [controller-runtime compatibility](https://github.com/kubernetes-sigs/controller-runtime#compatibility) v0.24 ailesini k8s.io v0.36 ve minimum Go 1.26 ile eşliyor. [v0.24.1 release](https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.24.1) doğrulandı. [Kubebuilder release listesi](https://github.com/kubernetes-sigs/kubebuilder/releases) v4.16.0 gösteriyor; taslaktaki v4.15.0 artık güncel release olarak sunulmamalıdır.

Proje hedefi Kubernetes 1.36.x + controller-runtime v0.24.1 + Go 1.26.x. Exact patch/tool sürümleri, envtest binaries ve kind/node image digest'leri 01. adımda birlikte doğrulanarak pinlenir. Bu backlog sürüm uyumluluğunun projede test edildiği anlamına gelmez. [kind v0.33.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0) default node image 1.36.1 kullandığından en son Kubernetes patch'i için image varlığı varsayılmaz.

## 4. Ortak uygulama sözleşmesi

- Kubernetes API tek source of truth; spec desired state, status observed state. Harici DB/broker yoktur.
- Controller aynı namespace içindeki Deployment, Service, ServiceAccount ve NetworkPolicy kaynaklarını parent UID ile yönetir. User Secret, Namespace ve shared infrastructure sahiplenilmez.
- Aynı isimde başka sahibin kaynağı varsa ResourceOwnershipConflict; otomatik adoption/silme yoktur.
- Dedicated workload ServiceAccount; token automount kapalı. Secret references aynı namespace'te envFrom ile kullanılır; Secret payload log/status/Event/metric içine yazılmaz.
- Secret create/delete/restore watch ile reconcile tetikler. Secret kaybı contract'ı Degraded yapar; çalışan process'teki credential'ı geri alma ve otomatik env rotation V0 garantisi değildir.
- NetworkPolicy default'u aynı namespace'ten workload container portuna ingress iznidir; egress izolasyonu getirmez. Generation kanıtı ile CNI trafik enforcement kanıtı ayrıdır.
- replicas=0 geçerlidir; Ready=False, Progressing=False, Degraded=False ve ScaledToZero reason'ı üretilir.
- Kustomize zorunlu install yolu; Helm gerekçeli opsiyoneldir. Prometheus/Grafana ve OTel Collector örneği dahildir; traces opsiyoneldir.
- Her story için amaç, uygulama adımları, teknik kararlar/hata sınırları, kabul kriterleri, test senaryoları, teslim kanıtı, bağımlılıklar ve kaynaklar bulunur.
- Testler ilgili davranışla birlikte yazılır. Yerel validation, hosted CI, etkin branch protection, yayın ve deployment ayrı kanıtlardır.

## 5. Geliştirme sırası ve izlenebilirlik

Kaynak taslaktaki AWC-1..16 kimlikleri mantıksal sıra numarasıdır. Bu projenin gerçek anahtarı AWCP'dir; Jira otomatik numara üretir. Bu epic altındaki 16 Story'nin başındaki 01..16 sırası ve gerçek Blocks ilişkileri kullanılır.

- [AWCP-2 — 01 — Technical baseline, scope ve architecture decisions](https://erayyilmmaz.atlassian.net/browse/AWCP-2) — 1.5–2 saat.
- [AWCP-3 — 02 — Repository foundation ve Kubebuilder project bootstrap](https://erayyilmmaz.atlassian.net/browse/AWCP-3) — 1.5–2 saat.
- [AWCP-4 — 03 — AIWorkload Custom Resource API ve validation contract](https://erayyilmmaz.atlassian.net/browse/AWCP-4) — 2–3 saat.
- [AWCP-5 — 04 — Reconciliation engine ve resource ownership modeli](https://erayyilmmaz.atlassian.net/browse/AWCP-5) — 2–3 saat.
- [AWCP-6 — 05 — Deployment reconciliation ve workload lifecycle](https://erayyilmmaz.atlassian.net/browse/AWCP-6) — 2–3 saat.
- [AWCP-7 — 06 — Service discovery ve exposure lifecycle](https://erayyilmmaz.atlassian.net/browse/AWCP-7) — 1–1.5 saat.
- [AWCP-8 — 07 — Workload identity, Secret references ve least-privilege security](https://erayyilmmaz.atlassian.net/browse/AWCP-8) — 2 saat.
- [AWCP-9 — 08 — Network isolation ve NetworkPolicy reconciliation](https://erayyilmmaz.atlassian.net/browse/AWCP-9) — 1.5–2 saat.
- [AWCP-10 — 09 — Status Conditions, failure model ve Kubernetes Events](https://erayyilmmaz.atlassian.net/browse/AWCP-10) — 2–3 saat.
- [AWCP-11 — 10 — Controller observability ve metrics](https://erayyilmmaz.atlassian.net/browse/AWCP-11) — 2–3 saat.
- [AWCP-12 — 11 — Deletion, garbage collection ve lifecycle edge cases](https://erayyilmmaz.atlassian.net/browse/AWCP-12) — 1–1.5 saat.
- [AWCP-13 — 12 — Unit + envtest integration test suite](https://erayyilmmaz.atlassian.net/browse/AWCP-13) — 3–4 saat.
- [AWCP-14 — 13 — kind-based end-to-end test environment](https://erayyilmmaz.atlassian.net/browse/AWCP-14) — 2–3 saat.
- [AWCP-15 — 14 — Packaging ve developer installation experience](https://erayyilmmaz.atlassian.net/browse/AWCP-15) — 1.5–2 saat.
- [AWCP-16 — 15 — CI quality gates ve supply-chain hygiene](https://erayyilmmaz.atlassian.net/browse/AWCP-16) — 2 saat.
- [AWCP-17 — 16 — Documentation, portfolio demo ve V0 release](https://erayyilmmaz.atlassian.net/browse/AWCP-17) — 2 saat.

Bağımlılık akışı: 01 → 02 → 03 → 04 → 05; 05 ardından 06/07/08; 05–08 ardından 09; 04 ve 09 ardından 10; 05–09 ardından 11; 05–11 ardından 12; 12 → 13; 13 → 14; 12/13 → 15; 14/15 → 16. Tek tek geliştirme için 01..16 sırası uygundur.

## 6. Efor ve çalışma takvimi

Story tahminlerinin aritmetik toplamı 29–39 saattir; taslaktaki 29–38 saat yuvarlaması bu toplamla düzeltilmiştir. Tahminler başlangıç tahminidir; öğrenme, entegrasyon ve hata ayıklama sonrası yeniden değerlendirilir.

2 saat/gün önerilen akış: gün 1–11 → adım 01–11; gün 12–13 → adım 12; gün 14 → adım 13; gün 15 → adım 14; gün 16 → adım 15; gün 17 → adım 16; gün 18–20 → regression/refactor/security review/buffer. 3–4 saatlik story iki güne taşabilir. 18–20 çalışma günü, haftada beş gün çalışılırsa yaklaşık dört iş haftası; her gün çalışılırsa yaklaşık üç takvim haftasıdır. Jira son tarihleri atanmaz.

## 7. V0 Definition of Done

Epic ancak şu uçtan uca senaryo çalışıyorsa kapanmalı:

```text
Given
  temiz bir local makine
  Docker + pinlenmiş Go/kubectl/kind/Kustomize araçları
  gerekli image ve test binary hazırlığı

When
  kind cluster kuruluyor
  operator deploy ediliyor
  AIWorkload oluşturuluyor

Then
  Deployment oluşuyor
  Service oluşuyor
  ServiceAccount oluşuyor
  NetworkPolicy oluşuyor
  workload Ready=True oluyor

When
  Deployment manuel siliniyor

Then
  controller tekrar yaratıyor

When
  image değiştiriliyor

Then
  Deployment rollout oluyor

When
  gerekli Secret kayboluyor

Then
  AIWorkload Degraded oluyor

When
  Secret geri geliyor

Then
  AIWorkload Ready oluyor

When
  AIWorkload siliniyor

Then
  controller-owned kaynaklar temizleniyor
  user-owned Secret korunuyor

And
  reconcile metrics görülebiliyor
  unit/envtest/E2E geçiyor
  GitHub Actions yeşil
  README üzerinden demo tekrar üretilebiliyor
```



Ek kapanış kriterleri:

- 16 Story'nin kabul kriterleri gerçek test/artefact bağlantılarıyla doğrulanmıştır.
- Envtest API/ownership davranışını; kind gerçek rollout, trafik ve garbage collection davranışını kanıtlar.
- Required checks'in workflow sonucu ve merge ruleset etkinliği ayrı doğrulanmıştır.
- README operator deploy/image hazırlama adımlarını içerir; make install tek başına yeterli kabul edilmez.
- Secret restore testi parent spec değiştirmeden geçer; foreign resource ve user Secret korunur.
- Release hazırlığı ile gerçek release publication birbirinden ayrılır; v0.1.0 yayınlanmışsa gerçek link/digest kaydedilir.
- CNI enforcement test edilmediyse bu sınırlama açıktır; production-ready/full tenant isolation iddiası yoktur.

## 8. Sonraki sürüm sınırı

Argo CD, Terraform, HPA, multi-tenancy ve cloud deployment ayrı bir Platform Hardening & GitOps epic'inde değerlendirilebilir. Bu aktarımda V1 epic'i veya V0 dışı görev açılmaz.


# AWCP-2 — 01 — Technical baseline, scope ve architecture decisions

## 1. Amaç

Kod yazılmadan önce V0'ın neyi çözdüğünü, Kubernetes resource modelini ve controller responsibility boundary'sini sabitlemek.

## 2. Kapsam ve uygulama adımları

* `AIWorkload` domain kavramını tanımla.
* V0 supported use-case'leri belge.
* V0 non-goals listesini oluştur.
* Kubernetes version baseline'ını sabitle.
* Go/Kubebuilder/controller-runtime version matrisini sabitle.
* Namespaced `AIWorkload` tercihinin ADR'ını yaz.
* Control plane / managed workload sınırını tanımla.
* Controller'ın sahip olduğu Kubernetes kaynaklarını belirle:

  * Deployment
  * Service
  * ServiceAccount
  * NetworkPolicy
* Controller'ın **sahip olmadığı** kaynakları tanımla:

  * user-created Secret
  * Namespace
  * CRD dışındaki shared infrastructure
* Reconciliation responsibility matrix oluştur.
* Resource naming convention belirle.
* Kubernetes labels/annotations convention belirle.
* OwnerReference politikasını belirle.
* Deletion / garbage collection davranışını belirle.
* Status/Condition modelinin ilk taslağını çıkar.
* Security trust-boundary ADR oluştur.
* V0'da admission webhook kullanılmaması kararını belge.
* V0 API compatibility/versioning stratejisini belirle:

  * `platform.example.io/v1alpha1`
* ADR dizini oluştur.

### Architecture decisions

En az:

```text
ADR-001 — Why Kubernetes Operator
ADR-002 — Why Go
ADR-003 — Why Namespaced AIWorkload
ADR-004 — Reconciliation and Ownership Model
ADR-005 — Security and Secret Boundaries
ADR-006 — Observability Model
ADR-007 — API Versioning Strategy
```

## 3. Teknik kararlar ve hata sınırları

- Baseline hedefi Kubernetes 1.36.x + controller-runtime v0.24.1 + Go 1.26.x olarak korunur. 12 Eylül 2026 kaynak kontrolünde Kubernetes 1.36.4, Kubebuilder v4.16.0 ve kind v0.33.0 görülmüştür. Bunlar birlikte test edilmiş proje sürümleri değildir. Güncel sürüm ile proje tarafından desteklenen sürüm ayrı yazılır.
- Kapanışta Go patch, Kubebuilder, tüm k8s.io/* modülleri, controller-tools, setup-envtest commit/tag, kind, node image digest, kubectl, Kustomize ve lint sürümlerini tek matriste kesinleştir. kind/node ve envtest binary bulunabilirliğini linux/amd64 ve darwin/arm64 için doğrula; bulunmayan artefact'a hayali digest yazma.
- Control plane Kubernetes API'ye controller-runtime client üzerinden erişir. kubectl çağıran bir reconcile implementation, harici database ve broker yoktur. Deployment/Service/ServiceAccount/NetworkPolicy aynı namespace içinde parent UID ile sahiplenilir.
- Adlandırmada uzun CR adlarını Kubernetes Service/label sınırlarına uygun kısalt ve deterministik hash ile çakışmayı önle. Adı aynı ama sahibi farklı kaynağı benimseme veya silme; ResourceOwnershipConflict üret.
- Güven sınırı: AIWorkload oluşturabilen kişi izin verilen namespace içinde image çalıştırabilir ve Secret referansı verebilir. Bu, tek başına tenant izolasyonu veya Secret bazında yetkilendirme sağlamaz. Watch kapsamı, RBAC kapsamı ve cluster yöneticisinin sorumluluğu ADR-005'te açıklanır.
- ADR-008 — Deletion and Finalizers ekle. Dış kaynak temizliği bulunmayan V0'da Kubernetes garbage collection tercih edilir. Kustomize zorunlu paketleme yolu; Helm gerekçeli opsiyoneldir. OpenTelemetry Collector örneği dahildir; trace instrumentasyonu opsiyoneldir.

## 4. Kabul kriterleri

* V0'ın işlevsel sınırı açıkça belgelenmiştir.
* Her generated Kubernetes resource için controller ownership kararı vardır.
* User-owned Secret'lar controller tarafından oluşturulmaz veya mutate edilmez.
* Source-of-truth'ın Kubernetes API olduğu açıkça belirtilmiştir.
* Harici database gerektirilmediği belgelenmiştir.
* `spec` desired state, `status` observed state olarak tanımlanmıştır.
* Unsupported özelliklerin listesi README/ADR'larda görünürdür.
* Kubernetes/controller-runtime sürüm baseline'ı pin edilmiştir.
* Architecture'da gereksiz message broker veya datastore bulunmamaktadır.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Sürüm matrisi ile scaffold go.mod/Makefile uyumunu karşılaştır; desteklenmeyen kombinasyonu başlangıç engeli olarak kaydet.
2. Her dört owned resource ve üç user/shared resource sınıfı için create/update/delete yetkisini matriste gözden geçir.
3. AWC-1..16 kapsamını ADR, test katmanı ve teslim çıktısıyla eşleştir; açık kararlar bu görevin kapanışında çözülmüş olsun.

## 6. Teslim çıktıları ve kapanış kanıtı

docs/architecture.md, docs/scope.md, docs/compatibility.md, docs/adr/ADR-001..008 ve sorumluluk matrisi.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 01 / 16. Kaynak taslaktaki kimlik: AWC-1 (gerçek Jira anahtarı değildir).

Bağımlılıklar: Yok; ilk başlanacak çalışma.

Başlangıç tahmini: 1.5–2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kubernetes.io/releases/](https://kubernetes.io/releases/)
- [github.com/kubernetes-sigs/controller-runtime#compatibility](https://github.com/kubernetes-sigs/controller-runtime#compatibility)
- [github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.24.1](https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.24.1)
- [github.com/kubernetes-sigs/kubebuilder/releases](https://github.com/kubernetes-sigs/kubebuilder/releases)
- [go.dev/dl/](https://go.dev/dl/)
- [github.com/kubernetes-sigs/kind/releases/tag/v0.33.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0)

---

# AWCP-3 — 02 — Repository foundation ve Kubebuilder project bootstrap

## 1. Amaç

Reproducible Go/Kubernetes operator geliştirme ortamını oluşturmak.

## 2. Kapsam ve uygulama adımları

* Git repository oluştur.
* Go module tanımla.
* Kubebuilder project scaffold oluştur.
* `AIWorkload` API/controller scaffold oluştur.
* Repository klasörlerini düzenle.

Beklenen ana yapı:

```text
.
├── api/
│   └── v1alpha1/
├── cmd/
├── config/
│   ├── crd/
│   ├── default/
│   ├── manager/
│   ├── rbac/
│   └── samples/
├── internal/
│   ├── controller/
│   ├── resource/
│   └── telemetry/
├── test/
│   └── e2e/
├── docs/
│   └── adr/
├── Dockerfile
├── Makefile
├── PROJECT
├── go.mod
└── README.md
```

* Dependency versiyonlarını pin et.
* `make generate`
* `make manifests`
* `make build`
* `make test`
* `make lint`
  workflow'larını doğrula.
* golangci-lint ekle.
* `.editorconfig` oluştur.
* `.gitignore` oluştur.
* LICENSE ekle.
* Makefile hedeflerini normalize et.
* controller container image build'i doğrula.
* multi-stage Dockerfile oluştur.
* non-root container user kullan.
* Docker image metadata/labels ekle.
* local dev prerequisites dokümante et.

Kubebuilder'ın scaffold'u zaten manifests, code generation, envtest, build ve e2e için uygun Makefile yapısını destekliyor. ([book-v3.book.kubebuilder.io](https://book.kubebuilder.io/cronjob-tutorial/basic-project.html))

## 3. Teknik kararlar ve hata sınırları

- Mevcut repository varsa scaffold öncesinde içeriğini ve yerel talimatları incele. Go module/Git remote kimliğini mevcut proje bilgisine göre belirle; example adresini gerçek remote gibi kullanma. Git işlemleri bu planın aktarılmasıyla yapılmış sayılmaz.
- make test gerekli envtest binary'lerini hazırlayan giriş noktasıdır; doğrudan go test ./... için KUBEBUILDER_ASSETS ön koşulunu belge. İlk tool/image indirmeleri ağ gerektirebilir; bağımlılıklar hazırlandıktan sonraki unit/envtest çalışması dış servise bağlı olmamalıdır.
- Make hedefleri unit, envtest ve ileride e2e olarak ayrıştırılabilir; scaffold boş testlerinin ürün davranışını doğruladığı iddia edilmez. go test -race uygun CI platformunda çalıştırılır.
- Controller container: non-root UID, allowPrivilegeEscalation=false, capabilities drop ALL ve uygun seccomp profili; gerekli writable path varsa açıkça tanımlanır. Base image sürümü/digest'i pinlenir.
- Manager health/readiness ve leader election ayarlarını yapılandır. Birden fazla manager replica desteklenecekse Lease RBAC ve tek aktif lider davranışı E2E'de doğrulanmadan destek iddiası yazma.

## 4. Kabul kriterleri

* Repository sıfırdan clone edildiğinde documented prerequisites ile build edilebilir.
* `go test ./...` başarılıdır.
* `go vet ./...` başarılıdır.
* lint başarılıdır.
* generated CRD/RBAC artefact'ları reproducible'dır.
* manager container image build edilebilir.
* container root olarak çalışmaz.
* generated code ile handwritten code sınırı nettir.
* repository'de secret/credential bulunmaz.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Temiz geçici çalışma alanında documented prerequisites ile generate, manifests, build, vet, lint ve test akışını çalıştır.
2. Üretimi iki kez çalıştır; ikinci çalışmada generated dosyalarda fark olmamasını doğrula.
3. Container image build et; image user/security configuration ve process health sonucunu kontrol et.

## 6. Teslim çıktıları ve kapanış kanıtı

Çalışan Kubebuilder scaffold, go.mod/go.sum, PROJECT, Makefile, Dockerfile, lint/editor ayarları ve geliştirme ön koşulları.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 02 / 16. Kaynak taslaktaki kimlik: AWC-2 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-2 — Adım 01](https://erayyilmmaz.atlassian.net/browse/AWCP-2)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 1.5–2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/cronjob-tutorial/basic-project.html](https://book.kubebuilder.io/cronjob-tutorial/basic-project.html)
- [github.com/kubernetes-sigs/kubebuilder/releases](https://github.com/kubernetes-sigs/kubebuilder/releases)

---

# AWCP-4 — 03 — AIWorkload Custom Resource API ve validation contract

## 1. Amaç

Developer-facing platform API'sini tanımlamak.

Kubernetes CRD'leri declarative API oluşturmak için custom controller ile birlikte kullanılabiliyor; ayrıca `/status` subresource, controller'ın observed state'i kullanıcı tarafından yazılan spec'ten ayırmasını sağlıyor. ([Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/))

## 2. Kapsam ve uygulama adımları

### Önerilen V0 spec

```yaml
apiVersion: platform.example.io/v1alpha1
kind: AIWorkload

metadata:
  name: demo-agent

spec:
  image: ghcr.io/example/demo-agent:v1

  replicas: 1

  container:
    port: 8080

  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"

  health:
    readiness:
      path: /ready
    liveness:
      path: /health

  service:
    enabled: true
    port: 80

  secretRefs:
    - agent-api-secrets

  network:
    enabled: true
```

### Tasks

* `AIWorkloadSpec` oluştur.
* `AIWorkloadStatus` oluştur.
* required/optional fields belirle.
* default values belirle.
* validation markers ekle.
* replica limitlerini belirle.
* container port validation ekle.
* image empty validation ekle.
* resource requirements modelle.
* readiness/liveness modelle.
* service configuration modelle.
* secret references modelle.
* network configuration modelle.
* status subresource etkinleştir.
* printer columns tanımla.

Örneğin:

```text
NAME
READY
REPLICAS
IMAGE
AGE
```

* CRD OpenAPI schema üret.
* sample valid manifest oluştur.
* invalid manifest fixtures oluştur.
* future compatibility için unknown-field davranışını incele.
* API field documentation ekle.

### Status taslağı

```yaml
status:
  observedGeneration: 4
  desiredReplicas: 2
  readyReplicas: 2
  endpoint: demo-agent.default.svc:80
  conditions:
    - type: Ready
      status: "True"
      reason: WorkloadReady
      message: "Current generation is ready."
      observedGeneration: 4
      lastTransitionTime: "2026-09-12T09:00:00Z"
    - type: Progressing
      status: "False"
      reason: WorkloadReady
      message: "Rollout is complete."
      observedGeneration: 4
      lastTransitionTime: "2026-09-12T09:00:00Z"
    - type: Degraded
      status: "False"
      reason: WorkloadReady
      message: "No blocking failure is observed."
      observedGeneration: 4
      lastTransitionTime: "2026-09-12T09:00:00Z"
```

## 3. Teknik kararlar ve hata sınırları

- V0 contract: image zorunlu ve boş/yalnız boşluk olamaz; replicas varsayılan 1, izin verilen aralık 0..20; container.port zorunlu 1..65535; service.enabled varsayılan true, service.port varsayılan 80 ve 1..65535; network.enabled varsayılan true; secretRefs varsayılan boş liste. Açık false ve 0 değerleri default tarafından ezilmez.
- resources için CPU/memory requests ve limits desteklenir. Miktar biçimi, negatif değer ve request > limit durumu validation fixtures ile test edilir. Şema/CEL ile güvenle doğrulanamayan bir kural varsa child API reddi actionable condition'a çevrilir; admission webhook eklenmez.
- health readiness/liveness HTTP path alanları opsiyoneldir: ilgili blok yoksa probe yoktur, varsa path '/' ile başlar ve named container port kullanılır. Period/timeout/failureThreshold sabit V0 değerleri API dokümanında açıklanır; slow-start/startupProbe ileri sürüm kapsamıdır.
- secretRefs aynı namespace içindeki Secret adlarıdır; boş, geçersiz ve tekrar eden adlar reddedilir. V0 envFrom kullanır; sıra korunur ve çakışan anahtarlarda sonraki Secret kazanır. envFrom key uyumluluğu ve gerekli anahtarların uygulama sorumluluğu olduğu belgelenir. Secret payload API'ye eklenmez.
- status koşulları metav1.Condition sözleşmesini izler: type/status/reason/message/lastTransitionTime/observedGeneration. conditions type anahtarlı map list olur. Top-level observedGeneration, desiredReplicas, readyReplicas ve endpoint yer alır. YAML örneği şemayla birebir test edilir.
- Unknown fields için structural schema pruning ve fieldValidation=Strict farkını belge; silent acceptance beklentisini engelleyen negative fixture ekle. API group platform.example.io örnek alanıdır; bu değer V0 contract seçimi olarak tutarlı kullanılır.

## 4. Kabul kriterleri

* CRD namespaced'dir.
* `/status` subresource aktiftir.
* Invalid replicas Kubernetes API validation seviyesinde reddedilir.
* Invalid port reddedilir.
* Required image olmadan resource kabul edilmez.
* `kubectl explain aiworkload.spec` anlamlı açıklamalar gösterir.
* valid sample API tarafından kabul edilir.
* invalid fixtures beklenen validation failure'larını üretir.
* controller henüz çalışmasa dahi CRD contract bağımsız olarak test edilebilir.
* `spec` ve `status` responsibility ayrımı korunur.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Valid manifesti gerçek API server'a gönder, default alanları geri oku; explicit replicas=0 ve enabled=false değerlerini ayrı test et.
2. image yok/boş, replicas=-1/21, port=0/65536, tekrar eden Secret ve geçersiz path örneklerinin reddedildiğini kontrol et.
3. kubectl explain, printer columns, status subresource ve status yazımının spec/generation davranışını doğrula.

## 6. Teslim çıktıları ve kapanış kanıtı

api/v1alpha1 types, generated CRD, alan/default/validation tablosu, valid/invalid fixtures ve status örnekleri.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 03 / 16. Kaynak taslaktaki kimlik: AWC-3 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-3 — Adım 02](https://erayyilmmaz.atlassian.net/browse/AWCP-3)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2–3 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [kubernetes.io/docs/concepts/configuration/secret/](https://kubernetes.io/docs/concepts/configuration/secret/)

---

# AWCP-5 — 04 — Reconciliation engine ve resource ownership modeli

## 1. Amaç

Controller'ın temel control-loop davranışını oluşturmak.

## 2. Kapsam ve uygulama adımları

* `AIWorkloadReconciler` oluştur.
* Resource fetch davranışını uygula.
* NotFound handling ekle.
* reconcile request logging context oluştur:

  * namespace
  * name
  * generation
* desired state builder abstraction oluştur.
* idempotent reconciliation modeli uygula.
* owner references ekle.
* controller-owned resource watch'larını ekle.
* Deployment drift'inin reconcile tetiklemesini sağla.
* Service drift'inin reconcile tetiklemesini sağla.
* ServiceAccount drift'inin reconcile tetiklemesini sağla.
* NetworkPolicy drift'inin reconcile tetiklemesini sağla.
* create/update/no-op ayrımını implemente et.
* conflict/error handling yap.
* controller-runtime requeue/backoff davranışını gereksiz yere override etme.
* reconcile result semantics belge.
* duplicate reconcile'ın duplicate resource üretmediğini test et.

Kubebuilder `Owns()` kullanımı sayesinde controller'ın oluşturduğu secondary resource değiştiğinde parent resource yeniden reconcile edilebilir. ([book.kubebuilder.io](https://book.kubebuilder.io/reference/watching-resources/secondary-owned-resources.html))

### Temel invariant

```text
Reconcile(x) + Reconcile(x)
```

sonucu:

```text
Reconcile(x)
```

ile aynı desired state'i üretmelidir.

Başka bir ifadeyle reconcile **idempotent** olmalıdır.

## 3. Teknik kararlar ve hata sınırları

- Reconcile sırası: parent fetch → deletion guard → Secret ön koşulları → ServiceAccount → Deployment → isteğe bağlı Service/NetworkPolicy → observed status. Geçici kısmi oluşturma sonraki reconcile ile tamamlanır; birden çok kaynağın atomik transaction içinde yaratıldığı varsayılmaz.
- Mutate/delete öncesi owner UID ve namespace doğrulanır. Başkasına ait aynı isimli kaynakta ResourceOwnershipConflict; otomatik adoption veya destructive recreate yoktur. Controller'a ait alanları patch et, Kubernetes defaultlarını normalize ederek sürekli update döngüsünü önle.
- Watch/predicate tasarımı parent spec değişimini, child deletion/drift ve Deployment status değişimini kapsar. GenerationChangedPredicate'in bütün watch'lara uygulanıp readiness/status olaylarını kaybetmesine izin verme.
- API Conflict, AlreadyExists, Forbidden, timeout ve throttling ayrı ele alınır. Retry edilebilir hata framework backoff'una döner; kalıcı input/ownership hatası condition/event ile görünür olur ve hot loop oluşturmaz.
- Secret watch/index tasarımının bağlantı noktası burada kurulur; gerçek Secret lifecycle davranışı 07. adımda tamamlanır. Secret silindikten sonra parent spec değişikliği gerektirmeden yeniden reconcile edilmelidir.

## 4. Kabul kriterleri

* Aynı `AIWorkload` için art arda reconcile çağrıları duplicate resource üretmez.
* Owned resource silinirse controller yeniden oluşturur.
* Managed field değiştirilirse desired state yeniden uygulanır.
* Controller restart sonrası state database gerektirmeden yeniden oluşturulabilir.
* Unexpected error diğer `AIWorkload` resource'larının reconcile edilmesini kalıcı olarak durdurmaz.
* User-owned resources owner reference ile yanlışlıkla controller'a bağlanmaz.
* Resource watch'ları gereksiz reconcile storm üretmeyecek şekilde yapılandırılmıştır.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Aynı parent için create → tekrar reconcile → no-op; child resourceVersion ve API yazma sayısını karşılaştır.
2. Dört owned resource için delete ve managed-field drift yap; yalnız hedef parent'ın düzeldiğini doğrula.
3. Foreign owner çakışması, injected conflict/Forbidden ve restart sonrası recovery çalıştır; unrelated parent'ın ilerlediğini kontrol et.

## 6. Teslim çıktıları ve kapanış kanıtı

Reconciler iskeleti, desired-state arayüzleri, watch/predicate tasarımı, ownership guard ve retry sözleşmesi.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 04 / 16. Kaynak taslaktaki kimlik: AWC-4 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-4 — Adım 03](https://erayyilmmaz.atlassian.net/browse/AWCP-4)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2–3 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/reference/watching-resources/secondary-owned-resources.html](https://book.kubebuilder.io/reference/watching-resources/secondary-owned-resources.html)
- [book.kubebuilder.io/reference/watching-resources/secondary-resources-not-owned.html](https://book.kubebuilder.io/reference/watching-resources/secondary-resources-not-owned.html)

---

# AWCP-6 — 05 — Deployment reconciliation ve workload lifecycle

## 1. Amaç

`AIWorkload.spec` → Kubernetes Deployment mapping'ini gerçekleştirmek.

## 2. Kapsam ve uygulama adımları

* Deployment desired-state builder oluştur.
* deterministic resource name üret.
* common labels oluştur.
* selector labels sabitle.
* container image mapping yap.
* replica mapping yap.
* resource requests/limits mapping yap.
* container port mapping yap.
* ServiceAccount binding ekle.
* Secret reference mapping yap.
* readiness probe oluştur.
* liveness probe oluştur.
* controller owner reference ekle.
* Deployment create flow implement et.
* Deployment update flow implement et.
* immutable-field durumlarını ele al.
* pod template değişikliğinin rollout tetiklemesini doğrula.
* replicas update doğrula.
* image update doğrula.
* deleted Deployment recreation test et.
* unmanaged annotations/metadata korunumu politikasını belirle.

Readiness başarısız olduğunda Kubernetes pod'a trafik göndermemeyi; liveness başarısız olduğunda ise unhealthy container'ı yeniden başlatmayı destekliyor. ([Kubernetes](https://kubernetes.io/docs/concepts/workloads/pods/probes/))

## 3. Teknik kararlar ve hata sınırları

- Deployment mapping'i bu adımda kurulur; dedicated ServiceAccount ve Secret ön koşulları 07. adımda tamamlanır. Bu nedenle 05'in kapanışı tam çalışan güvenlikli pod E2E kanıtı değildir; bu iki adım arasında döngüsel bağımlılık oluşturma.
- Pod security varsayılanları non-root, privilege escalation kapalı, capabilities drop ALL ve RuntimeDefault seccomp olur. V0 public/demo image non-root çalışabilir olmalıdır; keyfi/root gerektiren image'lar için uyumluluk garantisi verilmez.
- Image tag/digest değişimi veya probe/resources/secretRefs değişimi pod template'i günceller; sadece replicas değişimi ölçekler. Selector kimliği sabittir; immutable selector uyuşmazlığında açık hata üret, sessiz delete/recreate yapma.
- Pod template/metadata için controller-owned label ve field listesi ADR'a uyar; unrelated annotation/label korunur. ImagePullPolicy açık ve sample image stratejisiyle uyumlu olur; no-op reconcile rollout başlatmaz.
- ImagePullBackOff, CrashLoop, readiness başarısızlığı, quota/insufficient resource ve progress deadline davranışını Deployment/Pod teşhisi ile belge. Pod-level teşhis eklenirse read RBAC ve veri minimizasyonu birlikte gözden geçirilir.

## 4. Kabul kriterleri

Given:

```yaml
spec:
  image: demo:v1
  replicas: 2
```

controller:

```text
Deployment/demo
replicas=2
image=demo:v1
```

oluşturur.

Ayrıca:

* image değişince rollout oluşur.
* replicas değişince Deployment güncellenir.
* resource limits doğru propagate edilir.
* health probes doğru propagate edilir.
* deleted Deployment geri oluşturulur.
* unrelated `AIWorkload` etkilenmez.
* owner reference doğrudur.
* controller-generated fields deterministic'tir.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Builder ve envtest'te image, replicas, ports, requests/limits, probes, identity ve owner UID alanlarını doğrula.
2. Image v1→v2 ile template değişimini; replicas 1→2→0 ile scale davranışını; unchanged reconcile ile rollout olmamasını test et.
3. Deployment silme, immutable field conflict ve başarısız image senaryolarının beklenen sonuçlarını kaydet; gerçek pod lifecycle kanıtı E2E adımında üretilir.

## 6. Teslim çıktıları ve kapanış kanıtı

Deployment builder/reconciliation, field ownership tablosu, mapping tests ve failure fixtures.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 05 / 16. Kaynak taslaktaki kimlik: AWC-5 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-5 — Adım 04](https://erayyilmmaz.atlassian.net/browse/AWCP-5)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2–3 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kubernetes.io/docs/concepts/workloads/pods/probes/](https://kubernetes.io/docs/concepts/workloads/pods/probes/)

---

# AWCP-7 — 06 — Service discovery ve exposure lifecycle

## 1. Amaç

Workload'a cluster-local erişim sağlamak.

## 2. Kapsam ve uygulama adımları

* Service builder oluştur.
* V0 ServiceType'ı `ClusterIP` ile sınırla.
* `service.enabled` davranışını uygula.
* service port → targetPort mapping yap.
* label selectors üret.
* Service create/update/delete lifecycle implement et.
* `service.enabled=false` durumunu ele al.
* status endpoint üret.
* immutable ClusterIP field'ını koru.
* Service drift testleri yaz.
* invalid target port senaryolarını test et.

## 3. Teknik kararlar ve hata sınırları

- Service yalnız ClusterIP ve TCP destekler; targetPort named container port'a bağlanır. Servis portu 80 olsa bile NetworkPolicy ingress kuralı pod portunu hedefler.
- clusterIP/clusterIPs, ipFamilies/ipFamilyPolicy gibi API tarafından tahsis edilen alanları koru; NodePort/LoadBalancer/headless configurasyonu V0 API'sine ekleme.
- Endpoint yalnız Service mevcut ve enabled ise '<service>.<namespace>.svc:<port>' olarak yayınlanır; custom cluster DNS domain'i varsayılmaz. Endpoint varlığı pod readiness veya dışarıdan erişim kanıtı değildir.
- Enabled→disabled geçişinde yalnız güncel parent UID'ye ait Service silinir ve stale endpoint temizlenir; yanlış owner kaynak korunur.

## 4. Kabul kriterleri

* service enabled olduğunda ClusterIP Service oluşturulur.
* service disabled olduğunda controller Service oluşturmamalıdır.
* enabled → disabled geçişinde önceki controller-owned Service temizlenir.
* disabled → enabled yeniden oluşturur.
* Service selector yalnız ilgili workload podlarını seçer.
* mevcut `clusterIP` update sırasında yanlışlıkla sıfırlanmaz.
* status'ta endpoint doğru gösterilir.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Enabled/disabled ve iki yönlü toggle akışını çalıştır; endpoint'in eklenip temizlendiğini kontrol et.
2. Port güncellemesi sırasında ClusterIP aynı kalmalı, selector yalnız hedef podları seçmeli.
3. E2E'de aynı namespace test client'ından servis DNS/HTTP erişimini doğrula; envtest'in veri düzlemi trafiğini test etmediğini belirt.

## 6. Teslim çıktıları ve kapanış kanıtı

Service builder/lifecycle, endpoint contract ve toggle/immutable field tests.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 06 / 16. Kaynak taslaktaki kimlik: AWC-6 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-6 — Adım 05](https://erayyilmmaz.atlassian.net/browse/AWCP-6)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 1–1.5 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kubernetes.io/docs/concepts/services-networking/service/](https://kubernetes.io/docs/concepts/services-networking/service/)

---

# AWCP-8 — 07 — Workload identity, Secret references ve least-privilege security

## 1. Amaç

Her workload için security-conscious identity modeli oluşturmak.

Kubernetes, uygulamaya özel ServiceAccount ve minimum gerekli RBAC kullanımını öneriyor; wildcard permissions ve gereksiz ClusterRoleBindings least-privilege açısından önerilmiyor. ([Kubernetes](https://kubernetes.io/docs/concepts/security/service-accounts/))

## 2. Kapsam ve uygulama adımları

* her `AIWorkload` için dedicated ServiceAccount oluştur.
* workload'un `default` ServiceAccount kullanmasını engelle.
* `automountServiceAccountToken` politikasına karar ver.
* V0 default'u mümkünse:

```yaml
automountServiceAccountToken: false
```

yap.

* secretRef resolution davranışını oluştur.
* Secret'ın varlığını doğrula.
* Secret content loglama.
* Secret content status'a yazma.
* Secret kopyalama yapma.
* controller user-owned Secret mutate etmesin.
* eksik Secret için condition üret.
* Controller manager RBAC kurallarını incele.
* wildcard verb/resource kullanımlarını kaldır.
* minimum required RBAC seti oluştur.
* controller ServiceAccount permissions test et.
* README security model dokümante et.

## 3. Teknik kararlar ve hata sınırları

- Dedicated ServiceAccount ve Pod spec üzerinde automountServiceAccountToken=false zorunlu varsayılandır; workload için RoleBinding oluşturulmaz. Manager kimliği ve workload kimliği ayrıdır.
- Secret create/update/delete olayları için namespace+Secret adı field index ve non-owned watch/map kur. Var olmayan Secret'ı referanslayan parent da index'te bulunmalı; Secret restore olduğunda elle CR değiştirmeden reconcile tetiklenmelidir.
- Secret varlığı kontrolü payload okumadan metadata client/cache yaklaşımıyla tasarlanır. Ancak get/list/watch Secret RBAC'ı API düzeyinde payload'a erişim yetkisi verebilir; 'controller secret okuyamaz' iddiası yazma. Cache ve watch kapsamını namespace ile sınırla ve bu yetkiyi ADR'da açıkla.
- Missing Secret: Ready=False, Degraded=True, reason=SecretNotFound. API Forbidden/timeout'u SecretNotFound diye maskeleme. Secret geri geldiğinde koşulu tekrar değerlendir; desired Deployment nesnesi mevcut olabilir, fakat pod çalışabilirliği ayrı gözlemlenir.
- Secret silinmesi çalışan pod'un belleğindeki env değerini geri almaz; V0 credential revocation sağlamaz. Secret data değişince envFrom kullanan mevcut process otomatik güncellenmez; otomatik Secret rotation rollout V0 kapsam dışıdır, manuel rollout/new pod gereksinimi belgelenir.
- Manager için AIWorkload read/watch, status patch/update, gerekli child verbs, Event write ve kullanılıyorsa Lease yetkilerini ayrı listele. Secret write/delete ve gereksiz CR spec write izinleri bulunmamalı. Workload creator'ın namespace Secret erişim etkisini güven modeli kapsamında açıkla.

## 4. Kabul kriterleri

* her workload dedicated ServiceAccount ile çalışır.
* Secret içeriği hiçbir controller log'unda bulunmaz.
* Secret içeriği metric label olarak bulunmaz.
* Secret içeriği status'ta bulunmaz.
* controller Secret mutate/delete etmez.
* eksik Secret workload condition'ını `Ready=False` yapabilir.
* controller `cluster-admin` gerektirmez.
* wildcard RBAC yalnız teknik olarak kaçınılmazsa ADR ile açıklanır.
* controller generated RBAC review edilebilir durumdadır.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Eksik Secret ile oluştur → Secret ekle → Ready → Secret sil → Degraded → aynı isimle geri ekle → Ready; CR spec'ine dokunma.
2. İki namespace'te aynı Secret adı kullan; yalnız referans veren doğru namespace parent'larının reconcile olduğunu doğrula.
3. Sentinel sahte Secret ile log/status/event/metric çıktılarında değer sızıntısını ara; kubectl auth can-i ile manager/workload izinlerini ayrı kontrol et.

## 6. Teslim çıktıları ve kapanış kanıtı

ServiceAccount reconciliation, Secret index/watch, missing/restore conditions, RBAC manifests ve security documentation.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 07 / 16. Kaynak taslaktaki kimlik: AWC-7 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-6 — Adım 05](https://erayyilmmaz.atlassian.net/browse/AWCP-6)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/reference/watching-resources/secondary-resources-not-owned.html](https://book.kubebuilder.io/reference/watching-resources/secondary-resources-not-owned.html)
- [kubernetes.io/docs/concepts/configuration/secret/](https://kubernetes.io/docs/concepts/configuration/secret/)
- [kubernetes.io/docs/concepts/security/rbac-good-practices/](https://kubernetes.io/docs/concepts/security/rbac-good-practices/)
- [kubernetes.io/docs/concepts/security/service-accounts/](https://kubernetes.io/docs/concepts/security/service-accounts/)

---

# AWCP-9 — 08 — Network isolation ve NetworkPolicy reconciliation

## 1. Amaç

Workload network security'sini platform default'u haline getirmek.

Kubernetes NetworkPolicy, Pod'ların cluster içi/dışı L3/L4 trafik kurallarını tanımlayabiliyor; ancak enforcement kullanılan CNI'a bağlıdır. ([Kubernetes](https://kubernetes.io/docs/reference/kubernetes-api/networking/network-policy-v1/))

## 2. Kapsam ve uygulama adımları

* standard Kubernetes `networking.k8s.io/v1 NetworkPolicy` kullan.
* V0 için vendor-specific Cilium/Calico resource kullanma.
* `network.enabled` contract'ını tanımla.
* workload selector oluştur.
* ingress behavior belirle.
* Service traffic için gerekli ingress allowance oluştur.
* DNS egress ihtiyacını değerlendir.
* external model/API kullanan workload'ların egress gereksinimi nedeniyle aşırı katı default tanımlama.
* generated NetworkPolicy owner reference ekle.
* enable/disable lifecycle uygula.
* drift reconciliation test et.
* policy generation unit testleri ekle.
* kind E2E ortamında NetworkPolicy enforcement doğrulanacaksa destekleyen CNI kullandığını doğrula.

### Önemli scope kararı

V0:

```text
NetworkPolicy generation
```

kanıtlayacak.

V0'ın amacı:

```text
enterprise egress firewall
```

yazmak değildir.

## 3. Teknik kararlar ve hata sınırları

- Somut V0 default: network.enabled=true iken workload pod selector'ına Ingress policy uygula; yalnız aynı namespace'teki podlardan container TCP portuna girişe izin ver. Service IP'sini kaynak kimliği olarak kullanma. Cross-namespace ingress için V0 API parametresi yoktur.
- V0 policy Egress isolation eklemez; DNS ve dış model/API erişimi bu policy tarafından kısıtlanmaz. Başka policy'lerin getirdiği kurallar geçerliliğini korur. 'Default-deny tüm trafik' veya egress firewall iddiası yazılmaz.
- NetworkPolicy kuralları additive'dir; başka bir allow policy izolasyonu genişletebilir. Controller yalnız kendi policy'sini yönetir. Enabled=false diğer kullanıcı policy'lerini silmez.
- Zorunlu V0 kanıtı policy generation/lifecycle'dır. Trafik engellendiği iddia edilecekse destekleyen CNI ve allow/deny testleri ayrıca koşulmalıdır; varsayılan kind ortamında enforcement kanıtı varsayılmaz.
- Controller manager, DNS ve observability kaynaklarını workload selector'ından ayrı tut; policy pod portunu hedefler. service.enabled=false, network.enabled davranışını otomatik değiştirmez.

## 4. Kabul kriterleri

* enabled durumda standard NetworkPolicy oluşturulur.
* disabled durumda oluşturulmaz.
* existing controller-owned policy kaldırılabilir/reconcile edilir.
* policy doğru pod labels seçer.
* unrelated podları seçmez.
* network policy controller'ın kendisini etkilemez.
* vendor CRD dependency'si yoktur.
* README, enforcement'ın CNI'a bağlı olduğunu açıkça söyler.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Policy selector, namespace/pod peer ve container port mapping'ini unit test ile karşılaştır.
2. Enabled→disabled→enabled ve policy delete/drift recovery'yi doğrula; unrelated policy korunmalı.
3. CNI profili varsa same-namespace allow, cross-namespace deny ve gerekli DNS/API egress'i test et; yoksa enforcement sonucu açıkça NOT TESTED yazılır.

## 6. Teslim çıktıları ve kapanış kanıtı

NetworkPolicy builder/lifecycle, ingress/egress contract, generation tests ve CNI kanıt sınırı dokümanı.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 08 / 16. Kaynak taslaktaki kimlik: AWC-8 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-6 — Adım 05](https://erayyilmmaz.atlassian.net/browse/AWCP-6)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 1.5–2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kubernetes.io/docs/concepts/services-networking/network-policies/](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [kubernetes.io/docs/reference/kubernetes-api/networking/network-policy-v1/](https://kubernetes.io/docs/reference/kubernetes-api/networking/network-policy-v1/)

---

# AWCP-10 — 09 — Status Conditions, failure model ve Kubernetes Events

## 1. Amaç

Developer'ın sadece “çalışmıyor” değil, **neden çalışmadığını** anlayabilmesini sağlamak.

## 2. Kapsam ve uygulama adımları

* `observedGeneration` yönet.
* standard condition helper yapısı oluştur.
* minimum conditions:

```text
Ready
Progressing
Degraded
```

* reason code sözleşmesi oluştur.

Örnekler:

```text
Reconciling
DeploymentCreated
DeploymentUnavailable
SecretNotFound
ServiceCreated
PolicyCreated
ReconcileFailed
WorkloadReady
```

* human-readable condition messages ekle.
* status write-loop riskini önle.
* status yalnız değiştiğinde update edilsin.
* readyReplicas güncelle.
* desiredReplicas güncelle.
* endpoint güncelle.
* Kubernetes Events ekle.
* successful event spam'ini önle.
* failure event üret.
* transient vs persistent failure ayrımını tanımla.
* observedGeneration semantics test et.

## 3. Teknik kararlar ve hata sınırları

- Ready=True yalnız güncel CR generation işlenmiş, Secret/ownership ön koşulları sağlanmış, gerekli child kaynaklar desired state'te ve Deployment güncel rollout için gerekli ready replica sayısına erişmişse yazılır. Eski Deployment Available koşulu yeni image rollout'unu hazır gösteremez.
- Status tablosu: converging → Ready=False/Progressing=True/Degraded=False; healthy → True/False/False; missing Secret veya ownership conflict → False/False/True; replicas=0 → False/False/False, reason=ScaledToZero. Kalıcı Deployment progress deadline failure Degraded=True olur.
- observedGeneration reconcile'in değerlendirdiği spec generation'ı belirtir; başarılı rollout anlamına gelmez. Her condition da generation taşır. readyReplicas gözlemdir; desiredReplicas spec'ten gelir. Service kapatıldığında endpoint temizlenir.
- Reason sözlüğü kararlı ve sınırlıdır: Reconciling, WorkloadReady, DeploymentUnavailable, ProgressDeadlineExceeded, SecretNotFound, ResourceOwnershipConflict, ReconcileFailed, ScaledToZero. DeploymentCreated/ServiceCreated/PolicyCreated Event reason'ları olabilir.
- lastTransitionTime yalnız condition status değişince güncellenir. Semantik olarak aynı status yeniden yazılmaz; conflict durumunda taze obje üzerinden patch uygulanır. Başarı Event'leri create/gerçek transition ile sınırlanır; event/log message Secret payload veya ham hassas API body içermez.

## 4. Kabul kriterleri

Bir eksik Secret senaryosu:

```text
READY: False
DEGRADED: True
REASON: SecretNotFound
```

üretmelidir.

Ayrıca:

* status spec'i mutate etmez.
* condition transition time anlamlıdır.
* aynı unchanged status tekrar tekrar API'ye yazılmaz.
* current status hangi spec generation'a ait olduğu anlaşılır.
* `kubectl describe aiworkload` developer'a actionable bilgi verir.
* reconcile error sadece log'da kaybolmaz.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Missing Secret ve restore, healthy rollout, failed rollout, scale-to-zero ve ownership conflict için condition truth table assertion'ları yaz.
2. Yeni generation sırasında eski hazır status'u ver; controller Ready=True üretmemeli.
3. Unchanged reconcile döngülerinde status resourceVersion/transition timestamp sabit kalmalı; event spam olmamalı.

## 6. Teslim çıktıları ve kapanış kanıtı

Status calculator, condition helpers, reason/event sözlüğü, failure matrix ve generation/no-op tests.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 09 / 16. Kaynak taslaktaki kimlik: AWC-9 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-6 — Adım 05](https://erayyilmmaz.atlassian.net/browse/AWCP-6), [AWCP-7 — Adım 06](https://erayyilmmaz.atlassian.net/browse/AWCP-7), [AWCP-8 — Adım 07](https://erayyilmmaz.atlassian.net/browse/AWCP-8), [AWCP-9 — Adım 08](https://erayyilmmaz.atlassian.net/browse/AWCP-9)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2–3 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [book.kubebuilder.io/reference/watching-resources/secondary-owned-resources.html](https://book.kubebuilder.io/reference/watching-resources/secondary-owned-resources.html)

---

# AWCP-11 — 10 — Controller observability ve metrics

## 1. Amaç

Controller'ın çalışma durumunu, hatalarını ve gecikmesini ölçülebilir ve güvenli biçimde gözlemlemek.

## 2. Kapsam ve uygulama adımları

* structured controller logging standardı tanımla.
* reconcile duration metric oluştur.
* reconcile outcome metric oluştur.
* resource reconciliation counters oluştur.
* failed reconciliation counter oluştur.
* active AIWorkload gauge oluştur.
* degraded workload gauge değerlendir.
* metric cardinality review yap.
* workload adı gibi unbounded label'ları Prometheus metriclerine koymama kararını uygula.
* health endpoint kullan.
* readiness endpoint kullan.
* controller-runtime built-in metrics'lerini dokümante et.
* Prometheus scrape configuration ekle.
* Grafana dashboard oluştur.
* OpenTelemetry Collector örnek deployment ekle.
* traces gerekiyorsa controller operation span'ları ekle.
* telemetry failure controller core reconciliation'ını bozmamalı.

OpenTelemetry Collector vendor-neutral telemetry pipeline sağlıyor ve Kubernetes üzerinde Deployment, DaemonSet veya StatefulSet olarak kurulabiliyor. ([OpenTelemetry](https://opentelemetry.io/docs/collector/))

### Örnek dashboard

```text
AI Workload Control Plane

Reconcile Rate
Reconcile Errors
Reconcile P95 Duration
Managed Workloads
Ready Workloads
Degraded Workloads
Controller CPU
Controller Memory
```

## 3. Teknik kararlar ve hata sınırları

- Built-in reconcile duration/outcome/error metriklerini önce envanterle; aynı olayı ölçen duplicate custom collector oluşturma. Custom resource operation counter gerekiyorsa kind/operation/result gibi sınırlı label seti kullan.
- Managed/Ready/Degraded gauge'ları restart sonrası cache'den yeniden hesaplanmalı; her reconcile'de kör increment yapıp silmede drift üretmemeli. İsim, namespace, UID, image, URL, Secret ve serbest hata mesajı metric label olamaz.
- Tek kanonik demo pipeline'ı tanımla: controller /metrics → Prometheus → Grafana. OTel Collector için opsiyonel alternatif scrape/export örneği ekle; iki yolun aynı seriyi duplicate toplamasını engelle. Traces koşullu geliştirmedir.
- Metrics endpoint auth/TLS ve scrape RBAC ayarlarını pinned scaffold'a göre doğrula. ServiceMonitor yalnız Prometheus Operator CRD'si bulunan opsiyonel overlay'de olsun; temel install bu CRD'yi gerektirmesin.
- Exporter/Collector/Grafana hatası reconcile'i bloke etmemeli; bounded queue/timeouts kullan. Manager health ve readiness telemetry backend'e bağımlı olmamalı. Controller CPU/memory panelinde kullanılan metrik kaynağı açıkça yazılır.

## 4. Kabul kriterleri

* reconcile error metric'te görünür.
* reconcile latency ölçülebilir.
* controller restart metric endpoint'ini bozmaz.
* telemetry unavailable olduğunda reconcile çalışmaya devam eder.
* metric labels düşük-cardinality'dir.
* secret/user-sensitive data metriclerde bulunmaz.
* Grafana dashboard repo içinde version-control edilir.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Kontrollü reconcile failure yarat; error artışı ve duration histogramının gözlendiğini doğrula.
2. Workload create/degrade/delete ve manager restart sonrası gauge değerlerini gerçek obje sayılarıyla karşılaştır.
3. Collector/Prometheus'u devre dışı bırakıp drift recovery'nin sürdüğünü kontrol et; dashboard JSON ve scrape konfigurasyonunu çalışır demo ile doğrula.

## 6. Teslim çıktıları ve kapanış kanıtı

Structured logging contract, metrics katalogu, scrape/RBAC config, Grafana dashboard JSON, OTel Collector örneği ve telemetry-failure testi.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 10 / 16. Kaynak taslaktaki kimlik: AWC-10 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-5 — Adım 04](https://erayyilmmaz.atlassian.net/browse/AWCP-5), [AWCP-10 — Adım 09](https://erayyilmmaz.atlassian.net/browse/AWCP-10)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2–3 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/reference/metrics.html](https://book.kubebuilder.io/reference/metrics.html)
- [opentelemetry.io/docs/collector/](https://opentelemetry.io/docs/collector/)

---

# AWCP-12 — 11 — Deletion, garbage collection ve lifecycle edge cases

## 1. Amaç

Create/update kadar deletion davranışının da deterministik olmasını sağlamak.

## 2. Kapsam ve uygulama adımları

* ownerReference garbage collection davranışını test et.
* `AIWorkload` silindiğinde:

  * Deployment
  * Service
  * ServiceAccount
  * NetworkPolicy

resource'larının temizlendiğini doğrula.

* user-owned Secret'ın silinmediğini doğrula.
* finalizer ihtiyacını değerlendiren ADR yaz.
* V0'da yalnız Kubernetes-owned child resource cleanup varsa gereksiz finalizer ekleme.
* future external resources için finalizer extension point belge.
* partially-created workload deletion test et.
* deletion sırasında reconciliation race test et.
* child resource already missing durumunu test et.
* repeated delete/reconcile davranışını doğrula.

Kubernetes finalizer mekanizması özellikle controller'ın Kubernetes dışındaki kaynakları temizlemesi gerektiğinde kullanılıyor. Bu nedenle V0 yalnız owner-referenced Kubernetes kaynakları yönetiyorsa finalizer'ı sırf “operator projesinde olur” diye eklememek daha doğru. ([book.kubebuilder.io](https://book.kubebuilder.io/reference/using-finalizers.html))

## 3. Teknik kararlar ve hata sınırları

- Parent NotFound veya deletionTimestamp varsa yeni child üretme. Delete/create yarışı için owner UID ve gerekirse delete precondition kullan; aynı isimle yeniden yaratılan parent eski UID'den ayrılır.
- V0 yalnız Kubernetes owned child'ları yönettiğinden özel finalizer eklenmez; finalizer extension point ADR-008'de gelecekteki dış kaynak gereksinimiyle açıklanır.
- Parent GC temizliği asenkron ve bounded eventual assertion ile doğrulanır. Deployment'ın ReplicaSet/Pod alt ağacı da gerçek cluster testinde temizlenmelidir.
- Envtest yalnız ownerReference ve deletion guard davranışını test eder; gerçek cascading garbage collection, namespace sonlanması ve pod temizliği kind E2E'de kanıtlanır.
- Service/network disable işlemi yalnız güncel owner UID'ye ait child'a uygulanır. Operator uninstall ile CRD uninstall farklıdır; CRD silmek tüm CR'ları ve owned workload'ları etkileyebilir.

## 4. Kabul kriterleri

* parent silindiğinde owned child resources kalmaz.
* user Secret silinmez.
* deletion idempotent'tir.
* stuck terminating resource yaratılmaz.
* finalizer varsa somut bir cleanup requirement'ı vardır.
* gereksiz finalizer bulunmaz.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Normal, kısmi create, child zaten yok ve parent silinirken reconcile senaryolarını çalıştır.
2. Aynı isimle parent recreate et; eski UID child'ı yanlışlıkla sahiplenme veya yeni child'ı silme olmamalı.
3. kind'de parent silindikten sonra dört child ve pod alt ağacı yok, user Secret mevcut olmalı.

## 6. Teslim çıktıları ve kapanış kanıtı

Deletion guards, ADR-008, ownership tests ve E2E GC acceptance senaryoları.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 11 / 16. Kaynak taslaktaki kimlik: AWC-11 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-6 — Adım 05](https://erayyilmmaz.atlassian.net/browse/AWCP-6), [AWCP-7 — Adım 06](https://erayyilmmaz.atlassian.net/browse/AWCP-7), [AWCP-8 — Adım 07](https://erayyilmmaz.atlassian.net/browse/AWCP-8), [AWCP-9 — Adım 08](https://erayyilmmaz.atlassian.net/browse/AWCP-9), [AWCP-10 — Adım 09](https://erayyilmmaz.atlassian.net/browse/AWCP-10)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 1–1.5 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/reference/using-finalizers.html](https://book.kubebuilder.io/reference/using-finalizers.html)
- [book.kubebuilder.io/reference/envtest.html](https://book.kubebuilder.io/reference/envtest.html)

---

# AWCP-13 — 12 — Unit + envtest integration test suite

## 1. Amaç

Controller behavior'ını gerçek cluster kurmadan büyük ölçüde doğrulayabilmek.

Kubebuilder'ın `envtest` yaklaşımı lokal `etcd` + Kubernetes API server çalıştırarak controller integration testleri yapılmasını sağlıyor. ([book.kubebuilder.io](https://book.kubebuilder.io/reference/envtest))

## 2. Kapsam ve uygulama adımları

### Unit test grupları

Test et:

* naming functions
* labels
* Deployment builder
* Service builder
* ServiceAccount builder
* NetworkPolicy builder
* condition helpers
* defaulting/helper logic
* status calculation

### Envtest senaryoları

#### Valid create

```text
Create AIWorkload
→ Deployment exists
→ Service exists
→ ServiceAccount exists
→ NetworkPolicy exists
```

#### Update image

```text
v1 → v2
→ Deployment updated
```

#### Update replicas

```text
1 → 3
→ Deployment replicas=3
```

#### Drift

```text
Delete Deployment
→ controller recreates Deployment
```

#### Missing Secret

```text
secretRef missing
→ Ready=False
→ Degraded=True
```

#### Disable service

```text
enabled=true
→ enabled=false
→ Service deleted
```

#### Invalid CR

API server validation tarafından reddedilir.

#### Delete parent

Envtest: ownerReference ve deletion guard doğrulanır. Gerçek owned-resource garbage collection kind E2E'de doğrulanır.

## 3. Teknik kararlar ve hata sınırları

- Test piramidi: saf builder/condition unit testleri; API schema/default/status/watch için envtest; scheduler/kubelet/rollout/Service trafiği/GC için kind. envtest'te Deployment status'u gerekiyorsa test fixture patch'ler; gerçek scheduler sonucu diye sunulmaz.
- Parent delete envtest kabulü doğru owner UID ve deletion guard'dır. Gerçek child garbage collection assertion'ı E2E'ye aittir. Envtest namespace teardown için gerçek namespace controller beklenmez.
- Suite yalnız Secret missing değil create/delete/restore watcher, all-child drift, enable/disable, collision, status no-op, explicit false/zero, Forbidden/Conflict ve restart senaryolarını kapsar.
- İlk bağımlılık/binary hazırlığı internet gerektirebilir; hazır/cached asset ile unit/envtest çalıştırma dış API gerektirmez. Ağsız ilk kurulum garantisi verilmez.
- Eventually/Consistently gibi bounded assertion'lar, context cancellation, benzersiz namespace, goroutine cleanup ve tek seferlik envtest.Stop hata kontrolü kullan. Rastgele sleep ve blanket retry ile gerçek hataları gizleme.

## 4. Kabul kriterleri

* critical reconciliation branches testlidir.
* tests deterministic'tir.
* external cloud bağımlılığı yoktur.
* real OpenAI/Anthropic API çağrısı yoktur.
* Hazırlanmış tool/binary bağımlılıkları ile unit/envtest çalışması dış servise ihtiyaç duymaz; ilk indirme ayrı ön koşuldur.
* race-sensitive testlerde bounded eventual assertions kullanılır.
* code coverage raporu CI'da üretilebilir.
* generated manifests test öncesi güncellenir.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Unit suite + envtest'i temiz test state'iyle çalıştır; coverage ve uygun platformda race çıktısını kaydet.
2. API validation/defaulting ve watch-triggered reconcile'i gerçek envtest API ile doğrula; fake client sonucu bunun yerine geçmez.
3. Başarısız assertion ve shutdown senaryosunda test timeout/teardown'un anlaşılır hata ve temiz process bıraktığını kontrol et.

## 6. Teslim çıktıları ve kapanış kanıtı

Unit/envtest suites, traceability test matrisi, fixtures, coverage raporu ve test prerequisites.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 12 / 16. Kaynak taslaktaki kimlik: AWC-12 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-6 — Adım 05](https://erayyilmmaz.atlassian.net/browse/AWCP-6), [AWCP-7 — Adım 06](https://erayyilmmaz.atlassian.net/browse/AWCP-7), [AWCP-8 — Adım 07](https://erayyilmmaz.atlassian.net/browse/AWCP-8), [AWCP-9 — Adım 08](https://erayyilmmaz.atlassian.net/browse/AWCP-9), [AWCP-10 — Adım 09](https://erayyilmmaz.atlassian.net/browse/AWCP-10), [AWCP-11 — Adım 10](https://erayyilmmaz.atlassian.net/browse/AWCP-11), [AWCP-12 — Adım 11](https://erayyilmmaz.atlassian.net/browse/AWCP-12)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 3–4 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/reference/envtest.html](https://book.kubebuilder.io/reference/envtest.html)
- [github.com/kubernetes-sigs/kubebuilder/releases](https://github.com/kubernetes-sigs/kubebuilder/releases)
- [book.kubebuilder.io/reference/envtest](https://book.kubebuilder.io/reference/envtest)

---

# AWCP-14 — 13 — kind-based end-to-end test environment

## 1. Amaç

Gerçek Kubernetes scheduler/kubelet/controller-manager içeren lokal cluster'da uçtan uca demo ve regression testleri yapmak.

`kind`, Docker container'larını Kubernetes node olarak kullanıyor ve local development/CI için tasarlanmış durumda. ([kind.sigs.k8s.io](https://kind.sigs.k8s.io/))

## 2. Kapsam ve uygulama adımları

* pinned Kubernetes node image kullan.
* kind config oluştur.
* test namespace oluştur.
* controller image build et.
* image'ı kind cluster'a load et.
* CRD install et.
* controller deploy et.
* sample workload image oluştur.
* sample app health endpoints ekle.
* E2E harness oluştur.

### E2E scenario 1 — create

```text
kubectl apply AIWorkload

→ Deployment Available
→ Service exists
→ ServiceAccount exists
→ NetworkPolicy exists
→ AIWorkload Ready=True
```

### Scenario 2 — update

```text
image v1 → v2
```

rollout başarıyla tamamlanmalı.

### Scenario 3 — scale

```text
replicas 1 → 2
```

iki ready replica görülmeli.

### Scenario 4 — drift recovery

```text
kubectl delete deployment demo
```

controller yeniden yaratmalı.

### Scenario 5 — failure

Referenced Secret sil:

```text
Ready=False
Degraded=True
```

olmalı.

Secret geri oluştur:

```text
Ready=True
```

durumuna dönmeli.

### Scenario 6 — deletion

```text
kubectl delete aiworkload demo
```

owned resources temizlenmeli.

## 3. Teknik kararlar ve hata sınırları

- Cluster ve test image sürümü/digest'i 01. adım matrisine uyar. kind v0.33.0 release sayfasındaki default node 1.36.1'dir; Kubernetes 1.36.4 için hazır node image varlığı varsayılmaz. Seçilen exact node image ve digest ayrıca doğrulanır.
- make kind-up → make install → image build/load → make deploy → rollout wait → sample apply akışı tek komutlu E2E target'ında birleştirilir. make install tek başına operator process'ini başlatmaz.
- Demo app tamamen yereldir: /health, /ready, sürüm çıktısı; gerçek LLM credential veya ücretli API yoktur. v1/v2 image yerel build/load yapılır; registry publish zorunlu değildir.
- Her test explicit kube-context ve benzersiz cluster/namespace kullanır; mevcut kullanıcı cluster'ını yanlışlıkla silmez. Cleanup yalnız bu suite'in oluşturduğu kaynakları hedefler.
- Image failure, probe failure, Service request, all-child drift, Secret restore, operator restart ve GC için bounded timeout ve başarısızlık halinde log/Event/status artifact toplama ekle. Secret data dump alma.
- Zorunlu profile standard policy generation doğrular. CNI enforcement profili ayrı ve opsiyoneldir; uygulanırsa CNI sürümü pinlenir ve deny/allow trafik testleri koşulur. ARM64 yerel çalışma ve Linux AMD64 CI sonuçları ayrı raporlanır.

## 4. Kabul kriterleri

* tek komutla E2E ortamı kurulabilir.
* tek komutla silinebilir.
* manual intervention gerektirmez.
* test timeout'ları vardır.
* cluster cleanup başarısız olsa bile script anlaşılır hata verir.
* pinned Kubernetes sürümü kullanılır.
* ARM64/macOS geliştirme ortamı göz önünde bulundurulur.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Temiz kind cluster'da altı orijinal E2E senaryosunu sırayla otomatik çalıştır ve Ready/rollout/GC sonuçlarını kaydet.
2. Manager restart ardından drift oluştur; recovery ve gauges yeniden kurulsun.
3. Testi kontrollü başarısız kıl; artifacts ve cleanup çalışmalı, başka cluster/context etkilenmemeli.

## 6. Teslim çıktıları ve kapanış kanıtı

kind config, local demo app/image, E2E harness, Make targets, diagnostics/cleanup ve platform uyumluluk notu.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 13 / 16. Kaynak taslaktaki kimlik: AWC-13 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-13 — Adım 12](https://erayyilmmaz.atlassian.net/browse/AWCP-13)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2–3 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [github.com/kubernetes-sigs/kind/releases/tag/v0.33.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0)
- [kind.sigs.k8s.io/](https://kind.sigs.k8s.io/)

---

# AWCP-15 — 14 — Packaging ve developer installation experience

## 1. Amaç

Projeyi yalnız repository içinden çalışan kod olmaktan çıkarıp kurulabilir bir platform bileşenine çevirmek.

## 2. Kapsam ve uygulama adımları

* production Docker image oluştur.
* Kustomize default install akışını doğrula.
* Helm packaging gereksinimini değerlendir.
* V0 için Helm chart oluşturulacaksa:

  * namespace
  * image
  * tag
  * replicas
  * resources
  * metrics
  * leader election
    values'larını destekle.
* CRD installation sürecini belge.
* controller RBAC manifests dahil et.
* uninstall davranışını belge.
* CRD deletion riskini açıkla.
* version metadata ekle.
* container image tagging strategy oluştur.
* release artifact strategy belirle.

## 3. Teknik kararlar ve hata sınırları

- Kustomize V0'ın zorunlu canonical install yoludur. Helm opsiyonel karar kaydıdır; chart yapılmazsa bu açıkça yazılır ve V0 eksik sayılmaz. Yapılırsa template/lint + gerçek install/upgrade/uninstall testleri aynı contract'a bağlanır.
- CRD installation ile manager deployment ayrı anlatılır. Go gerektirmeyen release bundle ve image referansı hazırlanır; yayın öncesi kullanıcının erişemeyeceği image'la quick start başarılı gösterilmez.
- Operator undeploy/uninstall workload namespace'ini veya CRD'yi otomatik silmemeli. CRD ve AIWorkload silme komutları ayrı, etkisi açıklanmış cleanup bölümünde verilir. Workload Namespace controller'a ownerReference ile bağlanmaz.
- Image tag/digest, controller resources, namespace/watch kapsamı, metrics ve leader-election ayarları tüm paketlerde tutarlı olmalı. Helm eklenirse mevcut CRD'lerin upgrade davranışı ayrıca belgelenir.
- 5 dakika hedefi Docker/cluster ve gerekli image'lar hazır olduktan sonraki deploy/sample akışına aittir; ilk indirmeler ve cluster hazırlama süresi ayrı ölçülür.

## 4. Kabul kriterleri

Şunlardan biriyle controller kurulabilir:

```bash
make deploy
```

veya release sonrasında:

```bash
helm install ...
```

* sample `AIWorkload` 5 dakikadan kısa developer flow ile çalıştırılabilir.
* user'ın Go build environment kurması production install için gerekmez.
* install/uninstall documented'dır.
* uninstall user workload verisini sürpriz biçimde yok etmez.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Go kurulu olmayan kullanıcı akışını hazır image/bundle üzerinden doğrula; controller Ready ve sample Ready olsun.
2. Undeploy sonrası CR/CRD/user Secret korunmalı; explicit cleanup komutları beklendiği gibi çalışmalı.
3. Varsa Helm render/install/upgrade/uninstall ve Kustomize çıktısının image/RBAC/security eşdeğerliğini kontrol et.

## 6. Teslim çıktıları ve kapanış kanıtı

Kustomize install/release bundle, opsiyonel Helm kararı/chart, image/version metadata ve install/upgrade/uninstall kılavuzu.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 14 / 16. Kaynak taslaktaki kimlik: AWC-14 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-14 — Adım 13](https://erayyilmmaz.atlassian.net/browse/AWCP-14)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 1.5–2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [book.kubebuilder.io/cronjob-tutorial/basic-project.html](https://book.kubebuilder.io/cronjob-tutorial/basic-project.html)

---

# AWCP-16 — 15 — CI quality gates ve supply-chain hygiene

## 1. Amaç

Her PR'ın üretilebilir ve test edilebilir olduğunu kanıtlamak.

## 2. Kapsam ve uygulama adımları

### GitHub Actions jobs

```text
format
lint
vet
generate-check
manifest-check
unit-test
envtest
build
docker-build
e2e
```

### Tasks

* GitHub Actions workflow oluştur.
* dependency caching yapılandır.
* generated-code drift check oluştur.
* generated-manifest drift check oluştur.
* unit tests çalıştır.
* envtest çalıştır.
* kind E2E çalıştır.
* Docker build doğrula.
* Go vulnerability scan eklemeyi değerlendir.
* dependency update strategy belirle.
* build artifact version bilgisi ekle.
* timeout ayarla.
* concurrent duplicate CI runs cancel et.
* branch protection için required checks dokümante et.

### Quality gate

Aşağıdaki başarısızlıklar required-check olarak etkinleştirilmiş branch protection/ruleset altında merge'i engellemelidir:

```text
lint      FAIL
test      FAIL
envtest   FAIL
build     FAIL
e2e       FAIL
```

## 3. Teknik kararlar ve hata sınırları

- PR workflow varsayılan permissions contents:read; üçüncü taraf actions tam commit SHA ile pinlenir. Untrusted PR kodu privileged pull_request_target bağlamında çalıştırılmaz; build/test için publish credential gerekmez.
- Formatting, lint, vet, code generation, manifests, unit, envtest, build, Docker ve e2e check adları kararlı tutulur. Fork PR da aynı secretsiz doğrulamaları koşabilmelidir.
- Generate/manifests sonrası tracked değişiklik yanında yeni untracked generated dosyaları da yakala; yalnız git diff kontrolü yeterli olmayabilir. go.mod/go.sum drift kontrolü ekle.
- govulncheck kullanımı ve dependency update policy kararı yazılır; seçilen scan CI'a eklenir. Image/base dependency sürümleri sabitlenir. Vulnerability database güncellemesinin ağ gerektirdiği ve tarama snapshot zamanı belgelenir.
- Workflow başarısızlığı tek başına merge'i teknik olarak engellemez. Required status checks/ruleset ayrı yapılandırılmalı ve aktifliği kanıtlanmalı; yapılandırılmamışsa 'merge blocked' iddiası yapılmaz.
- Publish/release işi PR validation'dan ayrıdır. Code/test sonucu, hosted CI sonucu, branch protection ayarı, image publication ve deploy kanıtları ayrı raporlanır.

## 4. Kabul kriterleri

* clean checkout CI'da başarılıdır.
* local generated artefact değişmiş ama commit edilmemişse CI bunu yakalar.
* failing test merge gate'i kırar.
* test sonuçları görünürdür.
* CI external paid cloud hesabı gerektirmez.
* workflow secret gerektirmeden PR üzerinde çalışabilir.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Hosted temiz checkout PR run'ında bütün required jobs'u gözlemle; yalnız yerel komut başarısını CI başarısı sayma.
2. Kontrollü test failure ve generated drift değişikliğiyle ilgili gate'in kırıldığını doğrula.
3. Fork/secretsiz PR izinleri, timeout/concurrency ve retained diagnostics'i kontrol et; ruleset aktif değilse açık takip maddesi bırak.

## 6. Teslim çıktıları ve kapanış kanıtı

GitHub Actions workflows, check-name/branch-protection kılavuzu, supply-chain policy ve hosted run kanıtları.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 15 / 16. Kaynak taslaktaki kimlik: AWC-15 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-13 — Adım 12](https://erayyilmmaz.atlassian.net/browse/AWCP-13), [AWCP-14 — Adım 13](https://erayyilmmaz.atlassian.net/browse/AWCP-14)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [docs.github.com/en/actions/reference/security/secure-use](https://docs.github.com/en/actions/reference/security/secure-use)

---

# AWCP-17 — 16 — Documentation, portfolio demo ve V0 release

## 1. Amaç

Recruiter veya engineer repo'ya girdiğinde projenin değerini birkaç dakika içinde anlayabilsin.

## 2. Kapsam ve uygulama adımları

### README sections

* Problem
* Why this exists
* Architecture
* Key concepts
* Quick Start
* AIWorkload API
* Example
* Reconciliation model
* Drift recovery
* Security model
* Observability
* Testing strategy
* Architecture Decisions
* Limitations
* Roadmap

### Tasks

* architecture diagram ekle.
* sequence diagram ekle.

Örneğin:

```text
Developer
   |
   | kubectl apply
   v
API Server
   |
   v
Controller
   |
   +---- Get desired state
   |
   +---- Compare observed state
   |
   +---- Create/Update resources
   |
   +---- Update status
```

* demo script oluştur.
* `examples/basic.yaml`
* `examples/with-secrets.yaml`
* `examples/network-policy.yaml`
* failure demo hazırla.
* drift demo hazırla.
* screenshots/GIF gerekiyorsa portfolio aşamasında ekle.
* release checklist yaz.
* semantic versioning stratejisi oluştur.
* `v0.1.0` tag/release hazırlığı yap.

### Portfolio demo

Demo şu sırayı gösterebilmeli:

```text
1. Create AIWorkload

2. Deployment appears

3. Service appears

4. Workload becomes Ready

5. Manually delete Deployment

6. Controller restores it

7. Change image

8. Rolling update occurs

9. Remove required Secret

10. Workload becomes Degraded

11. Restore Secret

12. Workload becomes Ready

13. Open Grafana

14. Show reconcile metrics

15. Delete AIWorkload

16. Owned resources disappear
```

## 3. Teknik kararlar ve hata sınırları

- README quick start eksiksiz operator başlangıcını içermeli: prerequisites → kind-up → CRD install → manager image build/load veya hazır release image → deploy → manager rollout wait → sample apply → Ready wait. Sadece make install sonrası sample uygulamak yeterli değildir.
- Demo 16 adımın tamamını gerçek cluster üzerinde göstermeli. Secret failure/recovery condition kanıtıdır; çalışan process'ten credential silindiği iddiası değildir. Grafana açma adımı kullanıcı demo aşamasına aittir.
- README sınırları: alpha API, public/non-root image beklentisi, Secret env rotation kısıtı, CNI enforcement koşulu, aynı-namespace ingress default'u, egress kısıtlaması olmaması, custom DNS varsayımı yapılmaması ve full tenancy bulunmaması.
- Release readiness ile gerçek tag/GitHub release/image publication farklı sonuçlardır. v0.1.0 notları ve artefact'lar hazırlanır; yayınlanma ancak gerçek link/digest ve başarılı workflow ile tamamlandı sayılır.
- Story teslim standardı: değişen dosyalar, komutlar/sonuçları, kabul kriteri kanıtları, bilinen eksikler ve bir sonraki gerçek Jira issue bağlantısı. Test edilmemiş davranış Done kanıtı değildir.
- Tahmin aralıkları başlangıç varsayımıdır. 2 saat/gün ile 18–20 çalışma günü yaklaşık 4 iş haftası veya her gün çalışılırsa yaklaşık 3 takvim haftasıdır; çalışma günleri ile takvim günlerini karıştırma.

## 4. Kabul kriterleri

Yeni biri README üzerinden:

```bash
git clone ...
make kind-up
make install
# Manager image hazırlanır ve kind cluster'a yüklenir.
make deploy
# Manager Deployment Available olana kadar beklenir.
kubectl apply -f examples/basic.yaml
kubectl get aiworkloads
```

akışını tamamlayabilir.

Ayrıca:

* mimari README'de açıktır.
* project scope dürüstçe açıklanır.
* “production-ready Kubernetes platform” gibi aşırı claim yapılmaz.
* known limitations belgelenmiştir.
* demo gerçek reconciliation davranışını gösterir.
* GitHub release hazırlanabilir durumdadır.

- Teknik kararlar bölümündeki bütün zorunlu davranışlar uygulanmış ve aşağıdaki doğrulama senaryolarıyla kanıtlanmıştır; opsiyonel özelliklerin kararı belgelenmiştir.

## 5. Adım adım doğrulama

1. Projeyi bilmeyen kullanıcı akışıyla README komutlarını temiz kind ortamında sırayla uygula; eksik image/deploy/env adımı olmamalı.
2. 16 demo adımını ve Epic Definition of Done listesini kanıt linkleriyle tek tek karşılaştır.
3. Examples CRD validation'dan geçmeli; broken documentation link, secret placeholder ve release claim kontrolü yapılmalı.

## 6. Teslim çıktıları ve kapanış kanıtı

README, diagrams, üç example manifest, demo script, release checklist/notes ve V0 kabul kanıtı.

- Değişen dosyalar ve ilgili test adları kaydedilir.
- Çalıştırılan komutlar, ortam/sürüm ve sonuçlar eklenir; planlanan kontroller çalıştırılmış gibi raporlanmaz.
- Kabul kriterleri kanıtla eşleştirilir. Bilinen sınırlamalar ve sonraki adım belirtilir.
- İlgili testler davranış geliştirilirken yazılır; test-suite adımı eksik davranış testlerini sona erteleme gerekçesi değildir.

## 7. Sıra, bağımlılıklar ve tahmin

Uygulama sırası: 16 / 16. Kaynak taslaktaki kimlik: AWC-16 (gerçek Jira anahtarı değildir).

Bağımlılıklar: [AWCP-15 — Adım 14](https://erayyilmmaz.atlassian.net/browse/AWCP-15), [AWCP-16 — Adım 15](https://erayyilmmaz.atlassian.net/browse/AWCP-16)

Epic: [AWCP-1](https://erayyilmmaz.atlassian.net/browse/AWCP-1)

Başlangıç tahmini: 2 saat. Entegrasyon, öğrenme ve hata ayıklama süresine göre yeniden değerlendirilir; süre taahhüdü değildir.

## 8. Teknik kaynaklar

12 Eylül 2026 tarihinde backlog hazırlığı sırasında kullanılan resmi kaynaklar. Projede çalıştığı ayrıca testlerle doğrulanacaktır.

- [kind.sigs.k8s.io/](https://kind.sigs.k8s.io/)
- [kubernetes.io/docs/concepts/configuration/secret/](https://kubernetes.io/docs/concepts/configuration/secret/)
- [kubernetes.io/docs/concepts/services-networking/network-policies/](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
