package backup

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Config holds SMTP and backup settings.
type Config struct {
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	EmailTo  string
	// MaxBackups is the maximum number of local backup files to retain.
	MaxBackups int
}

// Enabled returns true if SMTP settings are configured.
func (c Config) Enabled() bool {
	return c.SMTPHost != "" && c.SMTPUser != "" && c.SMTPPass != "" && c.EmailTo != ""
}

// ConfigFromEnv reads backup config from environment variables.
func ConfigFromEnv() Config {
	max := 30
	return Config{
		SMTPHost:   os.Getenv("SMTP_HOST"),
		SMTPPort:   envOrDefault("SMTP_PORT", "587"),
		SMTPUser:   os.Getenv("SMTP_USER"),
		SMTPPass:   os.Getenv("SMTP_PASS"),
		EmailTo:    os.Getenv("BACKUP_EMAIL_TO"),
		MaxBackups: max,
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Run copies the database file to a timestamped backup and optionally emails it.
// The local backup is always created; the email is sent asynchronously if configured.
func Run(dbPath string, cfg Config) {
	backupPath, err := localBackup(dbPath, cfg.MaxBackups)
	if err != nil {
		log.Printf("backup: local backup failed: %v", err)
		return
	}
	log.Printf("backup: created %s", backupPath)

	if cfg.Enabled() {
		go func() {
			if err := sendEmail(cfg, backupPath); err != nil {
				log.Printf("backup: email failed: %v", err)
			} else {
				log.Printf("backup: email sent to %s", cfg.EmailTo)
			}
		}()
	} else {
		log.Printf("backup: email not configured, skipping")
	}
}

func localBackup(dbPath string, maxBackups int) (string, error) {
	dir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating backups dir: %w", err)
	}

	ts := time.Now().Format("20060102_150405")
	base := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	backupName := fmt.Sprintf("%s_backup_%s.db", base, ts)
	backupPath := filepath.Join(dir, backupName)

	src, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("opening db: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("creating backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("copying db: %w", err)
	}

	pruneBackups(dir, base, maxBackups)
	return backupPath, nil
}

func pruneBackups(dir, base string, maxBackups int) {
	if maxBackups <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := base + "_backup_"
	var backups []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e.Name())
		}
	}
	sort.Strings(backups)
	for len(backups) > maxBackups {
		os.Remove(filepath.Join(dir, backups[0]))
		backups = backups[1:]
	}
}

func sendEmail(cfg Config, backupPath string) error {
	fileData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("reading backup: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Headers
	boundary := writer.Boundary()
	fileName := filepath.Base(backupPath)
	ts := time.Now().Format("2006-01-02 15:04")

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: Suspense DB Backup — %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n",
		cfg.SMTPUser, cfg.EmailTo, ts, boundary,
	)

	// Text part
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPart, _ := writer.CreatePart(textHeader)
	fmt.Fprintf(textPart, "Automatic backup taken before import at %s.\nFile: %s\n", ts, fileName)

	// Attachment
	attHeader := make(textproto.MIMEHeader)
	attHeader.Set("Content-Type", "application/octet-stream")
	attHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	attHeader.Set("Content-Transfer-Encoding", "base64")
	attPart, _ := writer.CreatePart(attHeader)
	encoded := base64.StdEncoding.EncodeToString(fileData)
	// Write base64 in 76-char lines per RFC 2045
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		attPart.Write([]byte(encoded[i:end] + "\r\n"))
	}

	writer.Close()

	msg := []byte(headers + buf.String())

	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	return smtp.SendMail(addr, auth, cfg.SMTPUser, []string{cfg.EmailTo}, msg)
}
