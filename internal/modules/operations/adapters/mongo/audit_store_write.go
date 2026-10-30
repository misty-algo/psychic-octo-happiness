package mongo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// ClaimAudit takes a revision atomically. The filter accepts unclaimed rows,
// rows already owned by the caller, and rows whose lease expired, so a lost
// race can only be caused by a genuinely other owner.
func (s *Store) ClaimAudit(ctx context.Context, applicationID string, revision int, operator string, now time.Time, lease time.Duration) (domain.VehicleAudit, error) {
	expired := now.Add(-lease)
	filter := bson.M{
		"applicationId": applicationID,
		"revision":      revision,
		"state":         domain.AuditStatePending,
		"$or": bson.A{
			bson.M{"claimedBy": ""},
			bson.M{"claimedBy": bson.M{"$exists": false}},
			bson.M{"claimedBy": operator},
			bson.M{"claimedAt": bson.M{"$lt": expired}},
		},
	}
	var doc vehicleAuditDoc
	err := s.db.Collection(colVehicleAudits).FindOneAndUpdate(ctx, filter,
		bson.M{"$set": bson.M{"claimedBy": operator, "claimedAt": now}, "$inc": bson.M{"version": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.VehicleAudit{}, domain.ErrAuditClaimedByOther
	}
	if err != nil {
		return domain.VehicleAudit{}, fmt.Errorf("claim vehicle audit: %w", err)
	}
	return doc.toDomain(), nil
}

// DecideAudit writes the terminal decision under the version lock. Rejections
// need no projection, so they are marked projected immediately.
func (s *Store) DecideAudit(ctx context.Context, command usecase.DecideCommand) (domain.VehicleAudit, error) {
	filter := bson.M{
		"applicationId": command.ApplicationID,
		"revision":      command.Revision,
		"state":         domain.AuditStatePending,
		"version":       command.Version,
		"claimedBy":     command.Operator,
	}
	var doc vehicleAuditDoc
	err := s.db.Collection(colVehicleAudits).FindOneAndUpdate(ctx, filter, bson.M{
		"$set": bson.M{
			"state":     command.State,
			"reason":    command.Reason,
			"decidedBy": command.Operator,
			"decidedAt": command.At,
			"projected": command.State != domain.AuditStateApproved,
		},
		"$inc": bson.M{"version": 1},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.VehicleAudit{}, domain.ErrVersionConflict
	}
	if err != nil {
		return domain.VehicleAudit{}, fmt.Errorf("decide vehicle audit: %w", err)
	}
	return doc.toDomain(), nil
}

// ApplyVehicleSnapshot projects the approved vehicle onto the driver record.
// Driver status is deliberately untouched: the admission service recomputes
// eligibility from the emitted event.
func (s *Store) ApplyVehicleSnapshot(ctx context.Context, driverID string, vehicle domain.VehicleSnapshot, now time.Time) error {
	oid, err := bson.ObjectIDFromHex(strings.TrimSpace(driverID))
	if err != nil {
		return domain.ErrInvalidDriverID
	}
	result, err := s.db.Collection(colDrivers).UpdateOne(ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{
			"vehicle": vehicleDoc{
				Plate:      vehicle.Plate,
				Model:      vehicle.Model,
				Seats:      vehicle.Seats,
				CertNo:     vehicle.CertNo,
				CertExpire: vehicle.CertExpire,
			},
			"plate":     vehicle.Plate,
			"car":       vehicle.Model,
			"updatedAt": now,
		}})
	if err != nil {
		return fmt.Errorf("apply vehicle snapshot: %w", err)
	}
	if result.MatchedCount == 0 {
		return domain.ErrInvalidDriverID
	}
	return nil
}

// EmitQualificationEvent publishes the qualification-changed event exactly
// once: the dedupe key is unique, so replays are absorbed.
func (s *Store) EmitQualificationEvent(ctx context.Context, audit domain.VehicleAudit, now time.Time) error {
	doc := eventLogDoc{
		ID:          bson.NewObjectID(),
		EventID:     fmt.Sprintf("driver.qualification:%s:%d", audit.ApplicationID, audit.Revision),
		Type:        domain.QualificationEventType,
		Aggregate:   "driver",
		AggregateID: audit.DriverID,
		Payload: bson.M{
			"driver_id":      audit.DriverID,
			"application_id": audit.ApplicationID,
			"revision":       audit.Revision,
			"vehicle": bson.M{
				"plate":       audit.Vehicle.Plate,
				"model":       audit.Vehicle.Model,
				"seats":       audit.Vehicle.Seats,
				"cert_no":     audit.Vehicle.CertNo,
				"cert_expire": audit.Vehicle.CertExpire,
			},
			"reason": "车辆审核通过",
		},
		Status: "PENDING",
		At:     now,
	}
	if _, err := s.db.Collection(colEventLogs).InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}
		return fmt.Errorf("emit qualification event: %w", err)
	}
	return nil
}

// MarkAuditProjected records that projection and event are complete.
func (s *Store) MarkAuditProjected(ctx context.Context, applicationID string, revision int) error {
	_, err := s.db.Collection(colVehicleAudits).UpdateOne(ctx,
		bson.M{"applicationId": applicationID, "revision": revision},
		bson.M{"$set": bson.M{"projected": true}})
	if err != nil {
		return fmt.Errorf("mark audit projected: %w", err)
	}
	return nil
}
