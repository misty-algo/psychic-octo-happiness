package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/NSObjects/echo-admin/internal/modules/operations/domain"
)

// fakeStore is an in-memory Store double for usecase tests.
type fakeStore struct {
	accounts     []domain.Account
	restrictions []domain.Restriction
	logs         []domain.OperationLog

	nextID   int
	failFind bool
}

func (f *fakeStore) ListAccounts(_ context.Context, filter AccountFilter) ([]domain.Account, int64, error) {
	out := make([]domain.Account, 0, len(f.accounts))
	for _, account := range f.accounts {
		if filter.Role != "" && account.Role() != filter.Role {
			continue
		}
		if filter.Keyword != "" &&
			!strings.HasSuffix(account.Phone, filter.Keyword) &&
			!strings.Contains(account.RealName, filter.Keyword) {
			continue
		}
		out = append(out, account)
	}
	total := int64(len(out))
	if filter.Limit > 0 {
		start := filter.Skip
		if start >= len(out) {
			return []domain.Account{}, total, nil
		}
		end := start + filter.Limit
		if end > len(out) {
			end = len(out)
		}
		out = out[start:end]
	}
	return out, total, nil
}

func (f *fakeStore) FindAccount(_ context.Context, id string) (domain.Account, error) {
	if f.failFind {
		return domain.Account{}, errors.New("boom")
	}
	for _, account := range f.accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return domain.Account{}, domain.ErrInvalidAccountID
}

func (f *fakeStore) ListActiveRestrictions(_ context.Context, accountIDs []string) (map[string][]domain.Restriction, error) {
	want := make(map[string]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		want[id] = struct{}{}
	}
	out := make(map[string][]domain.Restriction)
	for _, restriction := range f.restrictions {
		if !restriction.Active() {
			continue
		}
		if _, ok := want[restriction.AccountID]; ok {
			out[restriction.AccountID] = append(out[restriction.AccountID], restriction)
		}
	}
	return out, nil
}

func (f *fakeStore) FindActiveRestriction(_ context.Context, accountID, restrictionType string) (domain.Restriction, bool, error) {
	for _, restriction := range f.restrictions {
		if restriction.Active() && restriction.AccountID == accountID && restriction.Type == restrictionType {
			return restriction, true, nil
		}
	}
	return domain.Restriction{}, false, nil
}

func (f *fakeStore) CreateRestriction(_ context.Context, restriction domain.Restriction) (domain.Restriction, error) {
	f.nextID++
	restriction.ID = "r" + string(rune('0'+f.nextID))
	f.restrictions = append(f.restrictions, restriction)
	return restriction, nil
}

func (f *fakeStore) FindRestriction(_ context.Context, id string) (domain.Restriction, error) {
	for _, restriction := range f.restrictions {
		if restriction.ID == id {
			return restriction, nil
		}
	}
	return domain.Restriction{}, domain.ErrInvalidRestrictionID
}

func (f *fakeStore) RevokeRestriction(_ context.Context, id string, input RevokeRestrictionInput) (domain.Restriction, error) {
	for i, restriction := range f.restrictions {
		if restriction.ID == id {
			f.restrictions[i].State = domain.RestrictionRevoked
			f.restrictions[i].RevokedAt = input.At
			f.restrictions[i].RevokedBy = input.Operator
			f.restrictions[i].RevokeReason = input.Reason
			return f.restrictions[i], nil
		}
	}
	return domain.Restriction{}, domain.ErrInvalidRestrictionID
}

func (f *fakeStore) RecordOperationLog(_ context.Context, log domain.OperationLog) error {
	f.logs = append(f.logs, log)
	return nil
}

func newTestStore() *fakeStore {
	return &fakeStore{
		accounts: []domain.Account{
			{ID: "acc-1", Phone: "13800006688", RealName: "乘客A", Status: 1, Passenger: true},
			{ID: "acc-2", Phone: "13900008888", RealName: "王师傅", Status: 1, DriverID: "drv-2"},
		},
	}
}

