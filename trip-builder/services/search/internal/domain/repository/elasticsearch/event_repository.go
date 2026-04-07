package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

const (
	EventsIndexName = "events"
)

type EventRepository struct {
	es  *ESClient
	log *logger.Logger
	cfg *struct{ MaxResultWindow int }
}

func NewEventRepository(es *ESClient, log *logger.Logger, maxResultWindow int) *EventRepository {
	return &EventRepository{
		es:  es,
		log: log,
		cfg: &struct{ MaxResultWindow int }{MaxResultWindow: maxResultWindow},
	}
}

func (r *EventRepository) getMapping() string {
	return `{
		"settings": {
			"number_of_shards": 1,
			"number_of_replicas": 0,
			"analysis": {
				"analyzer": {
					"russian_english": {
						"type": "custom",
						"tokenizer": "standard",
						"filter": ["lowercase", "russian_stop", "english_stop", "russian_stemmer", "english_stemmer"]
					}
				},
				"filter": {
					"russian_stop": {
						"type": "stop",
						"stopwords": "_russian_"
					},
					"english_stop": {
						"type": "stop",
						"stopwords": "_english_"
					},
					"russian_stemmer": {
						"type": "stemmer",
						"language": "russian"
					},
					"english_stemmer": {
						"type": "stemmer",
						"language": "english"
					}
				}
			},
			"index": {
				"max_result_window": ` + fmt.Sprintf("%d", r.cfg.MaxResultWindow) + `
			}
		},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"title": {
					"type": "text",
					"analyzer": "russian_english",
					"fields": {
						"keyword": {"type": "keyword"},
						"ru": {"type": "text", "analyzer": "russian"},
						"en": {"type": "text", "analyzer": "english"}
					}
				},
				"route_id": {"type": "keyword"},
				"start_date": {"type": "date"},
				"status": {"type": "keyword"},
				"participant_ids": {"type": "keyword"},
				"is_public": {"type": "boolean"},
				"suggest": {
					"type": "completion",
					"analyzer": "simple",
					"preserve_separators": true,
					"preserve_position_increments": true,
					"max_input_length": 50
				}
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

	r.log.Debug("Индекс events уже существует")
	return nil
}

func (r *EventRepository) IndexDocument(ctx context.Context, doc *models.EventDocument) error {
	// Индексируем только публичные поездки
	if !doc.IsPublic {
		r.log.Trace("Событие не публичное, пропускаем индексацию", "event_id", doc.ID)
		return nil
	}
	r.log.Trace("Индексация события", "event_id", doc.ID)
	return r.es.IndexDocument(ctx, EventsIndexName, doc.ID, doc)
}

func (r *EventRepository) DeleteDocument(ctx context.Context, id string) error {
	r.log.Trace("Удаление события из индекса", "event_id", id)
	return r.es.DeleteDocument(ctx, EventsIndexName, id)
}

func (r *EventRepository) Search(ctx context.Context, req *models.EventSearchRequest) (*models.EventSearchResponse, error) {
	r.log.Debug("Поиск событий", "query", req.Query, "page", req.Page, "size", req.Size)

	query := r.buildQuery(req)

	from := (req.Page - 1) * req.Size
	if from < 0 {
		from = 0
	}
	if from > r.cfg.MaxResultWindow {
		from = r.cfg.MaxResultWindow - req.Size
	}

	body := map[string]interface{}{
		"query": query,
		"from":  from,
		"size":  req.Size,
		"sort": []map[string]interface{}{
			{"start_date": map[string]string{"order": "desc"}},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"title": map[string]interface{}{"number_of_fragments": 1},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	res, err := r.es.client.Search(
		r.es.client.Search.WithContext(ctx),
		r.es.client.Search.WithIndex(EventsIndexName),
		r.es.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var esResponse struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID        string              `json:"_id"`
				Score     float64             `json:"_score"`
				Source    json.RawMessage     `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	items := make([]models.EventHit, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var doc models.EventDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			r.log.Error("Ошибка парсинга документа", "error", err)
			continue
		}

		items = append(items, models.EventHit{
			ID:         hit.ID,
			Title:      doc.Title,
			RouteID:    doc.RouteID,
			StartDate:  doc.StartDate,
			Status:     doc.Status,
			Score:      hit.Score,
			Highlights: hit.Highlight,
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

func (r *EventRepository) buildQuery(req *models.EventSearchRequest) map[string]interface{} {
	must := []map[string]interface{}{}

	// Только публичные поездки
	must = append(must, map[string]interface{}{
		"term": map[string]interface{}{
			"is_public": true,
		},
	})

	// Фильтр по статусу (по умолчанию completed)
	status := req.Status
	if status == "" {
		status = "completed"
	}
	must = append(must, map[string]interface{}{
		"term": map[string]interface{}{
			"status": status,
		},
	})

	// Полнотекстовый поиск
	if req.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     req.Query,
				"fields":    []string{"title^2"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
			},
		})
	}

	// Фильтр по дате начала
	if req.StartDateFrom != nil || req.StartDateTo != nil {
		rangeFilter := map[string]interface{}{}
		if req.StartDateFrom != nil {
			rangeFilter["gte"] = req.StartDateFrom
		}
		if req.StartDateTo != nil {
			rangeFilter["lte"] = req.StartDateTo
		}
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{
				"start_date": rangeFilter,
			},
		})
	}

	return map[string]interface{}{
		"bool": map[string]interface{}{
			"must": must,
		},
	}
}

func (r *EventRepository) Suggest(ctx context.Context, query string, size int) ([]models.Suggestion, error) {
	r.log.Debug("Автодополнение для событий", "query", query)

	body := map[string]interface{}{
		"suggest": map[string]interface{}{
			"event-suggest": map[string]interface{}{
				"prefix": query,
				"completion": map[string]interface{}{
					"field": "suggest",
					"size":  size,
				},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := r.es.client.Search(
		r.es.client.Search.WithContext(ctx),
		r.es.client.Search.WithIndex(EventsIndexName),
		r.es.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var esResponse struct {
		Suggest map[string][]struct {
			Options []struct {
				Text  string  `json:"text"`
				Score float64 `json:"_score"`
			} `json:"options"`
		} `json:"suggest"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, err
	}

	suggestions := []models.Suggestion{}
	if suggests, ok := esResponse.Suggest["event-suggest"]; ok && len(suggests) > 0 {
		for _, opt := range suggests[0].Options {
			suggestions = append(suggestions, models.Suggestion{
				Text:  opt.Text,
				Score: opt.Score,
				Type:  "event",
			})
		}
	}

	return suggestions, nil
}

func (r *EventRepository) ReindexAll(ctx context.Context, events []*models.EventDocument) error {
	r.log.Info("Переиндексация событий", "count", len(events))

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
