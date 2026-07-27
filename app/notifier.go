package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// notifyBatch sends multiple alerts in a single notification
func notifyBatch(n Notifications, alerts []Alert) {
	if len(alerts) == 0 {
		return
	}

	fmt.Printf("\n🔔 Sending batched notification for %d alert(s)\n", len(alerts))

	// Check if Google Chat webhook is properly configured (not empty and not a placeholder)
	isGoogleChatConfigured := n.GoogleChat.WebhookURL != "" &&
		!strings.Contains(n.GoogleChat.WebhookURL, "XXX") &&
		!strings.Contains(n.GoogleChat.WebhookURL, "YYY") &&
		!strings.Contains(n.GoogleChat.WebhookURL, "ZZZ")

	// Build Google Chat message
	if isGoogleChatConfigured {
		var chatMessages []string
		for i, a := range alerts {
			upgradeNotice := ""
			if a.Type == "image" || a.Type == "binary" || a.Type == "helm" {
				if strings.Contains(strings.ToLower(a.Reason), "pinned version") ||
					strings.Contains(strings.ToLower(a.Reason), "latest") ||
					strings.Contains(strings.ToLower(a.Reason), "major version") {
					upgradeNotice = fmt.Sprintf("\n⚠️  UPGRADE RECOMMENDED: A new %s version (%s) has been released.", a.Type, a.TagName)
				}
			}
			msg := fmt.Sprintf("%d. *[%s]* %s %s → %s\n   Reason: %s\n   Link: %s%s",
				i+1, strings.ToUpper(a.Severity), a.Repo, a.Type, a.TagName, a.Reason, a.URL, upgradeNotice)
			chatMessages = append(chatMessages, msg)
		}
		fullMsg := fmt.Sprintf("*GitHub Action Runner Alerts (%d)*\n\n%s",
			len(alerts), strings.Join(chatMessages, "\n\n"))

		fmt.Printf("  📤 Sending Google Chat notification...\n")
		if err := sendGoogleChat(n.GoogleChat, fullMsg); err != nil {
			fmt.Printf("  ❌ Google Chat notification failed: %v\n", err)
		} else {
			fmt.Printf("  ✅ Google Chat notification sent successfully\n")
		}
	} else if n.GoogleChat.WebhookURL != "" {
		fmt.Printf("  ⚠️  Google Chat webhook contains placeholder values - skipping\n")
	}

	// Build and send email
	if n.Email.SMTPHost != "" && n.Email.From != "" && len(n.Email.To) > 0 {
		fmt.Printf("  📧 Sending email notification to: %s\n", strings.Join(n.Email.To, ", "))
		if err := sendBatchEmail(n.Email, alerts); err != nil {
			fmt.Printf("  ❌ Email notification failed: %v\n", err)
		} else {
			fmt.Printf("  ✅ Email notification sent successfully\n")
		}
	}

	if !isGoogleChatConfigured && (n.Email.SMTPHost == "" || n.Email.From == "" || len(n.Email.To) == 0) {
		fmt.Printf("  ⚠️  No valid notification channels configured!\n")
	}
}

// sendGoogleChat sends a message to Google Chat via webhook
func sendGoogleChat(gc GoogleChatConfig, text string) error {
	payload := map[string]string{"text": text}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", gc.WebhookURL, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google chat status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// sendBatchEmail sends multiple alerts in a single email
func sendBatchEmail(ec EmailConfig, alerts []Alert) error {
	// Determine highest severity
	highestSeverity := "info"
	for _, a := range alerts {
		if strings.EqualFold(a.Severity, "critical") {
			highestSeverity = "action required"
			break
		} else if strings.EqualFold(a.Severity, "warning") && !strings.EqualFold(highestSeverity, "action required") {
			highestSeverity = "warning"
		}
	}

	subject := fmt.Sprintf("[%s] C3E GitHub Action Self Hosted Runner Alerts", strings.ToUpper(highestSeverity))

	// Build email body with all alerts
	var body strings.Builder
	body.WriteString("🔔 C3E GITHUB ACTION SELF HOSTED RUNNER ALERTS 🔔\n")
	body.WriteString(strings.Repeat("=", 60))
	body.WriteString("\n\n")
	body.WriteString(fmt.Sprintf("Summary: %d alert(s) detected\n\n", len(alerts)))

	for i, a := range alerts {
		body.WriteString(fmt.Sprintf("Alert #%d\n", i+1))
		body.WriteString(strings.Repeat("-", 50))
		body.WriteString("\n")
		body.WriteString(fmt.Sprintf("Severity: %s\n", strings.ToUpper(a.Severity)))
		body.WriteString(fmt.Sprintf("Repository: %s\n", a.Repo))
		body.WriteString(fmt.Sprintf("Type: %s\n", a.Type))
		body.WriteString(fmt.Sprintf("Version: %s\n", a.TagName))
		body.WriteString(fmt.Sprintf("Reason: %s\n", a.Reason))
		body.WriteString(fmt.Sprintf("Published: %s\n", a.PublishedAt.Format(time.RFC3339)))
		body.WriteString(fmt.Sprintf("Link: %s\n", a.URL))
		body.WriteString(fmt.Sprintf("Recommended Action: %s\n", a.Remediation))

		// Add upgrade notice if applicable
		if a.Type == "image" || a.Type == "binary" || a.Type == "helm" {
			if strings.Contains(strings.ToLower(a.Reason), "pinned version") ||
				strings.Contains(strings.ToLower(a.Reason), "latest") ||
				strings.Contains(strings.ToLower(a.Reason), "major version") ||
				strings.Contains(strings.ToLower(a.Reason), "deprecated") {
				body.WriteString(fmt.Sprintf(`
⚠️  UPGRADE RECOMMENDED ⚠️
A new %s version (%s) has been released. Please upgrade to avoid issues 
from deprecated or unsupported earlier versions. Continuing to use outdated 
versions may result in:
  • Compatibility issues with newer GitHub Actions features
  • Security vulnerabilities
  • Loss of official support
  • Potential workflow failures
`, a.Type, a.TagName))
			}
		}
		body.WriteString("\n")
	}

	// Construct email message
	msg := fmt.Sprintf("From: %s\r\n", ec.From)
	msg += fmt.Sprintf("To: %s\r\n", strings.Join(ec.To, ", "))
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "Content-Type: text/plain; charset=utf-8\r\n"
	msg += "\r\n"
	msg += body.String()

	// Parse SMTP host and port
	host := ec.SMTPHost
	addr := host
	if !strings.Contains(host, ":") {
		// Use port 25 for relay servers (unauthenticated SMTP)
		addr = host + ":25"
	}

	// Send email (anonymous/unauthenticated)
	err := smtp.SendMail(addr, nil, ec.From, ec.To, []byte(msg))
	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}
	return nil
}
