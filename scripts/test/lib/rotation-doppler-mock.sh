# shellcheck shell=bash
# In-memory Doppler mock for encryption rotation tests.

rotation_doppler_mock_install() {
  local fake_bin=$1
  local db=$2
  mkdir -p "${fake_bin}"
  cat > "${fake_bin}/doppler" <<MOCK
#!/usr/bin/bash
set -euo pipefail
db="\${ROTATION_DOPPLER_DB:?}"
cmd=\${1:-}
shift || true
case "\${cmd}" in
  secrets)
    sub=\${1:-}
    shift || true
    if [[ "\${sub}" == get ]]; then
      name=\${1:-}
      shift || true
      project=""
      config=""
      while [[ \$# -gt 0 ]]; do
        case "\$1" in
          -p) project=\$2; shift 2 ;;
          -c) config=\$2; shift 2 ;;
          --plain) shift ;;
          *) shift ;;
        esac
      done
      if [[ "\${MOCK_DOPPLER_NETWORK_ERROR:-}" == 1 ]]; then
        echo "Doppler Error: network timeout" >&2
        exit 1
      fi
      line=\$(awk -F '\t' -v n="\${name}" -v p="\${project}" -v c="\${config}" \
        '\$2 == n && \$3 == p && \$4 == c { print \$5 }' "\${db}" | tail -1)
      if [[ -z "\${line}" ]]; then
        echo "Doppler Error: Could not find requested secret" >&2
        exit 1
      fi
      printf '%s' "\${line}"
      exit 0
    fi
    if [[ "\${sub}" == set ]]; then
      name=\${1:-}
      shift || true
      project=""
      config=""
      while [[ \$# -gt 0 ]]; do
        case "\$1" in
          -p) project=\$2; shift 2 ;;
          -c) config=\$2; shift 2 ;;
          --silent|--no-interactive) shift ;;
          *) shift ;;
        esac
      done
      value="\$(cat)"
      printf '%s\t%s\t%s\t%s\t%s\n' "\$(date +%s)" "\${name}" "\${project}" "\${config}" "\${value}" >>"\${db}"
      exit 0
    fi
    if [[ "\${sub}" == delete ]]; then
      name=\${1:-}
      shift || true
      project=""
      config=""
      while [[ \$# -gt 0 ]]; do
        case "\$1" in
          -p) project=\$2; shift 2 ;;
          -c) config=\$2; shift 2 ;;
          *) shift ;;
        esac
      done
      awk -F '\t' -v n="\${name}" -v p="\${project}" -v c="\${config}" \
        '!( \$2 == n && \$3 == p && \$4 == c )' "\${db}" >"\${db}.tmp" && mv "\${db}.tmp" "\${db}"
      exit 0
    fi
    ;;
esac
echo "unexpected doppler: \$*" >&2
exit 1
MOCK
  chmod +x "${fake_bin}/doppler"
}

rotation_doppler_seed() {
  local db=$1 name=$2 project=$3 config=$4 value=$5
  printf '%s\t%s\t%s\t%s\t%s\n' 0 "${name}" "${project}" "${config}" "${value}" >>"${db}"
}
