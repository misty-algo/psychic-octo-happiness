package settingshttp_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	auditusecase "github.com/NSObjects/echo-admin/internal/modules/audit/usecase"
	settingsdomain "github.com/NSObjects/echo-admin/internal/modules/settings/domain"
	settingshttp "github.com/NSObjects/echo-admin/internal/modules/settings/http"
	settingsusecase "github.com/NSObjects/echo-admin/internal/modules/settings/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/requestctx"
	"github.com/NSObjects/echo-admin/internal/platform/server/middlewares"
)

const statusDictionaryImportBody = `{"dictionaries":[{"code":"status","name":"状态","items":[{"label":"启用","value":"enabled","sort":10,"active":true}]}]}`

func TestUpsertConfigRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodPut, "/api/system/configs/site_name", `{"name":"站点名称","value":"Echo Admin","public":true}`, 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert config status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", store.upsertCalls)
	}
	if got := store.config.Key; got != "site_name" {
		t.Fatalf("config key = %q, want site_name", got)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].ActorID; got != 42 {
		t.Fatalf("operation actor id = %d, want 42", got)
	}
}

func TestDeleteConfigRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodDelete, "/api/system/configs/feature_flag", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete config status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := store.deletedConfigKey; got != "feature_flag" {
		t.Fatalf("deletedConfigKey = %q, want feature_flag", got)
	}
	record := onlyOperation(t, recorder)
	if got := record.Resource; got != "config" {
		t.Fatalf("operation resource = %q, want config", got)
	}
}

func TestCreateParamRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	body := `{"name":"支付超时","key":"pay_timeout","value":"30","desc":"秒"}`
	rec := doJSON(t, e, http.MethodPost, "/api/system/params", body, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create param status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createdParam.Key != "pay_timeout" {
		t.Fatalf("createdParam.Key = %q, want pay_timeout", store.createdParam.Key)
	}
	record := onlyOperation(t, recorder)
	if got := record.Resource; got != "system_param" {
		t.Fatalf("operation resource = %q, want system_param", got)
	}
}

func TestBatchDeleteParamsRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/system/params/batch-delete", `{"ids":[1,2]}`, 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch delete params status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !sameInt64s(store.deletedParamIDs, []int64{1, 2}) {
		t.Fatalf("deletedParamIDs = %v, want [1 2]", store.deletedParamIDs)
	}
	record := onlyOperation(t, recorder)
	if got := record.ResourceID; got != "batch" {
		t.Fatalf("operation resource id = %q, want batch", got)
	}
}

func TestDeleteDictionaryRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodDelete, "/api/dictionaries/color", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete dictionary status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := store.deletedDictionaryCode; got != "color" {
		t.Fatalf("deletedDictionaryCode = %q, want color", got)
	}
	record := onlyOperation(t, recorder)
	if got := record.Resource; got != "dictionary" {
		t.Fatalf("operation resource = %q, want dictionary", got)
	}
}

func TestExportDictionaries(t *testing.T) {
	e, store, _ := newSettingsEcho()
	item, err := settingsdomain.RestoreDictionaryItem(1, 0, "启用", "enabled", "", 10, true, 0, "", nil)
	if err != nil {
		t.Fatalf("RestoreDictionaryItem() error = %v", err)
	}
	dictionary, err := settingsdomain.RestoreDictionary(1, "status", "状态", []settingsdomain.DictionaryItem{item}, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreDictionary() error = %v", err)
	}
	store.dictionaries = []settingsdomain.Dictionary{dictionary}

	rec := doJSON(t, e, http.MethodGet, "/api/dictionaries/export", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("export dictionaries status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q, want application/json; charset=utf-8", got)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code": "status"`)) {
		t.Fatalf("export body = %s, want status dictionary", rec.Body.String())
	}
}

func TestImportDictionariesRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/dictionaries/import", statusDictionaryImportBody, 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("import dictionaries status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	assertImportedStatusDictionary(t, store)
	assertOperationAction(t, recorder, "import")
}

func TestCreateVersionRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	body := `{"version":"v1.2.3","name":"稳定版","description":"权限后台初始化","published_at":"2026-06-20T08:00:00Z"}`
	rec := doJSON(t, e, http.MethodPost, "/api/system/versions", body, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create version status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createdVersion.Version != "v1.2.3" {
		t.Fatalf("created version = %q, want v1.2.3", store.createdVersion.Version)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	if got := recorder.records[0].Resource; got != "system_version" {
		t.Fatalf("operation resource = %q, want system_version", got)
	}
}

func TestExportVersionRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	body := `{"version":"v2.0.0","name":"权限包","description":"初始化权限"}`
	rec := doJSON(t, e, http.MethodPost, "/api/system/versions/export", body, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("export version status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if store.createdVersion.Data == "" {
		t.Fatal("createdVersion.Data is empty, want exported bundle")
	}
	record := onlyOperation(t, recorder)
	if got := record.Action; got != "export" {
		t.Fatalf("operation action = %q, want export", got)
	}
}

func TestImportVersionRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	body := `{"version":{"code":"v2.0.0","name":"权限包","description":"初始化权限"},"dictionaries":[{"code":"status","name":"状态","items":[{"label":"启用","value":"enabled","sort":10,"active":true}]}]}`
	rec := doJSON(t, e, http.MethodPost, "/api/system/versions/import", body, 42)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import version status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	assertImportedStatusDictionary(t, store)
	assertOperationAction(t, recorder, "import")
}

func TestImportVersionRejectsManagedAPIRouteCatalog(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	body := `{"version":{"code":"v2.0.0","name":"release"},"apis":[{"method":"GET","path":"/api/admins","description":"admins","group":"admin"}]}`
	rec := doJSON(t, e, http.MethodPost, "/api/system/versions/import", body, 42)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("import version status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if store.createdVersion.ID != 0 {
		t.Fatalf("created version id = %d, want 0", store.createdVersion.ID)
	}
	if got := len(recorder.records); got != 0 {
		t.Fatalf("operation records = %d, want 0", got)
	}
}

func TestDownloadVersion(t *testing.T) {
	e, _, _ := newSettingsEcho()

	rec := doJSON(t, e, http.MethodGet, "/api/system/versions/3/download", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("download version status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q, want application/json; charset=utf-8", got)
	}
}

func TestDeleteVersionRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodDelete, "/api/system/versions/7", "", 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete version status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := store.deletedVersionID; got != 7 {
		t.Fatalf("deletedVersionID = %d, want 7", got)
	}
	record := onlyOperation(t, recorder)
	if got := record.ResourceID; got != "7" {
		t.Fatalf("operation resource id = %q, want 7", got)
	}
}

func TestBatchDeleteVersionsRecordsOperation(t *testing.T) {
	e, store, recorder := newSettingsEcho()

	rec := doJSON(t, e, http.MethodPost, "/api/system/versions/batch-delete", `{"ids":[7,8]}`, 42)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch delete versions status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !sameInt64s(store.deletedVersionIDs, []int64{7, 8}) {
		t.Fatalf("deletedVersionIDs = %v, want [7 8]", store.deletedVersionIDs)
	}
	record := onlyOperation(t, recorder)
	if got := record.ResourceID; got != "batch" {
		t.Fatalf("operation resource id = %q, want batch", got)
	}
}

func newSettingsEcho() (*echo.Echo, *settingsStore, *operationRecorder) {
	store := &settingsStore{}
	uc := settingsusecase.New(store, settingsusecase.WithImportRunner(settingsImportRunner{store: store}))
	recorder := &operationRecorder{}
	handler := settingshttp.New(uc, oprec.New(recorder))

	e := echo.New()
	e.Validator = &middlewares.Validator{Validator: validator.New()}
	e.HTTPErrorHandler = middlewares.ErrorHandlerWithRecorder(nil)
	settingshttp.Register(e.Group("/api"), handler)
	return e, store, recorder
}

// settingsImportRunner mimics the boot runner without a database: dictionary
// replaces and version creates delegate straight to the fake store.
type settingsImportRunner struct {
	store *settingsStore
}

func (r settingsImportRunner) RunImport(ctx context.Context, fn func(context.Context, settingsusecase.ImportTransaction) error) error {
	return fn(ctx, settingsImportTx(r))
}

