package login

import (
	"Yearning-go/src/handler/common"
	"Yearning-go/src/handler/manage/user"
	"Yearning-go/src/i18n"
	"Yearning-go/src/model"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/cookieY/yee"
	"github.com/jinzhu/gorm"
	"io/ioutil"
	"log"
	"net/http"
)

const userURL = "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"

type Request struct {
	// ResponseType string `form:"response_type" binding:"required"` // 接收 response_type
	// APPID        string `form:"appid" binding:"required"`         // 接收 APPID
	// RedirectURI  string `form:"redirect_uri" binding:"required"`  // 接收 redirect_uri
	// State        string `form:"state"`                            // 接收 state
	Code  string `json:"code"`
	State string `json:"state"`
}
type UserList struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	UserID  string `json:"userid"`
}

// 接收企业微信 OAuth 回调的 code，换取用户身份信息，然后在 Yearning 里执行一次模拟登录
func UserWechatSwitch(c yee.Context) (err error) {
	// 如果日志里没有这一行，说明前端回调地址没正确指向这个 handler
	model.DefaultLogger.Debug("Enter UserWechatSwitch")
	model.DefaultLogger.Debugf("Raw query: %s", c.Request().URL.RawQuery)
	model.DefaultLogger.Debugf("Raw body: %v", c.Request().Body)

	var request Request
	token, err := user.GetAccessToken()
	if err = c.Bind(&request); err != nil {
		model.DefaultLogger.Debugf("Bind request failed: %v", err)
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, common.ERR_COMMON_TEXT_MESSAGE(i18n.DefaultLang.Load(i18n.ER_REQ_BIND)))
	}

	model.DefaultLogger.Debugf("Request parsed: Code=%s, State=%s", request.Code, request.State)

	reqURL := fmt.Sprintf("%s?access_token=%s&code=%s", userURL, token, request.Code)
	resp, _ := http.Get(reqURL)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
	}
	resp.Body.Close()

	var Response UserList
	if err = json.Unmarshal(body, &Response); err != nil {
		fmt.Println("???")
		model.DefaultLogger.Debugf("JSON unmarshal failed: %v", err)
	}
	if Response.ErrCode != 0 {
		err = fmt.Errorf("error: %d, Message: %s", Response.ErrCode, Response.ErrMsg)
		fmt.Println(err.Error())
	}
	model.DefaultLogger.Debugf("Wechat User Response: %+v", Response)

	//获取配置文件中sql审计数据库的数据库信息
	_, err = toml.DecodeFile("conf.toml", &model.C)
	if err != nil {
		log.Println("解析配置文件出错：", err)
	}
	newDb, err := gorm.Open("mysql", fmt.Sprintf("%s:%s@(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", model.C.Mysql.User, model.C.Mysql.Password, model.C.Mysql.Host, model.C.Mysql.Port, model.C.Mysql.Db))
	if err != nil {
		log.Println("连接主数据库时出错：", err)
	}
	defer func() {
		sqlDB := newDb.DB()
		_ = sqlDB.Close()
	}()
	var dataSource model.CoreAccount
	if err := newDb.Where("username = ?", Response.UserID).First(&dataSource).Error; err != nil {
		log.Println("Error retrieving data:", err)
		//return err
	}
	loginData := loginForm{
		Username: dataSource.Username,
		Password: dataSource.Password,
	}
	// 将 loginData 存储进上下文
	c.Put("loginData", loginData)
	// 调用 login 包的 UserGeneralLogin 函数
	if err := WechatGeneralLogin(c); err != nil {
		log.Println("Error during login:", err)
	}
	return c.JSON(http.StatusOK, common.SuccessPayload(map[string]string{"message": "Code received", "code": request.Code}))
}
