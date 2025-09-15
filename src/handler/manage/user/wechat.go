package user

import (
	"Yearning-go/src/lib/factory"
	"Yearning-go/src/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

const (
	tokenURL = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	userURL  = "https://qyapi.weixin.qq.com/cgi-bin/user/simplelist"
)

type WechatConfig struct {
	CorpID          string
	CorpSecret      string
	DefaultPassword string
	DepartmentID    []string
	Department      string
}

var Wechat WechatConfig

func LoadConfig() {
	viper.SetConfigName("conf")      // 文件名 (不带扩展名)
	viper.SetConfigType("toml")      // 文件类型
	viper.AddConfigPath(".")         // 检索当前目录
	viper.AddConfigPath("../")       // 检索上一级目录
	viper.AddConfigPath("../../")    // 检索上一级目录
	viper.AddConfigPath("../../../") // 检索上一级目录

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// 映射到结构体
	if err := viper.UnmarshalKey("Wechat", &Wechat); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
}

// AccessTokenResponse represents the response structure for AccessToken
type AccessTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// UserListResponse represents the response structure for user list
type UserListResponse struct {
	ErrCode  int    `json:"errcode"`
	ErrMsg   string `json:"errmsg"`
	UserList []User `json:"userlist"`
}

// User represents individual user details
type User struct {
	UserID string `json:"userid"`
	Name   string `json:"name"` // 真实姓名
	//Email  string `json:"email"` // 现在获取不到email，所以注释了
}

// GetAccessToken 调用企业微信 API 获取 access_token
func GetAccessToken() (string, error) {
	reqURL, err := url.Parse(tokenURL)
	if err != nil {
		return "", err
	}

	q := reqURL.Query()
	q.Set("corpid", Wechat.CorpID)
	q.Set("corpsecret", Wechat.CorpSecret)
	reqURL.RawQuery = q.Encode()

	//fmt.Printf("CorpID: %s, CorpSecret: %s\n", Wechat.CorpID, Wechat.CorpSecret)

	resp, err := http.Get(reqURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResponse AccessTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", err
	}

	if tokenResponse.ErrCode != 0 {
		return "", fmt.Errorf("error: %d, Message: %s", tokenResponse.ErrCode, tokenResponse.ErrMsg)
	}
	return tokenResponse.AccessToken, nil
}

// GetUsers 遍历部门 ID 列表，调用接口获取用户列表，拼成一个总用户列表返回。
func GetUsers(departmentID []string, accessToken string) ([]User, error) {
	var allUsers []User
	for _, department := range departmentID {
		reqURL := fmt.Sprintf("%s?access_token=%s&department_id=%s", userURL, accessToken, department)
		resp, err := http.Get(reqURL)
		if err != nil {
			return nil, err
		}
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close() // ?
		var userResponse UserListResponse
		if err := json.Unmarshal(body, &userResponse); err != nil {
			return nil, err
		}
		if userResponse.ErrCode != 0 {
			return nil, fmt.Errorf("error: %d, Message: %s", userResponse.ErrCode, userResponse.ErrMsg)
		}
		//fmt.Println(userResponse.UserList)
		allUsers = append(allUsers, userResponse.UserList...)
	}
	return allUsers, nil
}

// SaveUsersToDB 遍历用户 → 如果不存在则插入，否则更新
func SaveUsersToDB(users []User, db *gorm.DB) {
	// 定义默认权限组, 参考admin: {"ddl_source": [], "dml_source": [], "query_source": []}
	defaultPerms, _ := json.Marshal(map[string][]string{
		"ddl_source":   {},
		"dml_source":   {},
		"query_source": {},
	})

	// 判断是否存在默认部门（权限组），如果没有就进行创建
	var wechatGroup model.CoreRoleGroup
	err := db.Where("name = ?", Wechat.Department).First(&wechatGroup).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		wechatGroup = model.CoreRoleGroup{
			Name:        Wechat.Department,
			Permissions: defaultPerms,
			GroupId:     uuid.New().String(),
		}
		if err := db.Create(&wechatGroup).Error; err != nil {
			fmt.Printf("❌ Failed to create group: %v\n", err)
			return
		}
		fmt.Printf("✅ Successfully created the group: %v\n", Wechat.Department)
	} else if err != nil {
		fmt.Printf("❌ Failed to query group: %v\n", err)
		return
	}

	for _, user := range users {
		var account model.CoreAccount

		err := db.Where("username = ?", user.UserID).First(&account).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 用户不存在 → 创建
				account = model.CoreAccount{
					Username:   user.UserID,
					Password:   factory.DjangoEncrypt(Wechat.DefaultPassword, string(factory.GetRandom())), // 初始加密密码
					Department: Wechat.Department,
					RealName:   user.Name,
					//Email:      user.Email, // 2022.6.20 20:00:00 以后除了通信录以外的应用，调用接口不在返回邮箱以及企业邮箱字段，详见企业微信官网：通信录管理-成员管理-读取成员
					Email:      "",
					IsRecorder: 2,
				}

				if err := db.Create(&account).Error; err != nil {
					fmt.Printf("❌ Failed to create account for %s: %v\n", user.Name, err)
				} else {
					fmt.Printf("✅ Successfully created account for %s\n", user.Name)

					// 创建用户成功后授权默认权限组
					ix, _ := json.Marshal([]string{
						wechatGroup.GroupId,
					})
					db.Create(&model.CoreGrained{Username: user.UserID, Group: ix})
				}
			} else {
				fmt.Printf("❌ Error checking existing account for %s: %v\n", user.Name, err)
			}
		} else {
			// 用户存在 → 只更新非敏感字段
			account.RealName = user.Name
			//account.Email = user.Email

			if err := db.Save(&account).Error; err != nil {
				fmt.Printf("❌ Failed to update account for %s: %v\n", user.Name, err)
			} else {
				fmt.Printf("✅ Successfully updated account for %s\n", user.Name)
			}
		}
	}
}
