#!/usr/bin/env bash
# Roda um teste GOLDEN (ex.: comparar-cadastros.mjs): recria o sistema ANTIGO (banco descartável AureaRef, porta 8091) e o NOVO
# (banco temporário + dados de exemplo iguais aos do antigo, porta 8092) do zero e executa o roteiro nos dois.
#   bash rodar-golden.sh comparar-cadastros.mjs
# Só desenvolvimento. Não toca em nenhum banco real: o antigo usa AureaRef e o novo um arquivo em pasta temporária.
set -u
ROTEIRO="${1:?informe o roteiro, ex.: comparar-cadastros.mjs}"
AQUI="$(cd "$(dirname "$0")" && pwd)"
RAIZ="$(cd "$AQUI/.." && pwd)"
export PATH="/c/Program Files/Go/bin:$PATH"
TMPD="$(mktemp -d)"
SENHA_NOVO="${SENHA_NOVO:-Teste-Local-2026}"
PORTA_NOVO="${PORTA_NOVO:-8092}"

echo "== antigo: recriando o banco AureaRef e subindo na 8091"
(bash "$AQUI/ref-antigo.sh" reset > "$TMPD/ref.log" 2>&1) || { cat "$TMPD/ref.log"; exit 1; }
cat "$TMPD/ref.log"

echo "== novo: banco novo com os dados de exemplo e servidor na $PORTA_NOVO"
powershell -NoProfile -Command "Get-Process aurea -ErrorAction SilentlyContinue | Stop-Process -Force" >/dev/null 2>&1
sleep 1
(cd "$RAIZ" && ADMIN_SENHA_INICIAL="$SENHA_NOVO" go run ./tools/demo -db "$TMPD/aurea.db" && go build -o "$TMPD/aurea.exe" ./cmd/aurea) || exit 1
(cd "$TMPD" && PORT="$PORTA_NOVO" DB_PATH="$TMPD/aurea.db" nohup ./aurea.exe > "$TMPD/aurea.log" 2>&1 &)
for i in $(seq 1 20); do [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$PORTA_NOVO/healthz")" = "200" ] && break; sleep 1; done

echo "== roteiro $ROTEIRO"
(cd "$AQUI" && SENHA_NOVO="$SENHA_NOVO" BASE_NOVO="http://localhost:$PORTA_NOVO" node "$ROTEIRO")
CODIGO=$?
powershell -NoProfile -Command "Get-Process aurea -ErrorAction SilentlyContinue | Stop-Process -Force" >/dev/null 2>&1
exit $CODIGO
