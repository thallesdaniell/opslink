package gsheets

import (
	"context"
	"os"
	"strings"
	"time"

	"opslink/internal/gmail"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const DefaultRange = "Emails!A2:F"

type Client struct {
	service *sheets.Service
}

func Service(serviceAccount string) (*Client, error) {
	creds, err := os.ReadFile(serviceAccount)
	if err != nil {
		logrus.WithField("service", "sheets").Error("Falha ao ler o arquivo de credenciais", err)
		return nil, err
	}

	config, err := google.JWTConfigFromJSON(creds, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		logrus.WithField("service", "sheets").Error("Falha ao criar configuração OAuth2", err)
		return nil, err
	}

	ctx := context.Background()
	service, err := sheets.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		logrus.WithField("service", "sheets").Error("Não foi possível criar o cliente Sheets", err)
		return nil, err
	}

	return &Client{service: service}, nil
}

func (c *Client) ReadMessages(spreadsheetID, readRange string) ([]gmail.Message, error) {
	if strings.TrimSpace(readRange) == "" {
		readRange = DefaultRange
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.service.Spreadsheets.Values.Get(spreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		logrus.WithField("service", "sheets").Errorf(
			"Erro ao ler a planilha (compartilhe o Sheet com o client_email da SA e habilite Google Sheets API): %v",
			err,
		)
		return nil, err
	}

	messages := gmail.ParseRows(resp.Values)
	logrus.WithField("service", "sheets").Infof("Emails lidos da planilha: %d", len(messages))
	return messages, nil
}
