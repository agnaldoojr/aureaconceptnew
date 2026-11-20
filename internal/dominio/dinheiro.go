// Package dominio reúne as regras de negócio puras (sem banco nem HTTP) do Áurea Concept.
// Todas as contas usam decimais exatos (nunca float) com os MESMOS arredondamentos do sistema antigo (Java/BigDecimal):
// HALF_UP em Round/DivRound e ROUND_DOWN (truncar) onde o antigo usava RoundingMode.DOWN.
package dominio

import "github.com/shopspring/decimal"

var (
	// Zero e Um evitam recriar constantes decimais.
	Zero = decimal.Zero
	Um   = decimal.NewFromInt(1)
	Cem  = decimal.NewFromInt(100)
)

// fatorOuUm replica ItemDocumento.fatorOuUm: fator nulo, zero ou negativo vale 1 (unidade base).
func fatorOuUm(fator decimal.Decimal) decimal.Decimal {
	if fator.Sign() <= 0 {
		return Um
	}
	return fator
}

// QtdBase é a quantidade na unidade base do produto: qtd × fator, 3 casas HALF_UP (RN-EST-01).
func QtdBase(qtd, fator decimal.Decimal) decimal.Decimal {
	return qtd.Mul(fatorOuUm(fator)).Round(3)
}

// ValorBase é o valor por unidade base: valor ÷ fator, 4 casas HALF_UP (RN-EST-01).
func ValorBase(valor, fator decimal.Decimal) decimal.Decimal {
	return valor.DivRound(fatorOuUm(fator), 4)
}

// CustoMedio é o custo médio ponderado depois de uma ENTRADA (RN-EST-02):
// (saldo × custoAtual + qtdBase × valorBase) ÷ (saldo + qtdBase), 4 casas HALF_UP; novo saldo ≤ 0 → 0.
func CustoMedio(saldo, custoAtual, qtdBase, valorBase decimal.Decimal) decimal.Decimal {
	novaQtd := saldo.Add(qtdBase)
	if novaQtd.Sign() <= 0 {
		return Zero
	}
	total := saldo.Mul(custoAtual).Add(qtdBase.Mul(valorBase))
	return total.DivRound(novaQtd, 4)
}

// TotalItem é o total de um item: qtd × valor, SEM arredondar (o antigo só arredonda ao gravar; ver DUVIDAS D-05).
func TotalItem(qtd, valor decimal.Decimal) decimal.Decimal {
	return qtd.Mul(valor)
}

// TotalDocumento soma os totais dos itens sem arredondar e arredonda uma vez para 2 casas (RN-DOC-03).
func TotalDocumento(totaisItens []decimal.Decimal) decimal.Decimal {
	soma := Zero
	for _, t := range totaisItens {
		soma = soma.Add(t)
	}
	return soma.Round(2)
}

// Parcelas divide o total em n parcelas: valor = total ÷ n truncado em 2 casas (ROUND_DOWN) e a ÚLTIMA parcela leva o resto (RN-FIN-02).
func Parcelas(total decimal.Decimal, n int) []decimal.Decimal {
	if n < 1 {
		n = 1
	}
	valorParcela, _ := total.QuoRem(decimal.NewFromInt(int64(n)), 2)
	out := make([]decimal.Decimal, n)
	acumulado := Zero
	for i := 0; i < n; i++ {
		v := valorParcela
		if i == n-1 {
			v = total.Sub(acumulado)
		}
		acumulado = acumulado.Add(v)
		out[i] = v
	}
	return out
}

// TaxaCartao é a taxa descontada numa baixa: valor × % ÷ 100, 2 casas HALF_UP (RN-BAIXA-02).
func TaxaCartao(valor, percentual decimal.Decimal) decimal.Decimal {
	return valor.Mul(percentual).DivRound(Cem, 2)
}

// Comissao é a comissão de um item: total do item × % ÷ 100, 2 casas HALF_UP; sem percentual (nulo/≤ 0) é zero (RN-COM).
func Comissao(totalItem decimal.Decimal, percentual *decimal.Decimal) decimal.Decimal {
	if percentual == nil || percentual.Sign() <= 0 {
		return Zero
	}
	return totalItem.Mul(*percentual).DivRound(Cem, 2)
}

// PrecoSugeridoNaUnidade é o preço do produto × fator, 2 casas HALF_UP (RN-UND-02), usado quando a unidade extra não tem preço próprio.
func PrecoSugeridoNaUnidade(precoProduto, fator decimal.Decimal) decimal.Decimal {
	return precoProduto.Mul(fator).Round(2)
}

// ValorMovimentado é o lançamento no extrato de uma baixa: valor + juros + multa − desconto (RN-BAIXA-01).
func ValorMovimentado(valor, juros, multa, desconto decimal.Decimal) decimal.Decimal {
	return valor.Add(juros).Add(multa).Sub(desconto)
}

// ProgressoMeta é o andamento de uma meta de venda (RN-META).
type ProgressoMeta struct {
	Meta, Realizado decimal.Decimal
	Percentual      int // realizado × 100 ÷ meta, 0 casas ROUND_DOWN; pode passar de 100
	Falta           decimal.Decimal
	Atingida        bool
}

// NovoProgressoMeta calcula o progresso. A meta precisa ser > 0 (o banco garante).
func NovoProgressoMeta(meta, realizado decimal.Decimal) ProgressoMeta {
	pct, _ := realizado.Mul(Cem).QuoRem(meta, 0)
	falta := meta.Sub(realizado)
	if falta.Sign() < 0 {
		falta = Zero
	}
	return ProgressoMeta{Meta: meta, Realizado: realizado, Percentual: int(pct.IntPart()), Falta: falta, Atingida: realizado.Cmp(meta) >= 0}
}

// Barra é a largura da barra de progresso, limitada a 0–100.
func (p ProgressoMeta) Barra() int {
	if p.Percentual < 0 {
		return 0
	}
	if p.Percentual > 100 {
		return 100
	}
	return p.Percentual
}

// StatusTitulo é o estado de um título a receber/pagar.
type StatusTitulo string

const (
	Aberto    StatusTitulo = "ABERTO"
	Parcial   StatusTitulo = "PARCIAL"
	Baixado   StatusTitulo = "BAIXADO"
	Cancelado StatusTitulo = "CANCELADO"
)

// StatusPorBaixa recalcula o status de um título não cancelado (RN-BAIXA-01):
// baixado se valor baixado ≥ valor; parcial se > 0; senão aberto.
func StatusPorBaixa(valor, baixado decimal.Decimal) StatusTitulo {
	switch {
	case baixado.Cmp(valor) >= 0:
		return Baixado
	case baixado.Sign() > 0:
		return Parcial
	default:
		return Aberto
	}
}
