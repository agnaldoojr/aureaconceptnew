// Comando demo: carrega no banco SQLite os mesmos DADOS DE EXEMPLO que o `schema.sql` do sistema antigo criava
// (8 produtos, 8 pessoas, estoque e uma compra), para os testes comparados (golden) começarem do mesmo ponto nos dois sistemas.
// SÓ PARA DESENVOLVIMENTO: nunca rode em produção (o sistema novo nasce sem nenhum dado de exemplo).
//
//	ADMIN_SENHA_INICIAL=... go run ./tools/demo -db caminho/aurea.db
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/agnaldoojr/aureaconceptnew/internal/bootstrap"
	"github.com/agnaldoojr/aureaconceptnew/internal/db"
)

func main() {
	caminho := flag.String("db", "", "arquivo do banco SQLite (obrigatório)")
	flag.Parse()
	if *caminho == "" {
		fmt.Fprintln(os.Stderr, "informe -db")
		os.Exit(2)
	}
	if err := executar(*caminho); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func executar(caminho string) error {
	ctx := context.Background()
	b, err := db.Abrir(ctx, caminho)
	if err != nil {
		return err
	}
	defer b.Fechar()
	if _, err := bootstrap.GarantirAdmin(ctx, b.W, os.Getenv("ADMIN_SENHA_INICIAL")); err != nil {
		return err
	}
	var n int
	b.W.QueryRow(`SELECT (SELECT count(*) FROM Pessoas) + (SELECT count(*) FROM Produto) + (SELECT count(*) FROM Documento)`).Scan(&n)
	if n > 0 {
		return fmt.Errorf("o banco já tem dados (%d linhas em Pessoas/Produto/Documento): use um banco novo", n)
	}
	tx, err := b.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	exec := func(q string, a ...any) error { _, err := tx.ExecContext(ctx, q, a...); return err }

	// produtos (preço em centavos); o antigo os deixa no segmento 1 (Loja de roupa) depois da migration 10
	produtos := []struct {
		nome, compl, und string
		preco            int
	}{
		{"Colar Dourado Delicado", "Banhado a ouro 18k, corrente fina 45cm", "UN", 8990},
		{"Brinco Argola Pequena", "Banhado a ouro, diâmetro 2cm", "UN", 4990},
		{"Pulseira Riviera Cristal", "Pedras de cristal transparente", "UN", 12990},
		{"Anel Solitário Zircônia", "Aro dourado com zircônia central", "UN", 7990},
		{"Tornozeleira Estrela Dourada", "Banhado a ouro, com pingente estrela", "UN", 5990},
		{"Conjunto Luxo Pérola", "Colar + brinco com pérola sintética", "CJ", 19990},
		{"Pingente Coração Cravejado", "Zircônias brancas cravejadas", "UN", 6990},
		{"Bracelete Rígido Liso", "Banhado a ouro amarelo 18k", "UN", 9990},
	}
	for _, p := range produtos {
		if err := exec(`INSERT INTO Produto (upr_nome, upr_complemento, upr_und, upr_preco, upr_ativo, upr_seg_codigo) VALUES (?,?,?,?,1,1)`, p.nome, p.compl, p.und, p.preco); err != nil {
			return err
		}
	}
	pessoas := []struct {
		nome, cpf, tel         string
		cli, forn, vend, func_ int
	}{
		{"Maria Clara Oliveira", "123.456.789-00", "(11) 98765-4321", 1, 0, 0, 0},
		{"Ana Paula Ferreira", "234.567.890-11", "(11) 97654-3210", 1, 0, 0, 0},
		{"Juliana Santos Lima", "345.678.901-22", "(21) 96543-2109", 1, 0, 0, 0},
		{"Rafaela Costa Mendes", "456.789.012-33", "(31) 95432-1098", 1, 0, 0, 0},
		{"Joias Premium Ltda", "12.345.678/0001-90", "(11) 3333-4444", 0, 1, 0, 0},
		{"Acessórios Brilho S.A.", "98.765.432/0001-10", "(11) 4444-5555", 0, 1, 0, 0},
		{"Carlos Eduardo Vendedor", "567.890.123-44", "(11) 99999-1111", 0, 0, 1, 1},
		{"Fernanda Lopes Vendedora", "678.901.234-55", "(11) 99999-2222", 0, 0, 1, 1},
	}
	for _, p := range pessoas {
		if err := exec(`INSERT INTO Pessoas (ups_nome, ups_cpf, ups_telefone, ups_cliente, ups_fornecedor, ups_vendedor, ups_funcionario, ups_ativo) VALUES (?,?,?,?,?,?,?,1)`,
			p.nome, p.cpf, p.tel, p.cli, p.forn, p.vend, p.func_); err != nil {
			return err
		}
	}
	// estoque zerado para todos; a compra de exemplo entra 10 un. do produto 1 (custo 35,00) e 10 un. do produto 2 (custo 15,00)
	if err := exec(`INSERT INTO Estoque (est_upr_codigo, est_qtd, est_valor_custo) SELECT upr_codigo, 0, 0 FROM Produto`); err != nil {
		return err
	}
	if err := exec(`UPDATE Estoque SET est_qtd = 10000, est_valor_custo = 350000 WHERE est_upr_codigo = 1`); err != nil {
		return err
	}
	if err := exec(`UPDATE Estoque SET est_qtd = 10000, est_valor_custo = 150000 WHERE est_upr_codigo = 2`); err != nil {
		return err
	}
	hoje := time.Now().Format("2006-01-02")
	if err := exec(`INSERT INTO Documento (docto_tipo, docto_data, docto_valor, docto_ups_codigo, docto_usu_codigo, docto_seg_codigo)
		VALUES ('COMPRA', ?, 50000, (SELECT ups_codigo FROM Pessoas WHERE ups_fornecedor = 1 ORDER BY ups_codigo LIMIT 1), (SELECT usu_codigo FROM Usuarios WHERE usu_login = 'admin'), 1)`, hoje); err != nil {
		return err
	}
	if err := exec(`INSERT INTO Itens_Documento (itemdocto_docto_codigo, itemdocto_upr_codigo, itemdocto_upr_und, itemdocto_qtd, itemdocto_valor, itemdocto_total) VALUES (1,1,'UN',10000,3500,35000), (1,2,'UN',10000,1500,15000)`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Println("dados de exemplo carregados")
	return nil
}
