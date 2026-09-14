# asterisk-push-notify-mobile

Acordador de **softphone** para o **Asterisk**: um binário único (Go, sem
dependências) que o dialplan executa para **acordar o app do celular** (push)
quando um ramal está **sem registro ativo**.

- **Não cria ramais.** Funciona com os ramais que você já tem.
- **Não cria módulo.** É um binário externo chamado por uma linha no dialplan.
- **A ligação continua 100% no Asterisk** — isto só manda o push (APNs para iOS,
  FCM para Android).

```
Chamada → Asterisk (dialplan)
   ├─ ramal COM registro  → disca direto
   └─ ramal SEM registro  → asterisk-push-notify → APNs/FCM → app acorda
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
#    exten => _9XXX,1,Set(CONTACTS=${PJSIP_DIAL_CONTACTS(${EXTEN})})
#     same => n,GotoIf($["${CONTACTS}" != ""]?ja_registrado)
#     same => n,System(/usr/local/bin/asterisk-push-notify "${EXTEN}" "${CALLERID(num)}")
#     same => n,Wait(3)
#     same => n(ja_registrado),Dial(PJSIP/${EXTEN},30)
```

---

## Comandos

```
asterisk-push-notify <ramal> [caller]                     dispara o push
asterisk-push-notify --register <ramal> <token> [provider]  cadastra o token
asterisk-push-notify --list                               lista os tokens
```

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
