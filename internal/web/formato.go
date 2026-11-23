package web

import (
	"regexp"
	"strconv"
	"strings"
)

// escala escreve um inteiro escalado com `casas` decimais, como o BigDecimal do Java imprime um valor da coluna DECIMAL(x,casas):
// (8990, 2) → "89.90"; (12000000, 6) → "12.000000". É o texto que vai nos campos dos formulários.
func escala(v int64, casas int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatInt(v, 10)
	for len(s) <= casas {
		s = "0" + s
	}
	r := s[:len(s)-casas] + "." + s[len(s)-casas:]
	if casas == 0 {
		r = s
	}
	if neg {
		r = "-" + r
	}
	return r
}

// decimalBR é o `#numbers.formatDecimal(x, 1, 'POINT', casas, 'COMMA')` do Thymeleaf: 1234.5 → "1.234,50".
func decimalBR(v int64, casas int) string {
	e := escala(v, casas)
	sinal := ""
	if strings.HasPrefix(e, "-") {
		sinal, e = "-", e[1:]
	}
	inteiro, frac, _ := strings.Cut(e, ".")
	var b strings.Builder
	for i, c := range inteiro {
		if i > 0 && (len(inteiro)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if casas > 0 {
		return sinal + b.String() + "," + frac
	}
	return sinal + b.String()
}

// moeda escreve "R$ 1.234,56" a partir de centavos.
func moeda(centavos int64) string { return "R$ " + decimalBR(centavos, 2) }

var reDecimal = regexp.MustCompile(`^([+-]?)(\d*)(?:\.(\d*))?$`)

// textoBigDecimal reescreve um número digitado como o `new BigDecimal(texto).toString()` do Java o faria ao reexibir o formulário:
// "89.90" fica "89.90", "089.9" vira "89.9", ".5" vira "0.5", "+7" vira "7". ok=false se não for um decimal simples.
func textoBigDecimal(t string) (string, bool) {
	t = strings.TrimSpace(t)
	m := reDecimal.FindStringSubmatch(t)
	if m == nil || (m[2] == "" && m[3] == "") {
		return "", false
	}
	sinal, inteiro, frac := m[1], m[2], m[3]
	if sinal == "+" {
		sinal = ""
	}
	inteiro = strings.TrimLeft(inteiro, "0")
	if inteiro == "" {
		inteiro = "0"
	}
	if frac != "" {
		return sinal + inteiro + "." + frac, true
	}
	return sinal + inteiro, true
}
