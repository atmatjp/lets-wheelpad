## How to run

### 1. Building binary

```bash
git clone https://github.com/atmatjp/lets-wheelpad
cd lets-wheelpad
go build -mod=vendor -o wheelpad main.go
```

### 2. Registration to systemd

```bash
sudo cp wheelpad /usr/local/bin/wheelpad
sudo mkdir -p /etc/wheelpad
sudo cp -n config.toml /etc/wheelpad/config.toml
sudo vim /etc/systemd/system/wheelpad.service
```

```ini
[Unit]
Description=Let's Note Wheelpad
After=multi-user.target

[Service]
ExecStart=/usr/local/bin/wheelpad
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
```

### 3. Let's wheelpad !

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now wheelpad.service
```