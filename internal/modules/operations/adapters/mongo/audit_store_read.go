package mongo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// ListAudits returns one page of revisions, newest first.
func (s *Store) ListAudits(ctx context.Context, filter usecase.AuditFilter) ([]domain.VehicleAudit, int64, error) {
	collection := s.db.Collection(colVehicleAudits)
	query := auditQuery(filter)
	total, err := collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count vehicle audits: %w", err)
	}
	cur, err := collection.Find(ctx, query, options.Find().
		SetSort(bson.D{bson.E{Key: "submittedAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).
		SetLimit(int64(filter.Limit)))
	if err != nil {
		return nil, 0, fmt.Errorf("list vehicle audits: %w", err)
	}
	defer cur.Close(ctx)
	audits := make([]domain.VehicleAudit, 0)
	for cur.Next(ctx) {
		var doc vehicleAuditDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode vehicle audit: %w", err)
		}
		audits = append(audits, doc.toDomain())
	}
	return audits, total, cur.Err()
}

// FindAudit returns one revision of one application.
func (s *Store) FindAudit(ctx context.Context, applicationID string, revision int) (domain.VehicleAudit, error) {
	var doc vehicleAuditDoc
	err := s.db.Collection(colVehicleAudits).
		FindOne(ctx, bson.M{"applicationId": applicationID, "revision": revision}).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.VehicleAudit{}, domain.ErrAuditNotFound
	}
	if err != nil {
		return domain.VehicleAudit{}, fmt.Errorf("find vehicle audit: %w", err)
	}
	return doc.toDomain(), nil
}

// ListAuditRevisions returns the whole version chain, newest first.
func (s *Store) ListAuditRevisions(ctx context.Context, applicationID string) ([]domain.VehicleAudit, error) {
	cur, err := s.db.Collection(colVehicleAudits).Find(ctx,
		bson.M{"applicationId": applicationID},
		options.Find().SetSort(bson.D{bson.E{Key: "revision", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list audit revisions: %w", err)
	}
	defer cur.Close(ctx)
	audits := make([]domain.VehicleAudit, 0)
	for cur.Next(ctx) {
		var doc vehicleAuditDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode audit revision: %w", err)
		}
		audits = append(audits, doc.toDomain())
	}
	return audits, cur.Err()
}

// CountRejections counts rejected revisions per application.
func (s *Store) CountRejections(ctx context.Context, applicationIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(applicationIDs))
	if len(applicationIDs) == 0 {
		return out, nil
	}
	values := make(bson.A, 0, len(applicationIDs))
	for _, id := range applicationIDs {
		values = append(values, id)
	}
	cur, err := s.db.Collection(colVehicleAudits).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"applicationId": bson.M{"$in": values}, "state": domain.AuditStateRejected}}},
		{{Key: "$group", Value: bson.M{"_id": "$applicationId", "count": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, fmt.Errorf("count audit rejections: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode rejection count: %w", err)
		}
		out[row.ID] = row.Count
	}
	return out, cur.Err()
}

// DriverBriefs returns driver names and plates for the given ids.
func (s *Store) DriverBriefs(ctx context.Context, driverIDs []string) (map[string]usecase.DriverBrief, error) {
	out := make(map[string]usecase.DriverBrief, len(driverIDs))
	if len(driverIDs) == 0 {
		return out, nil
	}
	ids := make(bson.A, 0, len(driverIDs))
	for _, id := range driverIDs {
		if oid, err := bson.ObjectIDFromHex(strings.TrimSpace(id)); err == nil {
			ids = append(ids, oid)
		}
	}
	cur, err := s.db.Collection(colDrivers).Find(ctx,
		bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1, "plate": 1}))
	if err != nil {
		return nil, fmt.Errorf("list driver briefs: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc driverBriefDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode driver brief: %w", err)
		}
		out[doc.ID.Hex()] = usecase.DriverBrief{Name: doc.Name, Plate: doc.Plate}
	}
	return out, cur.Err()
}
