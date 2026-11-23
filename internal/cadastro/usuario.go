package cadastro

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agnaldoojr/aureaconceptnew/internal/auth"
	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

// Usuario é um usuário do sistema.
type Usuario struct {
	Codigo       int64
	Nome         string
	Login        string
	Ativo        bool
	VendedorID   *int64
	VendedorNome string
}

// ehAdmin diz se o login é o do administrador (comparado sem diferenciar maiúscula, como o antigo).
func ehAdmin(login string) bool { return strings.EqualFold(login, permissoes.LoginAdmin) }

// ListarUsuarios devolve todos, na ordem do código, com o nome do vendedor vinculado.
func (r *Repo) ListarUsuarios(ctx context.Context) ([]Usuario, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT u.usu_codigo, u.usu_nome, u.usu_login, u.usu_ativo, u.usu_ups_vendedor, COALESCE(p.ups_nome,'') FROM Usuarios u LEFT JOIN Pessoas p ON p.ups_codigo = u.usu_ups_vendedor ORDER BY u.usu_codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []Usuario
	for rows.Next() {
		var u Usuario
		var ativo int
		var vend sql.NullInt64
		if err := rows.Scan(&u.Codigo, &u.Nome, &u.Login, &ativo, &vend, &u.VendedorNome); err != nil {
			return nil, err
		}
		u.Ativo, u.VendedorID = ativo == 1, ptr(vend)
		lista = append(lista, u)
	}
	return lista, rows.Err()
}

