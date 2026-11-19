package incidentio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	BaseURL = "https://api.incident.io/v2"
)

type Schedule struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Timezone             string          `json:"timezone"`
	TeamIDs              []string        `json:"team_ids,omitempty"`
	HolidaysPublicConfig interface{}     `json:"holidays_public_config,omitempty"`
	CurrentShifts        []CurrentShifts `json:"current_shifts,omitempty"`
	Config               *Config         `json:"config,omitempty"`
	CreatedAt            time.Time       `json:"created_at,omitempty"`
	UpdatedAt            time.Time       `json:"updated_at,omitempty"`
	Annotations          interface{}     `json:"annotations,omitempty"`
}

type CurrentShifts struct {
	RotationID  string    `json:"rotation_id"`
	User        User      `json:"user"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	Fingerprint string    `json:"fingerprint"`
}

type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	SlackUserID string `json:"slack_user_id"`
	Role        string `json:"role"`
}

type Config struct {
	Rotations []Rotation `json:"rotations,omitempty"`
}

type Rotation struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Layers           []Layer           `json:"layers,omitempty"`
	Users            []User            `json:"users,omitempty"`
	WorkingInterval  []WorkingInterval `json:"working_interval,omitempty"`
	WorkingIntervals []WorkingInterval `json:"working_intervals,omitempty"`
	HandoverStartAt  time.Time         `json:"handover_start_at,omitempty"`
	Handovers        []Handover        `json:"handovers,omitempty"`
	SchedulingMode   string            `json:"scheduling_mode,omitempty"`
}

type Layer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type WorkingInterval struct {
	Weekday   string `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type Handover struct {
	IntervalType string `json:"interval_type"`
	Interval     int    `json:"interval"`
}

type IncidentIoClient struct {
	ApiKey  string
	BaseURL string
	Client  *http.Client
}

type SchedulesResponse struct {
	Schedules []Schedule `json:"schedules"`
}

type ScheduleResponse struct {
	Schedule Schedule `json:"schedule"`
}

type ScheduleEntry struct {
	RotationID  string    `json:"rotation_id"`
	User        User      `json:"user"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	Fingerprint string    `json:"fingerprint"`
}

type ScheduleEntriesResponse struct {
	ScheduleEntries struct {
		Scheduled []ScheduleEntry `json:"scheduled"`
		Overrides []ScheduleEntry `json:"overrides"`
		Final     []ScheduleEntry `json:"final"`
	} `json:"schedule_entries"`
}

func Client(ApiKey string) (*IncidentIoClient, error) {
	if ApiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	return &IncidentIoClient{
		ApiKey:  ApiKey,
		BaseURL: BaseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *IncidentIoClient) makeRequest(method, path string, payload interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("erro ao serializar payload: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao fazer requisição: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("erro na API: %d - %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

func (c *IncidentIoClient) ListSchedules() ([]Schedule, error) {
	body, err := c.makeRequest("GET", "/schedules", nil)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao listar schedules:", err)
		return nil, err
	}

	var response SchedulesResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao decodificar JSON:", err)
		return nil, err
	}

	logrus.WithField("service", "incidentio").Infof("Listados %d schedules", len(response.Schedules))
	return response.Schedules, nil
}

func (c *IncidentIoClient) GetScheduleWithCurrentShifts(scheduleID string) (*Schedule, error) {
	path := fmt.Sprintf("/schedules/%s", scheduleID)
	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao buscar schedule:", err)
		return nil, err
	}

	var response ScheduleResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao decodificar JSON:", err)
		return nil, err
	}

	logrus.WithField("service", "incidentio").Infof("Schedule %s recuperado com %s",
		response.Schedule.Name, currentShiftEmail(response.Schedule.CurrentShifts))

	return &response.Schedule, nil
}

func currentShiftEmail(shifts []CurrentShifts) string {
	if len(shifts) == 0 {
		return "(sem plantão atual)"
	}
	return shifts[0].User.Email
}

func (c *IncidentIoClient) GetScheduleEntries(scheduleID, startTime, endTime string) (*ScheduleEntriesResponse, error) {
	path := fmt.Sprintf("/schedule_entries?schedule_id=%s&entry_window_start=%s&entry_window_end=%s",
		scheduleID, startTime, endTime)

	body, err := c.makeRequest("GET", path, nil)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao buscar schedule entries:", err)
		return nil, err
	}

	var response ScheduleEntriesResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		logrus.WithField("service", "incidentio").Error("Erro ao decodificar JSON:", err)
		return nil, err
	}

	logrus.WithField("service", "incidentio").Infof("Schedule entries recuperadas: %d scheduled, %d overrides, %d final",
		len(response.ScheduleEntries.Scheduled), len(response.ScheduleEntries.Overrides), len(response.ScheduleEntries.Final))

	return &response, nil
}
