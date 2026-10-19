package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	settingsdomain "github.com/NSObjects/echo-admin/internal/modules/settings/domain"
	"github.com/NSObjects/echo-admin/internal/modules/settings/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
)

const statusDictionaryCode = "status"

func TestDeleteDictionaryRejectsSeedDictionary(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	err := uc.DeleteDictionary(context.Background(), statusDictionaryCode)
	if err == nil {
		t.Fatal("DeleteDictionary(status) error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("DeleteDictionary(status) kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if store.deletedDictionaryCode != "" {
		t.Fatalf("deletedDictionaryCode = %q, want empty", store.deletedDictionaryCode)
	}
}

func TestDeleteConfigRejectsSeedConfig(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	err := uc.DeleteConfig(context.Background(), "site_name")
	if err == nil {
		t.Fatal("DeleteConfig(site_name) error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("DeleteConfig(site_name) kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if store.deletedConfigKey != "" {
		t.Fatalf("deletedConfigKey = %q, want empty", store.deletedConfigKey)
	}
}

func TestDeleteConfigDeletesCustomConfig(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	if err := uc.DeleteConfig(context.Background(), "Feature_Flag"); err != nil {
		t.Fatalf("DeleteConfig() error = %v", err)
	}
	if store.deletedConfigKey != "feature_flag" {
		t.Fatalf("deletedConfigKey = %q, want feature_flag", store.deletedConfigKey)
	}
}

func TestListParamsFiltersAndPaginates(t *testing.T) {
	store := &storeSpy{
		params: []settingsdomain.SystemParam{
			mustParam(t, 1, "支付超时", "pay_timeout", "30", "秒"),
			mustParam(t, 2, "站点名称", "site_name", "Echo Admin", ""),
			mustParam(t, 3, "支付开关", "pay_enabled", "true", ""),
		},
	}
	uc := usecase.New(store)

	output, err := uc.ListParams(context.Background(), usecase.ParamListInput{
		Page:     1,
		PageSize: 1,
		Key:      "pay",
	})
	if err != nil {
		t.Fatalf("ListParams() error = %v", err)
	}
	if output.Total != 2 {
		t.Fatalf("ListParams() total = %d, want 2", output.Total)
	}
	if len(output.Items) != 1 || output.Items[0].Key != "pay_timeout" {
		t.Fatalf("ListParams() items = %+v, want pay_timeout only", output.Items)
	}
}

func TestCreateParamRejectsInvalidKey(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	_, err := uc.CreateParam(context.Background(), usecase.ParamInput{
		Name:  "参数",
		Key:   "",
		Value: "value",
	})
	if err == nil {
		t.Fatal("CreateParam() error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("CreateParam() kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if store.createdParam.Key != "" {
		t.Fatalf("createdParam = %+v, want empty", store.createdParam)
	}
}

func TestDeleteParamsRejectsEmptyIDs(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	err := uc.DeleteParams(context.Background(), nil)
	if err == nil {
		t.Fatal("DeleteParams(nil) error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("DeleteParams(nil) kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if len(store.deletedParamIDs) != 0 {
		t.Fatalf("deletedParamIDs = %v, want empty", store.deletedParamIDs)
	}
}

func TestDeleteDictionaryDeletesCustomDictionary(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	if err := uc.DeleteDictionary(context.Background(), "Color"); err != nil {
		t.Fatalf("DeleteDictionary() error = %v", err)
	}
	if store.deletedDictionaryCode != "color" {
		t.Fatalf("deletedDictionaryCode = %q, want color", store.deletedDictionaryCode)
	}
}

func TestAddDictionaryItemComputesChildPath(t *testing.T) {
	parent := mustDictionaryItem(t, 7, 0, "父项", "parent", 0, "")
	store := &storeSpy{dictionaries: []settingsdomain.Dictionary{
		mustDictionary(t, 1, statusDictionaryCode, "状态", []settingsdomain.DictionaryItem{parent}),
	}}
	uc := usecase.New(store)

	_, err := uc.AddDictionaryItem(context.Background(), statusDictionaryCode, usecase.DictionaryItemInput{
		ParentID: parent.ID,
		Label:    "子项",
		Value:    "child",
		Sort:     20,
		Active:   true,
	})
	if err != nil {
		t.Fatalf("AddDictionaryItem(child) error = %v", err)
	}
	if store.addedItem.Level != 1 || store.addedItem.Path != "7" {
		t.Fatalf("added item level/path = %d/%q, want 1/7", store.addedItem.Level, store.addedItem.Path)
	}
}

func TestUpdateDictionaryItemRejectsParentCycle(t *testing.T) {
	parent := mustDictionaryItem(t, 7, 0, "父项", "parent", 0, "")
	child := mustDictionaryItem(t, 8, 7, "子项", "child", 1, "7")
	store := &storeSpy{dictionaries: []settingsdomain.Dictionary{
		mustDictionary(t, 1, statusDictionaryCode, "状态", []settingsdomain.DictionaryItem{parent, child}),
	}}
	uc := usecase.New(store)

	_, err := uc.UpdateDictionaryItem(context.Background(), statusDictionaryCode, usecase.DictionaryItemInput{
		ID:       parent.ID,
		ParentID: child.ID,
		Label:    parent.Label,
		Value:    parent.Value,
		Sort:     parent.Sort,
		Active:   parent.Active,
	})
	if code := appCode(t, err); code != apperr.ErrBadRequest {
		t.Fatalf("UpdateDictionaryItem(cycle) code = %d, want %d", code, apperr.ErrBadRequest)
	}
	if store.updatedItem.ID != 0 {
		t.Fatalf("updated item id = %d, want 0", store.updatedItem.ID)
	}
}

func TestCreateVersionRejectsInvalidVersion(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	_, err := uc.CreateVersion(context.Background(), usecase.VersionInput{
		Version: "1.0.0 invalid",
		Name:    "Initial Release",
	})
	if err == nil {
		t.Fatal("CreateVersion() error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("CreateVersion() kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if store.createdVersion.Version != "" {
		t.Fatalf("createdVersion = %q, want empty", store.createdVersion.Version)
	}
}

func TestDeleteVersionRejectsInvalidID(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	err := uc.DeleteVersion(context.Background(), 0)
	if err == nil {
		t.Fatal("DeleteVersion(0) error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("DeleteVersion(0) kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if store.deletedVersionID != 0 {
		t.Fatalf("deletedVersionID = %d, want 0", store.deletedVersionID)
	}
}

func TestDeleteVersionsRejectsEmptyIDs(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	err := uc.DeleteVersions(context.Background(), nil)
	if err == nil {
		t.Fatal("DeleteVersions(nil) error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("DeleteVersions(nil) kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if len(store.deletedVersionIDs) != 0 {
		t.Fatalf("deletedVersionIDs = %v, want empty", store.deletedVersionIDs)
	}
}

func TestExportVersionStoresPortableBundle(t *testing.T) {
	store, catalog := newVersionExportFixture(t)
	uc := usecase.New(store, usecase.WithVersionCatalog(catalog))

	version, err := uc.ExportVersion(context.Background(), usecase.ExportVersionInput{
		Version:       "v2.0.0",
		Name:          "权限包",
		Description:   "初始化权限",
		MenuIDs:       []int64{2, 2},
		DictionaryIDs: []int64{9},
	})
	if err != nil {
		t.Fatalf("ExportVersion() error = %v", err)
	}
	if version.Data == "" {
		t.Fatal("ExportVersion() data is empty, want bundle JSON")
	}
	if !sameInt64s(catalog.exportMenuIDs, []int64{2}) {
		t.Fatalf("exportMenuIDs = %v, want [2]", catalog.exportMenuIDs)
	}
	bundle := decodeVersionBundle(t, version.Data)
	if bundle.Version.Code != "v2.0.0" {
		t.Fatalf("bundle version code = %q, want v2.0.0", bundle.Version.Code)
	}
	if len(bundle.Menus) != 1 || len(bundle.Dictionaries) != 1 {
		t.Fatalf("bundle counts = menus:%d dictionaries:%d, want 1/1", len(bundle.Menus), len(bundle.Dictionaries))
	}
}

func TestExportDictionariesReturnsPortableBundle(t *testing.T) {
	store, _ := newVersionExportFixture(t)
	uc := usecase.New(store)

	bundle, err := uc.ExportDictionaries(context.Background())
	if err != nil {
		t.Fatalf("ExportDictionaries() error = %v", err)
	}
	if bundle.ExportTime == "" {
		t.Fatal("ExportDictionaries() ExportTime is empty")
	}
	if len(bundle.Dictionaries) != 1 || bundle.Dictionaries[0].Code != statusDictionaryCode {
		t.Fatalf("ExportDictionaries() dictionaries = %+v, want status", bundle.Dictionaries)
	}
}

func TestImportDictionariesValidatesBeforeWriting(t *testing.T) {
	store := &storeSpy{}
	runner := &importRunnerSpy{}
	uc := usecase.New(store, usecase.WithImportRunner(runner))

	_, err := uc.ImportDictionaries(context.Background(), usecase.DictionaryBundle{
		Dictionaries: []usecase.VersionDictionary{{Code: "", Name: "状态"}},
	})
	if err == nil {
		t.Fatal("ImportDictionaries(invalid) error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("ImportDictionaries() kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if len(runner.replacedDictionaryCodes) != 0 {
		t.Fatalf("replacedDictionaryCodes = %v, want none", runner.replacedDictionaryCodes)
	}
}

func TestImportDictionariesWritesBundle(t *testing.T) {
	store := &storeSpy{}
	runner := &importRunnerSpy{}
	uc := usecase.New(store, usecase.WithImportRunner(runner))

	_, err := uc.ImportDictionaries(context.Background(), usecase.DictionaryBundle{
		Dictionaries: []usecase.VersionDictionary{{
			Code: statusDictionaryCode,
			Name: "状态",
			Items: []usecase.VersionDictionaryItem{{
				Label:  "启用",
				Value:  "enabled",
				Sort:   10,
				Active: true,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("ImportDictionaries() error = %v", err)
	}
	if len(runner.replacedDictionaryCodes) != 1 || runner.replacedDictionaryCodes[0] != statusDictionaryCode {
		t.Fatalf("replacedDictionaryCodes = %v, want [%s]", runner.replacedDictionaryCodes, statusDictionaryCode)
	}
}

func TestImportDictionariesStopsAtFirstFailure(t *testing.T) {
	store := &storeSpy{}
	runner := &importRunnerSpy{failDictionaryCode: "first"}
	uc := usecase.New(store, usecase.WithImportRunner(runner))

	_, err := uc.ImportDictionaries(context.Background(), usecase.DictionaryBundle{
		Dictionaries: []usecase.VersionDictionary{
			{Code: "first", Name: "一"},
			{Code: "second", Name: "二"},
		},
	})
	if err == nil {
		t.Fatal("ImportDictionaries() error = nil, want replace failure")
	}
	if len(runner.replacedDictionaryCodes) != 1 || runner.replacedDictionaryCodes[0] != "first" {
		t.Fatalf("replacedDictionaryCodes = %v, want only [first]", runner.replacedDictionaryCodes)
	}
}

func TestImportRequiresImportRunner(t *testing.T) {
	store := &storeSpy{}
	uc := usecase.New(store)

	_, err := uc.ImportDictionaries(context.Background(), usecase.DictionaryBundle{
		Dictionaries: []usecase.VersionDictionary{{Code: "status", Name: "状态"}},
	})
	if err == nil {
		t.Fatal("ImportDictionaries() error = nil, want missing runner failure")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindInternal {
		t.Fatalf("ImportDictionaries() kind = %v, want %s", def.Kind, apperr.KindInternal)
	}
}

func newVersionExportFixture(t *testing.T) (*storeSpy, *versionCatalogSpy) {
	t.Helper()
	item, err := settingsdomain.RestoreDictionaryItem(1, 0, "启用", "enabled", "", 10, true, 0, "", nil)
	if err != nil {
		t.Fatalf("RestoreDictionaryItem() error = %v", err)
	}
	dictionary, err := settingsdomain.RestoreDictionary(9, statusDictionaryCode, "状态", []settingsdomain.DictionaryItem{item}, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreDictionary() error = %v", err)
	}
	store := &storeSpy{dictionaries: []settingsdomain.Dictionary{dictionary}}
	catalog := &versionCatalogSpy{
		menus: []accessusecase.MenuTreeInput{{
			MenuInput: accessusecase.MenuInput{
				Name:      "角色权限",
				Path:      "/roles",
				Component: "./Roles",
				Active:    true,
			},
		}},
	}
	return store, catalog
}

func decodeVersionBundle(t *testing.T, data string) usecase.VersionBundle {
	t.Helper()
	var bundle usecase.VersionBundle
	if err := json.Unmarshal([]byte(data), &bundle); err != nil {
		t.Fatalf("unmarshal version data error = %v", err)
	}
	return bundle
}

func TestImportVersionImportsMenusAndDictionaries(t *testing.T) {
	store := &storeSpy{}
	runner := &importRunnerSpy{}
	uc := usecase.New(store, usecase.WithVersionCatalog(&versionCatalogSpy{}), usecase.WithImportRunner(runner))

	_, err := uc.ImportVersion(context.Background(), usecase.VersionBundle{
		Version: usecase.VersionInfo{Code: "v2.0.0", Name: "权限包", Description: "初始化权限"},
		Menus: []accessusecase.MenuTreeInput{{
			MenuInput: accessusecase.MenuInput{
				Name:      "角色权限",
				Path:      "/roles",
				Component: "./Roles",
				Active:    true,
			},
		}},
		Dictionaries: []usecase.VersionDictionary{{
			Code: statusDictionaryCode,
			Name: "状态",
			Items: []usecase.VersionDictionaryItem{{
				Label:  "启用",
				Value:  "enabled",
				Sort:   10,
				Active: true,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("ImportVersion() error = %v", err)
	}
	if len(runner.importedMenus) != 1 {
		t.Fatalf("importedMenus = %d, want 1", len(runner.importedMenus))
	}
	if len(runner.replacedDictionaryCodes) != 1 || runner.replacedDictionaryCodes[0] != statusDictionaryCode {
		t.Fatalf("replacedDictionaryCodes = %v, want [%s]", runner.replacedDictionaryCodes, statusDictionaryCode)
	}
	if runner.createdVersion.Version == "" {
		t.Fatal("createdVersion.Version is empty, want import audit version")
	}
}

func TestImportVersionRejectsInvalidDictionaryBeforeWriting(t *testing.T) {
	store := &storeSpy{}
	runner := &importRunnerSpy{}
	uc := usecase.New(store, usecase.WithVersionCatalog(&versionCatalogSpy{}), usecase.WithImportRunner(runner))

	_, err := uc.ImportVersion(context.Background(), usecase.VersionBundle{
		Version: usecase.VersionInfo{Code: "v2.0.0", Name: "权限包"},
		Menus: []accessusecase.MenuTreeInput{{
			MenuInput: accessusecase.MenuInput{
				Name:      "角色权限",
				Path:      "/roles",
				Component: "./Roles",
				Active:    true,
			},
		}},
		Dictionaries: []usecase.VersionDictionary{{Code: "", Name: "状态"}},
	})
	if err == nil {
		t.Fatal("ImportVersion() error = nil, want bad request")
	}
	def, ok := apperr.ParseRegistered(err)
	if !ok || def.Kind != apperr.KindBadRequest {
		t.Fatalf("ImportVersion() kind = %v, want %s", def.Kind, apperr.KindBadRequest)
	}
	if len(runner.importedMenus) != 0 {
		t.Fatalf("importedMenus = %d, want 0", len(runner.importedMenus))
	}
	if len(runner.replacedDictionaryCodes) != 0 {
		t.Fatalf("replacedDictionaryCodes = %v, want none", runner.replacedDictionaryCodes)
	}
	if runner.createdVersion.Version != "" {
		t.Fatalf("createdVersion = %q, want empty", runner.createdVersion.Version)
	}
}

// TestImportVersionStopsOnFailure locks the step order inside the import
// transaction: when menu import, dictionary replacement, or version creation
// fails, every later step is skipped so the whole import rolls back together.
func TestImportVersionStopsOnFailure(t *testing.T) {
	bundle := usecase.VersionBundle{
		Version:      usecase.VersionInfo{Code: "v2.0.0", Name: "权限包"},
		Menus:        []accessusecase.MenuTreeInput{{MenuInput: accessusecase.MenuInput{Name: "角色权限", Path: "/roles", Active: true}}},
		Dictionaries: []usecase.VersionDictionary{{Code: statusDictionaryCode, Name: "状态"}},
	}
	tests := []struct {
		name   string
		runner *importRunnerSpy
	}{
		{"menu import failure", &importRunnerSpy{failOnMenus: true}},
		{"dictionary replace failure", &importRunnerSpy{failDictionaryCode: statusDictionaryCode}},
		{"version create failure", &importRunnerSpy{failOnVersion: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := usecase.New(&storeSpy{}, usecase.WithImportRunner(tt.runner))

			_, err := uc.ImportVersion(context.Background(), bundle)
			if err == nil {
				t.Fatal("ImportVersion() error = nil, want import failure")
			}
			if tt.runner.createdVersion.Version != "" {
				t.Fatalf("createdVersion = %q, want empty", tt.runner.createdVersion.Version)
			}
			if tt.name == "menu import failure" && len(tt.runner.replacedDictionaryCodes) != 0 {
				t.Fatalf("replacedDictionaryCodes = %v, want none", tt.runner.replacedDictionaryCodes)
			}
		})
	}
}

type storeSpy struct {
	deletedConfigKey      string
	deletedDictionaryCode string
	params                []settingsdomain.SystemParam
	createdParam          settingsdomain.SystemParam
	deletedParamIDs       []int64
	dictionaries          []settingsdomain.Dictionary
	createdDictionaries   []settingsdomain.Dictionary
	addedItem             settingsdomain.DictionaryItem
	updatedItem           settingsdomain.DictionaryItem
	createdVersion        settingsdomain.SystemVersion
	deletedVersionID      int64
	deletedVersionIDs     []int64
}

func (s *storeSpy) ListConfigs(context.Context) ([]settingsdomain.SystemConfig, error) {
	return nil, nil
}

func (s *storeSpy) UpsertConfig(_ context.Context, config settingsdomain.SystemConfig) (settingsdomain.SystemConfig, error) {
	return config, nil
}

func (s *storeSpy) DeleteConfig(_ context.Context, key string) error {
	s.deletedConfigKey = key
	return nil
}

func (s *storeSpy) ListParams(context.Context) ([]settingsdomain.SystemParam, error) {
	return s.params, nil
}

func (s *storeSpy) FindParamByID(_ context.Context, id int64) (settingsdomain.SystemParam, error) {
	return mustParamNoT(id, "参数", "param_key", "value", "")
}

func (s *storeSpy) FindParamByKey(_ context.Context, key string) (settingsdomain.SystemParam, error) {
	return mustParamNoT(1, "参数", key, "value", "")
}

func (s *storeSpy) CreateParam(_ context.Context, param settingsdomain.SystemParam) (settingsdomain.SystemParam, error) {
	s.createdParam = param
	return param, nil
}

func (s *storeSpy) UpdateParam(_ context.Context, param settingsdomain.SystemParam) (settingsdomain.SystemParam, error) {
	return param, nil
}

func (s *storeSpy) DeleteParam(_ context.Context, id int64) error {
	s.deletedParamIDs = []int64{id}
	return nil
}

func (s *storeSpy) DeleteParams(_ context.Context, ids []int64) error {
	s.deletedParamIDs = append([]int64(nil), ids...)
	return nil
}

func (s *storeSpy) ListDictionaries(context.Context) ([]settingsdomain.Dictionary, error) {
	return s.dictionaries, nil
}

func (s *storeSpy) CreateDictionary(_ context.Context, dictionary settingsdomain.Dictionary) (settingsdomain.Dictionary, error) {
	s.createdDictionaries = append(s.createdDictionaries, dictionary)
	return dictionary, nil
}

func (s *storeSpy) UpdateDictionary(_ context.Context, dictionary settingsdomain.Dictionary) (settingsdomain.Dictionary, error) {
	return dictionary, nil
}

func (s *storeSpy) DeleteDictionary(_ context.Context, code string) error {
	s.deletedDictionaryCode = code
	return nil
}

func (s *storeSpy) AddDictionaryItem(_ context.Context, code string, item settingsdomain.DictionaryItem) (settingsdomain.Dictionary, error) {
	s.addedItem = item
	return dictionaryWithItem(code, item)
}

func (s *storeSpy) UpdateDictionaryItem(_ context.Context, code string, item settingsdomain.DictionaryItem) (settingsdomain.Dictionary, error) {
	s.updatedItem = item
	return dictionaryWithItem(code, item)
}

func (s *storeSpy) FindDictionaryItem(_ context.Context, code string, itemID int64) (settingsdomain.DictionaryItem, error) {
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

func (s *storeSpy) DeleteDictionaryItem(_ context.Context, code string, itemID int64) (settingsdomain.Dictionary, error) {
	item, err := settingsdomain.RestoreDictionaryItem(itemID, 0, "启用", "enabled", "", 10, true, 0, "", nil)
	if err != nil {
		return settingsdomain.Dictionary{}, err
	}
	return dictionaryWithItem(code, item)
}

func (s *storeSpy) ListVersions(context.Context) ([]settingsdomain.SystemVersion, error) {
	return nil, nil
}

func (s *storeSpy) FindVersionByID(_ context.Context, id int64) (settingsdomain.SystemVersion, error) {
	return settingsdomain.RestoreSystemVersion(id, "v1.2.3", "稳定版", "", "", fixedTime(), fixedTime(), fixedTime())
}

func (s *storeSpy) CreateVersion(_ context.Context, version settingsdomain.SystemVersion) (settingsdomain.SystemVersion, error) {
	s.createdVersion = version
	return version, nil
}

func (s *storeSpy) UpdateVersion(_ context.Context, version settingsdomain.SystemVersion) (settingsdomain.SystemVersion, error) {
	return version, nil
}

func (s *storeSpy) DeleteVersion(_ context.Context, id int64) error {
	s.deletedVersionID = id
	return nil
}

func (s *storeSpy) DeleteVersions(_ context.Context, ids []int64) error {
	s.deletedVersionIDs = append([]int64(nil), ids...)
	return nil
}

func dictionaryWithItem(code string, item settingsdomain.DictionaryItem) (settingsdomain.Dictionary, error) {
	return settingsdomain.RestoreDictionary(1, code, "字典", []settingsdomain.DictionaryItem{item}, fixedTime(), fixedTime())
}

func mustDictionaryItem(t *testing.T, id, parentID int64, label, value string, level int, path string) settingsdomain.DictionaryItem {
	t.Helper()
	item, err := settingsdomain.RestoreDictionaryItem(id, parentID, label, value, "", 10, true, level, path, nil)
	if err != nil {
		t.Fatalf("RestoreDictionaryItem() error = %v", err)
	}
	return item
}

func mustDictionary(t *testing.T, id int64, code, name string, items []settingsdomain.DictionaryItem) settingsdomain.Dictionary {
	t.Helper()
	dictionary, err := settingsdomain.RestoreDictionary(id, code, name, items, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreDictionary() error = %v", err)
	}
	return dictionary
}

func appCode(t *testing.T, err error) int {
	t.Helper()
	appErr, ok := apperr.Parse(err)
	if !ok {
		t.Fatalf("error = %v, want app error", err)
	}
	return appErr.Code()
}

func fixedTime() time.Time {
	return time.Unix(1_800_000_000, 0).UTC()
}

func mustParam(t *testing.T, id int64, name, key, value, desc string) settingsdomain.SystemParam {
	t.Helper()
	param, err := settingsdomain.RestoreSystemParam(id, name, key, value, desc, fixedTime(), fixedTime())
	if err != nil {
		t.Fatalf("RestoreSystemParam() error = %v", err)
	}
	return param
}

func mustParamNoT(id int64, name, key, value, desc string) (settingsdomain.SystemParam, error) {
	return settingsdomain.RestoreSystemParam(id, name, key, value, desc, fixedTime(), fixedTime())
}

type versionCatalogSpy struct {
	menus         []accessusecase.MenuTreeInput
	exportMenuIDs []int64
}

func (s *versionCatalogSpy) ExportVersionMenus(_ context.Context, ids []int64) ([]accessusecase.MenuTreeInput, error) {
	s.exportMenuIDs = append([]int64(nil), ids...)
	return s.menus, nil
}

// importRunnerSpy records the import steps executed through the transaction so
// tests can assert the step order and the skip-after-failure behavior.
type importRunnerSpy struct {
	failOnMenus        bool
	failDictionaryCode string // empty means never fail; otherwise the code that fails
	failOnVersion      bool

	importedMenus           []accessusecase.MenuTreeInput
	replacedDictionaryCodes []string
	createdVersion          settingsdomain.SystemVersion
}

func (s *importRunnerSpy) RunImport(ctx context.Context, fn func(context.Context, usecase.ImportTransaction) error) error {
	return fn(ctx, s)
}

func (s *importRunnerSpy) ImportMenus(_ context.Context, menus []accessusecase.MenuTreeInput) error {
	s.importedMenus = append([]accessusecase.MenuTreeInput(nil), menus...)
	if s.failOnMenus {
		return errors.New("menu import failed")
	}
	return nil
}

func (s *importRunnerSpy) ReplaceDictionary(_ context.Context, dictionary settingsdomain.Dictionary) error {
	s.replacedDictionaryCodes = append(s.replacedDictionaryCodes, dictionary.Code)
	if s.failDictionaryCode != "" && dictionary.Code == s.failDictionaryCode {
		return errors.New("replace dictionary failed")
	}
	return nil
}

func (s *importRunnerSpy) CreateVersion(_ context.Context, version settingsdomain.SystemVersion) (settingsdomain.SystemVersion, error) {
	if s.failOnVersion {
		return settingsdomain.SystemVersion{}, errors.New("create version failed")
	}
	s.createdVersion = version
	return version, nil
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

// TestVersionBundleMenuJSONContract locks the version-bundle file format for
// menus: the exchange shape serializes the historical field names and never
// leaks database identity (parent_id, button id).
func contractVersionBundle() usecase.VersionBundle {
	return usecase.VersionBundle{
		Version: usecase.VersionInfo{Code: "v9.9.9", Name: "契约包"},
		Menus: []accessusecase.MenuTreeInput{{
			MenuInput: accessusecase.MenuInput{
				ParentID:  123,
				Name:      "角色权限",
				Path:      "/roles",
				Icon:      "safety",
				Component: "./Roles",
				Meta:      accessusecase.MenuMetaInput{KeepAlive: true},
				Sort:      22,
				Active:    true,
				Buttons:   []accessusecase.MenuButtonInput{{ID: 77, Name: "create", Description: "新增角色"}},
			},
			Children: []accessusecase.MenuTreeInput{{
				MenuInput: accessusecase.MenuInput{Name: "子菜单", Path: "/roles/child", Component: "./Child", Active: true},
			}},
		}},
	}
}

func TestVersionBundleMenuJSONContract(t *testing.T) {
	data, err := json.Marshal(contractVersionBundle())
	if err != nil {
		t.Fatalf("marshal bundle error = %v", err)
	}
	payload := string(data)
	for _, want := range []string{
		`"menus":[{`,
		`"name":"角色权限"`,
		`"path":"/roles"`,
		`"icon":"safety"`,
		`"component":"./Roles"`,
		`"meta":{"active_name":"","keep_alive":true,"default_menu":false,"close_tab":false,"transition_type":""}`,
		`"permission":""`,
		`"sort":22`,
		`"active":true`,
		`"buttons":[{"name":"create","description":"新增角色"}]`,
		`"children":[{"name":"子菜单"`,
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("bundle JSON missing contract fragment %s in %s", want, payload)
		}
	}
	for _, forbidden := range []string{`"parent_id"`, `"id":77`, `"ID"`} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("bundle JSON must not contain %s: %s", forbidden, payload)
		}
	}

	decoded := decodeVersionBundle(t, string(data))
	if len(decoded.Menus) != 1 || decoded.Menus[0].Path != "/roles" {
		t.Fatalf("decoded menus = %+v, want one /roles entry", decoded.Menus)
	}
	if len(decoded.Menus[0].Children) != 1 || decoded.Menus[0].Children[0].Path != "/roles/child" {
		t.Fatalf("decoded children = %+v, want nested /roles/child", decoded.Menus[0].Children)
	}
}
