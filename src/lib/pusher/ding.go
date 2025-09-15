package pusher

import (
	"Yearning-go/src/i18n"
	"Yearning-go/src/model"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type imCryGeneric struct {
	Assigned string
	WorkId   string
	Source   string
	Username string
	Text     string
}

var Commontext = `
{
        "msgtype": "markdown",
        "markdown": {
                "title": "SQL Audit Platform",
                "content": "## 📌 SQL审计平台工单通知 \n \n > **工单编号:** $WORKID \n \n **数据源:** $SOURCE \n \n **工单说明:** $TEXT \n \n **提交人员:** <font color = \"#78beea\">$USER</font> \n \n **下一步操作人:** <font color=\"#fe8696\">$AUDITOR</font> \n \n **平台地址:** [点击跳转]($HOST) \n \n **状态:** <font color=\"#1abefa\">$STATE</font>"
        }
}
`

func PusherMessages(msg model.Message, sv string) {
	//请求地址模板

	//创建一个请求

	hook := msg.WebHook

	if msg.Key != "" {
		hook = Sign(msg.Key, msg.WebHook)
	}
	model.DefaultLogger.Debugf("hook:%v", hook)
	model.DefaultLogger.Debugf("sv:%v", sv)
	req, err := http.NewRequest("POST", hook, strings.NewReader(sv))
	if err != nil {
		model.DefaultLogger.Errorf("request:", err)
		return
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	//设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	//发送请求
	resp, err := client.Do(req)

	if err != nil {
		model.DefaultLogger.Errorf("resp:", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	model.DefaultLogger.Debugf("resp:%v", string(body))
	//关闭请求
	defer resp.Body.Close()
}

// 发送提醒信息
func SendDingRemind(msg model.Message, remind string) {
	// 构造信息
	var mx string

	if remind == "admin" { // 自己处理admin的请求
		remind = "zhangsichen"
	}
	mx = fmt.Sprintf(`{"msgtype": "text", "text": {"content": "📢 工单状态变更提醒，请及时处理。", "mentioned_list": "%s"}}`, remind)
	model.DefaultLogger.Debugf("mx: %v", mx)

	hook := msg.WebHook
	if msg.Key != "" {
		hook = Sign(msg.Key, msg.WebHook)
	}

	// 创建一个请求
	req, err := http.NewRequest("POST", hook, strings.NewReader(mx))
	if err != nil {
		log.Println(err.Error())
		return
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}
	//设置请求头
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	//发送请求
	resp, err := client.Do(req)

	if err != nil {
		model.DefaultLogger.Errorf("resp:", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	model.DefaultLogger.Debugf("resp:%v", string(body))
	//关闭请求
	defer resp.Body.Close()
}

func Sign(secret, hook string) string {
	timestamp := time.Now().UnixNano() / 1e6
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	sign := hmacSha256(stringToSign, secret)
	url := fmt.Sprintf("%s&timestamp=%d&sign=%s", hook, timestamp, sign)
	return url
}

func dingMsgTplHandler(state string, generic interface{}) string {

	var order imCryGeneric
	switch v := generic.(type) {
	case model.CoreSqlOrder:
		order = imCryGeneric{
			Assigned: v.Assigned,
			WorkId:   v.WorkId,
			Source:   v.Source,
			Username: v.Username,
			Text:     v.Text,
		}
	case model.CoreQueryOrder:
		order = imCryGeneric{
			Assigned: v.Assigned,
			WorkId:   v.WorkId + i18n.DefaultLang.Load(i18n.INFO_QUERY),
			Source:   i18n.DefaultLang.Load(i18n.ER_QUERY_NO_DATA_SOURCE),
			Username: v.Username,
			Text:     v.Text,
		}
	}
	text := Commontext
	if !stateHandler(state) {
		order.Assigned = "无"
	}
	text = strings.Replace(text, "$STATE", state, -1)
	text = strings.Replace(text, "$WORKID", order.WorkId, -1)
	text = strings.Replace(text, "$SOURCE", order.Source, -1)
	model.DefaultLogger.Debugf("$HOST: %v", model.GloOther.Domain)
	text = strings.Replace(text, "$HOST", model.GloOther.Domain, -1)
	text = strings.Replace(text, "$USER", order.Username, -1)
	text = strings.Replace(text, "$AUDITOR", order.Assigned, -1)
	text = strings.Replace(text, "$TEXT", order.Text, -1)
	model.DefaultLogger.Debugf("format: %v", text)
	return text
}

func stateHandler(state string) bool {
	switch state {
	case i18n.DefaultLang.Load(i18n.INFO_TRANSFERRED_TO_NEXT_AGENT), i18n.DefaultLang.Load(i18n.INFO_SUBMITTED):
		return true
	}
	return false
}

func hmacSha256(stringToSign string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
