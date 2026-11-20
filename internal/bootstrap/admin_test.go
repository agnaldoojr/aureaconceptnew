package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/agnaldoojr/aureaconceptnew/internal/auth"
	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

func novoBanco(t *testing.T) *db.Banco {
	t.Helper()
	b, err := db.Abrir(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Fechar() })
	return b
}

func TestCriaAdminComSenhaInicial(t *testing.T) {
	b := novoBanco(t)
	ctx := context.Background()
	criado, err := GarantirAdmin(ctx, b.W, "Bom-Segredo-2026")
	if err != nil || !criado {
		t.Fatalf("criado=%v err=%v", criado, err)
	}
	var nome, hash string
	var ativo int
	if err := b.R.QueryRow(`SELECT usu_nome, usu_senha, usu_ativo FROM Usuarios WHERE usu_login = 'ADMIN'`).Scan(&nome, &hash, &ativo); err != nil {
		t.Fatalf("login deve casar sem diferenciar maiúscula: %v", err)
	}
	if nome != "Administrador" || ativo != 1 {
		t.Errorf("nome=%q ativo=%d", nome, ativo)
	}
	if !auth.ConferirSenha(hash, "Bom-Segredo-2026") || auth.ConferirSenha(hash, "Admin@123") {
		t.Error("hash não confere com a senha inicial")
	}
	var n, completos int
	b.R.QueryRow(`SELECT count(*), sum(dusu_inclusao+dusu_exclusao+dusu_consulta+dusu_atualizacao = 4) FROM Direito_Usuario`).Scan(&n, &completos)
	if n != len(permissoes.Modulos) || completos != n {
		t.Errorf("direitos: %d linhas, %d completas (esperado %d)", n, completos, len(permissoes.Modulos))
	}
}

func TestSemSenhaInicialRecusa(t *testing.T) {
	b := novoBanco(t)
	if _, err := GarantirAdmin(context.Background(), b.W, ""); !errors.Is(err, ErrSemSenhaInicial) {
		t.Fatalf("esperava ErrSemSenhaInicial, veio %v", err)
	}
	var n int
	b.R.QueryRow(`SELECT count(*) FROM Usuarios`).Scan(&n)
	if n != 0 {
		t.Errorf("nenhum usuário deveria ter sido criado, há %d", n)
	}
}

func TestSenhaFracaOuPadraoNaoCria(t *testing.T) {
	b := novoBanco(t)
	for _, s := range []string{"Admin@123", "curta", "tudominusculo123"} {
		if _, err := GarantirAdmin(context.Background(), b.W, s); err == nil {
			t.Errorf("%q deveria ser recusada", s)
		}
	}
	var n int
	b.R.QueryRow(`SELECT count(*) FROM Usuarios`).Scan(&n)
	if n != 0 {
		t.Errorf("há %d usuários criados", n)
	}
}

func TestAdminExistenteNaoTemSenhaAlterada(t *testing.T) {
	b := novoBanco(t)
	ctx := context.Background()
	if _, err := GarantirAdmin(ctx, b.W, "Bom-Segredo-2026"); err != nil {
		t.Fatal(err)
	}
	criado, err := GarantirAdmin(ctx, b.W, "Outra-Senha-2026") // segundo boot com outra variável: ignorada
	if err != nil || criado {
		t.Fatalf("criado=%v err=%v", criado, err)
	}
	// e o segundo boot nem precisa da variável
	if criado, err = GarantirAdmin(ctx, b.W, ""); err != nil || criado {
		t.Fatalf("sem variável com admin existente deve passar: criado=%v err=%v", criado, err)
	}
	var hash string
	b.R.QueryRow(`SELECT usu_senha FROM Usuarios WHERE usu_login = 'admin'`).Scan(&hash)
	if !auth.ConferirSenha(hash, "Bom-Segredo-2026") {
		t.Error("a senha do admin foi alterada no segundo boot")
	}
}

// O hash do schema.sql do sistema antigo (Admin@123) é recusado: é o caso de um banco migrado sem troca de senha.
func TestAdminComSenhaPadraoAntigaERecusado(t *testing.T) {
	b := novoBanco(t)
	const hashAntigo = "$2a$10$yh88U12BkgBGn5mlgaa0v.q93fTZFhEyAL7YtpLZhlCk6tMP3j8CS"
	if _, err := b.W.Exec(`INSERT INTO Usuarios (usu_nome, usu_login, usu_senha) VALUES ('Administrador','admin',?)`, hashAntigo); err != nil {
		t.Fatal(err)
	}
	if _, err := GarantirAdmin(context.Background(), b.W, "Bom-Segredo-2026"); !errors.Is(err, ErrSenhaPadrao) {
		t.Fatalf("esperava ErrSenhaPadrao, veio %v", err)
	}
}
