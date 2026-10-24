# loadtest

Gerador de carga da UserApiV2. Cada estágio roda N workers em laço fechado (cada worker manda a
próxima requisição assim que a anterior volta) e imprime, por etapa, total, falhas agrupadas por
motivo, vazão e latência p50/p95/p99/max.

```powershell
go run ./loadtest -scenario login -stages 5:20s,20:20s,50:20s
```

## Cenários

| cenário | o que mede |
|---|---|
| `login` | `/auth/login` com a senha certa: custo do Argon2 `Verify` |
| `login-wrong` | senha errada: o que um ataque de força bruta custa à API |
| `login-unknown` | e-mail inexistente: deveria custar o mesmo que `login-wrong` (login em tempo constante) |
| `passkey` | login → `webauthn/begin` → assinatura (autenticador virtual) → `webauthn/finish` |

No preparo, o gerador cria contas `loadtest-<data>-<n>@example.com`. No cenário `passkey`, cada
conta também recebe uma passkey. Nesse cenário cada conta é usada por um único worker, porque duas
cerimônias da mesma conta ao mesmo tempo se atropelam. Por isso ele exige `-users` maior ou igual
ao maior número de workers. O padrão já é esse.

## Flags

| flag | padrão | |
|---|---|---|
| `-api` | `http://localhost:8080` | |
| `-scenario` | `login` | |
| `-stages` | `5:20s,20:20s,50:20s` | `workers:duração`, separados por vírgula |
| `-users` | automático | contas criadas no preparo |
| `-origin` / `-rp-id` | `http://localhost:3000` / `localhost` | precisam bater com `WEBAUTHN_RP_ORIGINS` / `WEBAUTHN_RP_ID` |
| `-timeout` | `30s` | timeout do cliente por requisição |
| `-setup-parallel` | `4` | requisições simultâneas no preparo |
| `-cleanup` | `true` | soft delete das contas ao final (`DELETE /v1/user/:id`) |

## Para o resultado valer

- **Use os parâmetros de Argon2 de produção.** Diminuir `ARGON2_MEMORY` deixa o login barato e o
  teste deixa de medir o que importa.
- **Rode a API isolada da carga:** em outra máquina, ou num container com `--cpus`/`--memory`.
  Gerador e API disputando a mesma CPU distorcem os números.
- **Não use o banco de desenvolvimento.** As contas criadas ficam no banco: o `-cleanup` faz só
  soft delete.
- **Acompanhe a API durante o teste**, porque o gerador só enxerga o lado do cliente:
  - memória do processo (`Get-Process`, `docker stats`): é aqui que o Argon2 aparece;
  - CPU;
  - logs com `latency` próxima de 15 s (o `WriteTimeout` do servidor).

Falhas classificadas como `conexão derrubada pelo servidor` geralmente significam que a resposta
passou do `WriteTimeout`: o servidor terminou o trabalho, mas não conseguiu mais escrever a resposta.

## Apagar as contas de teste

```sql
DELETE FROM users WHERE email LIKE 'loadtest-%@example.com';  -- passkeys vão junto (ON DELETE CASCADE)
```

O Redis limpa sozinho: `mfa_token`, cerimônia e sessão têm TTL.
