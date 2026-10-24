package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/descope/virtualwebauthn"
	"github.com/google/uuid"
)

type scenario struct {
	description    string
	needsPasskey   bool // o preparo cadastra uma passkey em cada conta
	exclusiveUsers bool // cada conta é usada por um worker só
	run            func(ctx context.Context, c *client, cfg config, rec *recorder, u *user)
}

var scenarios = map[string]scenario{
	"login": {
		description: "só /auth/login com a senha certa (custo do Argon2.Verify)",
		run: func(ctx context.Context, c *client, cfg config, rec *recorder, u *user) {
			rec.add("login", http.StatusOK, c.login(ctx, u.email, cfg.password))
		},
	},
	"login-wrong": {
		description: "/auth/login com senha errada: o que um ataque de força bruta custa à API",
		run: func(ctx context.Context, c *client, cfg config, rec *recorder, u *user) {
			rec.add("login (senha errada)", http.StatusUnauthorized, c.login(ctx, u.email, "senha-errada-123"))
		},
	},
	"login-unknown": {
		description: "/auth/login com e-mail inexistente: deveria custar o mesmo que a senha errada",
		run: func(ctx context.Context, c *client, cfg config, rec *recorder, u *user) {
			email := "loadtest-naoexiste-" + uuid.NewString() + "@example.com"
			rec.add("login (e-mail inexistente)", http.StatusUnauthorized, c.login(ctx, email, cfg.password))
		},
	},
	"passkey": {
		description:    "login com senha → webauthn/begin → assinatura → webauthn/finish",
		needsPasskey:   true,
		exclusiveUsers: true,
		run:            passkeyLogin,
	},
}

func passkeyLogin(ctx context.Context, c *client, cfg config, rec *recorder, u *user) {
	start := time.Now()

	res := c.login(ctx, u.email, cfg.password)
	if !rec.add("1. login", http.StatusOK, res) {
		return
	}
	mfaToken, err := mfaTokenOf(res.body)
	if err != nil {
		rec.add("1. login", http.StatusOK, result{err: err})
		return
	}

	res = c.post(ctx, "/v1/auth/webauthn/begin", mfaToken, nil)
	if !rec.add("2. webauthn/begin", http.StatusOK, res) {
		return
	}
	answer, err := u.device.assert(cfg.rp, res.body)
	if err != nil {
		rec.add("3. webauthn/finish", http.StatusOK, result{err: fmt.Errorf("assinando: %w", err)})
		return
	}

	res = c.post(ctx, "/v1/auth/webauthn/finish", mfaToken, answer)
	if !rec.add("3. webauthn/finish", http.StatusOK, res) {
		return
	}
	rec.addDuration("fluxo completo", time.Since(start))
}

func mfaTokenOf(body []byte) (string, error) {
	var lr struct {
		MfaToken string `json:"mfa_token"`
	}
	if err := json.Unmarshal(body, &lr); err != nil || lr.MfaToken == "" {
		return "", errors.New("resposta do login sem mfa_token")
	}
	return lr.MfaToken, nil
}

// --- contas de teste ---

type user struct {
	id     uuid.UUID
	email  string
	device *device
}

// device é o autenticador virtual da conta. O contador avança a cada login, como num aparelho real.
type device struct {
	auth virtualwebauthn.Authenticator
	cred virtualwebauthn.Credential
}

func (d *device) attest(rp virtualwebauthn.RelyingParty, options []byte) ([]byte, error) {
	opts, err := virtualwebauthn.ParseAttestationOptions(string(options))
	if err != nil {
		return nil, err
	}
	return []byte(virtualwebauthn.CreateAttestationResponse(rp, d.auth, d.cred, *opts)), nil
}

func (d *device) assert(rp virtualwebauthn.RelyingParty, options []byte) ([]byte, error) {
	opts, err := virtualwebauthn.ParseAssertionOptions(string(options))
	if err != nil {
		return nil, err
	}
	d.cred.Counter++
	return []byte(virtualwebauthn.CreateAssertionResponse(rp, d.auth, d.cred, *opts)), nil
}

