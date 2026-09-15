# asterisk-push-notify-mobile — acordador de softphone

Este pacote instala **apenas o "acordador"**: um binário que o dialplan do
Asterisk executa. Ele **checa se o app está vivo** (manda um SIP `OPTIONS`) e,
se estiver **"dormindo"** (processo morto/suspenso), **envia o push** para
acordá-lo — o dialplan então disca.

- **Não cria ramais.** Seus ramais atuais continuam como estão.
- **Não cria módulo.** É um binário externo chamado pelo dialplan.
- **Não depende de `qualify` do servidor** — o próprio binário faz o `OPTIONS`.
- A **ligação continua 100% no Asterisk** — o binário só decide "acorda ou não".

---

## 1. O que foi instalado

| Caminho | O que é |
|---|---|
| `/usr/local/bin/asterisk-push-notify` | o binário (o acordador) |
| `/etc/asterisk-push-notify-mobile/push.env` | configuração (credenciais) — **você edita isto** |
| `/etc/asterisk-push-notify-mobile/push.env.example` | modelo comentado |
| `/etc/asterisk-push-notify-mobile/devices.json` | onde ficam os tokens `ramal ↔ token` (criado no 1º uso) |

---

## 2. Configurar as credenciais

```bash
sudo nano /etc/asterisk-push-notify-mobile/push.env
```

Preencha (troque os `<...>`):

```
PUSH_DATA_FILE=/etc/asterisk-push-notify-mobile/devices.json
APNS_KEY_PATH=/etc/asterisk-push-notify-mobile/AuthKey.p8      # caminho da chave .p8
APNS_KEY_ID=<KEY_ID>
APNS_TEAM_ID=<TEAM_ID>
APNS_TOPIC=<BUNDLE_ID>.voip
APNS_ENVIRONMENT=production                    # sandbox p/ dev, production p/ TestFlight
# FCM_SERVICE_ACCOUNT_JSON=/etc/asterisk-push-notify-mobile/fcm-service-account.json   (Android)
```

A chave `.p8` você coloca em `/etc/asterisk-push-notify-mobile/AuthKey.p8` (permissão 640,
grupo do usuário do Asterisk, para o dialplan conseguir ler).

---

## 3. Registrar o token de cada ramal

Uma vez por aparelho (depois que o app gerar o token de push):

```bash
sudo -u asterisk /usr/local/bin/asterisk-push-notify --register <RAMAL> <TOKEN>
# Android (FCM):   ... --register <RAMAL> <TOKEN> fcm
```

Listar:
```bash
sudo -u asterisk /usr/local/bin/asterisk-push-notify --list
```

---

## 4. Gancho no dialplan (no SEU contexto)

No contexto que recebe as chamadas dos **seus ramais**, adicione, **antes do
`Dial`**. Adapte o padrão (`_9XXX` etc.) ao seu caso:

```
exten => _9XXX,1,System(/usr/local/bin/asterisk-push-notify "${EXTEN}" "${PJSIP_DIAL_CONTACTS(${EXTEN})}" "${CALLERID(num)}")
 same => n,GotoIf($["${SYSTEMSTATUS}" = "SUCCESS"]?dial:wake)
 same => n(wake),Wait(3)
 same => n(dial),Dial(PJSIP/${EXTEN},30)
 same => n,Hangup()
```

O que acontece:
1. O binário manda um `OPTIONS` para o contato do ramal.
2. Se o app **respondeu** (vivo) → sai com código `0` → `${SYSTEMSTATUS}=SUCCESS` → disca direto (sem push).
3. Se o app **não respondeu** (morto/suspenso) → manda o push → sai com código `1` → `${SYSTEMSTATUS}=FAILURE` → `Wait(3)` → disca.
4. `Wait(3)` dá tempo do app acordar e re-registrar antes do `Dial`.

> Sem depender de `qualify` no servidor: o próprio binário faz a checagem `OPTIONS`.

### 4.1 Plataforma no Contact (opcional)

O binário **pula o push** se algum contato estiver marcado como **"sempre online"**
(`desktop`/`web`). Para isso, o app deve mandar o parâmetro no registro:

```
Contact: <sip:user@host>;platform=ios        (ou android / desktop / web)
```

- `platform=desktop` ou `platform=web` → o binário sai com `0` **sem push** (o
  aparelho está sempre online e vai atender).
- `platform=ios` / `platform=android` (ou sem `platform`) → faz o `OPTIONS` e,
  se morto, manda o push.

No Linphone, isso é setado com:
```c
linphone_proxy_config_set_contact_uri_parameters(proxy, "platform=ios");
```

Depois de editar: `asterisk -rx "dialplan reload"`.

---

## 5. Testar

```bash
# força o push (contato vazio = app "morto")
sudo -u asterisk /usr/local/bin/asterisk-push-notify <RAMAL> "" <CALLER>

# log do Asterisk
tail -f /var/log/asterisk/full.log | grep -i "asterisk-push-notify"
```

---

## 6. Remover

```bash
sudo dpkg -r asterisk-push-notify-mobile
```

Sua configuração (`/etc/asterisk-push-notify-mobile/push.env`) e os tokens (`devices.json`) são
preservados.

---

## Documentação técnica

Para entender o código **linha a linha**, veja o `EXPLICACAO.md` do projeto
(pasta `asterisk-push-notify-mobile`).
