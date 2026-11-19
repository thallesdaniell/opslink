package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultModel   = "gemini-2.5-flash"
	defaultTimeout = 90 * time.Second
	maxCritico     = 4
	maxRelevante   = 3
	maxAcoes       = 3
)

type Item struct {
	Title   string `json:"titulo"`
	Detail  string `json:"detalhe"`
	Link    string `json:"link"`
	Product string `json:"produto"`
}

func (i *Item) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		i.Title = strings.TrimSpace(s)
		return nil
	}

	var aux struct {
		Titulo  string `json:"titulo"`
		Title   string `json:"title"`
		Detalhe string `json:"detalhe"`
		Detail  string `json:"detail"`
		Link    string `json:"link"`
		Produto string `json:"produto"`
		Product string `json:"product"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	i.Title = firstNonEmpty(aux.Titulo, aux.Title)
	i.Detail = firstNonEmpty(aux.Detalhe, aux.Detail)
	i.Link = strings.TrimSpace(aux.Link)
	i.Product = firstNonEmpty(aux.Produto, aux.Product)
	return nil
}

type Report struct {
	Critico    []Item `json:"critico"`
	Relevante  []Item `json:"relevante"`
	Acoes      []Item `json:"acoes"`
	Resolvidos int    `json:"resolvidos"`
}

type Client struct {
	apiKey string
	model  string
	http   *http.Client
}

func New(apiKey, model string) *Client {
	if strings.TrimSpace(model) == "" {
		model = defaultModel
	}
	return &Client{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) SummarizeWeekly(emails string) (*Report, error) {
	payload := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": weeklyPrompt(emails)}}},
		},
		"generationConfig": map[string]any{
			"temperature": 0.2,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		logrus.WithField("service", "gemini").Error(err)
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		logrus.WithField("service", "gemini").Errorf("HTTP %d: %s", resp.StatusCode, truncateForLog(string(raw)))
		return nil, fmt.Errorf("gemini HTTP %d", resp.StatusCode)
	}

	text, err := responseText(raw)
	if err != nil {
		logrus.WithField("service", "gemini").Error(err)
		return nil, err
	}

	report, err := ParseReport(text)
	if err != nil {
		logrus.WithField("service", "gemini").Errorf("JSON inválido: %v", err)
		return nil, err
	}
	return report, nil
}

func ParseReport(text string) (*Report, error) {
	raw, err := extractJSONObject(text)
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return nil, err
	}
	normalizeReport(&report)
	return &report, nil
}

func normalizeReport(report *Report) {
	report.Critico = capItems(cleanItems(report.Critico), maxCritico)
	report.Relevante = capItems(cleanItems(report.Relevante), maxRelevante)
	report.Acoes = capItems(cleanItems(report.Acoes), maxAcoes)
	if report.Resolvidos < 0 {
		report.Resolvidos = 0
	}
}

func cleanItems(items []Item) []Item {
	out := make([]Item, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		item.Title = strings.Join(strings.Fields(item.Title), " ")
		item.Detail = strings.Join(strings.Fields(item.Detail), " ")
		item.Product = strings.Join(strings.Fields(item.Product), " ")
		item.Link = strings.TrimSpace(item.Link)
		if item.Title == "" {
			continue
		}
		key := strings.ToLower(item.Title)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func capItems(items []Item, n int) []Item {
	if n <= 0 || len(items) <= n {
		return items
	}
	return items[:n]
}

func extractJSONObject(text string) (string, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return "", fmt.Errorf("objeto JSON não encontrado")
	}
	return text[start : end+1], nil
}

func responseText(raw []byte) (string, error) {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("%s", parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini não retornou texto")
	}
	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func truncateForLog(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

func weeklyPrompt(emails string) string {
	return `Você é um SRE gerando um resumo operacional SEMANAL a partir de comunicações oficiais da Google Cloud e Kubernetes.

Objetivo: um briefing curto para o canal de plantão. Não é um dump de emails.

Responda APENAS JSON válido neste formato:
{
  "critico": [{"titulo":"...","detalhe":"...","link":"...","produto":"GKE"}],
  "relevante": [{"titulo":"...","detalhe":"...","link":"...","produto":"Networking"}],
  "acoes": [{"titulo":"...","detalhe":"...","link":"...","produto":"IAM"}],
  "resolvidos": 0
}

Limites:
- critico: no máximo 4 itens (indisponibilidade, produção, IP, certificado, rede, IAM quebrado, perda de logs)
- relevante: no máximo 3 itens (manutenção com impacto possível, degradação, follow-up)
- acoes: no máximo 3 itens, só se o time SRE precisa fazer algo verificável
- resolvidos: CONTAGEM dos incidentes/avisos já encerrados na semana. NÃO listar "foi resolvido"

Regras:
- Não inventar eventos
- Não repetir o mesmo incidente em mais de uma lista
- Se o incidente abriu e fechou na mesma semana e não sobrou ação, some em resolvidos e omita das listas
- Se ainda precisa de ação, vá em critico/relevante + acoes
- titulo: uma linha, até 140 caracteres, técnico e objetivo
- detalhe: opcional, até 80 caracteres (região, serviço, impacto)
- produto: nome curto (GKE, IAM, Pub/Sub, Networking, GCE, Storage, Logging)
- link: URL http(s) extraída dos emails (status.cloud.google.com, console ou Gmail). String vazia se não houver
- Sem markdown, sem introdução, sem textos institucionais
- Português

Priorize: GKE, Kubernetes, Networking, Load Balancer, IP, certificados, Pub/Sub, IAM, Storage, GCE, incidentes, manutenções.

Emails:
` + emails
}
