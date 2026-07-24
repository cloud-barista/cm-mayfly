# CB-Tumblebug 0.12.9 라인업 업그레이드 가이드

> ⚠ **이 문서는 이력(historical) 문서입니다.** cb-tumblebug 0.12.9 라인업 마일스톤 시점의 업그레이드 절차를 기록한 것으로, 이후 라인업 버전 상향(cb-tumblebug 0.12.25·cb-spider 0.12.35·cb-mapui 0.12.50 등)과 OpenBao 운영 모델 변경(0.12.25 서버 내재화·Phase 0 init/unseal·state-consistency preflight)으로 일부 절차·버전이 현행과 다릅니다. **현행 OpenBao 운영은 [`docs/openbao-unseal.md`](openbao-unseal.md), 현행 라인업은 [`conf/docker/docker-compose.yaml`](../conf/docker/docker-compose.yaml)를 참조하세요.** 아래 본문은 당시 기록으로 보존합니다.

> 본 문서는 cm-mayfly가 오케스트레이션하는 CB-Tumblebug 라인업을 v0.12.9로 업그레이드하는 절차와 변경 사항 검증 방법을 안내합니다.

---

## 1. 변경 요약

| 항목 | Before | After |
|------|--------|------|
| cb-tumblebug | 0.12.1 | **0.12.9** |
| cb-spider | 0.12.0 | **0.12.17** |
| cb-mapui | 0.12.1 | **0.12.30** |
| mc-terrarium | — | **0.1.4** (신규) |
| openbao | — | **2.5.1** (신규, persistent 모드) |
| openbao-unseal | — | **alpine:3.20 sidecar** (신규, 자동 unseal) |
| cb-tumblebug-postgres | (기본 설정) | `max_connections=500` 추가 |

### 주요 변경 포인트

- **cb-spider 0.12.17 강제 인증**: `SPIDER_USERNAME`/`SPIDER_PASSWORD`가 비어있으면 `log.Fatal`로 컨테이너 기동 자체가 차단됩니다. cm-mayfly는 `default`/`default` 기본값으로 충족하지만 운영 환경에서는 `.env`로 외부 주입을 권장합니다.
- **multi-init.sh 도입**: 기존 `init.sh` 대신 OpenBao 자격증명 등록 + tumblebug init을 통합 처리합니다. `mayfly setup tumblebug-init`이 자동 분기합니다.
- **openbao-unseal sidecar**: 재기동 시 사용자가 unseal 명령을 입력할 필요 없이 sidecar가 자동 처리합니다.
- **공통 환경변수**: `conf/docker/.env.example`을 참조해 자격증명·DB·로그레벨을 한 곳에서 관리할 수 있습니다.

---

## 2. 사전 준비

