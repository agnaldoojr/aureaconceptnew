// Package msg tem o erro de regra de negócio compartilhado pelos pacotes de domínio: a mensagem vai direto para o usuário
// (flash "erro" ou texto na tela), como as IllegalArgumentException/IllegalStateException do sistema antigo.
package msg

import "fmt"

// Erro é um erro de regra de negócio com mensagem pronta para a tela.
type Erro string

func (e Erro) Error() string { return string(e) }

// Novo monta um Erro formatado.
func Novo(f string, a ...any) error { return Erro(fmt.Sprintf(f, a...)) }
