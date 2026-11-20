package db

import (
	"context"
	"path/filepath"
	"testing"
)

func abrirTemporario(t *testing.T) *Banco {
	t.Helper()
	b, err := Abrir(context.Background(), filepath.Join(t.TempDir(), "aurea.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { b.Fechar() })
	return b
}

func contar(t *testing.T, b *Banco, consulta string, args ...any) int {
	t.Helper()
	var n int
	if err := b.R.QueryRow(consulta, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", consulta, err)
	}
	return n
}

func TestMigrationsCriamAs25Tabelas(t *testing.T) {
	b := abrirTemporario(t)
	n := contar(t, b, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name <> 'goose_db_version'`)
	if n != 25 { // as 25 tabelas de negócio do SQL Server (Script_Aplicado era só controle do db-init; aqui é a goose_db_version)
		t.Errorf("tabelas de negócio = %d, esperado 25", n)
	}
	for _, tabela := range []string{"Pessoas", "Usuarios", "Direito_Usuario", "Token_App", "Produto", "Estoque", "Produto_Unidade", "Unidade_Medida",
		"Documento", "Itens_Documento", "Tipo_Titulo", "Conta_Financeira", "Categoria_Financeira", "Forma_Pagamento", "Titulo", "Baixa",
		"Movimento_Conta", "Transferencia_Conta", "Meta_Venda", "Segmento", "Pessoa_Segmento", "Agendamento", "Agendamento_Servico",
		"Agenda_Horario", "Agenda_Bloqueio"} {
		if contar(t, b, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, tabela) != 1 {
			t.Errorf("falta a tabela %s", tabela)
		}
	}
}

func TestMigrationsSaoIdempotentes(t *testing.T) {
	b := abrirTemporario(t)
	if err := Migrar(b.W); err != nil {
		t.Fatalf("segunda execução das migrations: %v", err)
	}
	if n := contar(t, b, `SELECT count(*) FROM goose_db_version WHERE version_id = 1`); n != 1 {
		t.Errorf("versão 1 registrada %d vezes", n)
	}
}

func TestDadosIniciaisIguaisAoAntigo(t *testing.T) {
	b := abrirTemporario(t)
	casos := map[string]int{"Segmento": 2, "Unidade_Medida": 5, "Tipo_Titulo": 9, "Conta_Financeira": 3, "Categoria_Financeira": 12, "Forma_Pagamento": 9}
	for tabela, esperado := range casos {
		if n := contar(t, b, "SELECT count(*) FROM "+tabela); n != esperado {
			t.Errorf("%s: %d linhas, esperado %d", tabela, n, esperado)
		}
	}
	// taxa do cartão de crédito = 3,99 % (399 centésimos), até 12 parcelas, vence em 30 dias
	var taxa, max, dias int
	if err := b.R.QueryRow(`SELECT fpg_taxa_percentual, fpg_max_parcelas, fpg_dias_vencimento FROM Forma_Pagamento WHERE fpg_nome='Cartão de crédito'`).Scan(&taxa, &max, &dias); err != nil {
		t.Fatal(err)
	}
	if taxa != 399 || max != 12 || dias != 30 {
		t.Errorf("cartão de crédito: taxa=%d max=%d dias=%d", taxa, max, dias)
	}
	// o salão tem acento e a busca sem acento o encontra
	if n := contar(t, b, `SELECT count(*) FROM Segmento WHERE seg_nome = 'SALAO'`); n != 1 {
		t.Errorf("segmento 'SALAO' (sem acento, maiúsculo) não casou com 'Salão': %d", n)
	}
}

func TestPragmasEUmaConexaoDeEscrita(t *testing.T) {
	b := abrirTemporario(t)
	var modo string
	if err := b.W.QueryRow(`PRAGMA journal_mode`).Scan(&modo); err != nil || modo != "wal" {
		t.Errorf("journal_mode = %q (%v), esperado wal", modo, err)
	}
	var fk int
	if err := b.W.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d (%v)", fk, err)
	}
	var espera int
	if err := b.W.QueryRow(`PRAGMA busy_timeout`).Scan(&espera); err != nil || espera != 5000 {
		t.Errorf("busy_timeout = %d (%v)", espera, err)
	}
	if st := b.W.Stats(); st.MaxOpenConnections != 1 {
		t.Errorf("conexões de escrita = %d, esperado 1", st.MaxOpenConnections)
	}
	if _, err := b.R.Exec(`INSERT INTO Segmento (seg_nome) VALUES ('x')`); err == nil {
		t.Error("o pool de leitura aceitou uma escrita")
	}
}

func TestChaveEstrangeiraBarraOrfaos(t *testing.T) {
	b := abrirTemporario(t)
	if _, err := b.W.Exec(`INSERT INTO Estoque (est_upr_codigo, est_qtd, est_valor_custo) VALUES (999, 0, 0)`); err == nil {
		t.Error("estoque de produto inexistente foi aceito (foreign_keys desligado?)")
	}
}

func TestLoginUnicoSemDiferenciarMaiusculaNemAcento(t *testing.T) {
	b := abrirTemporario(t)
	if _, err := b.W.Exec(`INSERT INTO Usuarios (usu_nome, usu_login, usu_senha) VALUES ('A','Joao','x')`); err != nil {
		t.Fatal(err)
	}
	for _, repetido := range []string{"JOAO", "joao", "João", "joão  "} {
		if _, err := b.W.Exec(`INSERT INTO Usuarios (usu_nome, usu_login, usu_senha) VALUES ('B', ?, 'x')`, repetido); err == nil {
			t.Errorf("login %q foi aceito, mas é igual a 'Joao' pela collation do SQL Server (CI_AI)", repetido)
		}
	}
	if n := contar(t, b, `SELECT count(*) FROM Usuarios WHERE usu_login = 'JOÃO'`); n != 1 {
		t.Errorf("busca por login sem acento/maiúscula = %d, esperado 1", n)
	}
}

func TestBuscaSemAcentoPelaFuncaoDeNormalizacao(t *testing.T) {
	b := abrirTemporario(t)
	for _, nome := range []string{"Calça Jeans Flare", "Conjunto tricô sal e pimenta", "Camisa branca"} {
		if _, err := b.W.Exec(`INSERT INTO Produto (upr_nome, upr_und, upr_preco) VALUES (?, 'UN', 1000)`, nome); err != nil {
			t.Fatal(err)
		}
	}
	busca := func(termo string) int {
		return contar(t, b, `SELECT count(*) FROM Produto WHERE aurea_norm(upr_nome) LIKE '%' || ? || '%'`, Normalizar(termo))
	}
	if busca("calca") != 1 || busca("CALÇA") != 1 || busca("trico") != 1 || busca("ca") != 2 {
		t.Errorf("busca sem acento falhou: calca=%d CALÇA=%d trico=%d ca=%d", busca("calca"), busca("CALÇA"), busca("trico"), busca("ca"))
	}
}

func TestRestricoesDoEsquema(t *testing.T) {
	b := abrirTemporario(t)
	casos := []struct{ nome, sql string }{
		{"documento com tipo inválido", `INSERT INTO Usuarios (usu_nome,usu_login,usu_senha) VALUES ('u','u1','x'); INSERT INTO Pessoas (ups_nome) VALUES ('p'); INSERT INTO Documento (docto_tipo,docto_data,docto_ups_codigo,docto_usu_codigo) VALUES ('XYZ','2026-01-01',1,1)`},
		{"produto com comissão acima de 100%", `INSERT INTO Produto (upr_nome,upr_und,upr_preco,upr_comissao_perc) VALUES ('s','UN',1000,10001)`},
		{"produto com duração acima de 24 h", `INSERT INTO Produto (upr_nome,upr_und,upr_preco,upr_duracao_min) VALUES ('s','UN',1000,1441)`},
		{"fator de unidade zero", `INSERT INTO Produto (upr_nome,upr_und,upr_preco) VALUES ('p','UN',1000); INSERT INTO Produto_Unidade (pun_upr_codigo,pun_sigla,pun_fator) VALUES (1,'CX',0)`},
		{"transferência para a mesma conta", `INSERT INTO Transferencia_Conta (trf_data,trf_cfn_origem,trf_cfn_destino,trf_valor) VALUES ('2026-01-01',1,1,100)`},
		{"transferência de valor zero", `INSERT INTO Transferencia_Conta (trf_data,trf_cfn_origem,trf_cfn_destino,trf_valor) VALUES ('2026-01-01',1,2,0)`},
		{"meta fora do dia 1", `INSERT INTO Meta_Venda (meta_mes,meta_valor) VALUES ('2026-11-15',100)`},
		{"meta de valor zero", `INSERT INTO Meta_Venda (meta_mes,meta_valor) VALUES ('2026-11-01',0)`},
		{"token de hash curto", `INSERT INTO Usuarios (usu_nome,usu_login,usu_senha) VALUES ('u','u2','x'); INSERT INTO Token_App (tok_usu_codigo,tok_hash,tok_expira_em) VALUES (1,'abc','2026-12-01 00:00:00')`},
		{"horário com fim antes do início", `INSERT INTO Pessoas (ups_nome) VALUES ('prof'); INSERT INTO Agenda_Horario (hor_prof_codigo,hor_dia_semana,hor_inicio,hor_fim) VALUES (1,1,'18:00','09:00')`},
		{"dia da semana 8", `INSERT INTO Pessoas (ups_nome) VALUES ('prof2'); INSERT INTO Agenda_Horario (hor_prof_codigo,hor_dia_semana,hor_inicio,hor_fim) VALUES (1,8,'09:00','18:00')`},
		{"agendamento sem cliente nem nome", `INSERT INTO Pessoas (ups_nome) VALUES ('prof3'); INSERT INTO Agendamento (agd_prof_codigo,agd_inicio,agd_fim) VALUES (1,'2026-11-20 10:00:00','2026-11-20 11:00:00')`},
		{"agendamento com fim antes do início", `INSERT INTO Pessoas (ups_nome) VALUES ('prof4'); INSERT INTO Agendamento (agd_cliente_nome,agd_prof_codigo,agd_inicio,agd_fim) VALUES ('x',1,'2026-11-20 11:00:00','2026-11-20 10:00:00')`},
	}
	for _, c := range casos {
		tx, err := b.W.Begin()
		if err != nil {
			t.Fatal(err)
		}
		var falhou error
		for _, parte := range dividir(c.sql) {
			if _, falhou = tx.Exec(parte); falhou != nil {
				break
			}
		}
		tx.Rollback()
		if falhou == nil {
			t.Errorf("%s: o banco aceitou, mas o SQL Server recusava", c.nome)
		}
	}
}

func TestMetaDaLojaUnicaPorMes(t *testing.T) {
	b := abrirTemporario(t)
	if _, err := b.W.Exec(`INSERT INTO Meta_Venda (meta_mes, meta_valor) VALUES ('2026-11-01', 1000000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.W.Exec(`INSERT INTO Meta_Venda (meta_mes, meta_valor) VALUES ('2026-11-01', 2000000)`); err == nil {
		t.Error("duas metas 'loja toda' no mesmo mês (D-07): o SQL Server recusava")
	}
	if _, err := b.W.Exec(`INSERT INTO Meta_Venda (meta_mes, meta_valor) VALUES ('2026-12-01', 2000000)`); err != nil {
		t.Errorf("meta da loja em outro mês deveria ser aceita: %v", err)
	}
}

func TestNormalizar(t *testing.T) {
	casos := map[string]string{"Calça": "calca", "AÇÃO ": "acao", "Pérola": "perola", "  x": "  x", "Salão": "salao", "": ""}
	for entrada, esperado := range casos {
		if got := Normalizar(entrada); got != esperado {
			t.Errorf("Normalizar(%q) = %q, esperado %q", entrada, got, esperado)
		}
	}
}

// dividir separa comandos por ';' (só para os casos acima, que não têm ';' dentro de texto).
func dividir(sql string) []string {
	var out []string
	atual := ""
	for _, r := range sql {
		if r == ';' {
			out = append(out, atual)
			atual = ""
			continue
		}
		atual += string(r)
	}
	if atual != "" {
		out = append(out, atual)
	}
	return out
}
