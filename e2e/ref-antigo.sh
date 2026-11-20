#!/usr/bin/env bash
# Sobe o sistema ANTIGO (Java + SQL Server local) como oráculo de referência, em um banco descartável.
# Nada do projeto antigo é alterado: usa o clone somente leitura em ../../aureaconcept-ref e o banco AureaRef.
#   bash ref-antigo.sh reset   -> (re)cria o banco AureaRef do zero e reinicia o sistema antigo na porta 8091
#   bash ref-antigo.sh parar   -> para o sistema antigo
# Requisitos: JDK 17+, SQL Server local (sa/123, instância SQLEXPRESS), jar já compilado no clone (mvn -o package -DskipTests).
set -u
REF="${REF_DIR:-/c/Users/agnal/aureaconcept-ref}"
JAVA="${JAVA_HOME:-/c/Program Files/Java/jdk-21.0.10}/bin/java"
PORTA="${PORTA_REF:-8091}"
NOME_BANCO="${NOME_BANCO_REF:-AureaRef}"
TMP="${TMPDIR:-/tmp}/ref-antigo-db"
SQL=(sqlcmd -S 'localhost\SQLEXPRESS' -U sa -P 123 -C -f 65001 -b)

parar() {
  # só encerra o Java que roda o jar do sistema antigo (não mexe em outros processos Java)
  powershell -NoProfile -Command "Get-CimInstance Win32_Process -Filter \"Name='java.exe'\" | Where-Object { \$_.CommandLine -like '*aurea-concept-1.0.0.jar*' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force }" >/dev/null 2>&1
  sleep 2
}

case "${1:-}" in
  parar) parar ;;
  reset)
    parar
    mkdir -p "$TMP"
    for f in schema.sql 02_financeiro.sql 04_app_vendas.sql 05_unidades_medida.sql 06_unidades_do_produto.sql 07_pessoa_segmento.sql \
             08_transferencia_contas.sql 09_vendas_todos.sql 10_segmento_vendas_financeiro.sql 11_salao_servicos_comissao.sql \
             12_metas_venda.sql 13_agenda_salao.sql; do
      sed "s/AureaConcept/$NOME_BANCO/g" "$REF/database/$f" > "$TMP/$f"
    done
    "${SQL[@]}" -Q "IF DB_ID('$NOME_BANCO') IS NOT NULL BEGIN ALTER DATABASE $NOME_BANCO SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE $NOME_BANCO END" >/dev/null
    (cd "$TMP" && for f in schema.sql 02_financeiro.sql 04_app_vendas.sql 05_unidades_medida.sql 06_unidades_do_produto.sql 07_pessoa_segmento.sql \
             08_transferencia_contas.sql 09_vendas_todos.sql 10_segmento_vendas_financeiro.sql 11_salao_servicos_comissao.sql \
             12_metas_venda.sql 13_agenda_salao.sql; do "${SQL[@]}" -i "$f" >/dev/null || { echo "FALHOU $f"; exit 1; }; done)
    (cd "$REF" && PORT="$PORTA" DB_HOST='localhost\SQLEXPRESS' DB_NAME="$NOME_BANCO" DB_USER=sa DB_PASSWORD=123 \
       nohup "$JAVA" -Xmx300m -jar target/aurea-concept-1.0.0.jar > "$TMP/app.log" 2>&1 &)
    for i in $(seq 1 40); do
      c=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$PORTA/login"); [ "$c" = "200" ] && { echo "antigo no ar em http://localhost:$PORTA (banco $NOME_BANCO)"; exit 0; }
      sleep 3
    done
    echo "não subiu; veja $TMP/app.log"; exit 1 ;;
  *) echo "uso: bash ref-antigo.sh reset|parar"; exit 2 ;;
esac
