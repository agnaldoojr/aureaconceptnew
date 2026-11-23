package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
)

// ───────────────────────────── ajudantes de formulário (a "ligação" de campos que o Spring fazia) ─────────────────────────────

// pedidoRuim é um pedido com parâmetro malformado: o Spring responde 400 nesses casos.
func pedidoRuim(w http.ResponseWriter) {
	http.Error(w, "Requisição inválida.", http.StatusBadRequest)
}

// idDoCaminho lê {id} (inteiro) do caminho.
func idDoCaminho(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	return n, err == nil
}

// inteiroOpcional lê um parâmetro numérico opcional (vazio = ausente). ok=false se vier algo que não é número.
func inteiroOpcional(valor string) (n int64, presente, ok bool) {
	if valor == "" {
		return 0, false, true
	}
	v, err := strconv.ParseInt(strings.TrimSpace(valor), 10, 32)
	if err != nil {
		return 0, false, false
	}
	return v, true, true
}

// booleano converte como o Spring: true/on/yes/1 e false/off/no/0.
func booleano(valor string) (v, ok bool) {
	switch strings.ToLower(strings.TrimSpace(valor)) {
	case "true", "on", "yes", "1":
		return true, true
	case "false", "off", "no", "0":
		return false, true
	}
	return false, false
}

// caixa lê uma caixa de marcação como o Thymeleaf+Spring: se o campo veio, vale o valor; se não veio mas veio o marcador "_campo"
// (o navegador sempre envia), está desmarcada; sem nenhum dos dois, vale o padrão do objeto.
func caixa(r *http.Request, campo string, padrao bool) bool {
	if vs, ok := r.PostForm[campo]; ok && len(vs) > 0 {
		if v, ok := booleano(vs[0]); ok {
			return v
		}
		return padrao
	}
	if _, ok := r.PostForm["_"+campo]; ok {
		return false
	}
	return padrao
}

// flash grava uma mensagem de uma só leitura para a próxima tela.
func (s *Servidor) flash(r *http.Request, tipo, mensagem string) {
	if c := ctxDe(r); c != nil {
		if err := s.sessoes.Flash(r.Context(), c.Sessao, tipo, mensagem); err != nil {
			s.log.Warn("gravar mensagem da sessão", "erro", err)
		}
	}
}

// redirecionar manda o navegador para destino; as mensagens de uma só leitura gravadas por flash() ficam amarradas ao caminho
// de destino (só aparecem lá), como o FlashMap do Spring.
func (s *Servidor) redirecionar(w http.ResponseWriter, r *http.Request, destino string) {
	if c := ctxDe(r); c != nil {
		caminho := destino
		if i := strings.IndexAny(caminho, "?#"); i >= 0 {
			caminho = caminho[:i]
		}
		if err := s.sessoes.DestinarFlash(r.Context(), c.Sessao, caminho); err != nil {
			s.log.Warn("encaminhar mensagem da sessão", "erro", err)
		}
	}
	http.Redirect(w, r, destino, http.StatusFound)
}

// mensagemDeErro devolve o texto para o usuário: erros de regra vão como estão; qualquer outro vai para o log e vira uma frase genérica.
func (s *Servidor) mensagemDeErro(err error, generica string) string {
	var regra cadastro.ErroRegra
	if errors.As(err, &regra) {
		return string(regra)
	}
	var cpf cadastro.ErroCPFDuplicado
	if errors.As(err, &cpf) {
		return string(cpf.ErroRegra)
	}
	s.log.Error("erro no cadastro", "erro", err)
	return generica
}
