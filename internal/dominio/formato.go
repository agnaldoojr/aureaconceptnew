package dominio

import (
	"math/big"
	"strings"
)

// Escala escreve um inteiro escalado com `casas` decimais, como o BigDecimal do Java imprime um valor DECIMAL(x,casas):
// (8990, 2) → "89.90"; (10000, 3) → "10.000".
func Escala(v int64, casas int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := big.NewInt(v).String()
	for len(s) <= casas {
		s = "0" + s
	}
	r := s
	if casas > 0 {
		r = s[:len(s)-casas] + "." + s[len(s)-casas:]
	}
	if neg {
		r = "-" + r
	}
	return r
}

// FormatarBR é o `#numbers.formatDecimal(x, 1, 'POINT', casas, 'COMMA')` do Thymeleaf: milhar com ponto, decimal com vírgula.
// O Java arredonda com HALF_EVEN (padrão do DecimalFormat): 0,125 → "0,12" e 0,135 → "0,14". `valor` é o número escalado
// (valor × 10^escalaOrigem) e pode ser maior que um int64 (produtos de quantidade × custo).
func FormatarBR(valor *big.Int, escalaOrigem, casas int) string {
	v := new(big.Int).Set(valor)
	neg := v.Sign() < 0
	v.Abs(v)
	if escalaOrigem > casas {
		div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(escalaOrigem-casas)), nil)
		q, r := new(big.Int).QuoRem(v, div, new(big.Int))
		dobro := new(big.Int).Mul(r, big.NewInt(2))
		switch c := dobro.Cmp(div); {
		case c > 0:
			q.Add(q, big.NewInt(1))
		case c == 0 && q.Bit(0) == 1: // exatamente no meio: vai para o par
			q.Add(q, big.NewInt(1))
		}
		v = q
	} else if escalaOrigem < casas {
		v.Mul(v, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(casas-escalaOrigem)), nil))
	}
	s := v.String()
	for len(s) <= casas {
		s = "0" + s
	}
	inteiro, frac := s[:len(s)-casas], s[len(s)-casas:]
	var b strings.Builder
	for i, c := range inteiro {
		if i > 0 && (len(inteiro)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if casas > 0 {
		out += "," + frac
	}
	if neg && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

// FormatarBR64 é FormatarBR para valores que cabem em int64.
func FormatarBR64(valor int64, escalaOrigem, casas int) string {
	return FormatarBR(big.NewInt(valor), escalaOrigem, casas)
}
