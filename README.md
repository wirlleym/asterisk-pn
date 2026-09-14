# asterisk-push-notify-mobile

Acordador de **softphone** para o **Asterisk**: um binário único (Go, sem
dependências) que o dialplan executa. Ele **checa se o app está vivo** (manda um
SIP `OPTIONS`) e, se estiver **"dormindo"** (processo morto/suspenso pelo iOS ou
Android), **envia o push** para acordá-lo — e o dialplan então disca.

- **Não cria ramais.** Funciona com os ramais que você já tem.
- **Não cria módulo.** É um binário externo chamado por uma linha no dialplan.
- **A ligação continua 100% no Asterisk** — o binário só decide "acorda ou não".

```
Chamada → Asterisk (dialplan)
   → asterisk-push-notify:
       OPTIONS pro app
         ├─ respondeu (vivo)   → não faz nada (disca direto)
         └─ não respondeu (morto) → APNs/FCM → app acorda → re-registra
   → Dial(PJSIP/...) → app atende
```

---

## Instalação rápida (`.deb`)

```bash
# 1. baixar e instalar
wget https://github.com/wirlleym/asterisk-push-notify-mobile/releases/latest/download/asterisk-push-notify-mobile_amd64.deb
sudo dpkg -i asterisk-push-notify-mobile_amd64.deb

# 2. credenciais
sudo nano /etc/asterisk-push-notify-mobile/push.env

# 3. registrar o token de cada ramal
sudo asterisk-push-notify --register <RAMAL> <TOKEN>

# 4. gancho no dialplan (no seu contexto, antes do Dial)
#    exten => _9XXX,1,System(/usr/local/bin/asterisk-push-notify "${EXTEN}" "${PJSIP_DIAL_CONTACTS(${EXTEN})}" "${CALLERID(num)}")
#     same => n,GotoIf($["${SYSTEMSTATUS}" = "SUCCESS"]?dial:wake)
#     same => n(wake),Wait(3)
#     same => n(dial),Dial(PJSIP/${EXTEN},30)
#     same => n,Hangup()
```

---

## Comandos

```
asterisk-push-notify <ramal> [contacts] [caller]         checa vivo; morto → push
asterisk-push-notify --register <ramal> <token> [provider]  cadastra o token
asterisk-push-notify --list                               lista os tokens
```

> `<contacts>` é a saída de `${PJSIP_DIAL_CONTACTS(<ramal>)}` (o dialplan passa),
> usada para o `OPTIONS`.

---

## Documentação

| Doc | Conteúdo |
|---|---|
| [`PACKAGE-README.md`](PACKAGE-README.md) | instalação e uso (para o admin) |
| [`INSTALL.md`](INSTALL.md) | instalação manual / compilando do fonte |
| [`EXPLICACAO.md`](EXPLICACAO.md) | o código explicado **linha a linha** |

## Compilar / empacotar

```bash
make build     # compila o binário
make deb       # gera o pacote .deb
make deb ARCH=arm64   # para Asterisk em ARM
```

## Licença

[MIT](LICENSE)
