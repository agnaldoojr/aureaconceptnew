package dominio

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// Compara as regras com os valores gerados pelo Java do sistema antigo (tools/golden/Golden.java -> testdata/golden/dinheiro.csv).

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("decimal inválido %q: %v", s, err)
	}
	return d
}

func igual(t *testing.T, linha int, nome string, obtido, esperado decimal.Decimal) {
	t.Helper()
	if !obtido.Equal(esperado) {
		t.Errorf("linha %d (%s): Go=%s Java=%s", linha, nome, obtido.String(), esperado.String())
	}
}

func TestContraValoresDoJava(t *testing.T) {
	f, err := os.Open("../../testdata/golden/dinheiro.csv")
	if err != nil {
		t.Fatalf("falta o arquivo golden (rode java tools/golden/Golden.java testdata/golden/dinheiro.csv): %v", err)
	}
	defer f.Close()
	contagem := map[string]int{}
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		linha := strings.TrimSpace(sc.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		c := strings.Split(linha, ";")
		contagem[c[0]]++
		switch c[0] {
		case "QB":
			igual(t, n, "QtdBase", QtdBase(dec(t, c[1]), dec(t, c[2])), dec(t, c[3]))
		case "VB":
			igual(t, n, "ValorBase", ValorBase(dec(t, c[1]), dec(t, c[2])), dec(t, c[3]))
		case "CM":
			igual(t, n, "CustoMedio", CustoMedio(dec(t, c[1]), dec(t, c[2]), dec(t, c[3]), dec(t, c[4])), dec(t, c[5]))
		case "PARC":
			qtd, _ := strconv.Atoi(c[2])
			obt := Parcelas(dec(t, c[1]), qtd)
			esp := strings.Split(c[3], ",")
			if len(obt) != len(esp) {
				t.Fatalf("linha %d: %d parcelas, Java %d", n, len(obt), len(esp))
			}
			soma := Zero
			for i := range obt {
				igual(t, n, "parcela "+strconv.Itoa(i+1), obt[i], dec(t, esp[i]))
				soma = soma.Add(obt[i])
			}
			igual(t, n, "soma das parcelas = total", soma, dec(t, c[1]))
		case "TAXA":
			igual(t, n, "TaxaCartao", TaxaCartao(dec(t, c[1]), dec(t, c[2])), dec(t, c[3]))
		case "COM":
			perc := dec(t, c[3])
			igual(t, n, "Comissao", Comissao(TotalItem(dec(t, c[1]), dec(t, c[2])), &perc), dec(t, c[4]))
		case "META":
			p := NovoProgressoMeta(dec(t, c[1]), dec(t, c[2]))
			pct, _ := strconv.Atoi(c[3])
			if p.Percentual != pct {
				t.Errorf("linha %d (META pct): Go=%d Java=%d", n, p.Percentual, pct)
			}
			igual(t, n, "META falta", p.Falta, dec(t, c[4]))
			if p.Atingida != (c[5] == "true") {
				t.Errorf("linha %d (META atingida): Go=%v Java=%s", n, p.Atingida, c[5])
			}
		case "PRECO":
			igual(t, n, "PrecoSugerido", PrecoSugeridoNaUnidade(dec(t, c[1]), dec(t, c[2])), dec(t, c[3]))
		case "DOC":
			var totais []decimal.Decimal
			for _, it := range strings.Split(c[1], ",") {
				p := strings.Split(it, "x")
				totais = append(totais, TotalItem(dec(t, p[0]), dec(t, p[1])))
			}
			igual(t, n, "TotalDocumento", TotalDocumento(totais), dec(t, c[2]))
		case "MOV":
			igual(t, n, "ValorMovimentado", ValorMovimentado(dec(t, c[1]), dec(t, c[2]), dec(t, c[3]), dec(t, c[4])), dec(t, c[5]))
		default:
			t.Fatalf("linha %d: tipo desconhecido %q", n, c[0])
		}
	}
	for _, tipo := range []string{"QB", "VB", "CM", "PARC", "TAXA", "COM", "META", "PRECO", "DOC", "MOV"} {
		if contagem[tipo] == 0 {
			t.Errorf("nenhum caso %s no arquivo golden", tipo)
		}
	}
	t.Logf("casos conferidos com o Java: %v", contagem)
}

// D-05: a soma dos itens já arredondados pode diferir do total que o antigo grava; o Go reproduz o do antigo.
func TestTotalDocumentoArredondaUmaVez(t *testing.T) {
	// 3 itens de 0,001 × 5,00 = 0,005 cada; soma 0,015 → 0,02 (HALF_UP); a soma de itens arredondados seria 0,03.
	itens := []decimal.Decimal{
		TotalItem(dec(t, "0.001"), dec(t, "5.00")),
		TotalItem(dec(t, "0.001"), dec(t, "5.00")),
		TotalItem(dec(t, "0.001"), dec(t, "5.00")),
	}
	igual(t, 0, "total do documento", TotalDocumento(itens), dec(t, "0.02"))
}

func TestFatorInvalidoViraUm(t *testing.T) {
	igual(t, 0, "qtd base com fator 0", QtdBase(dec(t, "10"), dec(t, "0")), dec(t, "10"))
	igual(t, 0, "valor base com fator negativo", ValorBase(dec(t, "12.5"), dec(t, "-3")), dec(t, "12.5"))
}

func TestExemploCaixaDoREADME(t *testing.T) {
	// "comprar 2 CX baixa/entra 24 UN e o custo fica por unidade base (R$ 30,00 a CX = R$ 2,50 a UN)"
	igual(t, 0, "qtd base", QtdBase(dec(t, "2"), dec(t, "12")), dec(t, "24"))
	igual(t, 0, "valor base", ValorBase(dec(t, "30.00"), dec(t, "12")), dec(t, "2.5"))
	igual(t, 0, "custo médio da 1ª entrada", CustoMedio(Zero, Zero, dec(t, "24"), dec(t, "2.5")), dec(t, "2.5"))
}

func TestStatusDoTitulo(t *testing.T) {
	v := dec(t, "100")
	casos := []struct {
		baixado string
		quer    StatusTitulo
	}{{"0", Aberto}, {"0.01", Parcial}, {"99.99", Parcial}, {"100", Baixado}, {"120", Baixado}}
	for _, c := range casos {
		if got := StatusPorBaixa(v, dec(t, c.baixado)); got != c.quer {
			t.Errorf("baixado %s: %s, esperado %s", c.baixado, got, c.quer)
		}
	}
}

func TestBarraDaMetaLimitada(t *testing.T) {
	p := NovoProgressoMeta(dec(t, "1000"), dec(t, "2500"))
	if p.Percentual != 250 || p.Barra() != 100 || !p.Atingida || !p.Falta.IsZero() {
		t.Errorf("meta superada: %+v barra=%d", p, p.Barra())
	}
	p = NovoProgressoMeta(dec(t, "10000"), dec(t, "1975.50"))
	if p.Percentual != 19 || p.Barra() != 19 || !p.Falta.Equal(dec(t, "8024.50")) {
		t.Errorf("meta do dashboard de referência (19%%, falta 8.024,50): %+v", p)
	}
}
