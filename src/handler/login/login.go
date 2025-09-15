// Copyright 2019 ZSCREDBACK.
//
// Licensed under the AGPL, Version 3.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.gnu.org/licenses/agpl-3.0.en.html
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package login

import (
	"Yearning-go/src/handler/common"
	"Yearning-go/src/i18n"
	"Yearning-go/src/lib/ad"
	"Yearning-go/src/lib/factory"
	"Yearning-go/src/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cookieY/yee"
	"gorm.io/gorm"
	"log"
	"net/http"
)

type loginForm struct {
	Username string `json:"username"`
	Password string `json:"password"`
	MFACode  string `json:"mfa_code"`
}

func UserLdapLogin(c yee.Context) (err error) {
	u := new(loginForm)
	if err = c.Bind(u); err != nil {
		return c.JSON(http.StatusOK, common.ERR_COMMON_TEXT_MESSAGE(i18n.DefaultLang.Load(i18n.ER_REQ_BIND)))
	}
	ldap := ad.ALdap{Ldap: model.GloLdap}
	isOk, err := ldap.LdapConnect(u.Username, u.Password, false)
	if err != nil {
		return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(err))
	}
	if isOk {
		var account model.CoreAccount
		if err := model.DB().Where("username = ?", u.Username).First(&account).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			model.DB().Create(&model.CoreAccount{
				Username:   u.Username,
				RealName:   ldap.RealName,
				Password:   factory.DjangoEncrypt(factory.GenWorkId(), string(factory.GetRandom())),
				Department: ldap.Department,
				Email:      ldap.Email,
				IsRecorder: 2,
			})
			ix, _ := json.Marshal([]string{})
			model.DB().Create(&model.CoreGrained{Username: u.Username, Group: ix})
		}

		token, tokenErr := factory.JwtAuth(factory.Token{
			Username: u.Username,
			RealName: account.RealName,
			IsRecord: account.IsRecorder == 1,
		})
		if tokenErr != nil {
			c.Logger().Error(tokenErr.Error())
			return
		}
		dataStore := map[string]interface{}{
			"token":     token,
			"real_name": account.RealName,
			"user":      u.Username,
			"is_record": account.IsRecorder,
		}
		return c.JSON(http.StatusOK, common.SuccessPayload(dataStore))
	}
	return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(errors.New(i18n.DefaultLang.Load(i18n.ER_LOGIN))))
}

func UserGeneralLogin(c yee.Context) (err error) {
	u := new(loginForm)
	if err = c.Bind(u); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, common.ERR_COMMON_TEXT_MESSAGE(i18n.DefaultLang.Load(i18n.ER_REQ_BIND)))
	}
	var account model.CoreAccount
	if err := model.DB().Where("username = ?", u.Username).First(&account).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		if account.Username != u.Username {
			return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(errors.New(i18n.DefaultLang.Load(i18n.ER_LOGIN))))
		}
		if factory.DjangoCheckPassword(&account, u.Password) {
			token, tokenErr := factory.JwtAuth(factory.Token{
				Username: u.Username,
				RealName: account.RealName,
				IsRecord: account.IsRecorder == 1,
			})

			if tokenErr != nil {
				c.Logger().Error(tokenErr.Error())
				return err
			}
			dataStore := map[string]interface{}{
				"token":     token,
				"real_name": account.RealName,
				"user":      account.Username,
				"is_record": account.IsRecorder,
			}
			return c.JSON(http.StatusOK, common.SuccessPayload(dataStore))
		}
	}
	return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(errors.New(i18n.DefaultLang.Load(i18n.ER_LOGIN))))
}

func UserRegister(c yee.Context) (err error) {

	if model.GloOther.Register {
		u := new(model.CoreAccount)
		if err = c.Bind(u); err != nil {
			c.Logger().Error(err.Error())
			return c.JSON(http.StatusOK, common.ERR_COMMON_TEXT_MESSAGE(i18n.DefaultLang.Load(i18n.ER_REQ_BIND)))
		}
		var unique model.CoreAccount
		ix, _ := json.Marshal([]string{})
		if model.DB().Where("username = ?", u.Username).Select("username").First(&unique).Error == gorm.ErrRecordNotFound {
			model.DB().Create(&model.CoreAccount{
				Username:   u.Username,
				RealName:   u.RealName,
				Password:   factory.DjangoEncrypt(u.Password, string(factory.GetRandom())),
				Department: u.Department,
				Email:      u.Email,
			})
			model.DB().Create(&model.CoreGrained{Username: u.Username, Group: ix})
			return c.JSON(http.StatusOK, common.SuccessPayLoadToMessage(i18n.DefaultLang.Load(i18n.INFO_REGISTRATION_SUCCESS)))
		}
		return c.JSON(http.StatusOK, common.ERR_COMMON_TEXT_MESSAGE(i18n.DefaultLang.Load(i18n.ER_USER_ALREADY_EXISTS)))
	}
	return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(errors.New(i18n.DefaultLang.Load(i18n.ER_REGISTER))))

}

// Wechat 通用登录
func WechatGeneralLogin(c yee.Context) (err error) {
	model.DefaultLogger.Debug("Enter WechatGeneralLogin")

	loginData, ok := c.Get("loginData").(loginForm)
	if !ok {
		log.Println("[ERROR] could not retrieve login data from context")
		return fmt.Errorf("could not retrieve login data from context")
	}
	model.DefaultLogger.Debugf("loginData: %+v\n", loginData)

	var account model.CoreAccount
	if err = model.DB().Where("username = ?", loginData.Username).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[WARN] user %s not found in DB\n", loginData.Username)
			return c.JSON(http.StatusOK,
				common.ERR_COMMON_MESSAGE(errors.New(i18n.DefaultLang.Load(i18n.ER_LOGIN))))
		}
		log.Printf("[ERROR] DB query failed: %v\n", err)
		return err
	}
	model.DefaultLogger.Debugf("Found account: %+v\n", account)

	// 用户存在，生成 token
	token, tokenErr := factory.JwtAuth(factory.Token{
		Username: loginData.Username,
		RealName: account.RealName,
		IsRecord: account.IsRecorder == 1, // ?
	})
	if tokenErr != nil {
		log.Printf("[ERROR] JwtAuth failed: %v\n", tokenErr)
		return
	}
	model.DefaultLogger.Debugf("Generated token for user %s: %s\n", loginData.Username, token)

	dataStore := map[string]string{
		"token":     token,
		"real_name": account.RealName,
		"user":      loginData.Username,
	}
	model.DefaultLogger.Debugf("Response payload: %+v\n", dataStore)

	return c.JSON(http.StatusOK, common.SuccessPayload(dataStore))
}
