# asterisk-pn — explicação linha a linha

Guia para entender **cada arquivo, cada função e cada linha** do serviço de push
(CLI Go). Feito para um desenvolvedor junior: não assume conhecimento prévio de
Go nem de push.

---

## O que é isso?

O `asterisk-pn` é um **programa de linha de comando (CLI)**. Ele não fica
rodando (não é um servidor): o **Asterisk** o chama quando precisa "acordar" o
app no celular.

O fluxo é:

```
Chamada → Asterisk (ramal frio) → executa "asterisk-pn <ramal> <caller>"
                                  → este programa:
                                      1. lê o token do ramal (arquivo JSON)
                                      2. assina um JWT com a chave .p8
                                      3. manda o push para a Apple (APNs) ou Google (FCM)
```

É um **binário único** (não precisa de Node, Python, etc.), compilado em Go.

---

## `main.go` — o ponto de entrada

### A função `main()` (linha ~22)

```go
func main() {
	cfg := loadConfig()          // lê a configuração (chave, token, tópico...)
	args := os.Args[1:]          // pega os argumentos da linha de comando
```

- `loadConfig()`: carrega as configurações do arquivo `/etc/asterisk-pn/push.env`
  (explicado no `config.go`).
- `os.Args`: é a lista de argumentos. `os.Args[0]` é o nome do programa;
  `os.Args[1:]` são os argumentos que vieram depois (ex.: `00506`, `1234`).

### O `switch args[0]` (linha ~30)

```go
	switch args[0] {
	case "--help", "-h", "help":
		usage()
	case "--list", "list":
		...
	case "--register", "register":
		...
	default:
		// modo notify
		...
	}
```

O primeiro argumento decide **o que o programa faz**:

| Comando | O que faz |
|---|---|
| `--help` | imprime o texto de ajuda |
| `--list` | mostra os tokens cadastrados |
| `--register <ramal> <token>` | cadastra (ou atualiza) o token de um ramal |
| *(nenhum)* → `default` | **modo notify**: dispara o push do ramal |

> `default` roda quando o primeiro argumento é o **ramal** (ex.: `asterisk-pn 00506`).

### Modo `notify` (linha ~48)

```go
	default:
		if err := notify(cfg, args[0], callerOf(args)); err != nil {
			fmt.Fprintln(os.Stderr, "asterisk-pn:", err)
		}
		os.Exit(0)
```

- Chama `notify(cfg, ramal, caller)`.
- **Importante**: se der erro, ele apenas imprime a mensagem e **sai com código 0**
  (sucesso). Por quê? Porque o push é um "extra": se falhar, a **chamada não pode
  ser derrubada**. O Asterisk continua o fluxo (tenta discar) mesmo sem push.
- `os.Exit(0)`: encerra o programa informando "tudo ok".

### A função `notify()` (linha ~90)

```go
func notify(cfg Config, ramal, caller string) error {
	store, err := NewStore(cfg.DataFile)      // abre o arquivo de tokens
	devices := store.FindByExtension(ramal)   // acha os aparelhos daquele ramal
	...
	for _, d := range withToken {
		switch d.Provider {
		case "apns_voip":
			delivered, reason = newAPNSSender(cfg).Send(...)   // iOS
		case "fcm":
			delivered, reason = newFCMSender(cfg).Send(...)    // Android
		}
		if delivered { ... return nil }
	}
	return fmt.Errorf("nao entregue: %s", lastReason)
}
```

Passo a passo:
1. `NewStore`: carrega o arquivo de tokens (`devices.json`).
2. `FindByExtension`: pega todos os aparelhos (devices) que registraram aquele ramal.
3. Para cada aparelho, escolhe o **provedor**:
   - `apns_voip` → Apple (iPhone).
   - `fcm` → Google/Firebase (Android).
4. Se **um** deles entregar, para e retorna sucesso.
5. Se nenhum entregar, retorna erro (mas o `main` já converte em "não derruba a chamada").

### A função `register()` (linha ~70)

```go
func register(cfg Config, args []string) error {
	ramal := strings.TrimSpace(args[1])
	token := strings.TrimSpace(args[2])
	provider := "apns_voip"
	...
	return store.Upsert(DeviceRecord{ ... })
}
```

