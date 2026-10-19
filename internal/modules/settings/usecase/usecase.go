package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	accessusecase "github.com/NSObjects/echo-admin/internal/modules/access/usecase"
	"github.com/NSObjects/echo-admin/internal/modules/settings/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/pagination"
)

const (
	seedSiteNameConfigKey   = "site_name"
	dictionaryCodeProbeName = "dictionary"
	maxVersionCodeLength    = 80
	maxVersionDescLength    = 4000
)

// ListConfigs returns system configs.
func (u *Usecase) ListConfigs(ctx context.Context) ([]SystemConfig, error) {
	if err := u.ready(); err != nil {
		return nil, err
	}
	configs, err := u.store.ListConfigs(ctx)
	if err != nil {
		return nil, err
	}
	return mapConfigs(configs), nil
}

// UpsertConfig creates or updates a system config.
func (u *Usecase) UpsertConfig(ctx context.Context, input ConfigInput) (SystemConfig, error) {
	if err := u.ready(); err != nil {
		return SystemConfig{}, err
	}
	config, err := domain.RestoreSystemConfig(input.Key, input.Name, input.Value, input.Public, time.Time{})
	if err != nil {
		return SystemConfig{}, mapDomainError(err)
	}
	saved, err := u.store.UpsertConfig(ctx, config)
	if err != nil {
		return SystemConfig{}, err
	}
	return fromConfig(saved), nil
}

// DeleteConfig removes one operator-managed config key.
func (u *Usecase) DeleteConfig(ctx context.Context, key string) error {
	if err := u.ready(); err != nil {
		return err
	}
	config, err := domain.RestoreSystemConfig(key, "probe", "", false, time.Time{})
	if err != nil {
		return mapDomainError(err)
	}
	if config.Key == seedSiteNameConfigKey {
		return apperr.NewBadRequest("seed config cannot be deleted")
	}
	return u.store.DeleteConfig(ctx, config.Key)
}

// ListParams returns paginated system parameters.
func (u *Usecase) ListParams(ctx context.Context, input ParamListInput) (ParamListOutput, error) {
	if err := u.ready(); err != nil {
		return ParamListOutput{}, err
	}
	filter, err := normalizeParamListInput(input)
	if err != nil {
		return ParamListOutput{}, err
	}
	params, err := u.store.ListParams(ctx)
	if err != nil {
		return ParamListOutput{}, err
	}
	params = filterParams(params, input.Name, input.Key)
	pageParams, err := paginateParams(params, filter)
	if err != nil {
		return ParamListOutput{}, err
	}
	return ParamListOutput{
		Items:    mapParams(pageParams),
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Total:    len(params),
	}, nil
}

// FindParam returns one system parameter by id.
func (u *Usecase) FindParam(ctx context.Context, id int64) (SystemParam, error) {
	if err := u.ready(); err != nil {
		return SystemParam{}, err
	}
	if id <= 0 {
		return SystemParam{}, apperr.NewBadRequest("invalid system param id")
	}
	param, err := u.store.FindParamByID(ctx, id)
	if err != nil {
		return SystemParam{}, err
	}
	return fromParam(param), nil
}

// FindParamByKey returns one system parameter by key.
func (u *Usecase) FindParamByKey(ctx context.Context, key string) (SystemParam, error) {
	if err := u.ready(); err != nil {
		return SystemParam{}, err
	}
	param, err := domain.RestoreSystemParam(0, "probe", key, "probe", "", time.Time{}, time.Time{})
	if err != nil {
		return SystemParam{}, mapDomainError(err)
	}
	found, err := u.store.FindParamByKey(ctx, param.Key)
	if err != nil {
		return SystemParam{}, err
	}
	return fromParam(found), nil
}

// CreateParam validates and stores one system parameter.
func (u *Usecase) CreateParam(ctx context.Context, input ParamInput) (SystemParam, error) {
	if err := u.ready(); err != nil {
		return SystemParam{}, err
	}
	param, err := domain.RestoreSystemParam(0, input.Name, input.Key, input.Value, input.Desc, time.Time{}, time.Time{})
	if err != nil {
		return SystemParam{}, mapDomainError(err)
	}
	created, err := u.store.CreateParam(ctx, param)
	if err != nil {
		return SystemParam{}, err
	}
	return fromParam(created), nil
}

