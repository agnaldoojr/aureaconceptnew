package cadastro

import (
	"sort"
	"testing"
)

// String.CASE_INSENSITIVE_ORDER do Java ignora a caixa, mas NÃO o acento: "Álvaro" fica depois de "Zé".
func TestCompararSemCaixaComoOJava(t *testing.T) {
	nomes := []string{"zé", "Álvaro", "ana", "Bia", "ANA"}
	sort.SliceStable(nomes, func(i, j int) bool { return compararSemCaixa(nomes[i], nomes[j]) < 0 })
	quer := []string{"ana", "ANA", "Bia", "zé", "Álvaro"}
	for i := range quer {
		if nomes[i] != quer[i] {
			t.Fatalf("ordem = %v, esperado %v", nomes, quer)
		}
	}
	if compararSemCaixa("abc", "ABC") != 0 || compararSemCaixa("ab", "abc") >= 0 || compararSemCaixa("b", "A") <= 0 {
		t.Error("comparação básica incorreta")
	}
}