func TestListUsersMasksPhoneAndJoinsRestrictions(t *testing.T) {
	store := newTestStore()
	store.restrictions = append(store.restrictions, domain.Restriction{
		ID: "r1", AccountID: "acc-2", Type: domain.RestrictionNoNewOrder,
		Reason: "测试原因", Scope: "全平台", State: domain.RestrictionActive, Operator: "admin:1",
	})
	usecase := New(store)

	output, err := usecase.ListUsers(context.Background(), ListUsersInput{Role: domain.RoleDriver})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if output.Total != 1 || len(output.Items) != 1 {
		t.Fatalf("ListUsers() total = %d items = %d, want 1/1", output.Total, len(output.Items))
	}
	item := output.Items[0]
	if item.PhoneMask != "139****8888" {
		t.Fatalf("PhoneMask = %q, want 139****8888", item.PhoneMask)
	}
	if item.Role != domain.RoleDriver {
		t.Fatalf("Role = %q, want drv", item.Role)
	}
	if len(item.Restrict) != 1 || item.Restrict[0].Type != domain.RestrictionNoNewOrder {
		t.Fatalf("Restrict = %+v, want one NO_NEW_ORDER", item.Restrict)
	}
}

func TestRestrictUserCreatesRecordAndLog(t *testing.T) {
	store := newTestStore()
	usecase := New(store)

	restriction, err := usecase.RestrictUser(context.Background(), RestrictInput{
		AccountID: "acc-1", Type: domain.RestrictionNoNewRide,
		Reason: "多次恶意取消订单", Scope: "全平台", OperatorID: 7, RequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("RestrictUser() error = %v", err)
	}
	if !restriction.Active() || restriction.Operator != "admin:7" {
		t.Fatalf("Restriction = %+v, want active by admin:7", restriction)
	}
	if len(store.logs) != 1 || store.logs[0].Action != "restrict" || store.logs[0].Target != "acc-1" {
		t.Fatalf("logs = %+v, want one restrict entry", store.logs)
	}

	// 同类型存在生效限制时拒绝重复添加。
	if _, err := usecase.RestrictUser(context.Background(), RestrictInput{
		AccountID: "acc-1", Type: domain.RestrictionNoNewRide,
		Reason: "再次限制同一类型", Scope: "全平台", OperatorID: 7,
	}); !errors.Is(err, domain.ErrRestrictionActive) {
		t.Fatalf("duplicate RestrictUser() error = %v, want ErrRestrictionActive", err)
	}
}

func TestRestrictUserValidatesPayload(t *testing.T) {
	store := newTestStore()
	usecase := New(store)

	cases := []struct {
		name string
		in   RestrictInput
		want error
	}{
		{"unknown account", RestrictInput{AccountID: "missing", Type: domain.RestrictionNoNewRide, Reason: "原因充分的长描述", Scope: "全平台", OperatorID: 1}, domain.ErrInvalidAccountID},
		{"unknown type", RestrictInput{AccountID: "acc-1", Type: "FREE_RIDES", Reason: "原因充分的长描述", Scope: "全平台", OperatorID: 1}, domain.ErrInvalidRestrictionType},
		{"short reason", RestrictInput{AccountID: "acc-1", Type: domain.RestrictionNoNewRide, Reason: "太短", Scope: "全平台", OperatorID: 1}, domain.ErrInvalidRestrictionReason},
		{"missing scope", RestrictInput{AccountID: "acc-1", Type: domain.RestrictionNoNewRide, Reason: "原因充分的长描述", Scope: "", OperatorID: 1}, domain.ErrInvalidRestrictionScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := usecase.RestrictUser(context.Background(), tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("RestrictUser() error = %v, want %v", err, tc.want)
			}
			if len(store.restrictions) != 0 {
				t.Fatalf("restrictions = %+v, want none persisted", store.restrictions)
			}
		})
	}
}

