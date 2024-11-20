package service

import (
	"gopkg.in/gomail.v2"
	"log"
	"strings"
)

type EmailService struct {
	Sender *gomail.Dialer
}

// NewEmailService 初始化邮件服务器
func NewEmailService() *EmailService {
	service := &EmailService{}
	service.Sender = gomail.NewDialer("smtp.qq.com", 465, "2175407289@qq.com", "rurlinhdptfwechf")
	return service
}

// SendMessage 发送邮件text参数作为内容的邮件到目标地址
func SendMessage(targetEmail string, text ...string) error {
	// 将所有参数合并为正文
	content := strings.Join(text, "\n")

	// Create a new email message
	m := gomail.NewMessage()
	m.SetHeader("From", "2175407289@qq.com")          // Sender's email
	m.SetHeader("To", targetEmail)                    // Recipient's email
	m.SetHeader("Subject", "lvdashuaibi恭喜你抢到羽毛球场地啦！") // Email subject
	m.SetBody("text/plain", content)                  // Email body

	// Initialize the email service
	d := NewEmailService()

	// Send the email
	if err := d.Sender.DialAndSend(m); err != nil {
		log.Println("邮件发送失败:", err)
		return err
	}
	log.Println("邮件发送成功！")
	return nil
}
