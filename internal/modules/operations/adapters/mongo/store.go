// Package mongo adapts operations-module persistence to the taxi business
// MongoDB database. Collection layouts follow the running business data:
// accounts/_id is a string hex id, and this module only owns the
// user_restrictions and operation_logs collections.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
	"github.com/NSObjects/echo-admin/internal/modules/operations/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

// Collection names inside the business database.
const (
	colAccounts           = "accounts"
	colDrivers            = "drivers"
	colRestrictions       = "user_restrictions"
	colOperationLog       = "operation_logs"
	colConfigDictionaries = "config_dictionaries"
)

// Store implements operations usecase.Store on MongoDB.
type Store struct {
	restrictions *mongo.Collection
	logs         *mongo.Collection
	db           *mongo.Database
}

// passengerDoc is the passenger projection embedded in accounts.
type passengerDoc struct {
	Last4 string `bson:"last4"`
}

// accountDoc mirrors the accounts collection written by the business side.
// Every _id/accountId in the business database is a real ObjectId.
type accountDoc struct {
	ID            bson.ObjectID `bson:"_id"`
	Phone         string        `bson:"phone"`
	RealName      string        `bson:"realName"`
	Status        int           `bson:"status"`
	CreatedAt     time.Time     `bson:"createdAt"`
	PassengerInfo *passengerDoc `bson:"passengerInfo"`
}

// driverDoc carries only the account link of the drivers collection.
type driverDoc struct {
	ID        bson.ObjectID `bson:"_id"`
	AccountID bson.ObjectID `bson:"accountId"`
}

// restrictionDoc is the operations-owned user_restrictions layout.
type restrictionDoc struct {
	ID           string     `bson:"_id"`
	AccountID    string     `bson:"account_id"`
	Type         string     `bson:"type"`
	Reason       string     `bson:"reason"`
	Scope        string     `bson:"scope"`
	State        string     `bson:"state"`
	Operator     string     `bson:"operator"`
	CreatedAt    time.Time  `bson:"created_at"`
	RevokedAt    *time.Time `bson:"revoked_at"`
	RevokedBy    string     `bson:"revoked_by"`
	RevokeReason string     `bson:"revoke_reason"`
}

// operationLogDoc is the operations-owned operation_logs layout. It carries
// both t and createdAt because the business-side indexes cover each of them.
type operationLogDoc struct {
	ID        string    `bson:"_id"`
	Module    string    `bson:"module"`
	Operator  string    `bson:"operator"`
	Action    string    `bson:"action"`
	Resource  string    `bson:"resource"`
	Target    string    `bson:"target"`
	RequestID string    `bson:"request_id"`
	Detail    string    `bson:"detail"`
	At        time.Time `bson:"t"`
	CreatedAt time.Time `bson:"createdAt"`
}

// NewStore opens the operations store and ensures its indexes.
func NewStore(ctx context.Context, db *mongo.Database) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("operations mongo store: nil context")
	}
	if db == nil {
		return nil, errors.New("operations mongo store: nil database")
	}
	store := &Store{
		restrictions: db.Collection(colRestrictions),
		logs:         db.Collection(colOperationLog),
		db:           db,
	}
	if err := store.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	if err := store.ensureAuditIndexes(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

// ensureIndexes keeps restriction lookups and log scans efficient. The
// partial unique index enforces one ACTIVE restriction per (account, type)
// at the storage layer, so a racing double-restrict cannot both commit.
func (s *Store) ensureIndexes(ctx context.Context) error {
	_, err := s.restrictions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			bson.E{Key: "account_id", Value: 1},
			bson.E{Key: "type", Value: 1},
			bson.E{Key: "state", Value: 1},
		},
		Options: options.Index().
			SetUnique(true).
			SetPartialFilterExpression(bson.M{"state": domain.RestrictionActive}).
			SetName("uniq_active_account_type"),
	})
	if err != nil {
		return fmt.Errorf("ensure user_restrictions index: %w", err)
	}
	_, err = s.logs.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "t", Value: -1}},
		Options: options.Index().SetName("idx_t_desc"),
	})
	if err != nil {
		return fmt.Errorf("ensure operation_logs index: %w", err)
	}
	return nil
}

// driverAccountIDs returns every account id that registered as a driver.
func (s *Store) driverAccountIDs(ctx context.Context) (map[string]struct{}, error) {
	cur, err := s.db.Collection(colDrivers).Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"accountId": 1}))
	if err != nil {
		return nil, fmt.Errorf("list driver account ids: %w", err)
	}
	defer cur.Close(ctx)
	ids := make(map[string]struct{})
	for cur.Next(ctx) {
		var doc driverDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode driver account id: %w", err)
		}
		if !doc.AccountID.IsZero() {
			ids[doc.AccountID.Hex()] = struct{}{}
		}
	}
	return ids, cur.Err()
}

