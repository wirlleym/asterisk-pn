# asterisk-push-notify-mobile — acordador de softphone

Este pacote instala **apenas o "acordador"**: um binário que o dialplan do
Asterisk executa para **acordar o app do celular** (push) quando um ramal está
**sem registro ativo**.

- **Não cria ramais.** Seus ramais atuais continuam como estão.
- **Não cria módulo.** É um binário externo chamado pelo dialplan.
- A **ligação continua 100% no Asterisk** — este binário só manda o push.

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
`Dial`**, a checagem de registro. Adapte o padrão (`_9XXX` etc.) ao seu caso:

```
exten => _9XXX,1,Set(CONTACTS=${PJSIP_DIAL_CONTACTS(${EXTEN})})
 same => n,GotoIf($["${CONTACTS}" != ""]?ja_registrado)
 same => n,System(/usr/local/bin/asterisk-push-notify "${EXTEN}" "${CALLERID(num)}")
 same => n,Wait(3)
 same => n(ja_registrado),Dial(PJSIP/${EXTEN},30)
 same => n,Hangup()
```

O que cada linha faz:
1. `PJSIP_DIAL_CONTACTS` devolve os registros ativos do ramal (vazio = aparelho offline).
2. Se **tem** registro → pula direto para o `Dial` (sem push).
3. Se **não tem** → executa o acordador (manda o push).
4. `Wait(3)` dá tempo do app acordar e re-registrar.
5. `Dial` disca para o ramal.

Depois de editar: `asterisk -rx "dialplan reload"`.

---

## 5. Testar

```bash
# push direto (o celular deve tocar)
sudo -u asterisk /usr/local/bin/asterisk-push-notify <RAMAL> <CALLER>

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
