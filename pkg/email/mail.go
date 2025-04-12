package email

import (
	"fmt"
	"net/smtp"
	"os"
)

// Mailer is the interface for sending emails
type Mailer interface {
	SendOTP(to, otp string) error
}

// SMTPMailer implements the Mailer interface using SMTP
type SMTPMailer struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// NewSMTPMailer creates a new SMTPMailer with settings from environment variables
func NewSMTPMailer() *SMTPMailer {
	return &SMTPMailer{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
}

// SendOTP sends an OTP code to the specified email address
func (m *SMTPMailer) SendOTP(to, otp string) error {
	subject := "Your Password Reset OTP"
	body := fmt.Sprintf(`
	<html>
		<body>
			<h1>Password Reset Request</h1>
			<p>Dear User,</p>
			<p>Your OTP for password reset is: <strong>%s</strong></p>
			<p>This OTP will expire in 15 minutes.</p>
			<p>If you did not request this password reset, please ignore this email.</p>
			<p>Regards,<br>Field Eyes Team</p>
		</body>
	</html>
	`, otp)

	message := fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, subject, body)

	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)

	return smtp.SendMail(addr, auth, m.From, []string{to}, []byte(message))
}

// MockMailer is a mock implementation of the Mailer interface for testing
type MockMailer struct{}

// SendOTP mocks sending an OTP code (for testing or development)
func (m *MockMailer) SendOTP(to, otp string) error {
	fmt.Printf("[MOCK EMAIL] To: %s, OTP: %s\n", to, otp)
	return nil
}