// UpdateParam replaces mutable fields for one system parameter.
func (u *Usecase) UpdateParam(ctx context.Context, input UpdateParamInput) (SystemParam, error) {
	if err := u.ready(); err != nil {
		return SystemParam{}, err
	}
	if input.ID <= 0 {
		return SystemParam{}, apperr.NewBadRequest("invalid system param id")
	}
	existing, err := u.store.FindParamByID(ctx, input.ID)
	if err != nil {
		return SystemParam{}, err
	}
	param, err := domain.RestoreSystemParam(input.ID, input.Name, input.Key, input.Value, input.Desc, existing.CreatedAt, time.Time{})
	if err != nil {
		return SystemParam{}, mapDomainError(err)
	}
	updated, err := u.store.UpdateParam(ctx, param)
	if err != nil {
		return SystemParam{}, err
	}
	return fromParam(updated), nil
}

// DeleteParam removes one system parameter by id.
func (u *Usecase) DeleteParam(ctx context.Context, id int64) error {
	return u.DeleteParams(ctx, []int64{id})
}

// DeleteParams removes system parameters by id.
func (u *Usecase) DeleteParams(ctx context.Context, ids []int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	ids, err := normalizePositiveIDs(ids, "system param ids are required", "invalid system param id")
	if err != nil {
		return err
	}
	return u.store.DeleteParams(ctx, ids)
}

// ListDictionaries returns dictionaries with their items.
func (u *Usecase) ListDictionaries(ctx context.Context) ([]Dictionary, error) {
	if err := u.ready(); err != nil {
		return nil, err
	}
	dictionaries, err := u.store.ListDictionaries(ctx)
	if err != nil {
		return nil, err
	}
	return mapDictionaries(dictionaries), nil
}

// ExportDictionaries returns all dictionaries in a portable JSON bundle shape.
func (u *Usecase) ExportDictionaries(ctx context.Context) (DictionaryBundle, error) {
	if err := u.ready(); err != nil {
		return DictionaryBundle{}, err
	}
	dictionaries, err := u.store.ListDictionaries(ctx)
	if err != nil {
		return DictionaryBundle{}, err
	}
	out := make([]VersionDictionary, 0, len(dictionaries))
	for _, dictionary := range dictionaries {
		out = append(out, versionDictionaryFromDomain(dictionary))
	}
	return DictionaryBundle{
		ExportTime:   time.Now().UTC().Format(time.RFC3339),
		Dictionaries: out,
	}, nil
}

// DictionaryBundleJSON returns a stable JSON export for all dictionaries.
func (u *Usecase) DictionaryBundleJSON(ctx context.Context) ([]byte, error) {
	bundle, err := u.ExportDictionaries(ctx)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal dictionary bundle: %w", err)
	}
	return data, nil
}