// BuscarUsuario devolve o usuário; (nil, nil) se não existe.
func (r *Repo) BuscarUsuario(ctx context.Context, id int64) (*Usuario, error) {
	var u Usuario
	var ativo int
	var vend sql.NullInt64
	err := r.B.R.QueryRowContext(ctx, `SELECT u.usu_codigo, u.usu_nome, u.usu_login, u.usu_ativo, u.usu_ups_vendedor, COALESCE(p.ups_nome,'') FROM Usuarios u LEFT JOIN Pessoas p ON p.ups_codigo = u.usu_ups_vendedor WHERE u.usu_codigo = ?`, id).
		Scan(&u.Codigo, &u.Nome, &u.Login, &ativo, &vend, &u.VendedorNome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Ativo, u.VendedorID = ativo == 1, ptr(vend)
	return &u, nil
}

// VendedorOpcao é um vendedor ativo que pode ser vinculado a um usuário.
type VendedorOpcao struct {
	Codigo int64
	Nome   string
}

// compararSemCaixa reproduz String.CASE_INSENSITIVE_ORDER do Java: letra a letra (maiúscula, depois minúscula), SEM ignorar acento.
func compararSemCaixa(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := min(len(ra), len(rb))
	for i := 0; i < n; i++ {
		c1, c2 := ra[i], rb[i]
		if c1 != c2 {
			c1, c2 = unicode.ToUpper(c1), unicode.ToUpper(c2)
			if c1 != c2 {
				c1, c2 = unicode.ToLower(c1), unicode.ToLower(c2)
				if c1 != c2 {
					return int(c1) - int(c2)
				}
			}
		}
	}
	return len(ra) - len(rb)
}

// VendedoresAtivos são as pessoas ativas marcadas como vendedor, em ordem alfabética sem diferenciar maiúscula (o antigo ordena no Java).
func (r *Repo) VendedoresAtivos(ctx context.Context) ([]VendedorOpcao, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT ups_codigo, ups_nome FROM Pessoas WHERE ups_ativo = 1 AND ups_vendedor = 1 ORDER BY ups_codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []VendedorOpcao
	for rows.Next() {
		var v VendedorOpcao
		if err := rows.Scan(&v.Codigo, &v.Nome); err != nil {
			return nil, err
		}
		lista = append(lista, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(lista, func(i, j int) bool { return compararSemCaixa(lista[i].Nome, lista[j].Nome) < 0 })
	return lista, nil
}

// ValidarUsuario aplica as anotações de Usuario.java. A tela do antigo não mostra texto para estes erros (só volta ao formulário).
func ValidarUsuario(u *Usuario) []ErroCampo {
	var erros []ErroCampo
	if strings.TrimSpace(u.Nome) == "" || utf8.RuneCountInString(u.Nome) > 150 {
		erros = append(erros, ErroCampo{Campo: "usuNome"})
	}
	if strings.TrimSpace(u.Login) == "" || utf8.RuneCountInString(u.Login) > 80 {
		erros = append(erros, ErroCampo{Campo: "usuLogin"})
	}
	return erros
}

func vendedorValido(ctx context.Context, tx consultor, id *int64) error {
	if id == nil {
		return nil
	}
	var nome string
	var vend, ativo int
	err := tx.QueryRowContext(ctx, `SELECT ups_nome, ups_vendedor, ups_ativo FROM Pessoas WHERE ups_codigo = ?`, *id).Scan(&nome, &vend, &ativo)
	if errors.Is(err, sql.ErrNoRows) {
		return regra("Vendedor não encontrado.")
	}
	if err != nil {
		return err
	}
	if vend != 1 || ativo != 1 {
		return regra("A pessoa \"%s\" não está cadastrada como vendedor ativo.", nome)
	}
	return nil
}

// SalvarUsuario cria ou atualiza o usuário (UsuarioService.salvar). Novo: senha em BCrypt, uma linha de direito por módulo (Consulta
// só nos módulos padrão) e, se vier um perfil, ele substitui tudo — numa só transação: perfil desconhecido desfaz o cadastro (e o
// código gerado fica queimado, como no SQL Server). Edição: copia só nome, login, status, vendedor e senha (em branco mantém);
// o administrador não muda de login nem é inativado. Devolve o código (também quando falha depois de inserir).
func (r *Repo) SalvarUsuario(ctx context.Context, f Usuario, novaSenha string, vendedor *int64, perfil string) (int64, error) {
	novo := f.Codigo == 0
	id, err := r.salvarUsuario(ctx, f, novaSenha, vendedor, perfil)
	if err != nil && novo && id != 0 {
		r.queimarCodigo(ctx, "Usuarios", id)
	}
	return id, err
}

func (r *Repo) salvarUsuario(ctx context.Context, f Usuario, novaSenha string, vendedor *int64, perfil string) (int64, error) {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	novo := f.Codigo == 0
	if err := vendedorValido(ctx, tx, vendedor); err != nil {
		return 0, err
	}
	login := strings.TrimSpace(f.Login)
	var outro int64
	err = tx.QueryRowContext(ctx, `SELECT usu_codigo FROM Usuarios WHERE usu_login = ?`, login).Scan(&outro)
	if err == nil && (novo || outro != f.Codigo) {
		return 0, regra("Já existe um usuário com o login \"%s\".", login)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	temSenha := strings.TrimSpace(novaSenha) != ""

	if novo {
		hash, err := auth.GerarHash(novaSenha)
		if err != nil {
			return 0, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO Usuarios (usu_nome, usu_login, usu_senha, usu_ativo, usu_ups_vendedor) VALUES (?,?,?,?,?)`, f.Nome, login, hash, b2i(f.Ativo), vendedor)
		if err != nil {
			return 0, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		for _, m := range permissoes.Modulos {
			consulta := false
			for _, p := range permissoes.PadraoNovoUsuario {
				consulta = consulta || p == m
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO Direito_Usuario (dusu_usu_codigo, dusu_descricao, dusu_inclusao, dusu_exclusao, dusu_consulta, dusu_atualizacao) VALUES (?,?,0,0,?,0)`, id, m, b2i(consulta)); err != nil {
				return id, err
			}
		}
		if strings.TrimSpace(perfil) != "" {
			if err := aplicarPerfil(ctx, tx, id, perfil); err != nil {
				return id, err
			}
		}
		return id, tx.Commit()
	}

	var atual string
	if err := tx.QueryRowContext(ctx, `SELECT usu_login FROM Usuarios WHERE usu_codigo = ?`, f.Codigo).Scan(&atual); errors.Is(err, sql.ErrNoRows) {
		return 0, regra("Usuário não encontrado.")
	} else if err != nil {
		return 0, err
	}
	if ehAdmin(atual) && !ehAdmin(login) {
		return 0, regra("O login do administrador não pode ser alterado.")
	}
	if ehAdmin(atual) && !f.Ativo {
		return 0, regra("O administrador não pode ser inativado.")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE Usuarios SET usu_nome = ?, usu_login = ?, usu_ativo = ?, usu_ups_vendedor = ? WHERE usu_codigo = ?`, f.Nome, login, b2i(f.Ativo), vendedor, f.Codigo); err != nil {
		return 0, err
	}
	if temSenha {
		hash, err := auth.GerarHash(novaSenha)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE Usuarios SET usu_senha = ? WHERE usu_codigo = ?`, hash, f.Codigo); err != nil {
			return 0, err
		}
	}
	return f.Codigo, tx.Commit()
}

// AlterarStatusUsuario ativa/inativa; o administrador não pode ser inativado; código inexistente não faz nada.
func (r *Repo) AlterarStatusUsuario(ctx context.Context, id int64, ativo bool) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var login string
	if err := tx.QueryRowContext(ctx, `SELECT usu_login FROM Usuarios WHERE usu_codigo = ?`, id).Scan(&login); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if !ativo && ehAdmin(login) {
		return regra("O administrador não pode ser inativado.")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE Usuarios SET usu_ativo = ? WHERE usu_codigo = ?`, b2i(ativo), id); err != nil {
		return err
	}
	return tx.Commit()
}

// ExcluirUsuario apaga o usuário e seus direitos e tokens. O administrador não pode ser excluído. Usuário que lançou documentos não
// pode ser apagado (a chave estrangeira barra; use Inativar). Código inexistente não faz nada.
func (r *Repo) ExcluirUsuario(ctx context.Context, id int64) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var login string
	err = tx.QueryRowContext(ctx, `SELECT usu_login FROM Usuarios WHERE usu_codigo = ?`, id).Scan(&login)
	if err == nil && ehAdmin(login) {
		return regra("O administrador não pode ser excluído.")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Usuarios WHERE usu_codigo = ?`, id); err != nil {
		return err // chave estrangeira (documentos do usuário): a tela mostra "Não foi possível excluir."
	}
	return tx.Commit()
}

// ───────────────────────────── permissões ─────────────────────────────

// Direito é a linha de permissões de um módulo de um usuário.
type Direito struct {
	Modulo                                    string
	Inclusao, Exclusao, Consulta, Atualizacao bool
}

// PermissoesDoUsuario devolve uma linha por módulo do catálogo, na ordem do catálogo. Módulo sem linha (criado depois do cadastro do
// usuário) é criado zerado, sem nenhum acesso, como no antigo. Usuário inexistente é erro de regra.
func (r *Repo) PermissoesDoUsuario(ctx context.Context, id int64) ([]Direito, error) {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	lista, err := permissoesDoUsuario(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return lista, tx.Commit()
}

func permissoesDoUsuario(ctx context.Context, tx consultor, id int64) ([]Direito, error) {
	var existe int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM Usuarios WHERE usu_codigo = ?`, id).Scan(&existe); err != nil {
		return nil, err
	}
	if existe == 0 {
		return nil, regra("Usuário não encontrado.")
	}
	rows, err := tx.QueryContext(ctx, `SELECT dusu_descricao, dusu_inclusao, dusu_exclusao, dusu_consulta, dusu_atualizacao FROM Direito_Usuario WHERE dusu_usu_codigo = ? ORDER BY dusu_codigo`, id)
	if err != nil {
		return nil, err
	}
	tem := map[string]Direito{}
	for rows.Next() {
		var d Direito
		var i, e, c, a int
		if err := rows.Scan(&d.Modulo, &i, &e, &c, &a); err != nil {
			rows.Close()
			return nil, err
		}
		d.Inclusao, d.Exclusao, d.Consulta, d.Atualizacao = i == 1, e == 1, c == 1, a == 1
		if _, ja := tem[d.Modulo]; !ja {
			tem[d.Modulo] = d
		}
	}
	rows.Close()
	var lista []Direito
	for _, m := range permissoes.Modulos {
		d, ok := tem[m]
		if !ok {
			if _, err := tx.ExecContext(ctx, `INSERT INTO Direito_Usuario (dusu_usu_codigo, dusu_descricao) VALUES (?,?)`, id, m); err != nil {
				return nil, err
			}
			d = Direito{Modulo: m}
		}
		lista = append(lista, d)
	}
	return lista, nil
}

func gravarDireito(ctx context.Context, tx consultor, id int64, d Direito) error {
	_, err := tx.ExecContext(ctx, `UPDATE Direito_Usuario SET dusu_inclusao = ?, dusu_exclusao = ?, dusu_consulta = ?, dusu_atualizacao = ? WHERE dusu_usu_codigo = ? AND dusu_descricao = ?`,
		b2i(d.Inclusao), b2i(d.Exclusao), b2i(d.Consulta), b2i(d.Atualizacao), id, d.Modulo)
	return err
}

// SalvarPermissoes grava a matriz da tela. marcadas é o conjunto de campos presentes ("consulta_PRODUTOS", ...): presente = marcado.
func (r *Repo) SalvarPermissoes(ctx context.Context, id int64, marcadas map[string]bool) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	lista, err := permissoesDoUsuario(ctx, tx, id)
	if err != nil {
		return err
	}
	for _, d := range lista {
		d.Inclusao = marcadas["inclusao_"+d.Modulo]
		d.Exclusao = marcadas["exclusao_"+d.Modulo]
		d.Consulta = marcadas["consulta_"+d.Modulo]
		d.Atualizacao = marcadas["atualizacao_"+d.Modulo]
		if err := gravarDireito(ctx, tx, id, d); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AplicarPerfil troca TODAS as permissões do usuário pelas do perfil (o que o perfil não cita fica sem acesso). Não vale para o admin.
func (r *Repo) AplicarPerfil(ctx context.Context, id int64, codigoPerfil string) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := aplicarPerfil(ctx, tx, id, codigoPerfil); err != nil {
		return err
	}
	return tx.Commit()
}

func aplicarPerfil(ctx context.Context, tx consultor, id int64, codigo string) error {
	var perfil *permissoes.Perfil
	for i := range permissoes.Perfis {
		if permissoes.Perfis[i].Codigo == codigo {
			perfil = &permissoes.Perfis[i]
		}
	}
	if perfil == nil {
		return regra("Perfil não encontrado.")
	}
	var login string
	if err := tx.QueryRowContext(ctx, `SELECT usu_login FROM Usuarios WHERE usu_codigo = ?`, id).Scan(&login); errors.Is(err, sql.ErrNoRows) {
		return regra("Usuário não encontrado.")
	} else if err != nil {
		return err
	}
	if ehAdmin(login) {
		return regra("O administrador sempre tem acesso total; não há perfil a aplicar.")
	}
	lista, err := permissoesDoUsuario(ctx, tx, id)
	if err != nil {
		return err
	}
	for _, d := range lista {
		d.Consulta = perfil.Tem(d.Modulo, permissoes.Consulta)
		d.Inclusao = perfil.Tem(d.Modulo, permissoes.Inclusao)
		d.Atualizacao = perfil.Tem(d.Modulo, permissoes.Atualizacao)
		d.Exclusao = perfil.Tem(d.Modulo, permissoes.Exclusao)
		if err := gravarDireito(ctx, tx, id, d); err != nil {
			return err
		}
	}
	return nil
}