// prepare cria as contas (e as passkeys, se o cenário pedir). Devolve as contas criadas mesmo
// em caso de erro, para a limpeza alcançar todas.
func prepare(ctx context.Context, c *client, cfg config, sc scenario) ([]*user, error) {
	runID := time.Now().Format("20060102-150405")
	users := make([]*user, cfg.users)
	errs := make([]error, cfg.users)
	var done atomic.Int32

	sem := make(chan struct{}, cfg.setupParallel)
	var wg sync.WaitGroup
	for i := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			email := fmt.Sprintf("loadtest-%s-%04d@example.com", runID, i)
			users[i], errs[i] = prepareUser(ctx, c, cfg, sc, email)
			fmt.Printf("\rpreparando contas: %d/%d", done.Add(1), cfg.users)
		}()
	}
	wg.Wait()
	fmt.Println()

	created := make([]*user, 0, len(users))
	for _, u := range users {
		if u != nil {
			created = append(created, u)
		}
	}
	return created, errors.Join(errs...)
}

func prepareUser(ctx context.Context, c *client, cfg config, sc scenario, email string) (*user, error) {
	res := c.post(ctx, "/v1/user/", "", map[string]string{"name": "Load Test", "email": email, "password": cfg.password})
	if err := expect(res, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("criando %s: %w", email, err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.body, &created); err != nil {
		return nil, fmt.Errorf("criando %s: %w", email, err)
	}
	id, err := uuid.Parse(created.ID)
	if err != nil {
		return nil, fmt.Errorf("criando %s: %w", email, err)
	}
	u := &user{id: id, email: email}
	if !sc.needsPasskey {
		return u, nil
	}

	u.device = &device{
		auth: virtualwebauthn.NewAuthenticatorWithOptions(virtualwebauthn.AuthenticatorOptions{UserHandle: id[:]}),
		cred: virtualwebauthn.NewCredential(virtualwebauthn.KeyTypeEC2),
	}
	res = c.login(ctx, email, cfg.password)
	if err := expect(res, http.StatusOK); err != nil {
		return u, fmt.Errorf("login de %s: %w", email, err)
	}
	mfaToken, err := mfaTokenOf(res.body)
	if err != nil {
		return u, err
	}
	res = c.post(ctx, "/v1/auth/webauthn/register/begin", mfaToken, nil)
	if err := expect(res, http.StatusOK); err != nil {
		return u, fmt.Errorf("register/begin de %s: %w", email, err)
	}
	answer, err := u.device.attest(cfg.rp, res.body)
	if err != nil {
		return u, fmt.Errorf("attestation de %s: %w", email, err)
	}
	res = c.post(ctx, "/v1/auth/webauthn/register/finish", mfaToken, answer)
	if err := expect(res, http.StatusOK); err != nil {
		return u, fmt.Errorf("register/finish de %s (confira -origin e -rp-id): %w", email, err)
	}
	u.device.auth.AddCredential(u.device.cred)
	return u, nil
}

func expect(res result, status int) error {
	if res.err != nil {
		return res.err
	}
	if res.status != status {
		return fmt.Errorf("esperava %d, veio %d: %s", status, res.status, res.body)
	}
	return nil
}

// cleanup faz soft delete das contas pela API. As linhas continuam no banco; o README mostra o
// SQL para apagá-las de vez.
func cleanup(ctx context.Context, c *client, users []*user) {
	failed := 0
	for _, u := range users {
		if expect(c.do(ctx, http.MethodDelete, "/v1/user/"+u.id.String(), "", nil), http.StatusOK) != nil {
			failed++
		}
	}
	fmt.Printf("\nlimpeza: %d contas desativadas, %d falhas\n", len(users)-failed, failed)
}

// --- execução de um estágio ---

func runStage(ctx context.Context, c *client, cfg config, sc scenario, users []*user, st stage, rec *recorder) time.Duration {
	stageCtx, cancel := context.WithTimeout(ctx, st.duration)
	defer cancel()

	start := time.Now()
	stopProgress := rec.progress(start)
	defer stopProgress()

	var wg sync.WaitGroup
	for w := range st.workers {
		mine := assign(users, w, st.workers)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// O fim do estágio só impede a próxima iteração; a que está em andamento termina com o
			// contexto raiz, para não contar como falha um corte feito pelo próprio teste.
			for i := 0; stageCtx.Err() == nil; i++ {
				sc.run(ctx, c, cfg, rec, mine[i%len(mine)])
			}
		}()
	}
	wg.Wait()
	return time.Since(start)
}

// assign divide as contas entre os workers sem repetir conta entre eles. Com menos contas que
// workers (só nos cenários de login com senha), as contas são compartilhadas.
func assign(users []*user, worker, workers int) []*user {
	if len(users) < workers {
		return []*user{users[worker%len(users)]}
	}
	var mine []*user
	for i := worker; i < len(users); i += workers {
		mine = append(mine, users[i])
	}
	return mine
}
