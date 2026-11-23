package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/agnaldoojr/aureaconceptnew/internal/db"
)

// NomeCookieSessao é o mesmo do sistema antigo (Tomcat), para o JS e os proxies que já conhecem esse nome continuarem valendo.
const NomeCookieSessao = "JSESSIONID"

// duração da sessão anônima (só serve para o token CSRF do formulário de login)
const duracaoAnonima = 30 * time.Minute

// atualiza ses_ultimo_acesso no máximo a cada tanto, para não gravar no banco a cada clique
const passoAtualizacao = time.Minute

// Sessao é uma sessão carregada do banco.
type Sessao struct {
	ID          string // identificador de verdade (só existe no cookie e em memória; no banco fica o SHA-256)
	hash        string
	UsuarioID   int64 // 0 = anônima
	CSRF        string
	ModoApp     bool
	Duracao     time.Duration
	Substituida bool
	ultimo      time.Time
}

// Autenticada diz se a sessão pertence a um usuário logado.
func (s *Sessao) Autenticada() bool { return s != nil && s.UsuarioID != 0 }

// Sessoes guarda as sessões no SQLite (D-12).
type Sessoes struct {
	b     *db.Banco
	agora func() time.Time
}

// NovaSessoes cria o armazenamento. agora pode ser nil (usa time.Now).
func NovaSessoes(b *db.Banco, agora func() time.Time) *Sessoes {
	if agora == nil {
		agora = time.Now
	}
	return &Sessoes{b: b, agora: agora}
}

func aleatorio(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashDe(id string) string {
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:])
}

