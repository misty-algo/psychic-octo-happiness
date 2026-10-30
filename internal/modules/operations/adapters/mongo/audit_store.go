package mongo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
)

// Collection names used by the audit workbench.
const (
	colVehicleAudits = "vehicle_audits"
	colEventLogs     = "event_logs"
)

// vehicleAuditDoc mirrors one audit revision. Field names follow the shared
// driver-side contract; a revision row is immutable except for claim/decision
// metadata guarded by the version lock.
type vehicleAuditDoc struct {
	ID            bson.ObjectID `bson:"_id"`
	ApplicationID string        `bson:"applicationId"`
	Revision      int           `bson:"revision"`
	DriverID      bson.ObjectID `bson:"driverId"`
	Submitter     string        `bson:"submitter"`
	State         string        `bson:"state"`
	Vehicle       vehicleDoc    `bson:"vehicle"`
	ClaimedBy     string        `bson:"claimedBy"`
	ClaimedAt     *time.Time    `bson:"claimedAt"`
	Version       int           `bson:"version"`
	Reason        string        `bson:"reason"`
	DecidedBy     string        `bson:"decidedBy"`
	DecidedAt     *time.Time    `bson:"decidedAt"`
	SubmittedAt   time.Time     `bson:"submittedAt"`
	Projected     bool          `bson:"projected"`
}

// vehicleDoc is the vehicle snapshot embedded in audit rows and drivers.
type vehicleDoc struct {
	Plate      string `bson:"plate"`
	Model      string `bson:"model"`
	Seats      int    `bson:"seats"`
	CertNo     string `bson:"certNo"`
	CertExpire string `bson:"certExpire"`
}

// eventLogDoc is the outbox row the admission service consumes. Field names
// follow the owner-defined indexes on event_logs: eventId (unique) and t.
type eventLogDoc struct {
	ID          bson.ObjectID `bson:"_id"`
	EventID     string        `bson:"eventId"`
	Type        string        `bson:"type"`
	Aggregate   string        `bson:"aggregate"`
	AggregateID string        `bson:"aggregateId"`
	Payload     bson.M        `bson:"payload"`
	Status      string        `bson:"status"`
	At          time.Time     `bson:"t"`
}

// driverBriefDoc projects only the identity fields the workbench shows.
type driverBriefDoc struct {
	ID    bson.ObjectID `bson:"_id"`
	Name  string        `bson:"name"`
	Plate string        `bson:"plate"`
}

func (d vehicleAuditDoc) toDomain() domain.VehicleAudit {
	audit := domain.VehicleAudit{
		ApplicationID: d.ApplicationID,
		Revision:      d.Revision,
		DriverID:      d.DriverID.Hex(),
		Submitter:     d.Submitter,
		State:         d.State,
		Vehicle: domain.VehicleSnapshot{
			Plate:      d.Vehicle.Plate,
			Model:      d.Vehicle.Model,
			Seats:      d.Vehicle.Seats,
			CertNo:     d.Vehicle.CertNo,
			CertExpire: d.Vehicle.CertExpire,
		},
		ClaimedBy:   d.ClaimedBy,
		Version:     d.Version,
		Reason:      d.Reason,
		DecidedBy:   d.DecidedBy,
		SubmittedAt: d.SubmittedAt,
		Projected:   d.Projected,
	}
	if d.ClaimedAt != nil {
		audit.ClaimedAt = *d.ClaimedAt
	}
	if d.DecidedAt != nil {
		audit.DecidedAt = *d.DecidedAt
	}
	return audit
}

// ensureAuditIndexes keeps event dedupe on the owner-defined unique index
// (eventId) and adds a searchable timestamp index only when missing. The audit
// version chain index (applicationId+revision) is owned by the driver side.
func (s *Store) ensureAuditIndexes(ctx context.Context) error {
	_, err := s.db.Collection(colEventLogs).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "t", Value: -1}},
		Options: options.Index().SetName("idx_t"),
	})
	if err != nil {
		return fmt.Errorf("ensure event_logs timestamp index: %w", err)
	}
	return nil
}

// auditQuery builds the work-queue predicate.
func auditQuery(filter usecase.AuditFilter) bson.M {
	query := bson.M{}
	if filter.State != "" {
		query["state"] = filter.State
	}
	if filter.Keyword != "" {
		escaped := regexp.QuoteMeta(strings.TrimSpace(filter.Keyword))
		query["$or"] = bson.A{
			bson.M{"applicationId": bson.M{"$regex": escaped}},
			bson.M{"vehicle.plate": bson.M{"$regex": escaped}},
		}
	}
	return query
}
