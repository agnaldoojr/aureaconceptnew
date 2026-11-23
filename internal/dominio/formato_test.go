package dominio

import (
	"math/big"
	"testing"
)

func TestEscala(t *testing.T) {
	casos := []struct {
		v     int64
		casas int
		quer  string
	}{{8990, 2, "89.90"}, {10000, 3, "10.000"}, {5, 2, "0.05"}, {0, 3, "0.000"}, {12000000, 6, "12.000000"}, {-150, 2, "-1.50"}, {7, 0, "7"}}
	for _, c := range casos {
		if got := Escala(c.v, c.casas); got != c.quer {
			t.Errorf("Escala(%d,%d) = %q, esperado %q", c.v, c.casas, got, c.quer)
		}
	}
}

// O `#numbers.formatDecimal` do Thymeleaf usa o DecimalFormat do Java, que arredonda HALF_EVEN: 0,125 → "0,12" e 0,135 → "0,14".
func TestFormatarBRArredondaHalfEven(t *testing.T) {
	casos := []struct {
		v      int64
		escala int
		casas  int
		quer   string
	}{
		{125, 3, 2, "0,12"},       // meio: vai para o par (2)
		{135, 3, 2, "0,14"},       // meio: vai para o par (4)
		{126, 3, 2, "0,13"},       // acima do meio
		{124, 3, 2, "0,12"},       // abaixo do meio
		{351250, 4, 2, "35,12"},   // custo médio 35,1250
		{351350, 4, 2, "35,14"},   // 35,1350
		{351251, 4, 2, "35,13"},   // 35,1251
		{1229375, 4, 2, "122,94"}, // 122,9375
		{8990, 2, 2, "89,90"},     // sem arredondamento
		{123456, 2, 2, "1.234,56"},
		{100000000, 2, 2, "1.000.000,00"},
		{5, 2, 2, "0,05"},
		{1234567, 3, 3, "1.234,567"},
		{0, 4, 2, "0,00"},
		{-125, 3, 2, "-0,12"},
		{-1, 3, 2, "0,00"}, // -0,001 vira zero sem sinal
		{7, 0, 2, "7,00"},  // escala menor que as casas: completa com zeros
	}
	for _, c := range casos {
		if got := FormatarBR64(c.v, c.escala, c.casas); got != c.quer {
			t.Errorf("FormatarBR(%d, escala %d, %d casas) = %q, esperado %q", c.v, c.escala, c.casas, got, c.quer)
		}
	}
	// números maiores que int64 (quantidade × custo de estoques enormes)
	grande, _ := new(big.Int).SetString("123456789012345678901234567", 10)
	if got := FormatarBR(grande, 7, 2); got != "12.345.678.901.234.567.890,12" {
		t.Errorf("FormatarBR grande = %q", got)
	}
}
