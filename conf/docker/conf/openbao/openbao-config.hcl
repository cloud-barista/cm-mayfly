# OpenBao server configuration
#
# 본 설정은 cb-tumblebug v0.12.9 라인업의 mc-terrarium 시크릿 백엔드로
# OpenBao를 사용하기 위한 cm-mayfly 측 설정입니다.
#
# Reference:
#   - https://openbao.org/docs/configuration/
#   - https://github.com/cloud-barista/cb-tumblebug/blob/v0.12.9/init/openbao/openbao-config.hcl

# Persistent storage backend — 데이터가 컨테이너 재기동에도 유지됨
storage "file" {
  path = "/openbao/data"
}

# TCP listener (TLS disabled for local development)
listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

# API address for self-reference
api_addr = "http://0.0.0.0:8200"

# Disable mlock for container compatibility
# (IPC_LOCK capability handles memory protection instead)
disable_mlock = true

# Enable Web UI
ui = true

# ─────────────────────────────────────────────────────────────────────
# (선택) 클라우드 KMS Auto-Unseal 활성화 시 아래 stanza 활용
# 운영 환경에서는 호스트 평문 키 자동 unseal(openbao-unseal sidecar)
# 대신 KMS 봉인 방식을 권장합니다. 자세한 절차는
# See the security guide section in docs/openbao-unseal.md.
# ─────────────────────────────────────────────────────────────────────
#
# AWS KMS 예시:
# seal "awskms" {
#   region     = "ap-northeast-2"
#   kms_key_id = "alias/openbao-unseal"
# }
#
# GCP Cloud KMS 예시:
# seal "gcpckms" {
#   project     = "your-gcp-project"
#   region      = "asia-northeast3"
#   key_ring    = "openbao-keyring"
#   crypto_key  = "openbao-unseal-key"
# }
#
# Azure Key Vault 예시:
# seal "azurekeyvault" {
#   tenant_id      = "your-tenant-id"
#   client_id      = "your-client-id"
#   client_secret  = "your-client-secret"
#   vault_name     = "your-vault-name"
#   key_name       = "openbao-unseal-key"
# }
