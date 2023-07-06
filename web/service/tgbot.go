package service

import (
	"embed"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"x-ui/config"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/locale"
	"x-ui/xray"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var bot *tgbotapi.BotAPI
var adminIds []int64
var isRunning bool
var hostname string

type LoginStatus byte

const (
	LoginSuccess LoginStatus = 1
	LoginFail    LoginStatus = 0
)

type Tgbot struct {
	inboundService InboundService
	settingService SettingService
	serverService  ServerService
	lastStatus     *Status
}

func (t *Tgbot) NewTgbot() *Tgbot {
	return new(Tgbot)
}

func (t *Tgbot) I18nBot(name string, params ...string) string {
	return locale.I18n(locale.Bot, name, params...)
}

func (t *Tgbot) Start(i18nFS embed.FS) error {
	err := locale.InitLocalizer(i18nFS, &t.settingService)
	if err != nil {
		return err
	}

	t.SetHostname()
	tgBottoken, err := t.settingService.GetTgBotToken()
	if err != nil || tgBottoken == "" {
		logger.Warning("Get TgBotToken failed:", err)
		return err
	}

	tgBotid, err := t.settingService.GetTgBotChatId()
	if err != nil {
		logger.Warning("Get GetTgBotChatId failed:", err)
		return err
	}

	if tgBotid != "" {
		for _, adminId := range strings.Split(tgBotid, ",") {
			id, err := strconv.Atoi(adminId)
			if err != nil {
				logger.Warning("Failed to get IDs from GetTgBotChatId:", err)
				return err
			}
			adminIds = append(adminIds, int64(id))
		}
	}

	bot, err = tgbotapi.NewBotAPI(tgBottoken)
	if err != nil {
		fmt.Println("Get tgbot's api error:", err)
		return err
	}
	bot.Debug = false

	// listen for TG bot income messages
	if !isRunning {
		logger.Info("Starting Telegram receiver ...")
		go t.OnReceive()
		isRunning = true
	}

	return nil
}

func (t *Tgbot) IsRunning() bool {
	return isRunning
}

func (t *Tgbot) SetHostname() {
	host, err := os.Hostname()
	if err != nil {
		logger.Error("get hostname error:", err)
		hostname = ""
		return
	}
	hostname = host
}

func (t *Tgbot) Stop() {
	bot.StopReceivingUpdates()
	logger.Info("Stop Telegram receiver ...")
	isRunning = false
	adminIds = nil
}

func (t *Tgbot) OnReceive() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		tgId := update.FromChat().ID
		chatId := update.FromChat().ChatConfig().ChatID
		isAdmin := checkAdmin(tgId)
		if update.Message == nil {
			if update.CallbackQuery != nil {
				t.asnwerCallback(update.CallbackQuery, isAdmin)
			}
		} else {
			if update.Message.IsCommand() {
				t.answerCommand(update.Message, chatId, isAdmin)
			}
		}
	}
}

func (t *Tgbot) answerCommand(message *tgbotapi.Message, chatId int64, isAdmin bool) {
	msg, onlyMessage := "", false

	command, commandArgs := message.Command(), message.CommandArguments()

	// Extract the command from the Message.
	switch command {
	case "help":
		msg += t.I18nBot("tgbot.commands.help")
		msg += t.I18nBot("tgbot.commands.pleaseChoose")
	case "start":
		msg += t.I18nBot("tgbot.commands.start", "Firstname=="+message.From.FirstName)
		if isAdmin {
			msg += t.I18nBot("tgbot.commands.welcome", "Hostname=="+hostname)
		}
		msg += "\n\n" + t.I18nBot("tgbot.commands.pleaseChoose")
	case "status":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.status")
	case "id":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.getID", "ID=="+strconv.FormatInt(message.From.ID, 10))
	case "usage":
		onlyMessage = true
		if len(commandArgs) > 1 {
			if isAdmin {
				t.searchClient(chatId, commandArgs)
			} else {
				t.searchForClient(chatId, commandArgs)
			}
		} else {
			msg += t.I18nBot("tgbot.commands.usage")
		}
	case "inbound":
		onlyMessage = true
		if isAdmin {
			t.searchInbound(chatId, commandArgs)
		} else {
			msg += t.I18nBot("tgbot.commands.unknown")
		}
	default:
		msg += t.I18nBot("tgbot.commands.unknown")
	}

	if onlyMessage {
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	t.SendAnswer(chatId, msg, isAdmin)
}

func (t *Tgbot) asnwerCallback(callbackQuery *tgbotapi.CallbackQuery, isAdmin bool) {
	// Respond to the callback query, telling Telegram to show the user
	// a message with the data received.
	callback := tgbotapi.NewCallback(callbackQuery.ID, callbackQuery.Data)
	if _, err := bot.Request(callback); err != nil {
		logger.Warning(err)
	}

	switch callbackQuery.Data {
	case "get_usage":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.getServerUsage())
	case "inbounds":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.getInboundUsages())
	case "deplete_soon":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.getExhausted())
	case "get_backup":
		t.sendBackup(callbackQuery.From.ID)
	case "client_traffic":
		t.getClientUsage(callbackQuery.From.ID, callbackQuery.From.UserName)
	case "client_commands":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.I18nBot("tgbot.commands.helpClientCommands"))
	case "commands":
		t.SendMsgToTgbot(callbackQuery.From.ID, t.I18nBot("tgbot.commands.helpAdminCommands"))
	}
}

