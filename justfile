set shell := ["bash", "-cu"]
set dotenv-load := true

default:
    @just --list

run:
    go run ./cmd/api

test:
    go test ./...

test-hurl base_url="http://localhost:8080" auth_email="" auth_password="" clinic_tax_id="" dentist_tax_id="":
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v hurl >/dev/null 2>&1; then
      echo "hurl not found. Install from https://hurl.dev/docs/installation.html"
      exit 1
    fi
    cnpj_digit() {
      local n="$1"; shift
      local weights=("$@")
      local sum=0
      local i d
      for ((i=0; i<${#weights[@]}; i++)); do
        d=${n:i:1}
        sum=$((sum + d * ${weights[$i]}))
      done
      local mod=$((sum % 11))
      if ((mod < 2)); then
        echo 0
      else
        echo $((11 - mod))
      fi
    }
    gen_cnpj() {
      local seed="$1"
      local base
      base=$(printf '%012d' "$seed")
      local d1 d2
      d1=$(cnpj_digit "$base" 5 4 3 2 9 8 7 6 5 4 3 2)
      d2=$(cnpj_digit "${base}${d1}" 6 5 4 3 2 9 8 7 6 5 4 3 2)
      echo "${base}${d1}${d2}"
    }
    cpf_digit() {
      local n="$1"
      local start_weight="$2"
      local sum=0
      local weight="$start_weight"
      local i d
      for ((i=0; i<${#n}; i++)); do
        d=${n:i:1}
        sum=$((sum + d * weight))
        weight=$((weight - 1))
      done
      local mod=$(((sum * 10) % 11))
      if ((mod == 10)); then
        mod=0
      fi
      echo "$mod"
    }
    gen_cpf() {
      local seed="$1"
      local base
      base=$(printf '%09d' "$seed")
      local d1 d2
      d1=$(cpf_digit "$base" 10)
      d2=$(cpf_digit "${base}${d1}" 11)
      echo "${base}${d1}${d2}"
    }
    chosen_clinic_tax_id="{{ clinic_tax_id }}"
    chosen_dentist_tax_id="{{ dentist_tax_id }}"
    chosen_auth_email="{{ auth_email }}"
    chosen_auth_password="{{ auth_password }}"
    if [ -z "$chosen_auth_email" ]; then
      chosen_auth_email="${AUTH_BOOTSTRAP_EMAIL:-}"
    fi
    if [ -z "$chosen_auth_password" ]; then
      chosen_auth_password="${AUTH_BOOTSTRAP_PASSWORD:-}"
    fi
    if [ -z "$chosen_auth_email" ] || [ -z "$chosen_auth_password" ]; then
      echo "AUTH_BOOTSTRAP_EMAIL/AUTH_BOOTSTRAP_PASSWORD not set. Configure .env or pass explicit args."
      exit 1
    fi
    if [ -z "$chosen_clinic_tax_id" ] || [ -z "$chosen_dentist_tax_id" ]; then
      seed="$(date +%s)"
      if [ -z "$chosen_clinic_tax_id" ]; then
        chosen_clinic_tax_id="$(gen_cnpj $((100000000000 + seed)))"
      fi
      if [ -z "$chosen_dentist_tax_id" ]; then
        chosen_dentist_tax_id="$(gen_cpf $(((seed % 800000000) + 100000000)))"
      fi
    fi
    hurl --test --location \
      --variable base_url='{{ base_url }}' \
      --variable auth_email="$chosen_auth_email" \
      --variable auth_password="$chosen_auth_password" \
      --variable clinic_tax_id="$chosen_clinic_tax_id" \
      --variable dentist_tax_id="$chosen_dentist_tax_id" \
      tests/hurl

test-hurl-docker:
    just test-hurl http://localhost:8081

lint:
    gofmt -w $(find . -name '*.go')
    go vet ./...
    if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; else echo "staticcheck not found, skipping"; fi

migrate-up:
    #!/usr/bin/env bash
    set -euo pipefail
    database_url="${DATABASE_URL:-}"
    if [ -z "$database_url" ]; then
      echo "DATABASE_URL not set. Configure .env before running migrations."
      exit 1
    fi
    if ! command -v psql >/dev/null 2>&1; then
      echo "psql not found. Install PostgreSQL client tools."
      exit 1
    fi
    psql "$database_url" -v ON_ERROR_STOP=1 -f db/schema.sql

repository-generate:
    sqlc generate -f sqlc.yaml

dev: migrate-up run
