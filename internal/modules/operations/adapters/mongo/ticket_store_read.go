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
)

// ListTickets returns one page of tickets, newest first.
func (s *Store) ListTickets(ctx context.Context, filter usecase.TicketFilter) ([]domain.Ticket, int64, error) {
	query := ticketQuery(filter)
	collection := s.db.Collection(colTickets)
	total, err := collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}
	cur, err := collection.Find(ctx, query, options.Find().
		SetSort(bson.D{bson.E{Key: "createdAt", Value: -1}}).
		SetSkip(int64(filter.Skip)).
		SetLimit(int64(filter.Limit)))
	if err != nil {
		return nil, 0, fmt.Errorf("list tickets: %w", err)
	}
	defer cur.Close(ctx)
	tickets := make([]domain.Ticket, 0)
	for cur.Next(ctx) {
		var doc ticketDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("decode ticket: %w", err)
		}
		tickets = append(tickets, doc.toDomain())
	}
	return tickets, total, cur.Err()
}

// FindTicket returns one ticket by ticket number.
func (s *Store) FindTicket(ctx context.Context, ticketNo string) (domain.Ticket, error) {
	var doc ticketDoc
	err := s.db.Collection(colTickets).
		FindOne(ctx, bson.M{"ticketNo": strings.TrimSpace(ticketNo)}).
		Decode(&doc)
	if err != nil {
		if isNoDocuments(err) {
			return domain.Ticket{}, domain.ErrTicketNotFound
		}
		return domain.Ticket{}, fmt.Errorf("find ticket: %w", err)
	}
	return doc.toDomain(), nil
}

// ListDuplicates returns same-order same-type tickets created inside the
// window: the 重复投诉合并 candidates shown next to a ticket.
func (s *Store) ListDuplicates(ctx context.Context, ticket domain.Ticket, window time.Duration) ([]domain.Ticket, error) {
	orderID, err := objectID(ticket.OrderID)
	if err != nil {
		return nil, nil
	}
	query := bson.M{
		"orderId":    orderID,
		"type":       ticket.Type,
		"ticketNo":   bson.M{"$ne": ticket.TicketNo},
		"mergedInto": bson.M{"$in": bson.A{"", nil}},
		"createdAt": bson.M{
			"$gte": ticket.CreatedAt.Add(-window),
			"$lte": ticket.CreatedAt.Add(window),
		},
	}
	cur, err := s.db.Collection(colTickets).Find(ctx, query,
		options.Find().SetSort(bson.D{bson.E{Key: "createdAt", Value: 1}}).SetLimit(20))
	if err != nil {
		return nil, fmt.Errorf("list duplicate tickets: %w", err)
	}
	defer cur.Close(ctx)
	out := make([]domain.Ticket, 0)
	for cur.Next(ctx) {
		var doc ticketDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode duplicate ticket: %w", err)
		}
		out = append(out, doc.toDomain())
	}
	return out, cur.Err()
}

// ticketQuery builds the workbench predicate.
func ticketQuery(filter usecase.TicketFilter) bson.M {
	query := bson.M{}
	if filter.State != "" {
		query["state"] = filter.State
	}
	if filter.Type != "" {
		query["type"] = filter.Type
	}
	if filter.Group != "" {
		query["group"] = bson.M{"$regex": "^" + regexp.QuoteMeta(filter.Group)}
	}
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		query["$or"] = bson.A{
			bson.M{"ticketNo": bson.M{"$regex": regexp.QuoteMeta(keyword)}},
			bson.M{"orderNo": bson.M{"$regex": regexp.QuoteMeta(keyword)}},
		}
	}
	return query
}

// isNoDocuments reports whether the driver returned an empty result.
func isNoDocuments(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments)
}
