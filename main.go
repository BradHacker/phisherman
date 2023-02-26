package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mileusna/useragent"
	"github.com/sirupsen/logrus"
)

type DiscordEmbedFields struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DiscordEmbed struct {
	Title       string               `json:"title,omitempty"`
	Type        string               `json:"type,omitempty"` // Always "rich" for webhook embeds
	Description string               `json:"description,omitempty"`
	Url         string               `json:"url,omitempty"`
	Color       string               `json:"color,omitempty"`
	Fields      []DiscordEmbedFields `json:"fields,omitempty"`
	Timestamp   string               `json:"timestamp,omitempty"`
}

type DiscordAllowedMentions struct {
	Parse       []string `json:"parse"`
	Users       []string `json:"users"`
	Roles       []string `json:"roles"`
	RepliedUser bool     `json:"replied_user"`
}

type DiscordWebhook struct {
	Content         string                 `json:"content,omitempty"`
	Embeds          []DiscordEmbed         `json:"embeds,omitempty"`
	AllowedMentions DiscordAllowedMentions `json:"allowed_mentions,omitempty"`
}

type PhishermanConfig struct {
	DiscordWebhookUrl string `json:"discord_webhook_url"`
	DiscordUserId     string `json:"discord_user_id"`
	RoutePath         string `json:"route_path"`
	RedirectUrl       string `json:"redirect_url"`
	IdQueryParam      string `json:"id_query_param"`
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})

	configBytes, err := os.ReadFile("config.json")
	if err != nil {
		logrus.Errorf("failed to read config.json: %v", err)
		return
	}
	var config PhishermanConfig
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		logrus.Errorf("failed to unmarshal config: %v", err)
		return
	}

	r := gin.Default()

	accessLogger := logrus.New()
	logFile, err := os.OpenFile("access.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		accessLogger.SetOutput(logFile)
	} else {
		logrus.Errorf("failed to set access log output file: %v", err)
		return
	}

	r.GET(config.RoutePath, func(c *gin.Context) {
		trackingId := c.DefaultQuery(config.IdQueryParam, "NOID")

		remoteIps, _ := c.Request.Header["X-Forwarded-For"]
		remoteIp := strings.Join(remoteIps, ",")

		if reqUserAgent, exists := c.Request.Header["User-Agent"]; exists {
			ua := useragent.Parse(strings.Join(reqUserAgent, " "))
			accessLogger.WithFields(logrus.Fields{
				"Bot?":        ua.Bot,
				"Desktop?":    ua.Desktop,
				"Mobile?":     ua.Mobile,
				"Tablet?":     ua.Tablet,
				"Device":      ua.Device,
				"Name":        ua.Name,
				"OS":          ua.OS,
				"OS Version":  ua.OSVersion,
				"URL":         ua.URL,
				"Version":     ua.Version,
				"RemoteIP":    remoteIp,
				"Tracking ID": trackingId,
			}).Println("tracking page accessed")

			embedWebhook := DiscordWebhook{
				Embeds: []DiscordEmbed{{
					Title:       "Page Access",
					Type:        "rich",
					Description: fmt.Sprintf("<@%s> Someone has accessed the tracking page", config.DiscordUserId),
					Timestamp:   time.Now().UTC().Format("2006-01-02T15:04:05-0700"),
					Fields: []DiscordEmbedFields{
						{
							Name:  "Bot?",
							Value: fmt.Sprintf("%t", ua.Bot),
						},
						{
							Name:  "Desktop?",
							Value: fmt.Sprintf("%t", ua.Desktop),
						},
						{
							Name:  "Mobile?",
							Value: fmt.Sprintf("%t", ua.Mobile),
						},
						{
							Name:  "Tablet?",
							Value: fmt.Sprintf("%t", ua.Tablet),
						},
						{
							Name:  "Device",
							Value: ua.Device,
						},
						{
							Name:  "Browser Name",
							Value: ua.Name,
						},
						{
							Name:  "Browser Version",
							Value: ua.Version,
						},
						{
							Name:  "OS",
							Value: ua.OS,
						},
						{
							Name:  "OS Version",
							Value: ua.Version,
						},
						{
							Name:  "URL",
							Value: c.Request.RequestURI,
						},
						{
							Name:  "RemoteIP",
							Value: remoteIp,
						},
						{
							Name:  "Tracking ID",
							Value: trackingId,
						},
					},
				}},
				AllowedMentions: DiscordAllowedMentions{
					Parse:       []string{"users"},
					Users:       []string{},
					Roles:       []string{},
					RepliedUser: false,
				},
			}
			pingWebhook := DiscordWebhook{
				Content: fmt.Sprintf("<@%s> Someone has accessed the tracking page", config.DiscordUserId),
				AllowedMentions: DiscordAllowedMentions{
					Parse:       []string{"users"},
					Users:       []string{},
					Roles:       []string{},
					RepliedUser: false,
				},
			}

			SendWebhook(config.DiscordWebhookUrl, &embedWebhook)
			SendWebhook(config.DiscordWebhookUrl, &pingWebhook)
		}

		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s?%s=%s", config.RedirectUrl, config.IdQueryParam, trackingId))
	})

	err = r.Run(":8080")
	if err != nil {
		logrus.Errorf("failed to start listener: %v", err)
	}
}

func SendWebhook(webhookUrl string, webhook *DiscordWebhook) {
	webClient := &http.Client{}

	webhookBody, err := json.Marshal(webhook)
	if err != nil {
		logrus.Errorf("failed to marshal webhook: %v", err)
	} else {
		logrus.Infof("%s", webhookBody)
		res, err := webClient.Post(webhookUrl, "application/json", bytes.NewReader(webhookBody))
		if err != nil {
			logrus.Errorf("failed to send webhook: %v", err)
		} else {
			defer res.Body.Close()
			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				logrus.Errorf("failed to read webhook res body: %v", err)
			} else {
				logrus.WithField("status", res.Status).Warnf("res body: %s", resBody)
			}
		}
	}
}
