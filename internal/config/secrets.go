package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const (
	jiraTokenKey    = "jira_api_token"
	mailPasswordKey = "ionos_imap_password"
)

// ErrSecretNotFound is returned when a secret is missing from keyring and environment.
var ErrSecretNotFound = errors.New("secret not found")

// SetJiraToken stores the Jira API token in the OS keyring.
func SetJiraToken(token string) error {
	return setSecret(jiraTokenKey, token)
}

// JiraToken returns the Jira API token from environment or the OS keyring.
func JiraToken() (string, error) {
	if token := os.Getenv("LAZYJIRA_JIRA_TOKEN"); token != "" {
		return token, nil
	}
	return getSecret(jiraTokenKey)
}

// SetMailPassword stores the IONOS IMAP password in the OS keyring.
func SetMailPassword(password string) error {
	return setSecret(mailPasswordKey, password)
}

// MailPassword returns the IONOS IMAP password from environment or the OS keyring.
func MailPassword() (string, error) {
	if password := os.Getenv("LAZYJIRA_MAIL_PASSWORD"); password != "" {
		return password, nil
	}
	return getSecret(mailPasswordKey)
}

func setSecret(key, value string) error {
	if value == "" {
		return nil
	}
	if err := keyring.Set(AppName, key, value); err != nil {
		return fmt.Errorf("store %s in keyring: %w", key, err)
	}
	return nil
}

func getSecret(key string) (string, error) {
	value, err := keyring.Get(AppName, key)
	if err == nil {
		return value, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrSecretNotFound
	}
	return "", fmt.Errorf("read %s from keyring: %w", key, err)
}
