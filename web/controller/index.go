package controller

import (
	"net/http"
	"time"
	"x-ui/logger"
	"x-ui/web/service"
	"x-ui/web/session"
  //"fmt"
	"github.com/gin-gonic/gin"
  "github.com/pquerna/otp/totp"
)

type LoginForm struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

type IndexController struct {
	BaseController

	settingService service.SettingService
	userService    service.UserService
	tgbot          service.Tgbot
}

func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
  g.GET("/totp", a.totp)
	g.POST("/login", a.login)
  g.POST("/login/2fa", a.totpLogin)
	g.GET("/logout", a.logout)
}

func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Redirect(http.StatusTemporaryRedirect, "xui/")
		return
	}
	html(c, "login.html", "pages.login.title", nil)
}

func (a *IndexController) totp(c *gin.Context) {
  if session.IsLogin(c) {
    c.Redirect(http.StatusTemporaryRedirect, "xui/")
    return
  }
  cookie , err := c.Cookie("SECURE_PAGE")
  if err != nil || cookie != "GOTOTOTPPAGE" {
    c.Redirect(http.StatusTemporaryRedirect, "/")
  }
  html(c, "totp.html", "pages.login.title", nil)
}

func (a *IndexController) login(c *gin.Context) {
	var form LoginForm
	err := c.ShouldBind(&form)
	if err != nil {
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return
	}

	user := a.userService.CheckUser(form.Username, form.Password)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	if user == nil {
		logger.Infof("wrong username or password: \"%s\" \"%s\"", form.Username, form.Password)
		a.tgbot.UserLoginNotify(form.Username, getRemoteIp(c), timeStr, 0)
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	} else {
    totp := a.userService.CheckUserTotpEnabled(form.Username)
    if totp {
		  logger.Infof("%s login attempt ,Ip Address: %s\n", form.Username, getRemoteIp(c))
   		logger.Infof("%s performing 2FA \n", form.Username)
      c.SetCookie("SECURE_PAGE", "GOTOTOTPPAGE", 1200, "/", "localhost", true, true)
      jsonObj(c, gin.H{"totp": true}, nil)
      return 
    }
		logger.Infof("%s login success ,Ip Address: %s\n", form.Username, getRemoteIp(c))
		a.tgbot.UserLoginNotify(form.Username, getRemoteIp(c), timeStr, 1)
	}	
  sessionMaxAge, err := a.settingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}

	if sessionMaxAge > 0 {
		err = session.SetMaxAge(c, sessionMaxAge*60)
		if err != nil {
			logger.Infof("Unable to set session's max age")
		}
	}

  user.Otp_qrcode = nil 
	err = session.SetLoginUser(c, user)
  if err != nil  {
    return 
  }
	logger.Info("user", user.Id, "login success")
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), nil)

}

func (a *IndexController) totpLogin(c *gin.Context){
  var payload = &OTPInput{}
  err := c.ShouldBind(payload)
  if err != nil { 
		jsonMsg(c, I18nWeb(c, "pages.login.toasts.totpSomethingWrong"), err)
  }
  user, err := a.userService.GetFirstUser()
  valid := totp.Validate(payload.Token, user.Otp_secret)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
  if !valid {
    logger.Infof("unsuccessfull attempt to login by 2FA, remote ip : %s\n", getRemoteIp(c))
		a.tgbot.UserLoginNotify(user.Username, getRemoteIp(c), timeStr, 0)
		pureJsonMsg(c, false, I18nWeb(c, "pages.login.toasts.totpTokenInvalid"))
  }else {
    logger.Infof("%s login success ,Ip Address: %s\n", user.Username, getRemoteIp(c))
	  a.tgbot.UserLoginNotify(user.Username, getRemoteIp(c), timeStr, 1)

    sessionMaxAge, err := a.settingService.GetSessionMaxAge()
	  if err != nil {
	  	logger.Infof("Unable to get session's max age from DB")
	  }

	  if sessionMaxAge > 0 {
	  	err = session.SetMaxAge(c, sessionMaxAge*60)
	  	if err != nil {
	  		logger.Infof("Unable to set session's max age")
	  	}
	  }
    user.Otp_qrcode = nil 
	  err = session.SetLoginUser(c, user)
    if err != nil  {
      return 
    }
	  logger.Info("user", user.Username, "login success")
    c.SetCookie("SECURE_PAGE", "NONE", 1200, "/", "localhost", true, true)
	  jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), nil)
  }
}

func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Info("user", user.Id, "logout")
	}
	session.ClearSession(c)
	c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
}
