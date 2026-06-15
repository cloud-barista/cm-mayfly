# Unreleased
### Changelog
* feat(docker): manage shared docker-compose env vars via a single conf/docker/.env file
  * Credentials, DB settings, SMTP, and log levels are now injected through `${VAR}` interpolation from `conf/docker/.env` instead of being hardcoded inline or spread across per-service env_file files.
  * Added `conf/docker/.env.example` template and gitignored the real `conf/docker/.env`.
  * The `infra` commands now fail with a clear message when `conf/docker/.env` is missing.
  * Copy step: `cp conf/docker/.env.example conf/docker/.env` then set the required values.
* CB-Tumblebug 라인업 0.12.9 업그레이드 (cb-tumblebug 0.12.1 → 0.12.9, cb-spider 0.12.0 → 0.12.17, cb-mapui 0.12.1 → 0.12.30)
* mc-terrarium 0.1.4 신규 추가 (OpenTofu 기반 자원 확장)
* openbao 2.5.1 신규 추가 (CSP 자격증명 시크릿 매니저, persistent 모드)
* openbao-unseal sidecar 신규 추가 — 재기동 시 OpenBao 자동 unseal (사용자 개입 불필요)
* cb-tumblebug-postgres `max_connections=500` 적용
* `setup tumblebug-init` 명령 확장 — multi-init.sh 호출로 openbao 초기화 + tumblebug init 통합 (최초 1회 사용)
* 이전 릴리스에서 *임시 보존*으로 두었던 per-service env_file 잔여물 일괄 제거 — `conf/docker/conf/cm-butterfly/api/.env`, `conf/docker/conf/cm-cicada/airflow_smtp.env`, 그리고 `docker-compose.yaml`의 주석 처리된 `# env_file:` 라인까지 모두 삭제. 환경변수는 `conf/docker/.env`로 일원화 완료.

### Feature
* cb-spider / cb-tumblebug / mc-terrarium 인증 환경변수(`SPIDER_USERNAME`/`SPIDER_PASSWORD`, `TB_API_*`, `TERRARIUM_API_*`)와 OpenBao 연동(`VAULT_ADDR`/`VAULT_TOKEN`)을 `conf/docker/.env`로 추가 관리
* `conf/openbao/openbao-config.hcl` 신규 — OpenBao persistent 모드 설정 (KMS Auto-Unseal stanza 주석 예시 포함)
* `mayfly infra` 하위 명령 실행 직전 `conf/docker/.env`의 필수값을 자체 검증 (FR-04, [BAR-866](https://mzdevs.atlassian.net/browse/BAR-866)). cb-spider 0.12.17의 REST 인증(`SPIDER_USERNAME`/`SPIDER_PASSWORD`)과 5종 DB 비밀번호(`TUMBLEBUG_DB_PASSWORD`, `BUTTERFLY_DB_PASSWORD`, `ANT_DB_PASSWORD`, `AIRFLOW_DB_PASSWORD`, `AIRFLOW_DB_ROOT_PASSWORD`)가 비어 있으면 어떤 키가 누락됐는지 명시한 뒤 docker compose 호출 전에 중단해, 컨테이너가 영문도 모르고 죽는 상황을 막는다.

### Notice
* cb-spider 0.12.17은 `SPIDER_USERNAME`/`SPIDER_PASSWORD`가 비어있으면 log.Fatal로 컨테이너 기동을 차단합니다. `conf/docker/.env.example`에는 다른 비밀번호 항목과 동일하게 **빈 값**으로 제공되므로, `cp conf/docker/.env.example conf/docker/.env` 후 두 값을 **반드시** 설정해야 합니다. 누락 시 `mayfly infra` 단계에서 위 *Feature*의 필수값 검증이 어떤 키가 비었는지 알려주고 docker compose 호출 전에 중단합니다.
* openbao-unseal sidecar는 호스트 파일에 평문 unseal 키를 보관합니다. 운영 등급에 따라 KMS Auto-Unseal 또는 수동 unseal 전환을 검토하세요. 자세한 가이드는 `docs/upgrade-cb-tb-0.12.9.md` 참조.
* clean 재기동 권장 — 자동 데이터 마이그레이션 미제공.

# v0.1.0 (2023.12.14.)
### Changelog
* cm-mayfly public (Based on Docker Compose)

### Feature
* pull: Download the Cloud-Migrator container images to your local image store
* run: Run Cloud-Migrator containers to drive the Cloud-Migrator system
* info: Display the status of Cloud-Migrator containers and the status of images
* stop: Stop the Cloud-Migrator system by stopping the Cloud-Migrator containers
* remove: Remove the Cloud-Migrator container (+ volumes, images)