// ImportDictionaries validates and imports a dictionary bundle. Every
// dictionary replacement commits in one transaction, so a mid-import failure
// changes nothing.
func (u *Usecase) ImportDictionaries(ctx context.Context, bundle DictionaryBundle) ([]Dictionary, error) {
	if err := u.readyForImport(); err != nil {
		return nil, err
	}
	if len(bundle.Dictionaries) == 0 {
		return nil, apperr.NewBadRequest("dictionary bundle is empty")
	}
	dictionaries, err := domainDictionariesFromVersion(bundle.Dictionaries)
	if err != nil {
		return nil, mapDomainError(err)
	}
	err = u.imports.RunImport(ctx, func(txCtx context.Context, tx ImportTransaction) error {
		for _, dictionary := range dictionaries {
			if replaceErr := tx.ReplaceDictionary(txCtx, dictionary); replaceErr != nil {
				return replaceErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return u.ListDictionaries(ctx)
}

// CreateDictionary validates and stores a dictionary.
func (u *Usecase) CreateDictionary(ctx context.Context, input DictionaryInput) (Dictionary, error) {
	if err := u.ready(); err != nil {
		return Dictionary{}, err
	}
	dictionary, err := domain.RestoreDictionary(0, input.Code, input.Name, nil, time.Time{}, time.Time{})
	if err != nil {
		return Dictionary{}, mapDomainError(err)
	}
	created, err := u.store.CreateDictionary(ctx, dictionary)
	if err != nil {
		return Dictionary{}, err
	}
	return fromDictionary(created), nil
}

// UpdateDictionary changes a dictionary name while keeping its stable code.
func (u *Usecase) UpdateDictionary(ctx context.Context, input UpdateDictionaryInput) (Dictionary, error) {
	if err := u.ready(); err != nil {
		return Dictionary{}, err
	}
	dictionary, err := domain.RestoreDictionary(0, input.Code, input.Name, nil, time.Time{}, time.Time{})
	if err != nil {
		return Dictionary{}, mapDomainError(err)
	}
	updated, err := u.store.UpdateDictionary(ctx, dictionary)
	if err != nil {
		return Dictionary{}, err
	}
	return fromDictionary(updated), nil
}

// DeleteDictionary removes a dictionary and its items unless it is a runtime seed invariant.
func (u *Usecase) DeleteDictionary(ctx context.Context, code string) error {
	if err := u.ready(); err != nil {
		return err
	}
	normalized, err := normalizeDictionaryCode(code)
	if err != nil {
		return err
	}
	if normalized == domain.StatusDictionaryCode {
		return apperr.NewBadRequest("seed dictionary cannot be deleted")
	}
	return u.store.DeleteDictionary(ctx, normalized)
}

// AddDictionaryItem appends one dictionary item.
func (u *Usecase) AddDictionaryItem(ctx context.Context, code string, input DictionaryItemInput) (Dictionary, error) {
	if err := u.ready(); err != nil {
		return Dictionary{}, err
	}
	normalized, err := normalizeDictionaryCode(code)
	if err != nil {
		return Dictionary{}, err
	}
	item, err := u.prepareDictionaryItem(ctx, normalized, 0, input)
	if err != nil {
		return Dictionary{}, mapDomainError(err)
	}
	dictionary, err := u.store.AddDictionaryItem(ctx, normalized, item)
	if err != nil {
		return Dictionary{}, err
	}
	return fromDictionary(dictionary), nil
}

// UpdateDictionaryItem updates one dictionary item.
func (u *Usecase) UpdateDictionaryItem(ctx context.Context, code string, input DictionaryItemInput) (Dictionary, error) {
	if err := u.ready(); err != nil {
		return Dictionary{}, err
	}
	normalized, err := normalizeDictionaryCode(code)
	if err != nil {
		return Dictionary{}, err
	}
	item, err := u.prepareDictionaryItem(ctx, normalized, input.ID, input)
	if err != nil {
		return Dictionary{}, mapDomainError(err)
	}
	dictionary, err := u.store.UpdateDictionaryItem(ctx, normalized, item)
	if err != nil {
		return Dictionary{}, err
	}
	return fromDictionary(dictionary), nil
}

// DeleteDictionaryItem removes one dictionary item under a dictionary.
func (u *Usecase) DeleteDictionaryItem(ctx context.Context, code string, itemID int64) (Dictionary, error) {
	if err := u.ready(); err != nil {
		return Dictionary{}, err
	}
	if itemID <= 0 {
		return Dictionary{}, apperr.NewBadRequest("invalid dictionary item id")
	}
	normalized, err := normalizeDictionaryCode(code)
	if err != nil {
		return Dictionary{}, err
	}
	dictionary, err := u.store.DeleteDictionaryItem(ctx, normalized, itemID)
	if err != nil {
		return Dictionary{}, err
	}
	return fromDictionary(dictionary), nil
}

func (u *Usecase) prepareDictionaryItem(ctx context.Context, code string, id int64, input DictionaryItemInput) (domain.DictionaryItem, error) {
	if id < 0 {
		return domain.DictionaryItem{}, domain.ErrInvalidDictItemID
	}
	parentID := input.ParentID
	level := 0
	path := ""
	if parentID > 0 {
		if id > 0 && parentID == id {
			return domain.DictionaryItem{}, domain.ErrInvalidDictParent
		}
		parent, err := u.store.FindDictionaryItem(ctx, code, parentID)
		if err != nil {
			return domain.DictionaryItem{}, err
		}
		if id > 0 && dictionaryParentCreatesCycle(parent, id) {
			return domain.DictionaryItem{}, apperr.NewBadRequest("dictionary item parent creates a cycle")
		}
		level = parent.Level + 1
		path = dictionaryChildPath(parent)
	}
	return domain.RestoreDictionaryItem(
		id,
		parentID,
		input.Label,
		input.Value,
		input.Extend,
		input.Sort,
		input.Active,
		level,
		path,
		nil,
	)
}

// ListVersions returns release records ordered by publication time.
func (u *Usecase) ListVersions(ctx context.Context) ([]SystemVersion, error) {
	if err := u.ready(); err != nil {
		return nil, err
	}
	versions, err := u.store.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	return mapVersions(versions), nil
}

// FindVersion returns one release record by id.
func (u *Usecase) FindVersion(ctx context.Context, id int64) (SystemVersion, error) {
	if err := u.ready(); err != nil {
		return SystemVersion{}, err
	}
	if id <= 0 {
		return SystemVersion{}, apperr.NewBadRequest("invalid system version id")
	}
	version, err := u.store.FindVersionByID(ctx, id)
	if err != nil {
		return SystemVersion{}, err
	}
	return fromVersion(version), nil
}

// VersionJSON returns a stable JSON representation of one release record.
func (u *Usecase) VersionJSON(ctx context.Context, id int64) ([]byte, SystemVersion, error) {
	version, err := u.FindVersion(ctx, id)
	if err != nil {
		return nil, SystemVersion{}, err
	}
	if version.Data != "" {
		return []byte(version.Data), version, nil
	}
	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return nil, SystemVersion{}, err
	}
	return data, version, nil
}

// ExportVersion creates a version bundle from selected menus and dictionaries.
func (u *Usecase) ExportVersion(ctx context.Context, input ExportVersionInput) (SystemVersion, error) {
	if err := u.ready(); err != nil {
		return SystemVersion{}, err
	}
	resources, err := u.exportVersionResources(ctx, input)
	if err != nil {
		return SystemVersion{}, err
	}
	exportedAt := time.Now().UTC()
	bundle := VersionBundle{
		Version: VersionInfo{
			Name:        input.Name,
			Code:        input.Version,
			Description: input.Description,
			ExportTime:  exportedAt.Format(time.RFC3339),
		},
		Menus:        resources.menus,
		Dictionaries: resources.dictionaries,
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return SystemVersion{}, fmt.Errorf("marshal version bundle: %w", err)
	}
	return u.CreateVersion(ctx, VersionInput{
		Version:     input.Version,
		Name:        input.Name,
		Description: input.Description,
		Data:        string(data),
		PublishedAt: exportedAt,
	})
}

type versionExportResources struct {
	menus        []accessusecase.MenuTreeInput
	dictionaries []VersionDictionary
}

func (u *Usecase) exportVersionResources(ctx context.Context, input ExportVersionInput) (versionExportResources, error) {
	menuIDs, err := normalizeOptionalPositiveIDs(input.MenuIDs, "invalid menu id")
	if err != nil {
		return versionExportResources{}, err
	}
	catalogErr := u.ensureVersionCatalog(menuIDs)
	if catalogErr != nil {
		return versionExportResources{}, catalogErr
	}
	menus := []accessusecase.MenuTreeInput{}
	if len(menuIDs) > 0 {
		menus, err = u.catalog.ExportVersionMenus(ctx, menuIDs)
		if err != nil {
			return versionExportResources{}, err
		}
	}
	dictionaries, err := u.exportVersionDictionaries(ctx, input.DictionaryIDs)
	if err != nil {
		return versionExportResources{}, err
	}
	return versionExportResources{menus: menus, dictionaries: dictionaries}, nil
}

// ImportVersion imports a version bundle and records the import result.
// Menu upserts, dictionary replacements, and the version record commit in one
// transaction, so a mid-import failure leaves stored data unchanged.
func (u *Usecase) ImportVersion(ctx context.Context, bundle VersionBundle) (SystemVersion, error) {
	if err := u.readyForImport(); err != nil {
		return SystemVersion{}, err
	}
	plan, err := u.prepareVersionImport(bundle)
	if err != nil {
		return SystemVersion{}, err
	}
	var created domain.SystemVersion
	err = u.imports.RunImport(ctx, func(txCtx context.Context, tx ImportTransaction) error {
		if len(bundle.Menus) > 0 {
			if menuErr := tx.ImportMenus(txCtx, bundle.Menus); menuErr != nil {
				return menuErr
			}
		}
		for _, dictionary := range plan.dictionaries {
			if replaceErr := tx.ReplaceDictionary(txCtx, dictionary); replaceErr != nil {
				return replaceErr
			}
		}
		var createErr error
		created, createErr = tx.CreateVersion(txCtx, plan.version)
		return createErr
	})
	if err != nil {
		return SystemVersion{}, err
	}
	return fromVersion(created), nil
}

type versionImportPlan struct {
	version      domain.SystemVersion
	dictionaries []domain.Dictionary
}

func (u *Usecase) prepareVersionImport(bundle VersionBundle) (versionImportPlan, error) {
	if bundle.Version.Code == "" || bundle.Version.Name == "" {
		return versionImportPlan{}, apperr.NewBadRequest("invalid version bundle")
	}
	dictionaries, err := domainDictionariesFromVersion(bundle.Dictionaries)
	if err != nil {
		return versionImportPlan{}, mapDomainError(err)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return versionImportPlan{}, fmt.Errorf("marshal imported version bundle: %w", err)
	}
	importedAt := time.Now().UTC()
	version, err := domain.RestoreSystemVersion(
		0,
		importedVersionCode(bundle.Version.Code, importedAt),
		bundle.Version.Name,
		importedVersionDescription(bundle.Version.Description),
		string(data),
		importedAt,
		time.Time{},
		time.Time{},
	)
	if err != nil {
		return versionImportPlan{}, mapDomainError(err)
	}
	return versionImportPlan{version: version, dictionaries: dictionaries}, nil
}

// CreateVersion validates and stores one release record.
func (u *Usecase) CreateVersion(ctx context.Context, input VersionInput) (SystemVersion, error) {
	if err := u.ready(); err != nil {
		return SystemVersion{}, err
	}
	version, err := domain.RestoreSystemVersion(0, input.Version, input.Name, input.Description, input.Data, input.PublishedAt, time.Time{}, time.Time{})
	if err != nil {
		return SystemVersion{}, mapDomainError(err)
	}
	created, err := u.store.CreateVersion(ctx, version)
	if err != nil {
		return SystemVersion{}, err
	}
	return fromVersion(created), nil
}

// UpdateVersion changes one release record by id.
func (u *Usecase) UpdateVersion(ctx context.Context, input UpdateVersionInput) (SystemVersion, error) {
	if err := u.ready(); err != nil {
		return SystemVersion{}, err
	}
	if input.ID <= 0 {
		return SystemVersion{}, apperr.NewBadRequest("invalid system version id")
	}
	existing, err := u.store.FindVersionByID(ctx, input.ID)
	if err != nil {
		return SystemVersion{}, err
	}
	data := input.Data
	if data == "" {
		data = existing.Data
	}
	version, err := domain.RestoreSystemVersion(input.ID, input.Version, input.Name, input.Description, data, input.PublishedAt, time.Time{}, time.Time{})
	if err != nil {
		return SystemVersion{}, mapDomainError(err)
	}
	updated, err := u.store.UpdateVersion(ctx, version)
	if err != nil {
		return SystemVersion{}, err
	}
	return fromVersion(updated), nil
}

// DeleteVersion removes one release record.
func (u *Usecase) DeleteVersion(ctx context.Context, id int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	if id <= 0 {
		return apperr.NewBadRequest("invalid system version id")
	}
	return u.store.DeleteVersion(ctx, id)
}

// DeleteVersions removes release records by id.
func (u *Usecase) DeleteVersions(ctx context.Context, ids []int64) error {
	if err := u.ready(); err != nil {
		return err
	}
	ids, err := normalizePositiveIDs(ids, "system version ids are required", "invalid system version id")
	if err != nil {
		return err
	}
	return u.store.DeleteVersions(ctx, ids)
}

func (u *Usecase) ready() error {
	if u == nil || u.store == nil {
		return apperr.New(apperr.ErrInternalServer, "settings store is not configured")
	}
	return nil
}

// readyForImport guards the import workflows, which write through the
// transaction-scoped import runner instead of the plain store.
func (u *Usecase) readyForImport() error {
	if err := u.ready(); err != nil {
		return err
	}
	if u.imports == nil {
		return apperr.New(apperr.ErrInternalServer, "settings import runner is not configured")
	}
	return nil
}

func (u *Usecase) ensureVersionCatalog(menuIDs []int64) error {
	if len(menuIDs) == 0 {
		return nil
	}
	if u.catalog == nil {
		return apperr.New(apperr.ErrInternalServer, "version catalog is not configured")
	}
	return nil
}

func (u *Usecase) exportVersionDictionaries(ctx context.Context, ids []int64) ([]VersionDictionary, error) {
	ids, err := normalizeOptionalPositiveIDs(ids, "invalid dictionary id")
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []VersionDictionary{}, nil
	}
	dictionaries, err := u.store.ListDictionaries(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]domain.Dictionary, len(dictionaries))
	for _, dictionary := range dictionaries {
		byID[dictionary.ID] = dictionary
	}
	out := make([]VersionDictionary, 0, len(ids))
	for _, id := range ids {
		dictionary, ok := byID[id]
		if !ok {
			return nil, apperr.NewNotFound("dictionary")
		}
		out = append(out, versionDictionaryFromDomain(dictionary))
	}
	return out, nil
}

func domainDictionariesFromVersion(inputs []VersionDictionary) ([]domain.Dictionary, error) {
	out := make([]domain.Dictionary, 0, len(inputs))
	for _, input := range inputs {
		items, err := domainItemsFromVersion(input.Items)
		if err != nil {
			return nil, err
		}
		dictionary, err := domain.RestoreDictionary(0, input.Code, input.Name, items, time.Time{}, time.Time{})
		if err != nil {
			return nil, err
		}
		out = append(out, dictionary)
	}
	return out, nil
}

func importedVersionCode(code string, importedAt time.Time) string {
	suffix := "_imported_" + importedAt.Format("20060102150405")
	if len(code)+len(suffix) <= maxVersionCodeLength {
		return code + suffix
	}
	prefixLimit := maxVersionCodeLength - len(suffix)
	if prefixLimit <= 0 {
		return importedAt.Format("20060102150405")
	}
	return code[:prefixLimit] + suffix
}

func importedVersionDescription(description string) string {
	if description == "" {
		return "导入版本"
	}
	prefix := "导入版本: "
	return prefix + truncateUTF8Bytes(description, maxVersionDescLength-len(prefix))
}

func truncateUTF8Bytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for index, r := range value {
		if index+utf8.RuneLen(r) > limit {
			return value[:index]
		}
	}
	return value
}