func checkAdmin(tgId int64) bool {
	for _, adminId := range adminIds {
		if adminId == tgId {
			return true
		}
	}
	return false
}

func (t *Tgbot) SendAnswer(chatId int64, msg string, isAdmin bool) {
	numericKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.serverUsage"), "get_usage"),
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.dbBackup"), "get_backup"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.getInbounds"), "inbounds"),
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.depleteSoon"), "deplete_soon"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.commands"), "commands"),
		),
	)
	numericKeyboardClient := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.clientUsage"), "client_traffic"),
			tgbotapi.NewInlineKeyboardButtonData(t.I18nBot("tgbot.buttons.commands"), "client_commands"),
		),
	)

	var keyboardMarkup tgbotapi.InlineKeyboardMarkup
	if isAdmin {
		keyboardMarkup = numericKeyboard
	} else {
		keyboardMarkup = numericKeyboardClient
	}
	t.SendMsgToTgbot(chatId, msg, keyboardMarkup)
}

func (t *Tgbot) SendMsgToTgbot(tgid int64, msg string, replyMarkup ...tgbotapi.InlineKeyboardMarkup) {
	if !isRunning {
		return
	}

	if msg == "" {
		logger.Info("[tgbot] message is empty!")
		return
	}

	var allMessages []string
	limit := 2000

	// paging message if it is big
	if len(msg) > limit {
		messages := strings.Split(msg, "\r\n \r\n")
		lastIndex := -1

		for _, message := range messages {
			if (len(allMessages) == 0) || (len(allMessages[lastIndex])+len(message) > limit) {
				allMessages = append(allMessages, message)
				lastIndex++
			} else {
				allMessages[lastIndex] += "\r\n \r\n" + message
			}
		}
	} else {
		allMessages = append(allMessages, msg)
	}
	for _, message := range allMessages {
		info := tgbotapi.NewMessage(tgid, message)
		info.ParseMode = "HTML"
		if len(replyMarkup) > 0 {
			info.ReplyMarkup = replyMarkup[0]
		}
		_, err := bot.Send(info)
		if err != nil {
			logger.Warning("Error sending telegram message :", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (t *Tgbot) SendMsgToTgbotAdmins(msg string) {
	for _, adminId := range adminIds {
		t.SendMsgToTgbot(adminId, msg)
	}
}

func (t *Tgbot) SendReport() {
	runTime, err := t.settingService.GetTgbotRuntime()
	if err == nil && len(runTime) > 0 {
		msg := ""
		msg += t.I18nBot("tgbot.messages.report", "RunTime=="+runTime)
		msg += t.I18nBot("tgbot.messages.datetime", "DateTime=="+time.Now().Format("2006-01-02 15:04:05"))
		t.SendMsgToTgbotAdmins(msg)
	}

	info := t.getServerUsage()
	t.SendMsgToTgbotAdmins(info)

	exhausted := t.getExhausted()
	t.SendMsgToTgbotAdmins(exhausted)

	backupEnable, err := t.settingService.GetTgBotBackup()
	if err == nil && backupEnable {
		t.SendBackupToAdmins()
	}
}

func (t *Tgbot) SendBackupToAdmins() {
	if !t.IsRunning() {
		return
	}
	for _, adminId := range adminIds {
		t.sendBackup(int64(adminId))
	}
}

func (t *Tgbot) getServerUsage() string {
	info, ipv4, ipv6 := "", "", ""
	info += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	info += t.I18nBot("tgbot.messages.version", "Version=="+config.GetVersion())

	// get ip address
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("net.Interfaces failed, err: ", err.Error())
		info += t.I18nBot("tgbot.messages.ip", "IP=="+t.I18nBot("tgbot.unknown"))
		info += " \r\n"
	} else {
		for i := 0; i < len(netInterfaces); i++ {
			if (netInterfaces[i].Flags & net.FlagUp) != 0 {
				addrs, _ := netInterfaces[i].Addrs()

				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ipv4 += ipnet.IP.String() + " "
						} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLinkLocalUnicast() {
							ipv6 += ipnet.IP.String() + " "
						}
					}
				}
			}
		}

		info += t.I18nBot("tgbot.messages.ipv4", "IPv4=="+ipv4)
		info += t.I18nBot("tgbot.messages.ipv6", "IPv6=="+ipv6)
	}

	// get latest status of server
	t.lastStatus = t.serverService.GetStatus(t.lastStatus)
	info += t.I18nBot("tgbot.messages.serverUpTime", "UpTime=="+strconv.FormatUint(t.lastStatus.Uptime/86400, 10), "Unit=="+t.I18nBot("tgbot.days"))
	info += t.I18nBot("tgbot.messages.serverLoad", "Load1=="+strconv.FormatFloat(t.lastStatus.Loads[0], 'f', 2, 64), "Load2=="+strconv.FormatFloat(t.lastStatus.Loads[1], 'f', 2, 64), "Load3=="+strconv.FormatFloat(t.lastStatus.Loads[2], 'f', 2, 64))
	info += t.I18nBot("tgbot.messages.serverMemory", "Current=="+common.FormatTraffic(int64(t.lastStatus.Mem.Current)), "Total=="+common.FormatTraffic(int64(t.lastStatus.Mem.Total)))
	info += t.I18nBot("tgbot.messages.tcpCount", "Count=="+strconv.Itoa(t.lastStatus.TcpCount))
	info += t.I18nBot("tgbot.messages.udpCount", "Count=="+strconv.Itoa(t.lastStatus.UdpCount))
	info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), "Upload=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), "Download=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv)))
	info += t.I18nBot("tgbot.messages.xrayStatus", "State=="+fmt.Sprint(t.lastStatus.Xray.State))

	return info
}

