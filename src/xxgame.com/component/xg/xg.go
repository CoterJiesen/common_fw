// 信鸽消息推送组件
package xg

import "xxgame.com/component/log"
import "net/http"
import "net/url"
import "fmt"
import "time"
import "strings"
import "io/ioutil"
import "encoding/json"
import "encoding/hex"
import "crypto/md5"

const (
	LOGTAG          = "xg"
	SINGLE_PUSH_URL = "http://openapi.xg.qq.com/v2/push/single_account"
	CONTENT_TYPE    = "application/x-www-form-urlencoded"
	EXPIRE_TIME     = 3600 * 24 * 3
)

var (
	XG_LOCATION, _ = time.LoadLocation("Asia/Shanghai") //信鸽服务器时区为北京时间
	xgErrDatabase  = make(map[int]error, 10)            //信鸽错误说明
)

func init() {
	xgErrDatabase[-1] = fmt.Errorf("参数错误")
	xgErrDatabase[-2] = fmt.Errorf("请求时间戳不在有效期内")
	xgErrDatabase[-3] = fmt.Errorf("sign校验无效")
	xgErrDatabase[2] = fmt.Errorf("参数错误")
	xgErrDatabase[7] = fmt.Errorf("账号绑定的终端设备已经满了（最多10个）")
	xgErrDatabase[14] = fmt.Errorf("收到非法token")
	xgErrDatabase[15] = fmt.Errorf("信鸽逻辑服务器繁忙")
	xgErrDatabase[19] = fmt.Errorf("操作时序错误")
	xgErrDatabase[20] = fmt.Errorf("鉴权错误")
	xgErrDatabase[40] = fmt.Errorf("推送的token没有在信鸽中注册，或者推送的账号没有绑定token")
	xgErrDatabase[48] = fmt.Errorf("推送的账号没有在信鸽中注册")
	xgErrDatabase[63] = fmt.Errorf("标签系统忙")
	xgErrDatabase[71] = fmt.Errorf("APNS服务器繁忙")
	xgErrDatabase[73] = fmt.Errorf("消息字符数超限")
	xgErrDatabase[76] = fmt.Errorf("请求过于频繁，请稍后再试")
	xgErrDatabase[78] = fmt.Errorf("循环任务参数错误")
	xgErrDatabase[100] = fmt.Errorf("APNS证书错误，请重新提交正确的证书")
}

//获取北京时间
func getBeijingTime() time.Time {
	return time.Now().In(XG_LOCATION)
}

//检查返回值，无错误则返回nil，否则返回详细错误描述
func checkResp(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed! status code:%d\n", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body failed, %s", err)
	}

	retParams := make(map[string]interface{})
	err = json.Unmarshal(body, &retParams)
	if err != nil {
		return fmt.Errorf("unmarshal response body failed, %s", err)
	}

	code := int(retParams["ret_code"].(float64))
	if code == 0 {
		return nil
	}

	err = xgErrDatabase[code]
	if err != nil {
		return err
	}

	return fmt.Errorf("unknow error %d: %s", code, retParams["err_msg"])
}

//信鸽实例
type XG struct {
	log *log.Logger
}

//分配并初始化信鸽通信实例
func NewXG(log *log.Logger) *XG {
	xg := new(XG)
	xg.log = log
	return xg
}

//向指定玩家推送消息
func (xg *XG) SendMsg(uid uint64, content string, channel int) error {
	var accessID string
	var secretKey string
	switch channel {
	case 1:
		//阿语android
		accessID = "2100144361"
		secretKey = "2799811c79e72ef83969a8c5dca12238"
	case 2:
		//阿语ios
		accessID = "2200167510"
		secretKey = "7dc9f6327aae90f887ca03486bfbb32a"
	case 3:
		//英语android
		accessID = "2100165926"
		secretKey = "95ffde3231d8084fc2c915f13007f947"
	case 4:
		//英语ios
		accessID = "2200165927"
		secretKey = "e6b9fda0e6f9b9790c281deafa4adfd7"
	case 5:
		//印度android
		accessID = "2100157917"
		secretKey = "1c45403183cac4a931413f7fc36b7b10"
	case 6:
		//印度ios
		accessID = "2200157918"
		secretKey = "a4e9914ce89630a791a2d9692ec99393"
	case 13:
		//阿语扎金花android
		accessID = "2100145983"
		secretKey = "ef6a4f97d94bd65ee02553403aac9034"
	case 14:
		//阿语扎金花ios
		accessID = "2200145984"
		secretKey = "123e2578a1161f25a8e4b414de73983a"
	default:
		xg.log.Errorf(LOGTAG, "unsupported channel %d!", channel)
		return fmt.Errorf("unsupported channel %d!", channel)
	}

	var param string
	var env string
	var msgType string
	var msgContent string
	if channel%2 == 0 {
		env = "1"
		msgType = "0"
		msgContent = fmt.Sprintf(`{"aps":{"alert":"%s"}}`, content)
	} else {
		env = "0"
		msgType = "1"
		msgContent = fmt.Sprintf(`{"content":"%s","title":"XimiPoker", "vibrate":0}`, content)
	}

	//获得当前北京时间
	curTimestamp := fmt.Sprintf("%d", getBeijingTime().Unix())

	//生成消息签名
	curUID := fmt.Sprintf("%d", uid)
	expireTime := fmt.Sprintf("%d", EXPIRE_TIME)
	rawStr := fmt.Sprintf("POST%saccess_id=%saccount=%senvironment=%sexpire_time=%smessage=%smessage_type=%stimestamp=%s%s",
		SINGLE_PUSH_URL[7:], accessID, curUID, env, expireTime, msgContent, msgType, curTimestamp, secretKey)
	sum := md5.Sum([]byte(rawStr))
	sign := hex.EncodeToString(sum[:])

	//生成http请求消息体
	param = fmt.Sprintf("%s=%s&%s=%s&%s=%s&%s=%s&%s=%s&%s=%s&%s=%s&%s=%s",
		url.QueryEscape("access_id"), url.QueryEscape(accessID),
		url.QueryEscape("timestamp"), url.QueryEscape(curTimestamp),
		url.QueryEscape("account"), url.QueryEscape(curUID),
		url.QueryEscape("message_type"), url.QueryEscape(msgType),
		url.QueryEscape("message"), url.QueryEscape(msgContent),
		url.QueryEscape("environment"), url.QueryEscape(env),
		url.QueryEscape("sign"), url.QueryEscape(sign),
		url.QueryEscape("expire_time"), url.QueryEscape(expireTime))

	resp, err := http.Post(SINGLE_PUSH_URL, CONTENT_TYPE, strings.NewReader(param))
	if err != nil {
		xg.log.Errorf(LOGTAG, "post msg to user %d failed, %s", uid, err)
		return err
	}
	return checkResp(resp)
}