- [`credentials.yaml.enc`](https://github.com/cloud-barista/cb-tumblebug?tab=readme-ov-file#3-initialize-cb-tumblebug-to-configure-multi-cloud-info) 파일을 `~/.cloud-barista/` 디렉터리에 미리 준비 (CSP 자격증명 암호화 파일)
- 디스크 여유 약 1GB 이상 (신규 컨테이너 3개)
- `uv` Python 패키지 관리자 (multi-init.sh의 자격증명 등록 과정에서 필요)

---

## 3. 신규 설치 절차

```bash
# (선택) 환경변수 외부 주입 시
cp conf/docker/.env.example conf/docker/.env
vi conf/docker/.env

# 1. 컨테이너 기동
./mayfly infra run

# 2. 최초 1회 초기화 (CSP 자격증명 등록 + OpenBao 초기화 + Tumblebug init)
./mayfly setup tumblebug-init
# → credentials.yaml.enc 비밀번호 입력
# → Step 1: OpenBao 초기화·unseal·자격증명 등록
# → Step 2: Tumblebug namespace·자산 등록

# 3. 정상 동작 확인
./mayfly infra info
# → 모든 컨테이너 status=healthy 확인
```

> ⚠ **tumblebug-init은 최초 인프라 구축 시 1회만 실행합니다.** 재기동 시 재실행은 불필요합니다.

---

## 4. 기존 환경 업그레이드 절차

cm-mayfly는 자동 데이터 마이그레이션을 제공하지 않습니다. clean 재기동 권장.

```bash
# (선택) 기존 데이터 백업 — 사용자 책임
cp -r data/cb-tumblebug/ /backup/cb-tumblebug-$(date +%Y%m%d)/
cp -r data/cb-spider/    /backup/cb-spider-$(date +%Y%m%d)/

# 1. 기존 컨테이너 정리
./mayfly infra remove

# 2. (필요 시) 데이터 디렉터리 정리
sudo rm -rf data/cb-tumblebug/ data/cb-spider/ data/openbao/

# 3. 새 라인업으로 기동
git pull               # 0.12.9 docker-compose.yaml 반영된 commit
./mayfly infra run

# 4. 최초 1회 초기화
./mayfly setup tumblebug-init
```

---

## 5. 재기동 시 (사용자 개입 없음)

```bash
./mayfly infra stop
./mayfly infra run
# → openbao-unseal sidecar가 data/openbao/secrets/openbao-init.json에서 키 읽어 자동 unseal
# → mc-terrarium → cb-tumblebug 순차 healthy
```

→ 시스템 재부팅·서비스 재시작 시에도 별도 명령 불필요.

---

## 6. 변경 사항 정상 동작 확인

### 6.1 컨테이너 healthy 확인

```bash
./mayfly infra info
```

다음 컨테이너가 모두 healthy 상태여야 합니다:
- cb-spider (★ 핵심)
- cb-tumblebug (★ 핵심)
- cb-tumblebug-etcd, cb-tumblebug-postgres
- cb-mapui
- mc-terrarium
- openbao
- openbao-unseal (healthy — 상시 폴링 watcher. `restart: unless-stopped`로 계속 떠 있으며 OpenBao가 unsealed면 healthy)

### 6.2 cb-spider 인증 변경 검증

```bash
# 정상 자격증명으로 호출
curl -u default:default http://localhost:1024/spider/readyz
# → 200 OK

# 잘못된 자격증명
curl -u wrong:wrong http://localhost:1024/spider/readyz
# → 401 Unauthorized
```

### 6.3 cb-tumblebug 정상 응답 확인

```bash
curl http://localhost:1323/tumblebug/readyz
# → {"message":"CB-Tumblebug is ready"}
```

### 6.4 mc-terrarium · openbao 확인

```bash
curl http://localhost:8055/terrarium/readyz
# → 200 OK

# openbao 상태 (sealed:false, initialized:true 이어야 함)
curl http://localhost:8200/v1/sys/seal-status
```

### 6.5 cm-butterfly 콘솔 로그인

브라우저에서 cm-butterfly 콘솔에 접속하여 로그인 화면이 정상 표시되는지 확인.

---

## 7. OpenBao 운영 — 보안 등급별 unseal 방식

본 태스크는 운영 편의를 위해 **호스트 평문 키 자동 unseal**(openbao-unseal sidecar)을 기본 채택했습니다. 이는 OpenBao의 Shamir 분산 키 보호 의도를 무력화하므로, 운영 보안 등급에 따라 다른 방식을 검토하세요.

| 등급 | 방식 | 동작 요약 | 트레이드오프 |
|------|------|---------|---------|
| **dev / 워크숍 / PoC** (기본) | sidecar 자동 unseal | 호스트 파일에서 키 읽어 자동 unseal | 편의 ↑ / 호스트 침해 시 시크릿 전체 노출 |
| **internal staging** | 수동 unseal | sidecar 주석 처리 후 운영자가 매번 unseal 입력 | 안전 / 가용성 ↓ |
| **production** | KMS Auto-Unseal | OpenBao config에 `seal "awskms"` stanza 추가, KMS 키로 봉인 | 가장 안전 / KMS 비용·CSP 종속 |
| **regulated** | HSM | 하드웨어 보안 모듈로 봉인 | 최강 보안 / 비용·복잡도 최대 |

### 7.1 수동 unseal로 전환

`conf/docker/docker-compose.yaml`의 `openbao-unseal` 서비스 블록 전체를 주석 처리한 뒤, `mc-terrarium`의 `depends_on`을 `openbao` `service_healthy`로 변경합니다. 재기동 시 다음을 실행:

```bash
docker exec -it openbao bao operator unseal <unseal_key>
```

`unseal_key`는 `data/openbao/secrets/openbao-init.json`의 `unseal_keys_b64[0]` 값.

### 7.2 KMS Auto-Unseal로 전환 (AWS KMS 예시)

`conf/openbao/openbao-config.hcl`에 다음 stanza 추가:

```hcl
seal "awskms" {
  region     = "ap-northeast-2"
  kms_key_id = "alias/openbao-unseal"
}
```

이후 `openbao-unseal` sidecar는 불필요(주석 처리). KMS 키 IAM 권한이 openbao 컨테이너 호스트(EC2 instance profile 등)에 부여되어야 합니다.

자세한 절차: [OpenBao Auto-Unseal 공식 문서](https://openbao.org/docs/configuration/seal/)

---

## 8. 트러블슈팅

| 증상 | 원인 | 해결 |
|------|------|------|
| `cb-spider` 컨테이너가 `[AUTH ERROR] SPIDER_USERNAME and SPIDER_PASSWORD must both be set` 로그로 종료 | 환경변수 미설정 | `conf/docker/.env`에 `SPIDER_USERNAME`/`SPIDER_PASSWORD` 값 추가 또는 docker-compose.yaml 기본값 확인 |
| `mc-terrarium` unhealthy 지속 | openbao sealed 상태 또는 미초기화 | `mayfly setup tumblebug-init` 1회 실행 |
| `openbao-unseal` sidecar가 `INIT_FILE not found` 에러 | secrets/openbao-init.json 손상 또는 삭제 | 백업본 복원 또는 clean 재기동 |
| `mayfly setup tumblebug-init` 비밀번호 입력 후 Step 1 실패 | credentials.yaml.enc 비밀번호 오류 | 올바른 비밀번호로 재시도 |
| postgres 연결 수 부족 에러 | `max_connections=500` 미적용 | docker-compose.yaml의 `cb-tumblebug-postgres` 서비스에 `command: postgres -c max_connections=500` 확인 |

---

## 9. 롤백 (0.12.9 → 0.12.1)

자동 다운그레이드는 미제공. 다음 절차로 수동 롤백:

```bash
./mayfly infra remove
sudo rm -rf data/cb-tumblebug/ data/cb-spider/ data/openbao/
git checkout <previous-commit>   # 0.12.1 라인업 commit
./mayfly infra run
```

---

## 10. 관련 문서

- [cb-tumblebug v0.12.9 릴리스 노트](https://github.com/cloud-barista/cb-tumblebug/releases/tag/v0.12.9)
- [cb-spider v0.12.17 릴리스 노트](https://github.com/cloud-barista/cb-spider/releases/tag/v0.12.17)
- [OpenBao 공식 문서](https://openbao.org/docs/)
- cm-mayfly CHANGELOG.md — v0.6.0 항목 참조