func (t *Tgbot) UserLoginNotify(username string, ip string, time string, status LoginStatus) {
	if !t.IsRunning() {
		return
	}

	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed,invalid info")
		return
	}

	loginNotifyEnabled, err := t.settingService.GetTgBotLoginNotify()
	if err != nil || !loginNotifyEnabled {
		return
	}

	msg := ""
	if status == LoginSuccess {
		msg += t.I18nBot("tgbot.messages.loginSuccess")
	} else if status == LoginFail {
		msg += t.I18nBot("tgbot.messages.loginFailed")
	}
	msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	msg += t.I18nBot("tgbot.messages.username", "Username=="+username)
	msg += t.I18nBot("tgbot.messages.ip", "IP=="+ip)
	msg += t.I18nBot("tgbot.messages.time", "Time=="+time)

	t.SendMsgToTgbotAdmins(msg)
}

func (t *Tgbot) getInboundUsages() string {
	info := ""
	// get traffic
	inbouds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		info += t.I18nBot("tgbot.answers.getInboundsFailed")
	} else {
		// NOTE:If there no any sessions here,need to notify here
		// TODO:Sub-node push, automatic conversion format
		for _, inbound := range inbouds {
			info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
			info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
			info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

			if inbound.ExpiryTime == 0 {
				info += t.I18nBot("tgbot.messages.expire", "DateTime=="+t.I18nBot("tgbot.unlimited"))
			} else {
				info += t.I18nBot("tgbot.messages.expire", "DateTime=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
		}
	}
	return info
}

func (t *Tgbot) getClientUsage(chatId int64, tgUserName string) {
	if len(tgUserName) == 0 {
		msg := t.I18nBot("tgbot.answers.askToAddUser")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserName)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(traffics) == 0 {
		msg := t.I18nBot("tgbot.answers.askToAddUserName", "TgUserName=="+tgUserName)
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, traffic := range traffics {
		expiryTime := ""
		if traffic.ExpiryTime == 0 {
			expiryTime = t.I18nBot("tgbot.unlimited")
		} else if traffic.ExpiryTime < 0 {
			expiryTime = fmt.Sprintf("%d %s", traffic.ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
		} else {
			expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
		}

		total := ""
		if traffic.Total == 0 {
			total = t.I18nBot("tgbot.unlimited")
		} else {
			total = common.FormatTraffic((traffic.Total))
		}

		output := ""
		output += t.I18nBot("tgbot.messages.active", "Enable=="+strconv.FormatBool(traffic.Enable))
		output += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
		output += t.I18nBot("tgbot.messages.upload", "Upload=="+common.FormatTraffic(traffic.Up))
		output += t.I18nBot("tgbot.messages.download", "Download=="+common.FormatTraffic(traffic.Down))
		output += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic((traffic.Up+traffic.Down)), "Total=="+total)
		output += t.I18nBot("tgbot.messages.expireIn", "Time=="+expiryTime)

		t.SendMsgToTgbot(chatId, output)
	}
	t.SendAnswer(chatId, t.I18nBot("tgbot.commands.pleaseChoose"), false)
}

func (t *Tgbot) searchClient(chatId int64, email string) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := t.I18nBot("tgbot.noResult")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	expiryTime := ""
	if traffic.ExpiryTime == 0 {
		expiryTime = t.I18nBot("tgbot.unlimited")
	} else if traffic.ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d %s", traffic.ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
	} else {
		expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
	}

	total := ""
	if traffic.Total == 0 {
		total = t.I18nBot("tgbot.unlimited")
	} else {
		total = common.FormatTraffic((traffic.Total))
	}

	output := ""
	output += t.I18nBot("tgbot.messages.active", "Enable=="+strconv.FormatBool(traffic.Enable))
	output += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
	output += t.I18nBot("tgbot.messages.upload", "Upload=="+common.FormatTraffic(traffic.Up))
	output += t.I18nBot("tgbot.messages.download", "Download=="+common.FormatTraffic(traffic.Down))
	output += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic((traffic.Up+traffic.Down)), "Total=="+total)
	output += t.I18nBot("tgbot.messages.expireIn", "Time=="+expiryTime)

	t.SendMsgToTgbot(chatId, output)
}

