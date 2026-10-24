// Command loadtest gera carga contra a UserApiV2 para achar o limite do login (Argon2) e do
// fluxo de login com passkey. Cada estágio roda N workers em laço fechado por um tempo e imprime
// latência (p50/p95/p99/max), vazão e falhas por etapa.
//
//	go run ./loadtest -scenario login -stages 5:20s,20:20s,50:20s
//	go run ./loadtest -scenario passkey -stages 10:30s,50:30s
//
// Veja loadtest/README.md para o roteiro e o que observar na API durante o teste.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/descope/virtualwebauthn"
)

type stage struct {
	workers  int
	duration time.Duration
}

type config struct {
	api           string
	scenario      string
	stages        []stage
	users         int
	password      string
	rp            virtualwebauthn.RelyingParty
	timeout       time.Duration
	setupParallel int
	cleanup       bool
}

func (c config) maxWorkers() int {
	max := 0
	for _, s := range c.stages {
		if s.workers > max {
			max = s.workers
		}
	}
	return max
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, "loadtest:", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "loadtest:", err)
		os.Exit(1)
	}
}

func parseFlags() (config, error) {
	var cfg config
	var stages, origin, rpID string

	flag.StringVar(&cfg.api, "api", "http://localhost:8080", "URL da UserApiV2")
	flag.StringVar(&cfg.scenario, "scenario", "login", "cenário: "+strings.Join(scenarioNames(), ", "))
	flag.StringVar(&stages, "stages", "5:20s,20:20s,50:20s", "estágios workers:duração separados por vírgula")
	flag.IntVar(&cfg.users, "users", 0, "contas criadas no preparo (0 = automático)")
	flag.StringVar(&cfg.password, "password", "Loadtest-senha-123", "senha das contas criadas")
	flag.StringVar(&origin, "origin", "http://localhost:3000", "origem vista pelo autenticador virtual (precisa estar em WEBAUTHN_RP_ORIGINS)")
	flag.StringVar(&rpID, "rp-id", "localhost", "WEBAUTHN_RP_ID da API")
	flag.DurationVar(&cfg.timeout, "timeout", 30*time.Second, "timeout de cada requisição")
	flag.IntVar(&cfg.setupParallel, "setup-parallel", 4, "requisições simultâneas no preparo das contas")
	flag.BoolVar(&cfg.cleanup, "cleanup", true, "apaga (soft delete) as contas criadas ao final")
	flag.Parse()

	if _, ok := scenarios[cfg.scenario]; !ok {
		return cfg, fmt.Errorf("cenário %q desconhecido; use um de: %s", cfg.scenario, strings.Join(scenarioNames(), ", "))
	}
	var err error
	if cfg.stages, err = parseStages(stages); err != nil {
		return cfg, err
	}
	if cfg.setupParallel < 1 {
		return cfg, fmt.Errorf("-setup-parallel precisa ser positivo")
	}

	sc := scenarios[cfg.scenario]
	if cfg.users == 0 {
		cfg.users = min(cfg.maxWorkers(), 10)
		if sc.exclusiveUsers {
			cfg.users = cfg.maxWorkers()
		}
	}
	// No fluxo com passkey cada conta é usada por um worker só: duas cerimônias da mesma conta ao
	// mesmo tempo se atropelam (um mfa_token e uma cerimônia por usuário) e o contador da passkey
	// sairia fora de ordem, o que a API trata como clone.
	if sc.exclusiveUsers && cfg.users < cfg.maxWorkers() {
		return cfg, fmt.Errorf("o cenário %s precisa de -users >= %d (maior número de workers)", cfg.scenario, cfg.maxWorkers())
	}

	cfg.rp = virtualwebauthn.RelyingParty{ID: rpID, Name: "UserApiV2", Origin: origin}
	return cfg, nil
}

func parseStages(s string) ([]stage, error) {
	var stages []stage
	for _, part := range strings.Split(s, ",") {
		w, d, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			return nil, fmt.Errorf("estágio %q: use workers:duração, ex.: 20:30s", part)
		}
		workers, err := strconv.Atoi(w)
		if err != nil || workers < 1 {
			return nil, fmt.Errorf("estágio %q: workers inválido", part)
		}
		duration, err := time.ParseDuration(d)
		if err != nil || duration <= 0 {
			return nil, fmt.Errorf("estágio %q: duração inválida", part)
		}
		stages = append(stages, stage{workers: workers, duration: duration})
	}
	return stages, nil
}

func scenarioNames() []string {
	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func run(ctx context.Context, cfg config) error {
	sc := scenarios[cfg.scenario]
	c := newClient(cfg.api, cfg.timeout, cfg.maxWorkers()+cfg.setupParallel)

	fmt.Printf("cenário %s: %s\n", cfg.scenario, sc.description)
	fmt.Printf("API %s, %d contas, estágios %s\n\n", cfg.api, cfg.users, describeStages(cfg.stages))

	users, err := prepare(ctx, c, cfg, sc)
	if cfg.cleanup {
		// Contexto próprio: o Ctrl+C que interrompe a carga não deve impedir a limpeza.
		defer cleanup(context.Background(), c, users)
	}
	if err != nil {
		return fmt.Errorf("preparo: %w", err)
	}

	for i, st := range cfg.stages {
		if ctx.Err() != nil {
			fmt.Println("\ninterrompido.")
			break
		}
		fmt.Printf("\n== estágio %d/%d: %d workers por %s ==\n", i+1, len(cfg.stages), st.workers, st.duration)
		rec := newRecorder()
		elapsed := runStage(ctx, c, cfg, sc, users, st, rec)
		rec.report(os.Stdout, elapsed)
	}
	return nil
}

func describeStages(stages []stage) string {
	parts := make([]string, len(stages))
	for i, s := range stages {
		parts[i] = fmt.Sprintf("%d×%s", s.workers, s.duration)
	}
	return strings.Join(parts, " → ")
}
