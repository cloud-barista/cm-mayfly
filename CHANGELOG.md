# v0.6.0
### Changelog
* Move every C-Mig framework in the lineup to `0.6.0` — cm-beetle, cm-butterfly (api and front), cm-honeybee, cm-damselfly, cm-cicada, airflow-server, cm-grasshopper and cm-ant. The Cloud-Barista components keep their own versioning: cb-tumblebug `0.12.25`, cb-spider `0.12.35`, cb-mapui `0.12.50`, mc-terrarium `0.1.4`. `conf/api.yaml` follows the same set.
* Cap the logs a running stack writes. MySQL binary logging is off by default — `airflow-mysql` is a single instance with no replication, point-in-time recovery or change-data-capture to replay it — and container log size and file count are set, both configurable from `.env`. A lineup left running had filled a disk until image pulls began to fail.
* Drop the bundled cb-tumblebug assets under `conf/docker/conf/cb-tumblebug/assets/` (33 files, about 33 MB). Both images already ship those files, so the copy added nothing, and the Spider half of it was older than the image it was meant for.
* `infra update` now updates only the services its version check found stale, instead of restarting everything. Floating tags are compared by digest.
* feat(docker): manage shared docker-compose env vars via a single conf/docker/.env file
  * Credentials, DB settings, SMTP, and log levels are now injected through `${VAR}` interpolation from `conf/docker/.env` instead of being hardcoded inline or spread across per-service env_file files.
  * Added `conf/docker/.env.example` template and gitignored the real `conf/docker/.env`.
  * The `infra` commands now fail with a clear message when `conf/docker/.env` is missing.
  * Copy step: `cp conf/docker/.env.example conf/docker/.env` then set the required values.
* CB-Tumblebug 라인업 업그레이드 (cb-tumblebug 0.12.1 → 0.12.25, cb-spider 0.12.0 → 0.12.35, cb-mapui 0.12.1 → 0.12.50)
* mc-terrarium 0.1.4 신규 추가 (OpenTofu 기반 자원 확장)
* openbao 2.5.1 신규 추가 (CSP 자격증명 시크릿 매니저, persistent 모드)
* openbao-unseal sidecar 신규 추가 — 재기동 시 OpenBao 자동 unseal (사용자 개입 불필요)
* cb-tumblebug-postgres `max_connections=500` 적용
* `setup tumblebug-init` 명령 확장 — multi-init.sh 호출로 openbao 초기화 + tumblebug init 통합 (최초 1회 사용)
* 이전 릴리스에서 *임시 보존*으로 두었던 per-service env_file 잔여물 일괄 제거 — `conf/docker/conf/cm-butterfly/api/.env`, `conf/docker/conf/cm-cicada/airflow_smtp.env`, 그리고 `docker-compose.yaml`의 주석 처리된 `# env_file:` 라인까지 모두 삭제. 환경변수는 `conf/docker/.env`로 일원화 완료.
* CLI 명령 처리 정비 (2026-07-18) — 서비스명 파싱을 단일 `resolveServices`로 통합해 `-s`에 넘긴 이름을 docker-compose 정의와 대조 검증(이전엔 존재하지 않는 이름도 exit 0으로 통과), docker 명령을 셸 문자열 조립에서 인자 배열 실행으로 전환, `common.SysCall`이 실제 종료코드를 반환하도록 변경, `rest`/`api` 명령에 종료코드 매핑과 타임아웃(`MAYFLY_HTTP_TIMEOUT`, 기본 30초) 도입, `.env` 필수값 검증 범위를 컨테이너를 기동하는 `run`/`update`로 한정.

### Feature
* cb-spider / cb-tumblebug / mc-terrarium 인증 환경변수(`SPIDER_USERNAME`/`SPIDER_PASSWORD`, `TB_API_*`, `TERRARIUM_API_*`)와 OpenBao 연동(`VAULT_ADDR`/`VAULT_TOKEN`)을 `conf/docker/.env`로 추가 관리
* `conf/openbao/openbao-config.hcl` 신규 — OpenBao persistent 모드 설정 (KMS Auto-Unseal stanza 주석 예시 포함)
* 컨테이너를 기동하는 `mayfly infra run`/`update` 실행 직전 `conf/docker/.env`의 필수값을 자체 검증. cb-spider 0.12.35의 REST 인증(`SPIDER_USERNAME`/`SPIDER_PASSWORD`)과 5종 DB 비밀번호(`TUMBLEBUG_DB_PASSWORD`, `BUTTERFLY_DB_PASSWORD`, `ANT_DB_PASSWORD`, `AIRFLOW_DB_PASSWORD`, `AIRFLOW_DB_ROOT_PASSWORD`)가 비어 있으면 어떤 키가 누락됐는지 명시한 뒤 docker compose 호출 전에 중단해, 컨테이너가 영문도 모르고 죽는 상황을 막는다. 필수값 검증은 컨테이너를 띄우는 `run`/`update`에만 적용되고, `remove`/`stop`/`info`/`logs` 등 나머지 하위 명령은 값 검증 없이 실행된다(파일 존재 검증은 전 하위 명령 유지).

### Notice
* cb-spider 0.12.35는 `SPIDER_USERNAME`/`SPIDER_PASSWORD`가 비어있으면 log.Fatal로 컨테이너 기동을 차단합니다. `conf/docker/.env.example`에는 다른 비밀번호 항목과 동일하게 **빈 값**으로 제공되므로, `cp conf/docker/.env.example conf/docker/.env` 후 두 값을 **반드시** 설정해야 합니다. 누락 시 `mayfly infra` 단계에서 위 *Feature*의 필수값 검증이 어떤 키가 비었는지 알려주고 docker compose 호출 전에 중단합니다.
* openbao-unseal sidecar는 호스트 파일에 평문 unseal 키를 보관합니다. 운영 등급에 따라 KMS Auto-Unseal 또는 수동 unseal 전환을 검토하세요. 자세한 가이드는 `docs/openbao-unseal.md` 참조.
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