type settingsImportTx struct {
	store *settingsStore
}

func (t settingsImportTx) ImportMenus(context.Context, []accessusecase.MenuTreeInput) error {
	return nil
}

func (t settingsImportTx) ReplaceDictionary(ctx context.Context, dictionary settingsdomain.Dictionary) error {
	if err := t.store.DeleteDictionary(ctx, dictionary.Code); err != nil {
		return err
	}
	_, err := t.store.CreateDictionary(ctx, dictionary)
	return err
}

func (t settingsImportTx) CreateVersion(ctx context.Context, version settingsdomain.SystemVersion) (settingsdomain.SystemVersion, error) {
	return t.store.CreateVersion(ctx, version)
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	req = req.WithContext(requestctx.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func onlyOperation(t *testing.T, recorder *operationRecorder) auditusecase.OperationInput {
	t.Helper()
	if len(recorder.records) != 1 {
		t.Fatalf("operation records = %d, want 1", len(recorder.records))
	}
	return recorder.records[0]
}

func assertImportedStatusDictionary(t *testing.T, store *settingsStore) {
	t.Helper()
	if len(store.createdDictionaries) != 1 || store.createdDictionaries[0].Code != "status" {
		t.Fatalf("createdDictionaries = %+v, want status dictionary", store.createdDictionaries)
	}
}

func assertOperationAction(t *testing.T, recorder *operationRecorder, want string) {
	t.Helper()
	record := onlyOperation(t, recorder)
	if got := record.Action; got != want {
		t.Fatalf("operation action = %q, want %s", got, want)
	}
}

type settingsStore struct {
	upsertCalls           int
	listDictionaryCalls   int
	listParamCalls        int
	config                settingsdomain.SystemConfig
	deletedConfigKey      string
	deletedDictionaryCode string
	dictionaries          []settingsdomain.Dictionary
	createdParam          settingsdomain.SystemParam
	deletedParamIDs       []int64
	createdDictionaries   []settingsdomain.Dictionary
	createdVersion        settingsdomain.SystemVersion
	deletedVersionID      int64
	deletedVersionIDs     []int64
}

func (s *settingsStore) ListConfigs(ctx context.Context) ([]settingsdomain.SystemConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *settingsStore) UpsertConfig(ctx context.Context, config settingsdomain.SystemConfig) (settingsdomain.SystemConfig, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemConfig{}, err
	}
	s.upsertCalls++
	s.config = config
	return settingsdomain.RestoreSystemConfig(config.Key, config.Name, config.Value, config.Public, fixedTime())
}

func (s *settingsStore) DeleteConfig(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedConfigKey = key
	return nil
}

func (s *settingsStore) ListParams(ctx context.Context) ([]settingsdomain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.listParamCalls++
	param, err := settingsdomain.RestoreSystemParam(1, "支付超时", "pay_timeout", "30", "秒", fixedTime(), fixedTime())
	if err != nil {
		return nil, err
	}
	return []settingsdomain.SystemParam{param}, nil
}

func (s *settingsStore) FindParamByID(ctx context.Context, id int64) (settingsdomain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemParam{}, err
	}
	return settingsdomain.RestoreSystemParam(id, "支付超时", "pay_timeout", "30", "秒", fixedTime(), fixedTime())
}

func (s *settingsStore) FindParamByKey(ctx context.Context, key string) (settingsdomain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemParam{}, err
	}
	return settingsdomain.RestoreSystemParam(1, "支付超时", key, "30", "秒", fixedTime(), fixedTime())
}

func (s *settingsStore) CreateParam(ctx context.Context, param settingsdomain.SystemParam) (settingsdomain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemParam{}, err
	}
	s.createdParam = param
	return settingsdomain.RestoreSystemParam(1, param.Name, param.Key, param.Value, param.Desc, fixedTime(), fixedTime())
}

func (s *settingsStore) UpdateParam(ctx context.Context, param settingsdomain.SystemParam) (settingsdomain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemParam{}, err
	}
	return settingsdomain.RestoreSystemParam(param.ID, param.Name, param.Key, param.Value, param.Desc, fixedTime(), fixedTime())
}

func (s *settingsStore) DeleteParam(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedParamIDs = []int64{id}
	return nil
}

