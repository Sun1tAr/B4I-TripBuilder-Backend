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
	UsersIndexName = "users"
)

type UserRepository struct {
	es  *ESClient
	log *logger.Logger
	cfg *struct{ MaxResultWindow int }
}

func NewUserRepository(es *ESClient, log *logger.Logger, maxResultWindow int) *UserRepository {
	return &UserRepository{
		es:  es,
		log: log,
		cfg: &struct{ MaxResultWindow int }{MaxResultWindow: maxResultWindow},
	}
}

func (r *UserRepository) getMapping() string {
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
				"first_name": {
					"type": "text",
					"analyzer": "russian_english",
					"fields": {
						"keyword": {"type": "keyword"}
					}
				},
				"last_name": {
					"type": "text",
					"analyzer": "russian_english",
					"fields": {
						"keyword": {"type": "keyword"}
					}
				},
				"bio": {
					"type": "text",
					"analyzer": "russian_english"
				},
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

func (r *UserRepository) EnsureIndex(ctx context.Context) error {
	exists, err := r.es.IndexExists(ctx, UsersIndexName)
	if err != nil {
		return err
	}

	if !exists {
		r.log.Info("Создание индекса users")
		return r.es.CreateIndex(ctx, UsersIndexName, r.getMapping())
	}

	r.log.Debug("Индекс users уже существует")
	return nil
}

func (r *UserRepository) IndexDocument(ctx context.Context, doc *models.UserDocument) error {
	r.log.Trace("Индексация пользователя", "user_id", doc.ID)
	return r.es.IndexDocument(ctx, UsersIndexName, doc.ID, doc)
}

func (r *UserRepository) DeleteDocument(ctx context.Context, id string) error {
	r.log.Trace("Удаление пользователя из индекса", "user_id", id)
	return r.es.DeleteDocument(ctx, UsersIndexName, id)
}

func (r *UserRepository) Search(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	r.log.Debug("Поиск пользователей", "query", req.Query, "page", req.Page, "size", req.Size)

	from := (req.Page - 1) * req.Size
	if from < 0 {
		from = 0
	}
	if from > r.cfg.MaxResultWindow {
		from = r.cfg.MaxResultWindow - req.Size
	}

	body := map[string]interface{}{
		"from": from,
		"size": req.Size,
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"first_name": map[string]interface{}{"number_of_fragments": 1},
				"last_name":  map[string]interface{}{"number_of_fragments": 1},
				"bio":        map[string]interface{}{"number_of_fragments": 1},
			},
		},
	}

	if req.Query != "" {
		body["query"] = map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     req.Query,
				"fields":    []string{"first_name^3", "last_name^3", "bio"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
			},
		}
	} else {
		body["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	res, err := r.es.client.Search(
		r.es.client.Search.WithContext(ctx),
		r.es.client.Search.WithIndex(UsersIndexName),
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

	items := make([]models.UserHit, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var doc models.UserDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			r.log.Error("Ошибка парсинга документа", "error", err)
			continue
		}

		items = append(items, models.UserHit{
			ID:         hit.ID,
			FirstName:  doc.FirstName,
			LastName:   doc.LastName,
			Bio:        doc.Bio,
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

	return &models.UserSearchResponse{
		Items:      items,
		Total:      esResponse.Hits.Total.Value,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *UserRepository) Suggest(ctx context.Context, query string, size int) ([]models.Suggestion, error) {
	r.log.Debug("Автодополнение для пользователей", "query", query)

	body := map[string]interface{}{
		"suggest": map[string]interface{}{
			"user-suggest": map[string]interface{}{
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
		r.es.client.Search.WithIndex(UsersIndexName),
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
	if suggests, ok := esResponse.Suggest["user-suggest"]; ok && len(suggests) > 0 {
		for _, opt := range suggests[0].Options {
			suggestions = append(suggestions, models.Suggestion{
				Text:  opt.Text,
				Score: opt.Score,
				Type:  "user",
			})
		}
	}

	return suggestions, nil
}

func (r *UserRepository) ReindexAll(ctx context.Context, users []*models.UserDocument) error {
	r.log.Info("Переиндексация пользователей", "count", len(users))

	if err := r.es.DeleteIndex(ctx, UsersIndexName); err != nil {
		r.log.Error("Ошибка удаления индекса users", "error", err)
	}

	if err := r.EnsureIndex(ctx); err != nil {
		return err
	}

	docs := make(map[string]interface{}, len(users))
	for _, user := range users {
		docs[user.ID] = user
	}

	return r.es.BulkIndex(ctx, UsersIndexName, docs)
}
