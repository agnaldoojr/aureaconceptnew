// Package bootstrap prepara o banco no primeiro boot: cria o usuário admin a partir de ADMIN_SENHA_INICIAL (SEG-03, decisão D-01).
// O sistema novo NUNCA cria usuário com senha fixa: o schema.sql do antigo trazia admin/Admin@123, e isso deixou de existir.
package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/agnaldoojr/aureaconceptnew/internal/auth"
	"github.com/agnaldoojr/aureaconceptnew/internal/config"
	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

// ErrSenhaPadrao é devolvido quando o admin do banco ainda tem a senha pública do sistema antigo (Admin@123).
var ErrSenhaPadrao = errors.New("o usuário admin ainda usa a senha padrão Admin@123 do sistema antigo; troque-a antes de subir o sistema (por exemplo, pela tela de usuários do sistema antigo, antes de migrar)")

// ErrSemSenhaInicial é devolvido quando precisa criar o admin e ADMIN_SENHA_INICIAL não foi informada.
var ErrSemSenhaInicial = errors.New("não existe o usuário admin e ADMIN_SENHA_INICIAL não foi informada: defina essa variável (mínimo de 10 caracteres, com maiúscula, minúscula e número) para o primeiro boot")

// senhaPublicaAntiga é a senha do schema.sql do sistema antigo.
const senhaPublicaAntiga = "Admin@123"

// GarantirAdmin faz o seguinte, dentro de uma transação:
//   - se o admin NÃO existe: exige senhaInicial válida, cria o usuário "admin" (nome "Administrador") com hash BCrypt e
//     linhas de permissão com tudo marcado para os 21 módulos; devolve criado=true;
//   - se o admin existe: nunca altera a senha (ADMIN_SENHA_INICIAL é ignorada), mas recusa se o hash for o do Admin@123.
func GarantirAdmin(ctx context.Context, w *sql.DB, senhaInicial string) (criado bool, err error) {
	tx, err := w.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var id int64
	var hash string
	erro := tx.QueryRowContext(ctx, `SELECT usu_codigo, usu_senha FROM Usuarios WHERE usu_login = ?`, permissoes.LoginAdmin).Scan(&id, &hash)
	switch {
	case erro == nil:
		if auth.ConferirSenha(hash, senhaPublicaAntiga) {
			return false, ErrSenhaPadrao
		}
		return false, tx.Commit() // já existe e está seguro: nada a fazer
	case !errors.Is(erro, sql.ErrNoRows):
		return false, erro
	}

	if senhaInicial == "" {
		return false, ErrSemSenhaInicial
	}
	if err := config.ValidarSenhaAdmin(senhaInicial); err != nil {
		return false, fmt.Errorf("ADMIN_SENHA_INICIAL: %w", err)
	}
	novoHash, err := auth.GerarHash(senhaInicial)
	if err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO Usuarios (usu_nome, usu_login, usu_senha, usu_ativo) VALUES ('Administrador', ?, ?, 1)`, permissoes.LoginAdmin, novoHash)
	if err != nil {
		return false, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return false, err
	}
	for _, m := range permissoes.Modulos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO Direito_Usuario (dusu_usu_codigo, dusu_descricao, dusu_inclusao, dusu_exclusao, dusu_consulta, dusu_atualizacao) VALUES (?, ?, 1, 1, 1, 1)`, id, m); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}