func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbouds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	if len(inbouds) == 0 {
		msg := t.I18nBot("tgbot.noInbounds")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, inbound := range inbouds {
		info := ""
		info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
		info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
		info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

		if inbound.ExpiryTime == 0 {
			info += t.I18nBot("tgbot.messages.expire", "DateTime=="+t.I18nBot("tgbot.unlimited"))
		} else {
			info += t.I18nBot("tgbot.messages.expire", "DateTime=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)

		for _, traffic := range inbound.ClientStats {
			expiryTime := ""
			if traffic.ExpiryTime == 0 {
				expiryTime = t.I18nBot("tgbot.unlimited")
			} else if traffic.ExpiryTime < 0 {
				expiryTime = fmt.Sprintf("%d %s", traffic.ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
			} else {
				expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
			}

			total := ""
			if traffic.Total == 0 {
				total = t.I18nBot("tgbot.unlimited")
			} else {
				total = common.FormatTraffic((traffic.Total))
			}

			output := ""
			output += t.I18nBot("tgbot.messages.active", "Enable=="+strconv.FormatBool(traffic.Enable))
			output += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
			output += t.I18nBot("tgbot.messages.upload", "Upload=="+common.FormatTraffic(traffic.Up))
			output += t.I18nBot("tgbot.messages.download", "Download=="+common.FormatTraffic(traffic.Down))
			output += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic((traffic.Up+traffic.Down)), "Total=="+total)
			output += t.I18nBot("tgbot.messages.expireIn", "Time=="+expiryTime)

			t.SendMsgToTgbot(chatId, output)
		}
	}
}

func (t *Tgbot) searchForClient(chatId int64, query string) {
	traffic, err := t.inboundService.SearchClientTraffic(query)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := t.I18nBot("tgbot.noResult")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	expiryTime := ""
	if traffic.ExpiryTime == 0 {
		expiryTime = t.I18nBot("tgbot.unlimited")
	} else if traffic.ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d %s", traffic.ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
	} else {
		expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
	}

	total := ""
	if traffic.Total == 0 {
		total = t.I18nBot("tgbot.unlimited")
	} else {
		total = common.FormatTraffic((traffic.Total))
	}

	output := ""
	output += t.I18nBot("tgbot.messages.active", "Enable=="+strconv.FormatBool(traffic.Enable))
	output += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
	output += t.I18nBot("tgbot.messages.upload", "Upload=="+common.FormatTraffic(traffic.Up))
	output += t.I18nBot("tgbot.messages.download", "Download=="+common.FormatTraffic(traffic.Down))
	output += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic((traffic.Up+traffic.Down)), "Total=="+total)
	output += t.I18nBot("tgbot.messages.expireIn", "Time=="+expiryTime)

	t.SendMsgToTgbot(chatId, output)
}

