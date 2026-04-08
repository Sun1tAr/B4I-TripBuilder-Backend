package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/domain/models"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
)

const RoutesIndexName = "routes"

type RouteRepository struct {
	es  *Client
	log *logger.Logger
	cfg *struct{ MaxResultWindow int }
}

func NewRouteRepository(es *Client, log *logger.Logger, maxResultWindow int) *RouteRepository {
	return &RouteRepository{
		es:  es,
		log: log,
		cfg: &struct{ MaxResultWindow int }{MaxResultWindow: maxResultWindow},
	}
}

func (r *RouteRepository) getMapping() string {
	return `{
		"settings": {"number_of_shards": 1, "number_of_replicas": 0, "index": {"max_result_window": ` + fmt.Sprintf("%d", r.cfg.MaxResultWindow) + `}},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"title": {"type": "text", "analyzer": "russian", "fields": {"keyword": {"type": "keyword"}}},
				"description": {"type": "text", "analyzer": "russian"},
				"country": {"type": "text", "fields": {"keyword": {"type": "keyword"}}},
				"tags": {"type": "keyword"},
				"total_days": {"type": "integer"},
				"user_id": {"type": "keyword"},
				"likes_count": {"type": "integer"},
				"is_public": {"type": "boolean"},
				"created_at": {"type": "date"}
			}
		}
	}`
}

func (r *RouteRepository) EnsureIndex(ctx context.Context) error {
	exists, err := r.es.IndexExists(ctx, RoutesIndexName)
	if err != nil {
		return err
	}
	if !exists {
		r.log.Info("Создание индекса routes")
		return r.es.CreateIndex(ctx, RoutesIndexName, r.getMapping())
	}
	return nil
}

func (r *RouteRepository) Index(ctx context.Context, doc *models.RouteDocument) error {
	return r.es.IndexDocument(ctx, RoutesIndexName, doc.ID, doc)
}

func (r *RouteRepository) Delete(ctx context.Context, id string) error {
	return r.es.DeleteDocument(ctx, RoutesIndexName, id)
}

func (r *RouteRepository) Search(ctx context.Context, req *models.RouteSearchRequest) (*models.RouteSearchResponse, error) {
	query := r.buildQuery(req)
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
				"title":       map[string]interface{}{"number_of_fragments": 1},
				"description": map[string]interface{}{"number_of_fragments": 1},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := r.es.GetClient().Search(
		r.es.GetClient().Search.WithContext(ctx),
		r.es.GetClient().Search.WithIndex(RoutesIndexName),
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

	items := make([]models.RouteHit, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		var doc models.RouteDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}
		items = append(items, models.RouteHit{
			ID:          hit.ID,
			Title:       doc.Title,
			Description: doc.Description,
			Country:     doc.Country,
			Tags:        doc.Tags,
			TotalDays:   doc.TotalDays,
			UserID:      doc.UserID,
			LikesCount:  doc.LikesCount,
			CreatedAt:   doc.CreatedAt,
			Score:       hit.Score,
			Highlights:  hit.Highlight,
		})
	}

	totalPages := int(esResponse.Hits.Total.Value) / req.Size
	if int(esResponse.Hits.Total.Value)%req.Size > 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	return &models.RouteSearchResponse{
		Items:      items,
		Total:      esResponse.Hits.Total.Value,
		Page:       req.Page,
		Size:       req.Size,
		TotalPages: totalPages,
	}, nil
}

func (r *RouteRepository) buildQuery(req *models.RouteSearchRequest) map[string]interface{} {
	must := []map[string]interface{}{}

	if req.UserID != "" {
		must = append(must, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"is_public": true}},
					{"term": map[string]interface{}{"user_id": req.UserID}},
				},
				"minimum_should_match": 1,
			},
		})
	} else {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"is_public": true},
		})
	}

	if req.Query != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     req.Query,
				"fields":    []string{"title^3", "description", "country", "tags"},
				"fuzziness": "AUTO",
			},
		})
	}

	if req.Country != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"country.keyword": req.Country},
		})
	}

	for _, tag := range req.Tags {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"tags": tag},
		})
	}

	if req.DurationFrom != nil || req.DurationTo != nil {
		rangeFilter := map[string]interface{}{}
		if req.DurationFrom != nil {
			rangeFilter["gte"] = *req.DurationFrom
		}
		if req.DurationTo != nil {
			rangeFilter["lte"] = *req.DurationTo
		}
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{"total_days": rangeFilter},
		})
	}

	return map[string]interface{}{"bool": map[string]interface{}{"must": must}}
}

func (r *RouteRepository) Suggest(ctx context.Context, query string, size int) ([]models.Suggestion, error) {
	r.log.Debug("ES Suggest", "query", query)

	body := map[string]interface{}{
		"suggest": map[string]interface{}{
			"route-suggest": map[string]interface{}{
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

	res, err := r.es.GetClient().Search(
		r.es.GetClient().Search.WithContext(ctx),
		r.es.GetClient().Search.WithIndex(RoutesIndexName),
		r.es.GetClient().Search.WithBody(bytes.NewReader(data)),
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
	if suggests, ok := esResponse.Suggest["route-suggest"]; ok && len(suggests) > 0 {
		for _, opt := range suggests[0].Options {
			suggestions = append(suggestions, models.Suggestion{
				Text:  opt.Text,
				Score: opt.Score,
				Type:  "route",
			})
		}
	}
	return suggestions, nil
}

func (r *RouteRepository) ReindexAll(ctx context.Context, routes []*models.RouteDocument) error {
	r.log.Info("Переиндексация маршрутов в ES", "count", len(routes))

	if err := r.es.DeleteIndex(ctx, RoutesIndexName); err != nil {
		r.log.Error("Ошибка удаления индекса routes", "error", err)
	}
	if err := r.EnsureIndex(ctx); err != nil {
		return err
	}

	docs := make(map[string]interface{}, len(routes))
	for _, route := range routes {
		docs[route.ID] = route
	}
	return r.es.BulkIndex(ctx, RoutesIndexName, docs)
}
