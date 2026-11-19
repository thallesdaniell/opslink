package incidentio

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

type AlertAttributeV2 struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AlertAttributeValueV2 struct {
	CatalogEntry *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"catalog_entry,omitempty"`
	Literal string `json:"literal,omitempty"`
}

type AlertAttributeEntry struct {
	Attribute  AlertAttributeV2        `json:"attribute"`
	Value      *AlertAttributeValueV2  `json:"value,omitempty"`
	ArrayValue []AlertAttributeValueV2 `json:"array_value,omitempty"`
}

type Alert struct {
	ID         string                `json:"id"`
	Status     string                `json:"status"`
	Title      string                `json:"title"`
	SourceURL  string                `json:"source_url,omitempty"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
	Attributes []AlertAttributeEntry `json:"attributes"`
}

type AlertsListResponse struct {
	Alerts         []Alert `json:"alerts"`
	PaginationMeta struct {
		After string `json:"after"`
	} `json:"pagination_meta"`
}

type AlertListFilter struct {
	Statuses    []string
	CreatedGte  time.Time
	AttributeID string
	CatalogIDs  []string
}

func (a Alert) TeamName(attributeID string) string {
	for _, entry := range a.Attributes {
		if entry.Attribute.ID != attributeID {
			continue
		}
		if entry.Value != nil && entry.Value.CatalogEntry != nil && entry.Value.CatalogEntry.Name != "" {
			return entry.Value.CatalogEntry.Name
		}
		for _, value := range entry.ArrayValue {
			if value.CatalogEntry != nil && value.CatalogEntry.Name != "" {
				return value.CatalogEntry.Name
			}
		}
	}
	return "-"
}

func (c *IncidentIoClient) ListAllAlerts(filter AlertListFilter) ([]Alert, error) {
	params := url.Values{}
	params.Set("page_size", "50")
	params.Set("include_maintenance_window[is]", "false")

	for _, status := range filter.Statuses {
		params.Add("status[one_of]", status)
	}

	if !filter.CreatedGte.IsZero() {
		params.Set("created_at[gte]", filter.CreatedGte.Format("2006-01-02"))
	}

	var all []Alert

	for {
		path := "/alerts?" + params.Encode()
		body, err := c.makeRequest("GET", path, nil)
		if err != nil {
			logrus.WithField("service", "incidentio").Error("Erro ao listar alerts:", err)
			return nil, err
		}

		var response AlertsListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			logrus.WithField("service", "incidentio").Error("Erro ao decodificar alerts:", err)
			return nil, err
		}

		all = append(all, response.Alerts...)

		if response.PaginationMeta.After == "" {
			break
		}
		params.Set("after", response.PaginationMeta.After)
	}

	logrus.WithField("service", "incidentio").Infof("Listados %d alerts (bruto da API)", len(all))
	return all, nil
}

func FilterAlertsByTeam(alerts []Alert, attributeID string, catalogIDs []string) []Alert {
	if attributeID == "" || len(catalogIDs) == 0 {
		return alerts
	}

	allowed := make(map[string]struct{}, len(catalogIDs))
	for _, id := range catalogIDs {
		allowed[id] = struct{}{}
	}

	filtered := make([]Alert, 0, len(alerts))
	for _, alert := range alerts {
		if alertMatchesTeam(alert, attributeID, allowed) {
			filtered = append(filtered, alert)
		}
	}

	logrus.WithField("service", "incidentio").Infof("Restaram %d alerts após filtro de Team", len(filtered))
	return filtered
}

func alertMatchesTeam(alert Alert, attributeID string, allowed map[string]struct{}) bool {
	for _, entry := range alert.Attributes {
		if entry.Attribute.ID != attributeID {
			continue
		}
		if entry.Value != nil && entry.Value.CatalogEntry != nil {
			if _, ok := allowed[entry.Value.CatalogEntry.ID]; ok {
				return true
			}
		}
		for _, value := range entry.ArrayValue {
			if value.CatalogEntry != nil {
				if _, ok := allowed[value.CatalogEntry.ID]; ok {
					return true
				}
			}
		}
	}
	return false
}