func (t *Tgbot) getExhausted() string {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000
	var exhaustedInbounds []model.Inbound
	var exhaustedClients []xray.ClientTraffic
	var disabledInbounds []model.Inbound
	var disabledClients []xray.ClientTraffic

	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		exDiff = int64(ExpireThreshold) * 86400000
	}
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load Inbounds", err)
	}

	for _, inbound := range inbounds {
		if inbound.Enable {
			if (inbound.ExpiryTime > 0 && (inbound.ExpiryTime-now < exDiff)) ||
				(inbound.Total > 0 && (inbound.Total-(inbound.Up+inbound.Down) < trDiff)) {
				exhaustedInbounds = append(exhaustedInbounds, *inbound)
			}
			if len(inbound.ClientStats) > 0 {
				for _, client := range inbound.ClientStats {
					if client.Enable {
						if (client.ExpiryTime > 0 && (client.ExpiryTime-now < exDiff)) ||
							(client.Total > 0 && (client.Total-(client.Up+client.Down) < trDiff)) {
							exhaustedClients = append(exhaustedClients, client)
						}
					} else {
						disabledClients = append(disabledClients, client)
					}
				}
			}
		} else {
			disabledInbounds = append(disabledInbounds, *inbound)
		}
	}

	// Inbounds
	output := ""
	output += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.inbounds"))
	output += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledInbounds)))
	output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(len(exhaustedInbounds)))
	output += "\r\n \r\n"

	if len(exhaustedInbounds) > 0 {
		output += t.I18nBot("tgbot.messages.exhaustedMsg", "Type=="+t.I18nBot("tgbot.inbounds"))

		for _, inbound := range exhaustedInbounds {
			output += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
			output += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
			output += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				output += t.I18nBot("tgbot.messages.expire", "DateTime=="+t.I18nBot("tgbot.unlimited"))
			} else {
				output += t.I18nBot("tgbot.messages.expire", "DateTime=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
			output += "\r\n \r\n"
		}
	}

	// Clients
	output += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.clients"))
	output += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledClients)))
	output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(len(exhaustedClients)))
	output += "\r\n \r\n"

	if len(exhaustedClients) > 0 {
		output += t.I18nBot("tgbot.messages.exhaustedMsg", "Type=="+t.I18nBot("tgbot.clients"))

		for _, traffic := range exhaustedClients {
			expiryTime := ""
			if traffic.ExpiryTime == 0 {
				expiryTime = t.I18nBot("tgbot.unlimited")
			} else if traffic.ExpiryTime < 0 {
				expiryTime += fmt.Sprintf("%d %s", traffic.ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
			} else {
				expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
			}

			total := ""
			if traffic.Total == 0 {
				total = t.I18nBot("tgbot.unlimited")
			} else {
				total = common.FormatTraffic((traffic.Total))
			}

			output += t.I18nBot("tgbot.messages.active", "Enable=="+strconv.FormatBool(traffic.Enable))
			output += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
			output += t.I18nBot("tgbot.messages.upload", "Upload=="+common.FormatTraffic(traffic.Up))
			output += t.I18nBot("tgbot.messages.download", "Download=="+common.FormatTraffic(traffic.Down))
			output += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic((traffic.Up+traffic.Down)), "Total=="+total)
			output += t.I18nBot("tgbot.messages.expireIn", "Time=="+expiryTime)
			output += "\r\n \r\n"
		}
	}

	return output
}

func (t *Tgbot) sendBackup(chatId int64) {
	if !t.IsRunning() {
		return
	}

	output := t.I18nBot("tgbot.messages.backupTime", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	t.SendMsgToTgbot(chatId, output)

	file := tgbotapi.FilePath(config.GetDBPath())
	msg := tgbotapi.NewDocument(chatId, file)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Warning("Error in uploading backup: ", err)
	}

	file = tgbotapi.FilePath(xray.GetConfigPath())
	msg = tgbotapi.NewDocument(chatId, file)
	_, err = bot.Send(msg)
	if err != nil {
		logger.Warning("Error in uploading config.json: ", err)
	}
}