// objectIDsFromHex converts hex account ids to ObjectIDs so $in/$nin
// predicates match the ObjectId values stored by the business side.
func objectIDsFromHex(ids map[string]struct{}) bson.A {
	out := make(bson.A, 0, len(ids))
	for id := range ids {
		if oid, err := bson.ObjectIDFromHex(id); err == nil {
			out = append(out, oid)
		}
	}
	return out
}

// roleFilter turns the role filter into an account-id set predicate.
func roleFilter(role string, driverIDs map[string]struct{}) bson.M {
	switch role {
	case domain.RoleDriver, domain.RolePassenger:
		operator := "$in"
		if role == domain.RolePassenger {
			operator = "$nin"
		}
		return bson.M{"_id": bson.M{operator: objectIDsFromHex(driverIDs)}}
	default:
		return bson.M{}
	}
}

// keywordFilter matches phone tail digits or a real-name substring.
func keywordFilter(keyword string) bson.M {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return bson.M{}
	}
	return bson.M{"$or": bson.A{
		bson.M{"phone": bson.M{"$regex": regexp.QuoteMeta(keyword) + "$"}},
		bson.M{"realName": bson.M{"$regex": regexp.QuoteMeta(keyword)}},
	}}
}

// ListAccounts returns one page of accounts matching the filter.
func (s *Store) ListAccounts(ctx context.Context, filter usecase.AccountFilter) ([]domain.Account, int64, error) {
	driverIDs, err := s.driverAccountIDs(ctx)
	if err != nil {
		return nil, 0, err
	}
	query := bson.M{}
	for key, value := range roleFilter(filter.Role, driverIDs) {
		query[key] = value
	}
	for key, value := range keywordFilter(filter.Keyword) {
		query[key] = value
	}
	accounts, total, err := s.queryAccounts(ctx, query, filter.Skip, filter.Limit)
	if err != nil {
		return nil, 0, err
	}
	drivers, err := s.driversByAccount(ctx, accounts)
	if err != nil {
		return nil, 0, err
	}
	for i := range accounts {
		accounts[i].DriverID = drivers[accounts[i].ID]
	}
	return accounts, total, nil
}

// queryAccounts runs one account query with paging and decodes the rows.
func (s *Store) queryAccounts(ctx context.Context, query bson.M, skip, limit int) ([]domain.Account, int64, error) {
	collection := s.db.Collection(colAccounts)
	total, err := collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count accounts: %w", err)
	}
	findOptions := options.Find().
		SetSort(bson.D{bson.E{Key: "createdAt", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))
	cur, err := collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("list accounts: %w", err)
	}
	defer cur.Close(ctx)
	accounts := make([]domain.Account, 0)
	for cur.Next(ctx) {
		var doc accountDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode account: %w", err)
		}
		accounts = append(accounts, doc.toDomain())
	}
	return accounts, total, cur.Err()
}

// driversByAccount maps account ids to driver ids for the given accounts.
func (s *Store) driversByAccount(ctx context.Context, accounts []domain.Account) (map[string]string, error) {
	if len(accounts) == 0 {
		return map[string]string{}, nil
	}
	ids := make(bson.A, 0, len(accounts))
	for _, account := range accounts {
		if oid, err := bson.ObjectIDFromHex(account.ID); err == nil {
			ids = append(ids, oid)
		}
	}
	cur, err := s.db.Collection(colDrivers).Find(ctx, bson.M{"accountId": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("list drivers by account: %w", err)
	}
	defer cur.Close(ctx)
	out := make(map[string]string, len(accounts))
	for cur.Next(ctx) {
		var doc driverDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode driver: %w", err)
		}
		if !doc.AccountID.IsZero() {
			out[doc.AccountID.Hex()] = doc.ID.Hex()
		}
	}
	return out, cur.Err()
}

// FindAccount returns one account or domain.ErrInvalidAccountID when missing.
func (s *Store) FindAccount(ctx context.Context, id string) (domain.Account, error) {
	oid, err := bson.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return domain.Account{}, domain.ErrInvalidAccountID
	}
	var doc accountDoc
	err = s.db.Collection(colAccounts).FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Account{}, domain.ErrInvalidAccountID
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("find account: %w", err)
	}
	drivers, err := s.driversByAccount(ctx, []domain.Account{{ID: doc.ID.Hex()}})
	if err != nil {
		return domain.Account{}, err
	}
	account := doc.toDomain()
	account.DriverID = drivers[account.ID]
	return account, nil
}

func (a accountDoc) toDomain() domain.Account {
	return domain.Account{
		ID:        a.ID.Hex(),
		Phone:     a.Phone,
		RealName:  a.RealName,
		Status:    a.Status,
		CreatedAt: a.CreatedAt,
		Passenger: a.PassengerInfo != nil,
	}
}

