package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

const EventsIndexName = "events"

type EventRepository struct {
	es  *Client
	log *logger.Logger
	cfg *struct{ MaxResultWindow int }
}

func NewEventRepository(es *Client, log *logger.Logger, maxResultWindow int) *EventRepository {
	return &EventRepository{
		es:  es,
		log: log,
		cfg: &struct{ MaxResultWindow int }{MaxResultWindow: maxResultWindow},
	}
}

func (r *EventRepository) getMapping() string {
	return `{
		"settings": {"number_of_shards": 1, "number_of_replicas": 0, "index": {"max_result_window": ` + fmt.Sprintf("%d", r.cfg.MaxResultWindow) + `}},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"title": {"type": "text", "analyzer": "russian", "fields": {"keyword": {"type": "keyword"}}},
				"route_id": {"type": "keyword"},
				"start_date": {"type": "date"},
				"status": {"type": "keyword"},
				"participant_ids": {"type": "keyword"},
				"is_public": {"type": "boolean"}
			}
		}
	}`
}

func (r *EventRepository) EnsureIndex(ctx context.Context) error {
	exists, err := r.es.IndexExists(ctx, EventsIndexName)
	if err != nil {
		return err
	}
	if !exists {
		r.log.Info("Создание индекса events")
		return r.es.CreateIndex(ctx, EventsIndexName, r.getMapping())
	}
	return nil
}

func (r *EventRepository) Index(ctx context.Context, doc *models.EventDocument) error {
	if !doc.IsPublic {
		return nil
	}
	return r.es.IndexDocument(ctx, EventsIndexName, doc.ID, doc)
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	return r.es.DeleteDocument(ctx, EventsIndexName, id)
}

func (r *EventRepository) Search(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	must := []map[string]interface{}{
		{"term": map[string]interface{}{"is_public": true}},
		{"term": map[string]interface{}{"status": "completed"}},
	}

	if req.Query != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{"title": req.Query},
		})
	}

	query := map[string]interface{}{"bool": map[string]interface{}{"must": must}}
	from := (req.Page - 1) * req.Size
	if from < 0 {
		from = 0
	}

	body := map[string]interface{}{
		"query": query,
		"from":  from,
		"size":  req.Size,
		"sort":  []map[string]interface{}{{"start_date": map[string]string{"order": "desc"}}},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := r.es.GetClient().Search(
		r.es.GetClient().Search.WithContext(ctx),
		r.es.GetClient().Search.WithIndex(EventsIndexName),
		r.es.GetClient().Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var esResponse struct {
		Hits struct {
			Total struct{ Value int64 } `json:"total"`
			Hits  []struct {
				ID     string          `json:"_id"`
				Score  float64         `json:"_score"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, err
	}

	items := make([]models.EventHit, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var doc models.EventDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}
		items = append(items, models.EventHit{
			ID:        hit.ID,
			Title:     doc.Title,
			RouteID:   doc.RouteID,
			StartDate: doc.StartDate,
			Status:    doc.Status,
			Score:     hit.Score,
		})
	}

	totalPages := int(esResponse.Hits.Total.Value) / req.Size
	if int(esResponse.Hits.Total.Value)%req.Size > 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.EventSearchResponse{
		Items:      items,
		Total:      esResponse.Hits.Total.Value,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *EventRepository) ReindexAll(ctx context.Context, events []*models.EventDocument) error {
	r.log.Info("Переиндексация событий в ES", "count", len(events))

	if err := r.es.DeleteIndex(ctx, EventsIndexName); err != nil {
		r.log.Error("Ошибка удаления индекса events", "error", err)
	}
	if err := r.EnsureIndex(ctx); err != nil {
		return err
	}

	docs := make(map[string]interface{}, len(events))
	for _, event := range events {
		if event.IsPublic {
			docs[event.ID] = event
		}
	}
	return r.es.BulkIndex(ctx, EventsIndexName, docs)
}
