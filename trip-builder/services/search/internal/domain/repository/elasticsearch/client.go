package elasticsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search/internal/config"
	"github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared/logger"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type ESClient struct {
	client *elasticsearch.Client
	log    *logger.Logger
	cfg    *config.Config
}

func NewESClient(cfg *config.Config, log *logger.Logger) (*ESClient, error) {
	esCfg := elasticsearch.Config{
		Addresses: cfg.ElasticsearchAddrs,
		Username:  cfg.ElasticsearchUser,
		Password:  cfg.ElasticsearchPass,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	// Проверяем соединение
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch returned error: %s", res.String())
	}

	log.Info("Подключение к Elasticsearch установлено", "addrs", cfg.ElasticsearchAddrs)

	return &ESClient{
		client: client,
		log:    log,
		cfg:    cfg,
	}, nil
}

func (c *ESClient) IndexExists(ctx context.Context, indexName string) (bool, error) {
	req := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == http.StatusOK, nil
}

func (c *ESClient) CreateIndex(ctx context.Context, indexName string, mapping string) error {
	req := esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != http.StatusBadRequest { // 400 = already exists
		return fmt.Errorf("error creating index %s: %s", indexName, res.String())
	}

	c.log.Info("Индекс создан", "index", indexName)
	return nil
}

func (c *ESClient) IndexDocument(ctx context.Context, indexName, docID string, doc interface{}) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to index document %s: %w", docID, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing document %s: %s", docID, res.String())
	}

	c.log.Trace("Документ проиндексирован", "index", indexName, "id", docID)
	return nil
}

func (c *ESClient) DeleteDocument(ctx context.Context, indexName, docID string) error {
	req := esapi.DeleteRequest{
		Index:      indexName,
		DocumentID: docID,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to delete document %s: %w", docID, err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("error deleting document %s: %s", docID, res.String())
	}

	c.log.Trace("Документ удалён", "index", indexName, "id", docID)
	return nil
}

func (c *ESClient) BulkIndex(ctx context.Context, indexName string, docs map[string]interface{}) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for id, doc := range docs {
		meta := []byte(fmt.Sprintf(`{"index":{"_id":"%s"}}`+"\n", id))
		data, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("failed to marshal document %s: %w", id, err)
		}
		buf.Write(meta)
		buf.Write(data)
		buf.Write([]byte("\n"))
	}

	req := esapi.BulkRequest{
		Index:   indexName,
		Body:    &buf,
		Refresh: "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to bulk index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error bulk indexing: %s", res.String())
	}

	c.log.Debug("Bulk индексация выполнена", "index", indexName, "count", len(docs))
	return nil
}

func (c *ESClient) DeleteIndex(ctx context.Context, indexName string) error {
	req := esapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to delete index %s: %w", indexName, err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("error deleting index %s: %s", indexName, res.String())
	}

	c.log.Info("Индекс удалён", "index", indexName)
	return nil
}

func (c *ESClient) GetClient() *elasticsearch.Client {
	return c.client
}