func (s *settingsStore) DeleteParams(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedParamIDs = append([]int64(nil), ids...)
	return nil
}

func (s *settingsStore) ListDictionaries(ctx context.Context) ([]settingsdomain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.listDictionaryCalls++
	return s.dictionaries, nil
}

func (s *settingsStore) CreateDictionary(ctx context.Context, dictionary settingsdomain.Dictionary) (settingsdomain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.Dictionary{}, err
	}
	s.createdDictionaries = append(s.createdDictionaries, dictionary)
	return dictionary, nil
}

func (s *settingsStore) UpdateDictionary(ctx context.Context, dictionary settingsdomain.Dictionary) (settingsdomain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.Dictionary{}, err
	}
	return dictionary, nil
}

func (s *settingsStore) DeleteDictionary(ctx context.Context, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedDictionaryCode = code
	return nil
}

func (s *settingsStore) AddDictionaryItem(ctx context.Context, _ string, item settingsdomain.DictionaryItem) (settingsdomain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.Dictionary{}, err
	}
	return settingsdomain.RestoreDictionary(1, "status", "状态", []settingsdomain.DictionaryItem{item}, fixedTime(), fixedTime())
}

func (s *settingsStore) UpdateDictionaryItem(ctx context.Context, _ string, item settingsdomain.DictionaryItem) (settingsdomain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.Dictionary{}, err
	}
	return settingsdomain.RestoreDictionary(1, "status", "状态", []settingsdomain.DictionaryItem{item}, fixedTime(), fixedTime())
}

func (s *settingsStore) FindDictionaryItem(ctx context.Context, code string, itemID int64) (settingsdomain.DictionaryItem, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.DictionaryItem{}, err
	}
	for _, dictionary := range s.dictionaries {
		if dictionary.Code != code {
			continue
		}
		for _, item := range dictionary.Items {
			if item.ID == itemID {
				return item, nil
			}
		}
	}
	return settingsdomain.DictionaryItem{}, apperr.NewNotFound("dictionary item")
}

func (s *settingsStore) DeleteDictionaryItem(ctx context.Context, _ string, itemID int64) (settingsdomain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.Dictionary{}, err
	}
	item, err := settingsdomain.RestoreDictionaryItem(itemID, 0, "启用", "enabled", "", 10, true, 0, "", nil)
	if err != nil {
		return settingsdomain.Dictionary{}, err
	}
	return settingsdomain.RestoreDictionary(1, "status", "状态", []settingsdomain.DictionaryItem{item}, fixedTime(), fixedTime())
}

func (s *settingsStore) ListVersions(ctx context.Context) ([]settingsdomain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *settingsStore) FindVersionByID(ctx context.Context, id int64) (settingsdomain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemVersion{}, err
	}
	return settingsdomain.RestoreSystemVersion(id, "v1.2.3", "稳定版", "权限后台初始化", "", fixedTime(), fixedTime(), fixedTime())
}

func (s *settingsStore) CreateVersion(ctx context.Context, version settingsdomain.SystemVersion) (settingsdomain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemVersion{}, err
	}
	s.createdVersion = version
	return settingsdomain.RestoreSystemVersion(3, version.Version, version.Name, version.Description, version.Data, version.PublishedAt, fixedTime(), fixedTime())
}

func (s *settingsStore) UpdateVersion(ctx context.Context, version settingsdomain.SystemVersion) (settingsdomain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return settingsdomain.SystemVersion{}, err
	}
	return settingsdomain.RestoreSystemVersion(version.ID, version.Version, version.Name, version.Description, version.Data, version.PublishedAt, fixedTime(), fixedTime())
}

func (s *settingsStore) DeleteVersion(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedVersionID = id
	return nil
}

func (s *settingsStore) DeleteVersions(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletedVersionIDs = append([]int64(nil), ids...)
	return nil
}

type operationRecorder struct {
	records []auditusecase.OperationInput
}

func (r *operationRecorder) RecordOperation(_ context.Context, input auditusecase.OperationInput) (auditusecase.OperationLog, error) {
	r.records = append(r.records, input)
	return auditusecase.OperationLog{}, nil
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}

func sameInt64s(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
