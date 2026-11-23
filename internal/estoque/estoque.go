// Package estoque movimenta o saldo e o custo médio dos produtos (EstoqueService do sistema antigo). Tudo na UNIDADE BASE do produto
// (RN-EST-01) e dentro da transação de quem chama. Quantidade em milésimos, custo médio em décimos de milésimo.
package estoque

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"

	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/dominio"
	"github.com/agnaldoojr/aureaconceptnew/internal/msg"
)

// Produto é o que o estoque precisa saber do produto.
type Produto struct {
	ID      int64
	Nome    string
	Servico bool
}

// Linha é a linha de Estoque de um produto.
type Linha struct {
	Qtd   int64 // milésimos
	Custo int64 // décimos de milésimo
}

// Buscar devolve a linha de estoque; (nil, nil) se o produto não tem.
func Buscar(ctx context.Context, x db.Executor, produto int64) (*Linha, error) {
	var l Linha
	err := x.QueryRowContext(ctx, `SELECT est_qtd, est_valor_custo FROM Estoque WHERE est_upr_codigo = ?`, produto).Scan(&l.Qtd, &l.Custo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func buscarOuCriar(ctx context.Context, x db.Executor, produto int64) (*Linha, error) {
	l, err := Buscar(ctx, x, produto)
	if err != nil || l != nil {
		return l, err
	}
	if _, err := x.ExecContext(ctx, `INSERT INTO Estoque (est_upr_codigo, est_qtd, est_valor_custo) VALUES (?,0,0)`, produto); err != nil {
		return nil, err
	}
	return &Linha{}, nil
}

func milesimos(d decimal.Decimal) int64 { return d.Shift(3).Round(0).IntPart() }

// Entrada soma ao saldo e recalcula o custo médio ponderado (RN-EST-02). Serviço não tem estoque.
func Entrada(ctx context.Context, x db.Executor, p Produto, qtdBase, valorBase decimal.Decimal) error {
	if p.Servico {
		return nil
	}
	l, err := buscarOuCriar(ctx, x, p.ID)
	if err != nil {
		return err
	}
	saldo := decimal.New(l.Qtd, -3)
	custo := decimal.New(l.Custo, -4)
	novaQtd := saldo.Add(qtdBase)
	novoCusto := dominio.CustoMedio(saldo, custo, qtdBase, valorBase)
	_, err = x.ExecContext(ctx, `UPDATE Estoque SET est_qtd = ?, est_valor_custo = ? WHERE est_upr_codigo = ?`, milesimos(novaQtd), novoCusto.Shift(4).Round(0).IntPart(), p.ID)
	return err
}

// Saida tira do saldo. Saldo insuficiente é erro e nada muda; o custo médio não se altera (RN-EST-03).
func Saida(ctx context.Context, x db.Executor, p Produto, qtdBase decimal.Decimal) error {
	if p.Servico {
		return nil
	}
	l, err := buscarOuCriar(ctx, x, p.ID)
	if err != nil {
		return err
	}
	pedido := milesimos(qtdBase)
	if l.Qtd < pedido {
		return msg.Novo("Estoque insuficiente para o produto '%s'. Disponível: %s | Solicitado: %s", p.Nome, dominio.Escala(l.Qtd, 3), dominio.Escala(pedido, 3))
	}
	_, err = x.ExecContext(ctx, `UPDATE Estoque SET est_qtd = ? WHERE est_upr_codigo = ?`, l.Qtd-pedido, p.ID)
	return err
}

// EntradaSemAlterarCusto devolve ao saldo sem mexer no custo médio (usado ao reverter uma saída).
func EntradaSemAlterarCusto(ctx context.Context, x db.Executor, p Produto, qtdBase decimal.Decimal) error {
	if p.Servico {
		return nil
	}
	l, err := buscarOuCriar(ctx, x, p.ID)
	if err != nil {
		return err
	}
	_, err = x.ExecContext(ctx, `UPDATE Estoque SET est_qtd = ? WHERE est_upr_codigo = ?`, l.Qtd+milesimos(qtdBase), p.ID)
	return err
}