- Pega `ramal` e `token` dos argumentos.
- `provider` padrão é `apns_voip` (se não informar, assume iPhone).
- `Upsert`: grava (ou atualiza) o registro no arquivo JSON.

### A função `newID()` (linha ~130)

```go
func newID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
```

Gera um identificador aleatório (ex.: `a1b2c3...`) usado como `callId` quando o
Asterisk não mandou um.

---

## `config.go` — como o programa se configura

### A struct `Config` (linha ~8)

```go
type Config struct {
	DataFile          string   // onde fica o arquivo de tokens
	APNSKeyPath       string   // caminho da chave .p8 da Apple
	APNSKeyID         string   // Key ID (Apple)
	APNSTeamID        string   // Team ID (Apple)
	APNSTopic         string   // "bundle-id.voip" (para qual app o push vai)
	APNSEnvironment   string   // "sandbox" ou "production"
	FCMServiceAccount string   // caminho do service account do Firebase
}
```

É o "bloco de configuração". Cada campo guarda um valor necessário.

### A função `env()` (linha ~20)

```go
func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
```

- `os.Getenv(key)`: lê uma **variável de ambiente** (ex.: `APNS_KEY_ID`).
- Se estiver vazia, usa o `fallback` (valor padrão).
- `strings.TrimSpace`: remove espaços no começo/fim.

### A função `loadEnvFile()` (linha ~28)

```go
func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") { continue }  // ignora vazio/comentário
		idx := strings.Index(line, "=")   // acha o "="
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if os.Getenv(key) == "" { _ = os.Setenv(key, val) }
	}
}
```

Lê um arquivo no formato `CHAVE=VALOR` (como o `/etc/asterisk-pn/push.env`) e
transforma cada linha em variável de ambiente.

**Por que isso existe?** Quando o Asterisk chama o programa pelo dialplan, ele
**não passa variáveis de ambiente**. Então o programa precisa ler a configuração
de um arquivo.

- `os.ReadFile`: lê o arquivo inteiro.
- `strings.Split(data, "\n")`: divide por linha.
- `strings.HasPrefix(line, "#")`: ignora linhas de comentário.
- `os.Setenv(key, val)`: define a variável (só se ainda não estiver definida,
  para a variável de ambiente ter prioridade).

### A função `loadConfig()` (linha ~60)

```go
func loadConfig() Config {
	loadEnvFile(env("PUSH_ENV_FILE", "/etc/asterisk-pn/push.env"))
	return Config{
		DataFile:        env("PUSH_DATA_FILE", "/etc/asterisk-pn/devices.json"),
		APNSKeyPath:     env("APNS_KEY_PATH", ""),
		...
	}
}
```

- Carrega o arquivo de env e devolve a struct `Config` preenchida.

---

## `store.go` — onde ficam os tokens

### A struct `DeviceRecord` (linha ~10)

```go
type DeviceRecord struct {
	Extension string `json:"extension"`
	Platform  string `json:"platform"`
	Provider  string `json:"provider"`
	Token     string `json:"token"`
	DeviceID  string `json:"deviceId"`
	UpdatedAt string `json:"updatedAt"`
}
```

É o "registro" de um aparelho. O `json:"..."` é só a **tag** que diz como gravar
esse campo no arquivo JSON.

- `Extension`: o ramal (ex.: `00506`).
- `Token`: o token de push daquele aparelho.
- `Provider`: `apns_voip` (iOS) ou `fcm` (Android).

### A struct `Store` (linha ~19)

```go
type Store struct {
	mu      sync.Mutex
	path    string
	devices map[string]DeviceRecord
}
```

- `devices`: um **mapa** (dicionário) `deviceId → DeviceRecord`.
- `mu` (mutex): um "trava" para evitar que duas execuções gravem o arquivo ao
  mesmo tempo e corrompam os dados.

### `NewStore()` (linha ~27)

```go
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, devices: map[string]DeviceRecord{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s.devices)
	}
	return s, nil
}
```

- Cria o `Store` e, se o arquivo já existir, lê o JSON e preenche o mapa.

### `persist()` (linha ~36)

