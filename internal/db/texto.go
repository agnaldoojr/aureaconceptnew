package db

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Normalizar reproduz a comparação do SQL Server com collation Latin1_General_CI_AI: sem diferenciar maiúscula/minúscula
// nem acentos, e ignorando espaços no fim (RN: INVENTARIO §8-1 e §8-2). Ex.: "Calça " ≡ "CALCA".
func Normalizar(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	semAcento, _, err := transform.String(t, s)
	if err != nil {
		semAcento = s
	}
	return strings.TrimRight(strings.ToLower(semAcento), " ")
}

// compararCIAI é a função de comparação da collation AUREA_CIAI do SQLite.
func compararCIAI(a, b string) int {
	return strings.Compare(Normalizar(a), Normalizar(b))
}
