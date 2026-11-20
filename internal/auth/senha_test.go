package auth

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// Hashes REAIS gerados pelo sistema Java (testdata/golden/hashes-bcrypt-java.csv): o admin vem do schema.sql
// (senha pública de desenvolvimento) e os demais foram criados pela tela de usuários do sistema antigo com a senha de teste.
func TestHashesDoJavaContinuamValidos(t *testing.T) {
	f, err := os.Open("../../testdata/golden/hashes-bcrypt-java.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	senhas := map[string]string{"admin": "Admin@123", "vendedor1": "Teste@123", "gerente1": "Teste@123", "salao1": "Teste@123", "recepcao1": "Teste@123"}
	conferidos := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		partes := strings.SplitN(strings.TrimSpace(sc.Text()), ";", 2)
		if len(partes) != 2 {
			continue
		}
		login, hash := partes[0], partes[1]
		senha, ok := senhas[login]
		if !ok {
			t.Fatalf("login inesperado no arquivo: %s", login)
		}
		if !strings.HasPrefix(hash, "$2a$10$") {
			t.Errorf("%s: hash não começa com $2a$10$", login)
		}
		if !ConferirSenha(hash, senha) {
			t.Errorf("%s: a senha correta NÃO confere com o hash do Java", login)
		}
		if ConferirSenha(hash, senha+"x") || ConferirSenha(hash, "") {
			t.Errorf("%s: senha errada ou vazia conferiu", login)
		}
		conferidos++
	}
	if conferidos != len(senhas) {
		t.Errorf("conferidos %d hashes, esperado %d", conferidos, len(senhas))
	}
}

func TestNovosHashesSaoBCryptFator10(t *testing.T) {
	h, err := GerarHash("Senha@Nova1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$2a$10$") && !strings.HasPrefix(h, "$2b$10$") {
		t.Errorf("hash %q não é BCrypt de fator 10", h[:7])
	}
	if !ConferirSenha(h, "Senha@Nova1") || ConferirSenha(h, "senha@nova1") {
		t.Error("conferência do hash novo falhou")
	}
}
