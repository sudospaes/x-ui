# X-UI

![](https://img.shields.io/github/v/release/sudospaes/x-ui?style=flat-square)
![Downloads](https://img.shields.io/github/downloads/sudospaes/x-ui/total?color=367cc0&style=flat-square)
![GO Version](https://img.shields.io/github/go-mod/go-version/sudospaes/x-ui.svg?style=flat-square)
![License](https://img.shields.io/github/license/sudospaes/x-ui?color=1C7947&style=flat-square)

> **Disclaimer: This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment**

Xray web panel supporting multi-protocol and multi-lang (English, Farsi, Chinese, Russian)
<br>
<br>
It's a fork from [alirezaahmadi's x-ui](https://github.com/alireza0/x-ui).

| Features                             |      Enable?       |
| ------------------------------------ | :----------------: |
| Multi-lang                           | :heavy_check_mark: |
| Dark/Light Theme                     | :heavy_check_mark: |
| Search in deep                       | :heavy_check_mark: |
| Inbound Multi User                   | :heavy_check_mark: |
| Multi User Traffic & Expiration time | :heavy_check_mark: |
| REST API                             | :heavy_check_mark: |
| Telegram BOT (admin + clients)       | :heavy_check_mark: |
| Backup database using Telegram BOT   | :heavy_check_mark: |
| Subscription link + userInfo         | :heavy_check_mark: |
| Calculate expire date on first usage | :heavy_check_mark: |

## Supported operating systems 💻
-   Debian 11 or higher
-   Ubuntu 20.04 or higher
-   Fedora 32 or higher
-   CentOS 9 Stream or higher
-   AlmaLinux 9.1 or higher
-   Rocky Linux 9 or higher

## Single Command Install ✨

    bash <(curl -Ls https://raw.githubusercontent.com/sudospaes/x-ui/master/install.sh)
    
> If you have already installed this panel and want to update to the latest version, read here : [How to update](https://github.com/sudospaes/x-ui#how-to-update)

    
## Manual install ⚙️
1.  First update your system and run the following commands. (Must have root user permissions)

   ```sh
sudo su
cd
```

2.  Then download the latest compressed package from  [https://github.com/sudospaes/x-ui/releases/latest](https://github.com/sudospaes/x-ui/releases/latest), generally choose  `amd64`  architecture

2.  Run the following commands respectively:

> If your server cpu architecture is not  `amd64`, replace  `*`  in the command with another architecture

  ```sh
rm x-ui/ /usr/local/x-ui/ /usr/bin/x-ui -rf
tar zxvf x-ui-linux-amd64.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-* x-ui/x-ui.sh
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
mv x-ui/ /usr/local/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
x-ui
```

## How to Update
1.  Run this command.
  ```sh
x-ui
```
2.  After that will show x-ui script menu and enter the number 2
<br><br>
![](https://github.com/sudospaes/x-ui-old/raw/main/media/how_to_update/Screenshot%202023-04-06%20201330.png)
<br><br>
3. Then a message will be shown to confirm the update. enter y
<br><br>
![](https://github.com/sudospaes/x-ui-old/raw/main/media/how_to_update/Screenshot%202023-04-06%20201739.png)
<br><br>
4. Wait until the successful update message is displayed
<br><br>
![](https://github.com/sudospaes/x-ui-old/raw/main/media/how_to_update/Screenshot%202023-04-06%20201811.png)

## API routes

- `/login` with `PUSH` user data: `{username: '', password: ''}` for login
- `/xui/API/inbounds` base for following actions:

| Method | Path                            | Action                                    |
| :----: | ------------------------------- | ----------------------------------------- |
| `GET`  | `"/"`                           | Get all inbounds                          |
| `GET`  | `"/get/:id"`                    | Get inbound with inbound.id               |
| `GET`  | `"/createbackup"`               | Telegram bot sends backup to admins       |
| `POST` | `"/add"`                        | Add inbound                               |
| `POST` | `"/del/:id"`                    | Delete Inbound                            |
| `POST` | `"/update/:id"`                 | Update Inbound                            |
| `POST` | `"/addClient/"`                 | Add Client to inbound                     |
| `POST` | `"/:id/delClient/:clientId"`    | Delete Client by clientId\*               |
| `POST` | `"/updateClient/:clientId"`     | Update Client by clientId\*               |
| `POST` | `"/getClientTraffics/:email"`   | Get Client's Traffic                      |
| `POST` | `"/resetAllTraffics"`           | Reset traffics of all inbounds            |
| `POST` | `"/resetAllClientTraffics/:id"` | Reset inbound clients traffics (-1: all)  |
| `POST` | `"/delDepletedClients/:id"`     | Delete inbound depleted clients (-1: all) |

\*- The field `clientId` should be filled by:

- `client.id` for VMESS and VLESS
- `client.password` for TROJAN
- `client.email` for Shadowsocks

## Environment Variables

| Variable       |                      Type                      | Default       |
| -------------- | :--------------------------------------------: | :------------ |
| XUI_LOG_LEVEL  | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | `"info"`      |
| XUI_DEBUG      |                   `boolean`                    | `false`       |
| XUI_BIN_FOLDER |                    `string`                    | `"bin"`       |
| XUI_DB_FOLDER  |                    `string`                    | `"/etc/x-ui"` |

## Screenshot

## Tg robot use

<details>
  <summary>Click for details</summary>

X-UI supports daily traffic notification, panel login reminder and other functions through the Tg robot. To use the Tg robot, you need to apply for the specific application tutorial. You can refer to the [blog](https://coderfan.net/how-to-use-telegram-bot-to-alarm-you-when-someone-login-into-your-vps.html)
Set the robot-related parameters in the panel background, including:

- Tg robot Token
- Tg robot ChatId
- Tg robot cycle runtime, in crontab syntax
- Tg robot Expiration threshold
- Tg robot Traffic threshold
- Tg robot Enable send backup in cycle runtime
- Tg robot Enable CPU usage alarm threshold

Reference syntax:

- 30 \* \* \* \* \* //Notify at the 30s of each point
- 0 \*/10 \* \* \* \* //Notify at the first second of each 10 minutes
- @hourly // hourly notification
- @daily // Daily notification (00:00 in the morning)
- @every 8h // notify every 8 hours

### Telegram Bot Features

- Report periodic
- Login notification
- CPU threshold notification
- Threshold for Expiration time and Traffic to report in advance
- Support client report menu if client's telegram username added to the user's configurations
- Support telegram traffic report searched with UUID (VMESS/VLESS) or Password (TROJAN) - anonymously
- Menu based bot
- Search client by email ( only admin )
- Check all inbounds
- Check server status
- Check depleted users
- Receive backup by request and in periodic reports
- Multi language bot
</details>

## Special thanks to

- [HexaSoftwareTech](https://github.com/HexaSoftwareTech/)
- [MHSanaei](https://github.com/MHSanaei)
- [AlirezaAhmadi](https://github.com/alireza0)
