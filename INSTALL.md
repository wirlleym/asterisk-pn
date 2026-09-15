# Instalação manual — asterisk-pn (CLI Go, sem daemon)

O `push-service` aqui é um **CLI** (binário único, sem HTTP, sem systemd, sem porta).
O **Asterisk** chama o CLI direto pelo dialplan; ele só **acorda o app**. A ligação continua 100% no Asterisk.

```
Chamada → Asterisk (dialplan)
   → asterisk-pn: OPTIONS pro app
        ├─ respondeu (vivo)   → nada (disca direto)
        └─ não respondeu (morto) → APNs/FCM → app acorda → re-registra
   → Dial(PJSIP/...) → app atende
```

> Esta documentação é genérica: troque os placeholders `<...>` pelos valores da
> sua instalação.

---

## 1. Compilar o binário

Requer **Go ≥ 1.22** apenas para compilar.

```bash
cd /caminho/para/asterisk-pn
go build -o asterisk-pn .
ls -la asterisk-pn      # ~7.6 MB, binário único, sem dependências
```

Cross-compile (ex.: Asterisk em ARM):
```bash
GOOS=linux GOARCH=arm64 go build -o asterisk-pn .
```

---

## 2. Instalar o CLI + config

### 2.1 Binário
```bash
sudo install -m 0755 asterisk-pn /usr/local/bin/asterisk-pn
```

### 2.2 Chave APNs (`.p8`) e diretório
```bash
sudo install -d -m 0750 /etc/asterisk-pn
sudo install -m 0640 <CAMINHO_DA_CHAVE>.p8 /etc/asterisk-pn/AuthKey.p8
```

### 2.3 Config (`/etc/asterisk-pn/push.env`)
O CLI lê esse arquivo sozinho (o dialplan não passa ambiente).
```bash
sudo tee /etc/asterisk-pn/push.env >/dev/null <<'EOF'
PUSH_DATA_FILE=/etc/asterisk-pn/devices.json
APNS_KEY_PATH=/etc/asterisk-pn/AuthKey.p8
APNS_KEY_ID=<KEY_ID>
APNS_TEAM_ID=<TEAM_ID>
APNS_TOPIC=<BUNDLE_ID>.voip
APNS_ENVIRONMENT=production
# Android (opcional):
# FCM_SERVICE_ACCOUNT_JSON=/etc/asterisk-pn/fcm-service-account.json
EOF
```
> - `<KEY_ID>` / `<TEAM_ID>`: da chave APNs criada no portal Apple.
> - `<BUNDLE_ID>`: o bundle id do app; o tópico de VoIP termina em `.voip`.
> - `APNS_ENVIRONMENT`: `sandbox` para build de dev; `production` para TestFlight/App Store.

### 2.4 Permissões (o Asterisk roda como usuário `asterisk`)
```bash
sudo chown root:asterisk /etc/asterisk-pn/push.env /etc/asterisk-pn/AuthKey.p8
sudo chmod 640 /etc/asterisk-pn/push.env /etc/asterisk-pn/AuthKey.p8
sudo touch /etc/asterisk-pn/devices.json
sudo chown asterisk:asterisk /etc/asterisk-pn/devices.json
sudo chmod 600 /etc/asterisk-pn/devices.json
```

Teste rápido:
```bash
sudo -u asterisk /usr/local/bin/asterisk-pn --list
```

---

## 3. Registrar o token do ramal

Uma vez, com o token VoIP do app (o mesmo `APNS_TOPIC` do `push.env`):
```bash
sudo -u asterisk /usr/local/bin/asterisk-pn \
  --register <RAMAL> <TOKEN_VOIP_DO_APP>
# ok

sudo -u asterisk /usr/local/bin/asterisk-pn --list
```
> Formato: `--register <ramal> <token> [provider] [deviceId]`
> (provider default `apns_voip`; use `fcm` no Android).
>
> ⚠️ **Token e tópico têm de ser do MESMO app.** Se trocar o bundle id, troque o
> `APNS_TOPIC` junto.

---

## 4. Integrar no Asterisk (gancho no dialplan)

Este pacote **não cria ramais**. Você só adiciona um gancho no contexto que
recebe as chamadas dos **seus ramais existentes**.

No `extensions.conf`, no contexto dos seus ramais, adicione **antes do `Dial`**
(troque o padrão `_9XXX` pelo seu):

```
exten => _9XXX,1,System(/usr/local/bin/asterisk-pn "${EXTEN}" "${PJSIP_DIAL_CONTACTS(${EXTEN})}" "${CALLERID(num)}")
 same => n,GotoIf($["${SYSTEMSTATUS}" = "SUCCESS"]?dial:wake)
 same => n(wake),Wait(3)
 same => n(dial),Dial(PJSIP/${EXTEN},30)
 same => n,Hangup()
```

O que acontece:
1. O CLI manda um `OPTIONS` para o contato do ramal.
2. Se o app **respondeu** (vivo) → código `0` → `${SYSTEMSTATUS}=SUCCESS` → disca direto.
3. Se **não respondeu** (morto/suspenso) → manda o push → código `1` → `Wait(3)` → disca.
4. `Wait(3)` dá tempo do app acordar e re-registrar antes do `Dial`.

Recarregue:
```bash
sudo asterisk -rx "dialplan reload"
```

---

## 5. Testar

### 5.1 Push direto
```bash
# força o push (contato vazio = app "morto")
sudo -u asterisk /usr/local/bin/asterisk-pn <RAMAL> "" <CALLER>
# delivered=true provider=apns_voip ramal=<RAMAL>
```

### 5.2 Chamada real (app frio)
1. App registra o ramal no Asterisk e **fecha**.
2. Ligue para o ramal de outro cliente SIP.
3. Asterisk (sem Contact) → CLI → APNs → **app toca** → re-registra → atende.

Log do dialplan:
```bash
sudo tail -f /var/log/asterisk/full.log | grep -i "asterisk-pn"
```

---

## 6. Rollback
```bash
sudo rm -f /usr/local/bin/asterisk-pn
sudo rm -f /etc/asterisk-pn/push.env /etc/asterisk-pn/AuthKey.p8 /etc/asterisk-pn/devices.json
# remover o gancho que voce adicionou no extensions.conf
sudo asterisk -rx "pjsip reload" && sudo asterisk -rx "dialplan reload"
```

---

## 7. Notas de produção
- **Sem daemon**: nada rodando; o binário é executado só quando há chamada para ramal frio.
- **Registro do token**: `--register` é manual. Para registro automático pelo app, use a versão **daemon** (HTTP).
- **Segurança**: o CLI roda como `asterisk`; restrinja a leitura do `push.env`/`.p8` a `root:asterisk`.
- **APNs**: a `.p8` vale para todo o time; `APNS_TOPIC = <bundle-id>.voip`.
- **FCM**: preencha `FCM_SERVICE_ACCOUNT_JSON` para Android (mesmo CLI).