// Criar cria uma sessão nova com identificador de 256 bits. usuarioID 0 cria uma sessão anônima.
// Para o login: o chamador cria a sessão autenticada e depois chama Encerrar na anônima (novo id a cada login).
// Uma sessão autenticada substitui as outras do mesmo usuário (maximumSessions(1) do Spring): as antigas ficam marcadas e, ao
// serem usadas, levam o usuário a /login?expired=true.
func (s *Sessoes) Criar(ctx context.Context, usuarioID int64, modoApp bool, duracao time.Duration) (*Sessao, error) {
	id, err := aleatorio(32)
	if err != nil {
		return nil, err
	}
	csrf, err := aleatorio(32)
	if err != nil {
		return nil, err
	}
	if usuarioID == 0 {
		duracao = duracaoAnonima
	}
	agora := s.agora()
	tx, err := s.b.W.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if usuarioID != 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE Sessao_Web SET ses_substituida = 1 WHERE ses_usu_codigo = ?`, usuarioID); err != nil {
			return nil, err
		}
	}
	var uid any
	if usuarioID != 0 {
		uid = usuarioID
	}
	app := 0
	if modoApp {
		app = 1
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO Sessao_Web (ses_hash, ses_usu_codigo, ses_csrf, ses_criada_em, ses_ultimo_acesso, ses_duracao_seg, ses_modo_app)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, hashDe(id), uid, csrf, agora.Unix(), agora.Unix(), int64(duracao.Seconds()), app); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Sessao{ID: id, hash: hashDe(id), UsuarioID: usuarioID, CSRF: csrf, ModoApp: modoApp, Duracao: duracao, ultimo: agora}, nil
}

// Buscar devolve a sessão do identificador, ou (nil, nil) se não existe ou expirou por inatividade.
// Atualiza o último acesso (a inatividade só conta o tempo parado, como no Tomcat).
func (s *Sessoes) Buscar(ctx context.Context, id string) (*Sessao, error) {
	if id == "" || len(id) > 128 {
		return nil, nil
	}
	h := hashDe(id)
	var ss Sessao
	var uid sql.NullInt64
	var ultimo, dur int64
	var app, subst int
	err := s.b.R.QueryRowContext(ctx, `SELECT ses_usu_codigo, ses_csrf, ses_ultimo_acesso, ses_duracao_seg, ses_modo_app, ses_substituida FROM Sessao_Web WHERE ses_hash = ?`, h).
		Scan(&uid, &ss.CSRF, &ultimo, &dur, &app, &subst)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	agora := s.agora()
	if agora.Unix()-ultimo > dur {
		_, _ = s.b.W.ExecContext(ctx, `DELETE FROM Sessao_Web WHERE ses_hash = ?`, h)
		return nil, nil
	}
	ss.ID, ss.hash = id, h
	ss.UsuarioID = uid.Int64
	ss.ModoApp = app == 1
	ss.Duracao = time.Duration(dur) * time.Second
	ss.Substituida = subst == 1
	ss.ultimo = time.Unix(ultimo, 0)
	if agora.Sub(ss.ultimo) >= passoAtualizacao && !ss.Substituida {
		if _, err := s.b.W.ExecContext(ctx, `UPDATE Sessao_Web SET ses_ultimo_acesso = ? WHERE ses_hash = ?`, agora.Unix(), h); err == nil {
			ss.ultimo = agora
		}
	}
	return &ss, nil
}

// Encerrar apaga a sessão (logout, ou troca da anônima pela autenticada).
func (s *Sessoes) Encerrar(ctx context.Context, id string) error {
	_, err := s.b.W.ExecContext(ctx, `DELETE FROM Sessao_Web WHERE ses_hash = ?`, hashDe(id))
	return err
}

// Flash grava uma mensagem de uma só leitura (tipo: "erro", "sucesso" ou "info") no item pendente da sessão (cria um se não
// houver). Ela só ganha endereço de destino em DestinarFlash, quando a requisição redireciona. Como no antigo, cada redirect
// gera um item próprio e eles formam uma fila.
func (s *Sessoes) Flash(ctx context.Context, sess *Sessao, tipo, mensagem string) error {
	col, err := colunaFlash(tipo)
	if err != nil {
		return err
	}
	tx, err := s.b.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT flash_codigo FROM Flash_Web WHERE flash_ses_hash = ? AND flash_destino IS NULL ORDER BY flash_codigo LIMIT 1`, sess.hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := tx.ExecContext(ctx, `INSERT INTO Flash_Web (flash_ses_hash) SELECT ses_hash FROM Sessao_Web WHERE ses_hash = ?`, sess.hash)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil || id == 0 {
			return err // sessão que já não existe: nada a gravar
		}
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE Flash_Web SET `+col+` = ? WHERE flash_codigo = ?`, mensagem, id); err != nil {
		return err
	}
	return tx.Commit()
}

// duracaoFlash é quanto tempo a mensagem espera pela tela de destino (o FlashMap do Spring vale 180 s).
const duracaoFlash = 180 * time.Second

// DestinarFlash diz em que endereço (só o caminho, sem a consulta) o item pendente deve aparecer: o do redirect.
func (s *Sessoes) DestinarFlash(ctx context.Context, sess *Sessao, destino string) error {
	_, err := s.b.W.ExecContext(ctx, `UPDATE Flash_Web SET flash_destino = ?, flash_expira = ? WHERE flash_ses_hash = ? AND flash_destino IS NULL`,
		destino, s.agora().Add(duracaoFlash).Unix(), sess.hash)
	return err
}

// LerFlash consome o item MAIS ANTIGO destinado a este caminho (os vencidos são descartados). Visitar outra tela antes não
// consome nem mostra nada, e duas mensagens para a mesma tela saem uma por visita, como no Spring.
func (s *Sessoes) LerFlash(ctx context.Context, sess *Sessao, caminho string) (erro, sucesso, info string, err error) {
	tx, err := s.b.W.BeginTx(ctx, nil)
	if err != nil {
		return "", "", "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM Flash_Web WHERE flash_ses_hash = ? AND flash_expira IS NOT NULL AND flash_expira < ?`, sess.hash, s.agora().Unix()); err != nil {
		return "", "", "", err
	}
	var id int64
	var e, su, in sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT flash_codigo, flash_erro, flash_sucesso, flash_info FROM Flash_Web WHERE flash_ses_hash = ? AND flash_destino = ? ORDER BY flash_codigo LIMIT 1`, sess.hash, caminho).Scan(&id, &e, &su, &in)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", tx.Commit()
	}
	if err != nil {
		return "", "", "", err
	}
	// consumir um item remove TODOS os iguais (mesmo destino e mesmas mensagens): o FlashMap do Spring compara por conteúdo e o
	// antigo apaga com removeAll. Mensagens diferentes continuam na fila, uma por visita.
	if _, err := tx.ExecContext(ctx, `DELETE FROM Flash_Web WHERE flash_ses_hash = ? AND flash_destino = ? AND flash_erro IS ? AND flash_sucesso IS ? AND flash_info IS ?`, sess.hash, caminho, e, su, in); err != nil {
		return "", "", "", err
	}
	return e.String, su.String, in.String, tx.Commit()
}

func colunaFlash(tipo string) (string, error) {
	switch tipo {
	case "erro":
		return "flash_erro", nil
	case "sucesso":
		return "flash_sucesso", nil
	case "info":
		return "flash_info", nil
	}
	return "", errors.New("tipo de mensagem desconhecido: " + tipo)
}

// Limpar apaga as sessões vencidas (anônimas e de usuários) e as substituídas há mais de um dia. Chamada de tempos em tempos pelo servidor.
func (s *Sessoes) Limpar(ctx context.Context) (int64, error) {
	agora := s.agora().Unix()
	r, err := s.b.W.ExecContext(ctx, `DELETE FROM Sessao_Web WHERE ses_ultimo_acesso + ses_duracao_seg < ? OR (ses_substituida = 1 AND ses_ultimo_acesso < ?)`, agora, agora-86400)
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}