func normalizeDictionaryCode(code string) (string, error) {
	dictionary, err := domain.RestoreDictionary(0, code, dictionaryCodeProbeName, nil, time.Time{}, time.Time{})
	if err != nil {
		return "", mapDomainError(err)
	}
	return dictionary.Code, nil
}

func normalizeOptionalPositiveIDs(ids []int64, invalidMessage string) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, apperr.NewBadRequest(invalidMessage)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func normalizePositiveIDs(ids []int64, emptyMessage, invalidMessage string) ([]int64, error) {
	if len(ids) == 0 {
		return nil, apperr.NewBadRequest(emptyMessage)
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, apperr.NewBadRequest(invalidMessage)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

type paramListFilter struct {
	pagination.Window
}

func normalizeParamListInput(input ParamListInput) (paramListFilter, error) {
	window, err := pagination.Normalize(input.Page, input.PageSize, pagination.DefaultOptions)
	if err != nil {
		return paramListFilter{}, apperr.NewBadRequest("invalid pagination")
	}
	return paramListFilter{Window: window}, nil
}

func filterParams(params []domain.SystemParam, name, key string) []domain.SystemParam {
	name = strings.TrimSpace(name)
	key = strings.TrimSpace(key)
	if name == "" && key == "" {
		return params
	}
	out := make([]domain.SystemParam, 0, len(params))
	for _, param := range params {
		if name != "" && !strings.Contains(param.Name, name) {
			continue
		}
		if key != "" && !strings.Contains(param.Key, key) {
			continue
		}
		out = append(out, param)
	}
	return out
}

func paginateParams(params []domain.SystemParam, filter paramListFilter) ([]domain.SystemParam, error) {
	start, end, ok, err := pagination.Bounds(len(params), filter.Offset, filter.Limit)
	if err != nil {
		return nil, apperr.NewBadRequest("invalid pagination")
	}
	if !ok {
		return []domain.SystemParam{}, nil
	}
	return params[start:end], nil
}

func versionDictionaryFromDomain(dictionary domain.Dictionary) VersionDictionary {
	return VersionDictionary{
		Code:  dictionary.Code,
		Name:  dictionary.Name,
		Items: versionDictionaryItemsFromDomain(dictionary.Items),
	}
}

func versionDictionaryItemsFromDomain(items []domain.DictionaryItem) []VersionDictionaryItem {
	out := make([]VersionDictionaryItem, 0, len(items))
	for _, item := range items {
		out = append(out, VersionDictionaryItem{
			ParentID: item.ParentID,
			Label:    item.Label,
			Value:    item.Value,
			Extend:   item.Extend,
			Sort:     item.Sort,
			Active:   item.Active,
			Level:    item.Level,
			Path:     item.Path,
		})
	}
	return out
}

func domainItemsFromVersion(items []VersionDictionaryItem) ([]domain.DictionaryItem, error) {
	out := make([]domain.DictionaryItem, 0, len(items))
	for _, input := range items {
		item, err := domain.RestoreDictionaryItem(
			0,
			input.ParentID,
			input.Label,
			input.Value,
			input.Extend,
			input.Sort,
			input.Active,
			input.Level,
			input.Path,
			nil,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func mapConfigs(configs []domain.SystemConfig) []SystemConfig {
	out := make([]SystemConfig, 0, len(configs))
	for _, config := range configs {
		out = append(out, fromConfig(config))
	}
	return out
}

func mapParams(params []domain.SystemParam) []SystemParam {
	out := make([]SystemParam, 0, len(params))
	for _, param := range params {
		out = append(out, fromParam(param))
	}
	return out
}

func mapDictionaries(dictionaries []domain.Dictionary) []Dictionary {
	out := make([]Dictionary, 0, len(dictionaries))
	for _, dictionary := range dictionaries {
		out = append(out, fromDictionary(dictionary))
	}
	return out
}

func mapDictionaryItems(items []domain.DictionaryItem) []DictionaryItem {
	return dictionaryItemTree(items)
}

func dictionaryItemTree(items []domain.DictionaryItem) []DictionaryItem {
	childrenByParent := make(map[int64][]domain.DictionaryItem, len(items))
	for _, item := range items {
		childrenByParent[item.ParentID] = append(childrenByParent[item.ParentID], item)
	}
	return dictionaryItemChildren(childrenByParent, 0)
}

func dictionaryItemChildren(childrenByParent map[int64][]domain.DictionaryItem, parentID int64) []DictionaryItem {
	children := childrenByParent[parentID]
	out := make([]DictionaryItem, 0, len(children))
	for _, child := range children {
		item := fromDictionaryItem(child)
		item.Children = dictionaryItemChildren(childrenByParent, child.ID)
		out = append(out, item)
	}
	return out
}

func mapVersions(versions []domain.SystemVersion) []SystemVersion {
	out := make([]SystemVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, fromVersion(version))
	}
	return out
}

func dictionaryChildPath(parent domain.DictionaryItem) string {
	parentID := strconv.FormatInt(parent.ID, 10)
	if parent.Path == "" {
		return parentID
	}
	return parent.Path + "," + parentID
}

func dictionaryParentCreatesCycle(parent domain.DictionaryItem, id int64) bool {
	if parent.ID == id {
		return true
	}
	for _, raw := range strings.Split(parent.Path, ",") {
		if raw == "" {
			continue
		}
		ancestorID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		if ancestorID == id {
			return true
		}
	}
	return false
}

func fromConfig(config domain.SystemConfig) SystemConfig {
	return SystemConfig{
		Key:       config.Key,
		Name:      config.Name,
		Value:     config.Value,
		Public:    config.Public,
		UpdatedAt: config.UpdatedAt,
	}
}

func fromParam(param domain.SystemParam) SystemParam {
	return SystemParam{
		ID:        param.ID,
		Name:      param.Name,
		Key:       param.Key,
		Value:     param.Value,
		Desc:      param.Desc,
		CreatedAt: param.CreatedAt,
		UpdatedAt: param.UpdatedAt,
	}
}

func fromDictionary(dictionary domain.Dictionary) Dictionary {
	return Dictionary{
		ID:        dictionary.ID,
		Code:      dictionary.Code,
		Name:      dictionary.Name,
		Items:     mapDictionaryItems(dictionary.Items),
		CreatedAt: dictionary.CreatedAt,
		UpdatedAt: dictionary.UpdatedAt,
	}
}

func fromDictionaryItem(item domain.DictionaryItem) DictionaryItem {
	return DictionaryItem{
		ID:       item.ID,
		ParentID: item.ParentID,
		Label:    item.Label,
		Value:    item.Value,
		Extend:   item.Extend,
		Sort:     item.Sort,
		Active:   item.Active,
		Level:    item.Level,
		Path:     item.Path,
		Children: mapDictionaryItems(item.Children),
	}
}

func fromVersion(version domain.SystemVersion) SystemVersion {
	return SystemVersion{
		ID:          version.ID,
		Version:     version.Version,
		Name:        version.Name,
		Description: version.Description,
		Data:        version.Data,
		PublishedAt: version.PublishedAt,
		CreatedAt:   version.CreatedAt,
		UpdatedAt:   version.UpdatedAt,
	}
}

func mapDomainError(err error) error {
	for _, entry := range domainErrorMessages {
		if errors.Is(err, entry.err) {
			return apperr.NewBadRequest(entry.message)
		}
	}
	return err
}

var domainErrorMessages = []struct {
	err     error
	message string
}{
	{domain.ErrInvalidConfigKey, "invalid config key"},
	{domain.ErrInvalidConfigName, "invalid config name"},
	{domain.ErrInvalidDictID, "invalid dictionary id"},
	{domain.ErrInvalidDictCode, "invalid dictionary code"},
	{domain.ErrInvalidDictName, "invalid dictionary name"},
	{domain.ErrInvalidDictItemID, "invalid dictionary item id"},
	{domain.ErrInvalidDictParent, "invalid dictionary item parent"},
	{domain.ErrInvalidDictLabel, "invalid dictionary item label"},
	{domain.ErrInvalidDictValue, "invalid dictionary item value"},
	{domain.ErrInvalidDictExtend, "invalid dictionary item extend"},
	{domain.ErrInvalidDictLevel, "invalid dictionary item level"},
	{domain.ErrInvalidDictPath, "invalid dictionary item path"},
	{domain.ErrInvalidParamID, "invalid system param id"},
	{domain.ErrInvalidParamName, "invalid system param name"},
	{domain.ErrInvalidParamKey, "invalid system param key"},
	{domain.ErrInvalidParamValue, "invalid system param value"},
	{domain.ErrInvalidParamDesc, "invalid system param description"},
	{domain.ErrInvalidVersionID, "invalid system version id"},
	{domain.ErrInvalidVersion, "invalid system version"},
	{domain.ErrInvalidVersionName, "invalid system version name"},
	{domain.ErrInvalidVersionDesc, "invalid system version description"},
}
