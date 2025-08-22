package main

import (
	"time"
)

func (app *Config) listenForErrors() {
	for {
		select {
		case err := <-app.ErrorChan:
			app.ErrorLog.Println(err)
		case <-app.ErrorChanDone:
			return
		}
	}
}

// SendEmailInBackground sends an email in the background
func (app *Config) SendEmailInBackground(recipient, otp string) {
	app.Wait.Add(1)
	go func(recipient, otp string) {
		defer app.Wait.Done()

		// Track start time for logging
		startTime := time.Now()

		// Log the beginning of the process
		app.InfoLog.Printf("Starting to send OTP email to %s", recipient)

		// Attempt to send the email
		err := app.Mailer.SendOTP(recipient, otp)
		if err != nil {
			app.ErrorLog.Printf("Failed to send OTP email to %s: %v", recipient, err)
			app.ErrorChan <- err
			return
		}

		// Calculate time taken
		duration := time.Since(startTime)

		// Log successful completion
		app.InfoLog.Printf("Successfully sent OTP email to %s (took %v)", recipient, duration)
	}(recipient, otp)
}

// SendHTMLInBackground sends a generic HTML email in the background
func (app *Config) SendHTMLInBackground(recipient, subject, htmlBody string) {
    app.Wait.Add(1)
    go func(recipient, subject, htmlBody string) {
        defer app.Wait.Done()

        startTime := time.Now()
        app.InfoLog.Printf("Starting to send email to %s with subject '%s'", recipient, subject)

        if err := app.Mailer.SendHTML(recipient, subject, htmlBody); err != nil {
            app.ErrorLog.Printf("Failed to send email to %s: %v", recipient, err)
            app.ErrorChan <- err
            return
        }

        app.InfoLog.Printf("Successfully sent email to %s (took %v)", recipient, time.Since(startTime))
    }(recipient, subject, htmlBody)
}
