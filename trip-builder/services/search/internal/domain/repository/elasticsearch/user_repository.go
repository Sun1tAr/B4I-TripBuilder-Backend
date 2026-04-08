package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

const UsersIndexName = "users"

type UserRepository struct {
	es  *Client
	log *logger.Logger
	cfg *struct{ MaxResultWindow int }
}

func NewUserRepository(es *Client, log *logger.Logger, maxResultWindow int) *UserRepository {
	return &UserRepository{
		es:  es,
		log: log,
		cfg: &struct{ MaxResultWindow int }{MaxResultWindow: maxResultWindow},
	}
}

func (r *UserRepository) getMapping() string {
	return `{
		"settings": {"number_of_shards": 1, "number_of_replicas": 0, "index": {"max_result_window": ` + fmt.Sprintf("%d", r.cfg.MaxResultWindow) + `}},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"first_name": {"type": "text", "analyzer": "russian", "fields": {"keyword": {"type": "keyword"}}},
				"last_name": {"type": "text", "analyzer": "russian", "fields": {"keyword": {"type": "keyword"}}},
				"bio": {"type": "text", "analyzer": "russian"}
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
	return nil
}

func (r *UserRepository) Index(ctx context.Context, doc *models.UserDocument) error {
	return r.es.IndexDocument(ctx, UsersIndexName, doc.ID, doc)
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return r.es.DeleteDocument(ctx, UsersIndexName, id)
}

func (r *UserRepository) Search(ctx context.Context, req *models.UserSearchRequest) (*models.UserSearchResponse, error) {
	query := map[string]interface{}{
		"multi_match": map[string]interface{}{
			"query":     req.Query,
			"fields":    []string{"first_name^3", "last_name^3", "bio"},
			"fuzziness": "AUTO",
		},
	}

	from := (req.Page - 1) * req.Size
	if from < 0 {
		from = 0
	}

	body := map[string]interface{}{
		"query": query,
		"from":  from,
		"size":  req.Size,
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"first_name": map[string]interface{}{"number_of_fragments": 1},
				"last_name":  map[string]interface{}{"number_of_fragments": 1},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := r.es.GetClient().Search(
		r.es.GetClient().Search.WithContext(ctx),
		r.es.GetClient().Search.WithIndex(UsersIndexName),
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
				ID        string              `json:"_id"`
				Score     float64             `json:"_score"`
				Source    json.RawMessage     `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, err
	}

	items := make([]models.UserHit, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var doc models.UserDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
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

func (r *UserRepository) ReindexAll(ctx context.Context, users []*models.UserDocument) error {
	r.log.Info("Переиндексация пользователей в ES", "count", len(users))

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