func TestUnrestrictUserRevokesWithTrail(t *testing.T) {
	store := newTestStore()
	usecase := New(store)
	created, err := usecase.RestrictUser(context.Background(), RestrictInput{
		AccountID: "acc-1", Type: domain.RestrictionNoWithdraw,
		Reason: "发现异常提现行为", Scope: "全平台", OperatorID: 7,
	})
	if err != nil {
		t.Fatalf("RestrictUser() error = %v", err)
	}

	revoked, err := usecase.UnrestrictUser(context.Background(), UnrestrictInput{
		AccountID: "acc-1", RestrictionID: created.ID, Reason: "用户已提交澄清材料", OperatorID: 8, RequestID: "req-2",
	})
	if err != nil {
		t.Fatalf("UnrestrictUser() error = %v", err)
	}
	if revoked.Active() || revoked.RevokedBy != "admin:8" || revoked.RevokeReason != "用户已提交澄清材料" {
		t.Fatalf("revoked = %+v, want REVOKED by admin:8", revoked)
	}
	if len(store.logs) != 2 || store.logs[1].Action != "unrestrict" {
		t.Fatalf("logs = %+v, want restrict+unrestrict", store.logs)
	}

	// 重复解除拒绝。
	if _, err := usecase.UnrestrictUser(context.Background(), UnrestrictInput{
		AccountID: "acc-1", RestrictionID: created.ID, Reason: "重复解除无意义", OperatorID: 8,
	}); !errors.Is(err, domain.ErrRestrictionNotActive) {
		t.Fatalf("second UnrestrictUser() error = %v, want ErrRestrictionNotActive", err)
	}
}

func TestUnrestrictUserRejectsForeignAccount(t *testing.T) {
	store := newTestStore()
	usecase := New(store)
	created, err := usecase.RestrictUser(context.Background(), RestrictInput{
		AccountID: "acc-1", Type: domain.RestrictionNoNewRide,
		Reason: "多次恶意取消订单", Scope: "全平台", OperatorID: 7,
	})
	if err != nil {
		t.Fatalf("RestrictUser() error = %v", err)
	}
	if _, err := usecase.UnrestrictUser(context.Background(), UnrestrictInput{
		AccountID: "acc-2", RestrictionID: created.ID, Reason: "跨账号解除不允许", OperatorID: 8,
	}); !errors.Is(err, domain.ErrInvalidAccountID) {
		t.Fatalf("UnrestrictUser() error = %v, want ErrInvalidAccountID", err)
	}
}

func TestUserDetailAuditsFullPhoneRead(t *testing.T) {
	store := newTestStore()
	usecase := New(store)

	detail, err := usecase.UserDetail(context.Background(), "acc-1", 9, "req-3")
	if err != nil {
		t.Fatalf("UserDetail() error = %v", err)
	}
	if detail.Phone != "13800006688" {
		t.Fatalf("Phone = %q, want full number", detail.Phone)
	}
	if len(store.logs) != 1 || store.logs[0].Action != "view_full_phone" || store.logs[0].Operator != "admin:9" {
		t.Fatalf("logs = %+v, want one view_full_phone entry", store.logs)
	}
}

func TestListUsersKeywordAndPagination(t *testing.T) {
	store := newTestStore()
	store.accounts = append(store.accounts,
		domain.Account{ID: "acc-3", Phone: "13700001111", RealName: "李四", Status: 1, Passenger: true},
		domain.Account{ID: "acc-4", Phone: "13600002222", RealName: "赵六", Status: 1, Passenger: true},
	)
	usecase := New(store)

	output, err := usecase.ListUsers(context.Background(), ListUsersInput{Keyword: "1111"})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if output.Total != 1 || output.Items[0].ID != "acc-3" {
		t.Fatalf("keyword search = %+v, want acc-3", output.Items)
	}

	output, err = usecase.ListUsers(context.Background(), ListUsersInput{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if output.Total != 4 || len(output.Items) != 2 {
		t.Fatalf("page = %+v total=%d, want 2 items of 4", output.Items, output.Total)
	}
}

func TestOperatorNameFormat(t *testing.T) {
	if got := operatorName(42); got != "admin:42" {
		t.Fatalf("operatorName(42) = %q, want admin:42", got)
	}
	_ = time.Now
}
