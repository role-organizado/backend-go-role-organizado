package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/admin"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- AdminMetricsRepository (dashboard) ----

// AdminMetricsRepository implements out.AdminMetricsRepository over MongoDB.
type AdminMetricsRepository struct {
	client *Client
}

// NewAdminMetricsRepository creates a new AdminMetricsRepository.
func NewAdminMetricsRepository(client *Client) *AdminMetricsRepository {
	return &AdminMetricsRepository{client: client}
}

// DashboardCounts returns the aggregate "big number" totals.
func (r *AdminMetricsRepository) DashboardCounts(ctx context.Context) (admin.DashboardCounts, error) {
	c := admin.DashboardCounts{}
	c.TotalUsuarios, _ = r.client.Collection("usuarios").CountDocuments(ctx, bson.M{})
	c.TotalEventos, _ = r.client.Collection("eventos").CountDocuments(ctx, bson.M{})
	c.TotalDrafts, _ = r.client.Collection("eventos_draft").CountDocuments(ctx, bson.M{})
	c.TotalPagamentos, _ = r.client.Collection("pagamentos_mensais").CountDocuments(ctx, bson.M{})
	c.TotalNotificacoes, _ = r.client.Collection("notificacoes").CountDocuments(ctx, bson.M{})
	return c, nil
}

// Ping reports database reachability.
func (r *AdminMetricsRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx)
}

// FinanceSummaryTotals aggregates the finance_summaries collection.
func (r *AdminMetricsRepository) FinanceSummaryTotals(ctx context.Context) (map[string]any, error) {
	pipeline := bson.A{
		bson.M{"$group": bson.M{
			"_id":          nil,
			"totalEventos": bson.M{"$sum": 1},
		}},
	}
	cursor, err := r.client.Collection("finance_summaries").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return map[string]any{"totalEventos": 0}, nil
	}
	out := map[string]any(results[0])
	delete(out, "_id")
	return out, nil
}

// ---- FeatureFlagRepository ----

type featureFlagDocument struct {
	ID           string         `bson:"_id"`
	Chave        string         `bson:"chave"`
	Nome         string         `bson:"nome"`
	Enabled      bool           `bson:"enabled"`
	Descricao    string         `bson:"descricao"`
	Categoria    string         `bson:"categoria"`
	Metadata     map[string]any `bson:"metadata"`
	CriadoEm     string         `bson:"criado_em"`
	AtualizadoEm string         `bson:"atualizado_em"`
}

func featureFlagFromDoc(d featureFlagDocument) admin.FeatureFlag {
	return admin.FeatureFlag{
		ID:           d.ID,
		Chave:        d.Chave,
		Nome:         d.Nome,
		Enabled:      d.Enabled,
		Descricao:    d.Descricao,
		Categoria:    d.Categoria,
		Metadata:     d.Metadata,
		CriadoEm:     d.CriadoEm,
		AtualizadoEm: d.AtualizadoEm,
	}
}

// FeatureFlagRepository implements out.FeatureFlagRepository over MongoDB.
type FeatureFlagRepository struct {
	col *mongo.Collection
}

// NewFeatureFlagRepository creates a new FeatureFlagRepository.
func NewFeatureFlagRepository(client *Client) *FeatureFlagRepository {
	return &FeatureFlagRepository{col: client.Collection("feature_flags")}
}

// FindAll lists all feature flags.
func (r *FeatureFlagRepository) FindAll(ctx context.Context) ([]admin.FeatureFlag, error) {
	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return []admin.FeatureFlag{}, nil
	}
	defer cursor.Close(ctx)

	var docs []featureFlagDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return []admin.FeatureFlag{}, nil
	}
	out := make([]admin.FeatureFlag, 0, len(docs))
	for _, d := range docs {
		out = append(out, featureFlagFromDoc(d))
	}
	return out, nil
}

// Update patches a feature flag by chave and returns the updated document.
func (r *FeatureFlagRepository) Update(ctx context.Context, chave string, upd admin.FeatureFlagUpdate) (*admin.FeatureFlag, error) {
	set := bson.M{"atualizado_em": time.Now().UTC().Format(time.RFC3339)}
	if upd.Enabled != nil {
		set["enabled"] = *upd.Enabled
	}
	if upd.Nome != nil {
		set["nome"] = *upd.Nome
	}
	if upd.Descricao != nil {
		set["descricao"] = *upd.Descricao
	}

	res, err := r.col.UpdateOne(ctx, bson.M{"chave": chave}, bson.M{"$set": set})
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFoundMsg("feature flag não encontrada")
	}

	var doc featureFlagDocument
	if err := r.col.FindOne(ctx, bson.M{"chave": chave}).Decode(&doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	ff := featureFlagFromDoc(doc)
	return &ff, nil
}

// ---- ApprovalRepository ----

// ApprovalRepository implements out.ApprovalRepository over MongoDB.
// approver_id / evento_id / solicitante_id are UUID Binary subtype 4 (Java schema).
type ApprovalRepository struct {
	col *mongo.Collection
}

// NewApprovalRepository creates a new ApprovalRepository.
func NewApprovalRepository(client *Client) *ApprovalRepository {
	return &ApprovalRepository{col: client.Collection("approval_items")}
}

// CountPending counts PENDING approval items for the approver.
func (r *ApprovalRepository) CountPending(ctx context.Context, approverID string) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{
		"approver_id": UUIDStringToBinary(approverID),
		"status":      "PENDING",
	})
}

// FindPending lists PENDING approval items for the approver.
func (r *ApprovalRepository) FindPending(ctx context.Context, approverID string) ([]admin.ApprovalItem, error) {
	return r.find(ctx, bson.M{
		"approver_id": UUIDStringToBinary(approverID),
		"status":      "PENDING",
	})
}

// FindHistory lists resolved approval items for the approver.
func (r *ApprovalRepository) FindHistory(ctx context.Context, approverID string) ([]admin.ApprovalItem, error) {
	return r.find(ctx, bson.M{
		"approver_id": UUIDStringToBinary(approverID),
		"status":      bson.M{"$in": bson.A{"APROVADO", "REJEITADO", "CANCELADO"}},
	})
}

func (r *ApprovalRepository) find(ctx context.Context, filter bson.M) ([]admin.ApprovalItem, error) {
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return []admin.ApprovalItem{}, nil
	}
	defer cursor.Close(ctx)

	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return []admin.ApprovalItem{}, nil
	}
	out := make([]admin.ApprovalItem, 0, len(raw))
	for _, item := range raw {
		out = append(out, admin.ApprovalItem{
			ID:            BinaryToUUIDString(item["_id"]),
			Tipo:          asString(item["tipo"]),
			EventoID:      BinaryToUUIDString(item["evento_id"]),
			SolicitanteID: BinaryToUUIDString(item["solicitante_id"]),
			Status:        asString(item["status"]),
			CriadoEm:      item["criado_em"],
		})
	}
	return out, nil
}

// asString coerces a BSON value to a string, returning "" for nil.
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