// ListActiveRestrictions returns ACTIVE restrictions grouped by account id.
func (s *Store) ListActiveRestrictions(ctx context.Context, accountIDs []string) (map[string][]domain.Restriction, error) {
	out := make(map[string][]domain.Restriction)
	if len(accountIDs) == 0 {
		return out, nil
	}
	ids := make(bson.A, 0, len(accountIDs))
	for _, id := range accountIDs {
		ids = append(ids, id)
	}
	query := bson.M{"account_id": bson.M{"$in": ids}, "state": domain.RestrictionActive}
	cur, err := s.restrictions.Find(ctx, query, options.Find().SetSort(bson.D{bson.E{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list active restrictions: %w", err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var doc restrictionDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode restriction: %w", err)
		}
		out[doc.AccountID] = append(out[doc.AccountID], doc.toDomain())
	}
	return out, cur.Err()
}

// FindActiveRestriction returns the ACTIVE restriction of one type.
func (s *Store) FindActiveRestriction(ctx context.Context, accountID, restrictionType string) (domain.Restriction, bool, error) {
	var doc restrictionDoc
	err := s.restrictions.FindOne(ctx, bson.M{
		"account_id": accountID,
		"type":       restrictionType,
		"state":      domain.RestrictionActive,
	}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Restriction{}, false, nil
	}
	if err != nil {
		return domain.Restriction{}, false, fmt.Errorf("find active restriction: %w", err)
	}
	return doc.toDomain(), true, nil
}

// CreateRestriction appends one ACTIVE restriction record. The partial unique
// index maps a racing duplicate to a conflict error.
func (s *Store) CreateRestriction(ctx context.Context, restriction domain.Restriction) (domain.Restriction, error) {
	doc := restrictionDoc{
		ID:        bson.NewObjectID().Hex(),
		AccountID: restriction.AccountID,
		Type:      restriction.Type,
		Reason:    restriction.Reason,
		Scope:     restriction.Scope,
		State:     domain.RestrictionActive,
		Operator:  restriction.Operator,
		CreatedAt: restriction.CreatedAt,
	}
	if _, err := s.restrictions.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.Restriction{}, apperr.NewConflict("该限制类型已生效，不能重复添加")
		}
		return domain.Restriction{}, fmt.Errorf("create user restriction: %w", err)
	}
	return doc.toDomain(), nil
}

// FindRestriction returns one restriction by id.
func (s *Store) FindRestriction(ctx context.Context, id string) (domain.Restriction, error) {
	var doc restrictionDoc
	err := s.restrictions.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Restriction{}, domain.ErrInvalidRestrictionID
	}
	if err != nil {
		return domain.Restriction{}, fmt.Errorf("find restriction: %w", err)
	}
	return doc.toDomain(), nil
}

// RevokeRestriction flips one ACTIVE restriction to REVOKED atomically; the
// state predicate in the filter makes concurrent double-revoke safe.
func (s *Store) RevokeRestriction(ctx context.Context, id string, input usecase.RevokeRestrictionInput) (domain.Restriction, error) {
	revokedAt := input.At
	var doc restrictionDoc
	err := s.restrictions.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id, "state": domain.RestrictionActive},
		bson.M{"$set": bson.M{
			"state":         domain.RestrictionRevoked,
			"revoked_at":    revokedAt,
			"revoked_by":    input.Operator,
			"revoke_reason": input.Reason,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Restriction{}, domain.ErrInvalidRestrictionID
	}
	if err != nil {
		return domain.Restriction{}, fmt.Errorf("revoke restriction: %w", err)
	}
	return doc.toDomain(), nil
}

// RecordOperationLog appends one business-side audit record.
func (s *Store) RecordOperationLog(ctx context.Context, log domain.OperationLog) error {
	doc := operationLogDoc{
		ID:        bson.NewObjectID().Hex(),
		Module:    log.Module,
		Operator:  log.Operator,
		Action:    log.Action,
		Resource:  log.Resource,
		Target:    log.Target,
		RequestID: log.RequestID,
		Detail:    log.Detail,
		At:        log.At,
		CreatedAt: log.At,
	}
	if _, err := s.logs.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("record operation log: %w", err)
	}
	return nil
}

func (d restrictionDoc) toDomain() domain.Restriction {
	restriction := domain.Restriction{
		ID:           d.ID,
		AccountID:    d.AccountID,
		Type:         d.Type,
		Reason:       d.Reason,
		Scope:        d.Scope,
		State:        d.State,
		Operator:     d.Operator,
		CreatedAt:    d.CreatedAt,
		RevokedBy:    d.RevokedBy,
		RevokeReason: d.RevokeReason,
	}
	if d.RevokedAt != nil {
		restriction.RevokedAt = *d.RevokedAt
	}
	return restriction
}