```go
func (s *Store) persist() error {
	data, err := json.MarshalIndent(s.devices, "", "  ")
	...
	tmp := s.path + ".tmp"
	os.WriteFile(tmp, data, 0o600)
	return os.Rename(tmp, s.path)
}
```

Grava o mapa no arquivo. Detalhe importante:
- Escreve primeiro num arquivo **temporário** (`.tmp`) e depois **renomeia**.
- Isso evita um arquivo pela metade se o programa for interrompido no meio
  (escrita atômica).

### `Upsert()` e `FindByExtension()` (linhas ~55 e ~68)

```go
func (s *Store) Upsert(rec DeviceRecord) error {
	s.mu.Lock(); defer s.mu.Unlock()
	s.devices[rec.DeviceID] = rec
	return s.persist()
}

func (s *Store) FindByExtension(ext string) []DeviceRecord {
	s.mu.Lock(); defer s.mu.Unlock()
	for _, rec := range s.devices {
		if rec.Extension == ext { out = append(out, rec) }
	}
	return out
}
```

- `Upsert`: grava/atualiza um aparelho (usado no `--register`).
- `FindByExtension`: devolve todos os aparelhos de um ramal (usado no `notify`).
- `s.mu.Lock()` / `defer s.mu.Unlock()`: trava/libera o mutex, garantindo que
  ninguém leia enquanto outro grava.

---

## `apns.go` — como fala com a Apple (iPhone)

### O que é o APNs?

**APNs** (Apple Push Notification service) é o serviço da Apple que entrega
notificações no iPhone. Para mandar um push, você faz uma requisição **HTTP/2**
para `api.push.apple.com` com um **token de autenticação** (JWT).

### `authorization()` — assina o JWT (linha ~27)

```go
header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","kid":"` + keyID + `"}`))
claims := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"` + teamID + `","iat":` + ... + `}`))
signingInput := header + "." + claims
r, s, err := ecdsa.Sign(rand.Reader, ecKey, digest[:])
sig := make([]byte, 64)
r.FillBytes(sig[:32]); s.FillBytes(sig[32:])
```

**JWT** é um token assinado digitalmente. Ele tem 3 partes: `header.claims.assinatura`.

1. `header`: diz o algoritmo (`ES256`) e o `kid` (Key ID da chave).
2. `claims`: diz `iss` (Team ID) e `iat` (data/hora atual, para o token expirar).
3. `assinatura`: prova que o token foi gerado por quem tem a chave `.p8`.

Detalhes:
- `base64.RawURLEncoding`: codifica em base64 "url-safe" (sem `+`, `/`, `=`), que é o que o JWT exige.
- `ecdsa.Sign`: assina com a chave **ECDSA P-256** (a `.p8` é essa chave).
- `r.FillBytes(sig[:32])` + `s.FillBytes(sig[32:])`: o JWT quer a assinatura como
  `R||S` (64 bytes), mas o Go devolve `r` e `s` separados — então junta os dois.
  **Este é o passo que torna inviável fazer isso em shell puro.**

### `Send()` — envia o push (linha ~62)

```go
host := "https://api.push.apple.com"
if a.cfg.APNSEnvironment == "sandbox" { host = "https://api.sandbox.push.apple.com" }

req, _ := http.NewRequest("POST", host+"/3/device/"+device.Token, strings.NewReader(body))
req.Header.Set("authorization", "bearer "+auth)
req.Header.Set("apns-topic", a.cfg.APNSTopic)
req.Header.Set("apns-push-type", "voip")
req.Header.Set("apns-priority", "10")
```

Cada header tem um papel:

| Header | O que significa |
|---|---|
| `authorization` | o JWT que assinamos (a "senha" da requisição) |
| `apns-topic` | para qual app o push vai (`bundle-id.voip`) |
| `apns-push-type` | `voip` = é push de chamada (faz o PushKit acordar o app) |
| `apns-priority` | `10` = prioridade máxima (entrega imediata) |

- `sandbox` = servidor de testes da Apple; `production` = servidor real
  (TestFlight/App Store).

O corpo da requisição (`body`) é o payload:
```json
{"callId":"...","handle":"00506","type":"incoming_call"}
```
O app lê isso para saber quem está ligando.

