package controller

import (
	"x-ui/web/service"
	"x-ui/web/session"
	"errors"
  "github.com/pquerna/otp/totp"
	"github.com/gin-gonic/gin"
  "image/png"
  "bytes"
)

type OTPInput struct { 
  Token    string `json:"token" form:"token"`
}


type OTPController struct {
	settingService service.SettingService
	userService    service.UserService
	panelService   service.PanelService
  serverService  service.ServerService
}

func NewOTPController(g *gin.RouterGroup) *OTPController {
	a := &OTPController{}
	a.initRouter(g)
	return a
}

func (a *OTPController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/totp")

	g.POST("/generate", a.generateToken)
	g.POST("/verify", a.verifyToken)
	g.POST("/disable", a.disableOTP)
  g.POST("/qrcode", a.getQrCode)
}

func (a *OTPController) generateToken(c *gin.Context){
  if !session.IsLogin(c) {
    return
  }
	user := session.GetLoginUser(c)

  issuer4, issuer6 := a.serverService.GetServerAddress()
  issuer := "N/A"
  if issuer4 != "N/A" {
    issuer = issuer4
  } else {
    issuer = issuer6
  }
  username := user.Username
  key, err := totp.Generate(totp.GenerateOpts{
  	Issuer:      issuer, 
  	AccountName: username+"@"+issuer,
  	SecretSize:  32,
  })
  var buf bytes.Buffer
  img, err := key.Image(300, 300)
	png.Encode(&buf, img)

  if err != nil { 
     panic(err) 
  }
  err = a.userService.UpdateUserTotp(user.Id, key.Secret(), false, false, key.URL(), buf.Bytes())
  if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.originalUserTotpIncorrect"), err)
    return
  }
  jsonObj(c, buf.Bytes(), nil)
}

func (a *OTPController) verifyToken(c *gin.Context){
  if !session.IsLogin(c) {
    return
  }
  var payload = &OTPInput{}
  
  err := c.ShouldBind(payload)
  if err != nil { 
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpOperationFailed"), err)
    return 
  }
	loggedInUser := session.GetLoginUser(c)
  user, err := a.userService.GetFirstUser()
  if loggedInUser.Username != user.Username || loggedInUser.Password != user.Password {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpOperationFailed"), errors.New("User mismatch"))
    return 
  }
  valid := totp.Validate(payload.Token, user.Otp_secret)
  if !valid {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpIncorrect"), errors.New("Token is incorrect"))
    return 
  }

  err = a.userService.UpdateUserTotp(user.Id, user.Otp_secret, true, true, user.Otp_auth_url, user.Otp_qrcode)
  if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpOperationFailed"), err)
    return
  }
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpEnabled"), nil)
}

func (a *OTPController) disableOTP(c *gin.Context){
  if !session.IsLogin(c) {
    return
  }
  user, err := a.userService.GetFirstUser()

  err = a.userService.UpdateUserTotp(user.Id, "", false, false, "", nil)
  if err != nil {
    jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpOperationFailed"), err)
    return
  }
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpDisabled"), nil)
}

func (a *OTPController) getQrCode(c *gin.Context){
  if !session.IsLogin(c) {
    return
  }
  user, err := a.userService.GetFirstUser()
  if err != nil { 
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.userTotpOperationFailed"), errors.New("User mismatch"))
    return
  }
  jsonObj(c, user.Otp_qrcode, nil)

}
