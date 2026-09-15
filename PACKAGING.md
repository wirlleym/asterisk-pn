# Empacotamento — asterisk-pn

O mesmo binário Go (`asterisk-pn`) é distribuído de três formas. O nome
descritivo do projeto é **asterisk-push-notify** (título do README); o
comando/pacote é `asterisk-pn`.

| Formato | Distros | Arquivo/ferramenta |
|---|---|---|
| `.deb` | Debian/Ubuntu | `scripts/build-deb.sh` |
| Snap | qualquer com `snapd` | `snap/snapcraft.yaml` |
| Flatpak | qualquer com `flatpak` | `packaging/flatpak/io.github.wirlleym.asterisk-pn.yml` |

## Como o Asterisk chama (por formato)

O gancho no dialplan aponta para o binário de acordo com o formato instalado:

```
; .deb
exten => _9XXX,1,System(/usr/local/bin/asterisk-pn "${EXTEN}" "${PJSIP_DIAL_CONTACTS(${EXTEN})}" "${CALLERID(num)}")

; snap (classic -> symlink em /snap/bin)
exten => _9XXX,1,System(/snap/bin/asterisk-pn "${EXTEN}" "${PJSIP_DIAL_CONTACTS(${EXTEN})}" "${CALLERID(num)}")

; flatpak
exten => _9XXX,1,System(flatpak run --command=asterisk-pn io.github.wirlleym.asterisk-pn "${EXTEN}" "${PJSIP_DIAL_CONTACTS(${EXTEN})}" "${CALLERID(num)}")
```

## 1. .deb (já pronto)

```bash
make deb                 # -> asterisk-pn_<ver>_amd64.deb
make deb ARCH=arm64      # Asterisk em ARM
```

## 2. Snap

O snap usa `confinement: classic` (acesso total ao filesystem), então o binário
continua lendo `/etc/asterisk-pn/push.env` e escrevendo
`/etc/asterisk-pn/devices.json` como o `.deb`.

```bash
# 1. instalar o snapcraft (uma vez)
sudo snap install snapcraft --classic

# 2. buildar
snapcraft            # -> asterisk-pn_<ver>_amd64.snap

# 3. instalar local
sudo snap install --dangerous asterisk-pn_<ver>_amd64.snap

# 4. publicar na Snap Store (requer conta e registro do nome)
snapcraft login
snapcraft register asterisk-pn
snapcraft upload --release=stable asterisk-pn_<ver>_amd64.snap
```

Nota: publicar um snap `classic` na Snap Store passa por **revisão manual**
(precisa justificar o `classic`).

## 3. Flatpak

```bash
# 1. instalar flatpak-builder + SDK (uma vez)
sudo apt install flatpak-builder
flatpak install org.freedesktop.Sdk//23.08 org.freedesktop.Sdk.Extension.golang//23.08

# 2. buildar
flatpak-builder build-dir packaging/flatpak/io.github.wirlleym.asterisk-pn.yml \
  --force-clean --install --user

# 3. rodar
flatpak run --command=asterisk-pn io.github.wirlleym.asterisk-pn --help

# 4. publicar no Flathub (requer conta + manifest + metainfo/appdata)
flatpak build-bundle repo asterisk-pn.flatpak io.github.wirlleym.asterisk-pn
```

O Flatpak usa `--filesystem=host`, então também enxerga `/etc/asterisk-pn/`.
Para publicar no Flathub é preciso um arquivo metainfo
(`io.github.wirlleym.asterisk-pn.metainfo.xml`) e um `repo` via
`flatpak-builder --repo`.

## Caminhos (todos os formatos)

| O que | Onde |
|---|---|
| binário | `/usr/local/bin/asterisk-pn` (deb) · `/snap/bin/asterisk-pn` (snap) · `flatpak run ...` |
| config | `/etc/asterisk-pn/push.env` |
| tokens | `/etc/asterisk-pn/devices.json` |
| doc | `/usr/share/doc/asterisk-pn/README.md` (deb) |

> Override via env (se precisar de outro caminho):
> `PUSH_ENV_FILE` (config) e `PUSH_DATA_FILE` (tokens).