---

## `fcm.go` — como fala com o Google (Android)

### O que é o FCM?

**FCM** (Firebase Cloud Messaging) é o serviço do Google para notificações no
Android. O fluxo tem **duas etapas**: (1) trocar as credenciais por um
`access_token`, (2) mandar a mensagem.

### `accessToken()` — troca a credencial por token (linha ~35)

```go
claims := `{"iss":"...","scope":"https://www.googleapis.com/auth/firebase.messaging","aud":"...","iat":...,"exp":...}`
// assina com a chave RSA do service account (RS256)
form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
form.Set("assertion", assertion)
http.PostForm(f.account.TokenURI, form)
```

- O Google usa **OAuth2**. Você assina um JWT (RS256, com a chave do service
  account) e troca por um `access_token` de curta duração.
- O token é **cacheado** por ~50 minutos (não precisa renovar a cada push).

### `Send()` — manda a mensagem (linha ~96)

```go
payload := map[string]any{
	"message": map[string]any{
		"token": device.Token,
		"android": map[string]any{"priority": "HIGH", "ttl": "30s"},
		"data": map[string]string{"type": "incoming_call", "call_id": callID, "caller": handle},
	},
}
```

Pontos importantes:
- **`data`** (e não `notification`): é uma mensagem "data-only", que o app
  processa sem mostrar nada (necessário para acordar o app e tocar via
  ConnectionService).
- **`priority: HIGH`**: prioridade alta, para o Android entregar mesmo com o
  aparelho em economia de bateria.
- **`ttl: 30s`**: se não entregar em 30 segundos, descarta (não faz sentido
  tocar por uma chamada que já acabou).

---

## `options.go` — como checa se o app está vivo

Antes de mandar o push, o binário **checa se o app está vivo** mandando um
`OPTIONS` SIP para o contato do ramal. Se responder, não precisa de push.

### `checkAlive(contacts)`

```go
for _, c := range strings.Split(contacts, "&") {
	if optionsPing(strings.TrimSpace(c)) { return true }
}
return false
```

- O dialplan passa `${PJSIP_DIAL_CONTACTS(...)}` como `contacts` (uma string com
  os contatos separados por `&`).
- Se **qualquer** contato responder → `true` (vivo). Se **nenhum** → `false` (morto).

### `optionsPing(contact)`

```go
host, port, user, transport := parseContact(contact)
msg := buildOptions(transport, host, port, user)
return pingTransport(transport, host, port, msg)
```

1. `parseContact` extrai `host`, `porta`, `usuário` e `transporte` de um contato
   `sip:user@host:porta;transport=udp|tcp|tls`.
2. `buildOptions` monta a mensagem `OPTIONS` (com o `Via` correto por transporte).
3. `pingTransport` envia:
   - `udp` → socket UDP;
   - `tcp` → socket TCP;
   - `tls` → conexão TLS (sem validar o certificado — é só uma checagem de vida).

### `pingUDP` / `pingStream`

Ambas fazem o mesmo: conectam, escrevem o `OPTIONS` e ficam lendo por até
`optionsTimeout` (2s). Se **chegar qualquer resposta** → `true` (vivo); se der
**timeout** → `false` (morto). Não importa o conteúdo da resposta (200, 401, 404
etc.) — qualquer resposta prova que o app está vivo.

### Por que isso evita o `qualify` do servidor

O `qualify` é periódico (tem janela). Aqui a checagem é **na hora da chamada**:
o próprio binário pergunta "você está aí?" e decide naquele instante.

---

## Resumo do fluxo completo

```
Asterisk: System(asterisk-pn <ramal> <contacts> <caller>)
    │
    ├─ main.go: modo notify
    ├─ options.go: manda OPTIONS pro contato
    │    ├─ respondeu → vivo → exit 0 (disca direto, SEM push)
    │    └─ não respondeu → morto
    │         ├─ store.go: acha o token do ramal
    │         ├─ apns.go / fcm.go: assina o JWT e manda o push
    │         └─ exit 1 (o dialplan faz Wait(3) e disca)
    └─ Dial(PJSIP/...) → app atende
```
