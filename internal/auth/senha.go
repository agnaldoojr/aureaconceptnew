// Package auth reúne senhas (BCrypt), tokens do app e, adiante, sessões e limite de login.
package auth

import "golang.org/x/crypto/bcrypt"

// CustoBCrypt é o fator do sistema antigo (BCryptPasswordEncoder(10)); RN-SEG-01.
const CustoBCrypt = 10

// GerarHash devolve o hash BCrypt (fator 10) da senha. O Go grava o prefixo $2a$, igual ao Spring.
func GerarHash(senha string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(senha), CustoBCrypt)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// ConferirSenha diz se a senha confere com o hash (inclusive os hashes $2a$10$ já gravados pelo sistema Java).
// Senha vazia nunca confere.
func ConferirSenha(hash, senha string) bool {
	if senha == "" || hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(senha)) == nil
}
