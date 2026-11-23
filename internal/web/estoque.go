package web

import (
	"math/big"
	"net/http"

	"github.com/agnaldoojr/aureaconceptnew/internal/dominio"
)

func (s *Servidor) rotasEstoque() {
	s.mux.HandleFunc("GET /estoque", s.estoqueTela)
}

// linhaEstoque é uma linha da tela de estoque, com os números já no formato do antigo.
type linhaEstoque struct {
	Codigo     int64
	Nome, Und  string
	Qtd        string // 3 casas
	Custo      string // 4 casas guardadas, mostradas com 2 (HALF_EVEN, como o DecimalFormat do Java)
	ValorTotal string // qtd × custo, mostrado com 2 casas
	Classe     string // row-danger, row-warning ou vazio
	Situacao   string // "zero", "baixo" ou "normal"
}

func (s *Servidor) estoqueTela(w http.ResponseWriter, r *http.Request) {
	filtro := r.URL.Query().Get("filtro")
	const baixo = 5000 // 5 unidades, em milésimos
	q := `SELECT e.est_upr_codigo, p.upr_nome, p.upr_und, e.est_qtd, e.est_valor_custo FROM Estoque e JOIN Produto p ON p.upr_codigo = e.est_upr_codigo`
	var args []any
	if filtro == "baixo" {
		q += ` WHERE e.est_qtd <= ? ORDER BY e.est_qtd ASC, e.est_upr_codigo`
		args = append(args, baixo)
	} else {
		q += ` ORDER BY e.est_upr_codigo`
	}
	rows, err := s.banco.R.QueryContext(r.Context(), q, args...)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	var linhas []linhaEstoque
	for rows.Next() {
		var l linhaEstoque
		var qtd, custo int64
		if err := rows.Scan(&l.Codigo, &l.Nome, &l.Und, &qtd, &custo); err != nil {
			rows.Close()
			s.erroInterno(w, err)
			return
		}
		l.Qtd = dominio.FormatarBR64(qtd, 3, 3)
		l.Custo = "R$ " + dominio.FormatarBR64(custo, 4, 2)
		total := new(big.Int).Mul(big.NewInt(qtd), big.NewInt(custo)) // escala 3 + 4 = 7
		l.ValorTotal = "R$ " + dominio.FormatarBR(total, 7, 2)
		switch {
		case qtd == 0:
			l.Classe, l.Situacao = "row-danger", "zero"
		case qtd <= baixo:
			l.Classe, l.Situacao = "row-warning", "baixo"
		default:
			l.Situacao = "normal"
		}
		linhas = append(linhas, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		s.erroInterno(w, err)
		return
	}

	// o valor total é sempre o de TODO o estoque, mesmo com o filtro de estoque baixo
	totalGeral := new(big.Int)
	trows, err := s.banco.R.QueryContext(r.Context(), `SELECT est_qtd, est_valor_custo FROM Estoque`)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	for trows.Next() {
		var qtd, custo int64
		if err := trows.Scan(&qtd, &custo); err != nil {
			trows.Close()
			s.erroInterno(w, err)
			return
		}
		totalGeral.Add(totalGeral, new(big.Int).Mul(big.NewInt(qtd), big.NewInt(custo)))
	}
	trows.Close()

	s.tela(w, r, "estoque/index.html", "Controle de Estoque", "", map[string]any{
		"Linhas": linhas, "Filtro": filtro, "ValorTotal": "R$ " + dominio.FormatarBR(totalGeral, 7, 2),
	})
}
