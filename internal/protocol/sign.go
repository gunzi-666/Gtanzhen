package protocol

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// SignExec 为远程执行命令生成 HMAC-SHA256 签名。
// 参与签名的字段：task_id、command(target)、nonce、ts。
// 面板用每台机器的 secret 作为密钥签名，Agent 用同一 secret 校验。
func SignExec(secret, taskID, command, nonce string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%s\n%s\n%d", taskID, command, nonce, ts)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyExec 校验远程执行命令签名是否合法。
func VerifyExec(secret, taskID, command, nonce string, ts int64, sign string) bool {
	expected := SignExec(secret, taskID, command, nonce, ts)
	return hmac.Equal([]byte(expected), []byte(sign))
}
