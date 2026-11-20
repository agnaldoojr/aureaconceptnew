// Comando aurea: o servidor do Áurea Concept (Go + SQLite).
//
//	aurea              sobe o servidor (configuração por variáveis de ambiente; ver .env.example)
//	aurea healthcheck  consulta /healthz do próprio servidor (usado no HEALTHCHECK do Docker; sai com 0 se estiver ok)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/agnaldoojr/aureaconceptnew/internal/bootstrap"
	"github.com/agnaldoojr/aureaconceptnew/internal/config"
	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/web"
)

func main() {
	if err := executar(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func executar(args []string) error {
	cfg, err := config.Carregar(os.Getenv)
	if err != nil {
		return err
	}
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck(cfg)
	}
	if len(args) > 0 {
		return fmt.Errorf("comando desconhecido %q (use: aurea | aurea healthcheck)", args[0])
	}
	return servir(cfg)
}

func novoLog(cfg *config.Config) *slog.Logger {
	var nivel slog.Level
	_ = nivel.UnmarshalText([]byte(cfg.LogNivel))
	op := &slog.HandlerOptions{Level: nivel}
	if cfg.LogJSON {
		return slog.New(slog.NewJSONHandler(os.Stdout, op))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, op))
}

func servir(cfg *config.Config) error {
	cfg.AplicarFuso()
	log := novoLog(cfg)
	slog.SetDefault(log)

	if err := os.MkdirAll(filepath.Dir(cfg.CaminhoBanco), 0o750); err != nil {
		return fmt.Errorf("criar a pasta do banco: %w", err)
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	banco, err := db.Abrir(ctx, cfg.CaminhoBanco)
	if err != nil {
		return err
	}
	defer banco.Fechar()

	criado, err := bootstrap.GarantirAdmin(ctx, banco.W, cfg.AdminSenhaInicial)
	if err != nil {
		return err
	}
	if criado {
		log.Info("usuário admin criado a partir de ADMIN_SENHA_INICIAL; a variável pode ser removida do .env")
	}

	srv, err := web.Novo(cfg, banco, log, web.Opcoes{})
	if err != nil {
		return err
	}
	servidor := &http.Server{
		Addr:              cfg.Endereco,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// limpeza periódica das sessões vencidas
	go func() {
		t := time.NewTicker(15 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := srv.Sessoes().Limpar(ctx); err != nil {
					log.Warn("limpeza de sessões", "erro", err)
				} else if n > 0 {
					log.Debug("sessões vencidas apagadas", "quantidade", n)
				}
			}
		}
	}()

	erros := make(chan error, 1)
	go func() {
		log.Info("servidor no ar", "endereco", cfg.Endereco, "ambiente", cfg.Ambiente, "banco", cfg.CaminhoBanco)
		erros <- servidor.ListenAndServe()
	}()
	select {
	case err := <-erros:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("encerrando (desligamento gracioso)")
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return servidor.Shutdown(shutdown)
	}
	return nil
}

// healthcheck pede /healthz ao servidor local (mesma porta configurada) e falha se não vier 200.
func healthcheck(cfg *config.Config) error {
	cliente := &http.Client{Timeout: 3 * time.Second}
	resp, err := cliente.Get("http://127.0.0.1:" + cfg.Porta + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz respondeu %d", resp.StatusCode)
	}
	return nil
}
