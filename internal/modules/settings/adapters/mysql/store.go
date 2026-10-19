// Package mysql persists system settings and dictionaries in MySQL.
package mysql

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/modules/settings/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	inframysql "github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysql"
)

// Store persists settings and dictionaries in MySQL.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the MySQL settings tables.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create settings store: nil context")
	}
	if db == nil {
		return nil, errors.New("create settings store: nil db")
	}
	store := &Store{db: db}
	if err := db.WithContext(ctx).AutoMigrate(&configModel{}, &paramModel{}, &dictionaryModel{}, &dictionaryItemModel{}, &versionModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate settings tables")
	}
	return store, nil
}

// WithDB returns a store bound to db for transaction-scoped settings operations.
func (s *Store) WithDB(db *gorm.DB) *Store {
	return &Store{db: db}
}

// ListConfigs returns configs ordered by key.
func (s *Store) ListConfigs(ctx context.Context) ([]domain.SystemConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []configModel
	if err := s.db.WithContext(ctx).Order("`key` ASC").Find(&models).Error; err != nil {
		return nil, apperr.WrapDatabase(err, "list configs")
	}
	configs := make([]domain.SystemConfig, 0, len(models))
	for _, model := range models {
		config, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}

// UpsertConfig creates or updates one config by key.
func (s *Store) UpsertConfig(ctx context.Context, config domain.SystemConfig) (domain.SystemConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemConfig{}, err
	}
	var model configModel
	err := s.db.WithContext(ctx).First(&model, "`key` = ?", config.Key).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.SystemConfig{}, apperr.WrapDatabase(err, "find config")
	}
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		model = configModelFromDomain(config, now)
		if createErr := s.db.WithContext(ctx).Create(&model).Error; createErr != nil {
			return domain.SystemConfig{}, inframysql.MapWriteError(createErr, "config key already exists", "create config")
		}
		return model.toDomain()
	}
	model.Name = config.Name
	model.Value = config.Value
	model.Public = config.Public
	model.UpdatedAt = now
	if saveErr := s.db.WithContext(ctx).Save(&model).Error; saveErr != nil {
		return domain.SystemConfig{}, inframysql.MapWriteError(saveErr, "config key already exists", "update config")
	}
	return model.toDomain()
}

// DeleteConfig removes one config by key.
func (s *Store) DeleteConfig(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Delete(&configModel{}, "`key` = ?", key)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete config")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("config")
	}
	return nil
}

// ListParams returns parameters ordered by creation time.
func (s *Store) ListParams(ctx context.Context) ([]domain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []paramModel
	if err := s.db.WithContext(ctx).Order("id DESC").Find(&models).Error; err != nil {
		return nil, apperr.WrapDatabase(err, "list system params")
	}
	params := make([]domain.SystemParam, 0, len(models))
	for _, model := range models {
		param, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		params = append(params, param)
	}
	return params, nil
}

// FindParamByID returns one parameter by id.
func (s *Store) FindParamByID(ctx context.Context, id int64) (domain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemParam{}, err
	}
	return s.findParamByID(ctx, id)
}

// FindParamByKey returns one parameter by key.
func (s *Store) FindParamByKey(ctx context.Context, key string) (domain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemParam{}, err
	}
	var model paramModel
	if err := s.db.WithContext(ctx).First(&model, "`key` = ?", key).Error; err != nil {
		return domain.SystemParam{}, inframysql.MapReadError(err, "system param", "find system param")
	}
	return model.toDomain()
}

// CreateParam inserts one parameter.
func (s *Store) CreateParam(ctx context.Context, param domain.SystemParam) (domain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemParam{}, err
	}
	now := time.Now().UTC()
	model := paramModelFromDomain(param, now)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.SystemParam{}, inframysql.MapWriteError(err, "system param key already exists", "create system param")
	}
	return model.toDomain()
}

// UpdateParam replaces mutable fields for one parameter.
func (s *Store) UpdateParam(ctx context.Context, param domain.SystemParam) (domain.SystemParam, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemParam{}, err
	}
	now := time.Now().UTC()
	model := paramModelFromDomain(param, now)
	result := s.db.WithContext(ctx).Model(&paramModel{}).
		Where("id = ?", param.ID).
		Updates(map[string]interface{}{
			"name":       model.Name,
			"key":        model.Key,
			"value":      model.Value,
			"desc":       model.Desc,
			"updated_at": now,
		})
	if result.Error != nil {
		return domain.SystemParam{}, inframysql.MapWriteError(result.Error, "system param key already exists", "update system param")
	}
	if result.RowsAffected == 0 {
		return domain.SystemParam{}, apperr.NewNotFound("system param")
	}
	return s.findParamByID(ctx, param.ID)
}

// DeleteParam removes one parameter by id.
func (s *Store) DeleteParam(ctx context.Context, id int64) error {
	return s.DeleteParams(ctx, []int64{id})
}

// DeleteParams removes parameters by id.
func (s *Store) DeleteParams(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Delete(&paramModel{}, "id IN ?", ids)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete system params")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("system param")
	}
	return nil
}

// ListDictionaries returns dictionaries with items ordered for display.
func (s *Store) ListDictionaries(ctx context.Context) ([]domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []dictionaryModel
	err := s.db.WithContext(ctx).
		Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("level ASC, sort ASC, id ASC") }).
		Order("id DESC").
		Find(&models).Error
	if err != nil {
		return nil, apperr.WrapDatabase(err, "list dictionaries")
	}
	dictionaries := make([]domain.Dictionary, 0, len(models))
	for _, model := range models {
		dictionary, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		dictionaries = append(dictionaries, dictionary)
	}
	return dictionaries, nil
}

// CreateDictionary inserts a dictionary.
func (s *Store) CreateDictionary(ctx context.Context, dictionary domain.Dictionary) (domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return domain.Dictionary{}, err
	}
	now := time.Now().UTC()
	model := dictionaryModelFromDomain(dictionary, now)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.Dictionary{}, inframysql.MapWriteError(err, "dictionary code already exists", "create dictionary")
	}
	return s.findDictionaryByCode(ctx, model.Code)
}

// UpdateDictionary changes a dictionary name by stable code.
func (s *Store) UpdateDictionary(ctx context.Context, dictionary domain.Dictionary) (domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return domain.Dictionary{}, err
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&dictionaryModel{}).
		Where("code = ?", dictionary.Code).
		Updates(map[string]interface{}{
			"name":       dictionary.Name,
			"updated_at": now,
		})
	if result.Error != nil {
		return domain.Dictionary{}, inframysql.MapWriteError(result.Error, "dictionary code already exists", "update dictionary")
	}
	if result.RowsAffected == 0 {
		return domain.Dictionary{}, apperr.NewNotFound("dictionary")
	}
	return s.findDictionaryByCode(ctx, dictionary.Code)
}

// DeleteDictionary removes a dictionary and all of its items.
func (s *Store) DeleteDictionary(ctx context.Context, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return deleteDictionary(ctx, tx, code)
	})
}

// ReplaceDictionary removes any dictionary stored under code and creates the
// replacement in its place. It runs on the receiver's db, so the outer import
// transaction scopes it; a missing dictionary is not an error.
func (s *Store) ReplaceDictionary(ctx context.Context, dictionary domain.Dictionary) (domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return domain.Dictionary{}, err
	}
	if err := deleteDictionary(ctx, s.db, dictionary.Code); err != nil && !isNotFound(err) {
		return domain.Dictionary{}, err
	}
	now := time.Now().UTC()
	model := dictionaryModelFromDomain(dictionary, now)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.Dictionary{}, inframysql.MapWriteError(err, "dictionary code already exists", "replace dictionary")
	}
	return s.findDictionaryByCode(ctx, model.Code)
}

// deleteDictionary removes one dictionary and its items on db and reports a
// missing dictionary as not-found.
func deleteDictionary(ctx context.Context, db *gorm.DB, code string) error {
	var dictionary dictionaryModel
	if err := db.WithContext(ctx).First(&dictionary, "code = ?", code).Error; err != nil {
		return inframysql.MapReadError(err, "dictionary", "find dictionary")
	}
	if err := db.WithContext(ctx).Delete(&dictionaryItemModel{}, "dictionary_id = ?", dictionary.ID).Error; err != nil {
		return apperr.WrapDatabase(err, "delete dictionary items")
	}
	result := db.WithContext(ctx).Delete(&dictionaryModel{}, "id = ?", dictionary.ID)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete dictionary")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("dictionary")
	}
	return nil
}

func isNotFound(err error) bool {
	appErr, ok := apperr.Parse(err)
	return ok && appErr.Code() == apperr.ErrNotFound
}

// AddDictionaryItem inserts one dictionary item under code.
func (s *Store) AddDictionaryItem(ctx context.Context, code string, item domain.DictionaryItem) (domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return domain.Dictionary{}, err
	}
	var dictionary dictionaryModel
	if err := s.db.WithContext(ctx).First(&dictionary, "code = ?", code).Error; err != nil {
		return domain.Dictionary{}, inframysql.MapReadError(err, "dictionary", "find dictionary")
	}
	model := dictionaryItemModelFromDomain(dictionary.ID, item)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.Dictionary{}, apperr.WrapDatabase(err, "create dictionary item")
	}
	return s.findDictionaryByCode(ctx, code)
}

// UpdateDictionaryItem updates one dictionary item under code.
func (s *Store) UpdateDictionaryItem(ctx context.Context, code string, item domain.DictionaryItem) (domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return domain.Dictionary{}, err
	}
	var dictionary dictionaryModel
	if err := s.db.WithContext(ctx).First(&dictionary, "code = ?", code).Error; err != nil {
		return domain.Dictionary{}, inframysql.MapReadError(err, "dictionary", "find dictionary")
	}
	model := dictionaryItemModelFromDomain(dictionary.ID, item)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing dictionaryItemModel
		if err := tx.First(&existing, "id = ? AND dictionary_id = ?", item.ID, dictionary.ID).Error; err != nil {
			return inframysql.MapReadError(err, "dictionary item", "find dictionary item")
		}
		result := tx.Model(&existing).Updates(map[string]interface{}{
			"parent_id": model.ParentID,
			"label":     model.Label,
			"value":     model.Value,
			"extend":    model.Extend,
			"sort":      model.Sort,
			"active":    model.Active,
			"level":     model.Level,
			"path":      model.Path,
		})
		if result.Error != nil {
			return apperr.WrapDatabase(result.Error, "update dictionary item")
		}
		return refreshDictionaryItemChildren(tx, dictionary.ID, item.ID)
	})
	if err != nil {
		return domain.Dictionary{}, err
	}
	return s.findDictionaryByCode(ctx, code)
}

// FindDictionaryItem returns one item under a dictionary code.
func (s *Store) FindDictionaryItem(ctx context.Context, code string, itemID int64) (domain.DictionaryItem, error) {
	if err := ctx.Err(); err != nil {
		return domain.DictionaryItem{}, err
	}
	var item dictionaryItemModel
	err := s.db.WithContext(ctx).
		Joins("JOIN settings_dictionaries ON settings_dictionaries.id = settings_dictionary_items.dictionary_id").
		Where("settings_dictionaries.code = ? AND settings_dictionary_items.id = ?", code, itemID).
		First(&item).Error
	if err != nil {
		return domain.DictionaryItem{}, inframysql.MapReadError(err, "dictionary item", "find dictionary item")
	}
	return item.toDomain()
}

// DeleteDictionaryItem removes one dictionary item under code.
func (s *Store) DeleteDictionaryItem(ctx context.Context, code string, itemID int64) (domain.Dictionary, error) {
	if err := ctx.Err(); err != nil {
		return domain.Dictionary{}, err
	}
	var dictionary dictionaryModel
	if err := s.db.WithContext(ctx).First(&dictionary, "code = ?", code).Error; err != nil {
		return domain.Dictionary{}, inframysql.MapReadError(err, "dictionary", "find dictionary")
	}
	var children int64
	if err := s.db.WithContext(ctx).Model(&dictionaryItemModel{}).
		Where("dictionary_id = ? AND parent_id = ?", dictionary.ID, itemID).
		Count(&children).Error; err != nil {
		return domain.Dictionary{}, apperr.WrapDatabase(err, "count dictionary item children")
	}
	if children > 0 {
		return domain.Dictionary{}, apperr.NewBadRequest("dictionary item has children")
	}
	result := s.db.WithContext(ctx).Delete(&dictionaryItemModel{}, "id = ? AND dictionary_id = ?", itemID, dictionary.ID)
	if result.Error != nil {
		return domain.Dictionary{}, apperr.WrapDatabase(result.Error, "delete dictionary item")
	}
	if result.RowsAffected == 0 {
		return domain.Dictionary{}, apperr.NewNotFound("dictionary item")
	}
	return s.findDictionaryByCode(ctx, code)
}

// ListVersions returns release records in reverse chronological order.
func (s *Store) ListVersions(ctx context.Context) ([]domain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []versionModel
	if err := s.db.WithContext(ctx).Order("published_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, apperr.WrapDatabase(err, "list system versions")
	}
	versions := make([]domain.SystemVersion, 0, len(models))
	for _, model := range models {
		version, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, nil
}

// FindVersionByID returns one release record by id.
func (s *Store) FindVersionByID(ctx context.Context, id int64) (domain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemVersion{}, err
	}
	return s.findVersionByID(ctx, id)
}

// CreateVersion inserts one release record.
func (s *Store) CreateVersion(ctx context.Context, version domain.SystemVersion) (domain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemVersion{}, err
	}
	now := time.Now().UTC()
	model := versionModelFromDomain(version, now)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.SystemVersion{}, inframysql.MapWriteError(err, "system version already exists", "create system version")
	}
	return model.toDomain()
}

// UpdateVersion changes one release record by id.
func (s *Store) UpdateVersion(ctx context.Context, version domain.SystemVersion) (domain.SystemVersion, error) {
	if err := ctx.Err(); err != nil {
		return domain.SystemVersion{}, err
	}
	now := time.Now().UTC()
	model := versionModelFromDomain(version, now)
	result := s.db.WithContext(ctx).Model(&versionModel{}).
		Where("id = ?", version.ID).
		Updates(map[string]interface{}{
			"version":      model.Version,
			"name":         model.Name,
			"description":  model.Description,
			"data":         model.Data,
			"published_at": model.PublishedAt,
			"updated_at":   now,
		})
	if result.Error != nil {
		return domain.SystemVersion{}, inframysql.MapWriteError(result.Error, "system version already exists", "update system version")
	}
	if result.RowsAffected == 0 {
		return domain.SystemVersion{}, apperr.NewNotFound("system version")
	}
	return s.findVersionByID(ctx, version.ID)
}

// DeleteVersion removes one release record.
func (s *Store) DeleteVersion(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.DeleteVersions(ctx, []int64{id})
}

// DeleteVersions removes release records by id.
func (s *Store) DeleteVersions(ctx context.Context, ids []int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Delete(&versionModel{}, "id IN ?", ids)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete system version")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("system version")
	}
	return nil
}

// InstallInitialSettings creates the settings required by a fresh installation.
func (s *Store) InstallInitialSettings(ctx context.Context, siteName string) error {
	config, err := domain.RestoreSystemConfig("site_name", "站点名称", siteName, true, time.Now().UTC())
	if err != nil {
		return err
	}
	if _, err := s.UpsertConfig(ctx, config); err != nil {
		return err
	}
	return s.seedStatusDictionary(ctx)
}

func (s *Store) seedStatusDictionary(ctx context.Context) error {
	var existing dictionaryModel
	err := s.db.WithContext(ctx).Where("code = ?", domain.StatusDictionaryCode).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.WrapDatabase(err, "find seed dictionary")
	}
	now := time.Now().UTC()
	dictionary, err := domain.StatusDictionaryBaseline(now)
	if err != nil {
		return err
	}
	model := dictionaryModelFromDomain(dictionary, now)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return apperr.WrapDatabase(err, "create seed dictionary")
	}
	return nil
}

func (s *Store) findDictionaryByCode(ctx context.Context, code string) (domain.Dictionary, error) {
	var model dictionaryModel
	err := s.db.WithContext(ctx).
		Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC, id ASC") }).
		First(&model, "code = ?", code).Error
	if err != nil {
		return domain.Dictionary{}, inframysql.MapReadError(err, "dictionary", "find dictionary")
	}
	return model.toDomain()
}

func (s *Store) findVersionByID(ctx context.Context, id int64) (domain.SystemVersion, error) {
	var model versionModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return domain.SystemVersion{}, inframysql.MapReadError(err, "system version", "find system version")
	}
	return model.toDomain()
}

func (s *Store) findParamByID(ctx context.Context, id int64) (domain.SystemParam, error) {
	var model paramModel
	if err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return domain.SystemParam{}, inframysql.MapReadError(err, "system param", "find system param")
	}
	return model.toDomain()
}

type configModel struct {
	Key       string    `gorm:"primaryKey;type:varchar(80)"`
	Name      string    `gorm:"type:varchar(120);not null"`
	Value     string    `gorm:"type:text;not null"`
	Public    bool      `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (configModel) TableName() string {
	return "settings_configs"
}

func configModelFromDomain(config domain.SystemConfig, updatedAt time.Time) configModel {
	return configModel{
		Key:       config.Key,
		Name:      config.Name,
		Value:     config.Value,
		Public:    config.Public,
		UpdatedAt: updatedAt,
	}
}

func (m configModel) toDomain() (domain.SystemConfig, error) {
	return domain.RestoreSystemConfig(m.Key, m.Name, m.Value, m.Public, m.UpdatedAt)
}

type paramModel struct {
	ID        int64     `gorm:"primaryKey"`
	Name      string    `gorm:"type:varchar(120);not null"`
	Key       string    `gorm:"column:key;type:varchar(80);not null;uniqueIndex"`
	Value     string    `gorm:"type:text;not null"`
	Desc      string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (paramModel) TableName() string {
	return "settings_system_params"
}

func paramModelFromDomain(param domain.SystemParam, now time.Time) paramModel {
	return paramModel{
		ID:        param.ID,
		Name:      param.Name,
		Key:       param.Key,
		Value:     param.Value,
		Desc:      param.Desc,
		CreatedAt: coalesceTime(param.CreatedAt, now),
		UpdatedAt: coalesceTime(param.UpdatedAt, now),
	}
}

func (m paramModel) toDomain() (domain.SystemParam, error) {
	return domain.RestoreSystemParam(m.ID, m.Name, m.Key, m.Value, m.Desc, m.CreatedAt, m.UpdatedAt)
}

type dictionaryModel struct {
	ID        int64                 `gorm:"primaryKey"`
	Code      string                `gorm:"type:varchar(80);not null;uniqueIndex"`
	Name      string                `gorm:"type:varchar(120);not null"`
	Items     []dictionaryItemModel `gorm:"foreignKey:DictionaryID"`
	CreatedAt time.Time             `gorm:"not null"`
	UpdatedAt time.Time             `gorm:"not null"`
}

func (dictionaryModel) TableName() string {
	return "settings_dictionaries"
}

func dictionaryModelFromDomain(dictionary domain.Dictionary, now time.Time) dictionaryModel {
	items := dictionary.Items
	modelItems := make([]dictionaryItemModel, 0, len(items))
	for _, item := range items {
		modelItems = append(modelItems, dictionaryItemModelFromDomain(0, item))
	}
	return dictionaryModel{
		ID:        dictionary.ID,
		Code:      dictionary.Code,
		Name:      dictionary.Name,
		Items:     modelItems,
		CreatedAt: coalesceTime(dictionary.CreatedAt, now),
		UpdatedAt: coalesceTime(dictionary.UpdatedAt, now),
	}
}

func (m dictionaryModel) toDomain() (domain.Dictionary, error) {
	items := make([]domain.DictionaryItem, 0, len(m.Items))
	for _, itemModel := range m.Items {
		item, err := itemModel.toDomain()
		if err != nil {
			return domain.Dictionary{}, err
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Sort == items[j].Sort {
			return items[i].ID < items[j].ID
		}
		return items[i].Sort < items[j].Sort
	})
	return domain.RestoreDictionary(m.ID, m.Code, m.Name, items, m.CreatedAt, m.UpdatedAt)
}

type dictionaryItemModel struct {
	ID           int64  `gorm:"primaryKey"`
	DictionaryID int64  `gorm:"not null;index"`
	ParentID     int64  `gorm:"not null;default:0;index"`
	Label        string `gorm:"type:varchar(120);not null"`
	Value        string `gorm:"type:varchar(120);not null"`
	Extend       string `gorm:"type:text;not null"`
	Sort         int    `gorm:"not null"`
	Active       bool   `gorm:"not null"`
	Level        int    `gorm:"not null;default:0;index"`
	Path         string `gorm:"type:text;not null"`
}

func (dictionaryItemModel) TableName() string {
	return "settings_dictionary_items"
}

func dictionaryItemModelFromDomain(dictionaryID int64, item domain.DictionaryItem) dictionaryItemModel {
	return dictionaryItemModel{
		ID:           item.ID,
		DictionaryID: dictionaryID,
		ParentID:     item.ParentID,
		Label:        item.Label,
		Value:        item.Value,
		Extend:       item.Extend,
		Sort:         item.Sort,
		Active:       item.Active,
		Level:        item.Level,
		Path:         item.Path,
	}
}

func (m dictionaryItemModel) toDomain() (domain.DictionaryItem, error) {
	return domain.RestoreDictionaryItem(m.ID, m.ParentID, m.Label, m.Value, m.Extend, m.Sort, m.Active, m.Level, m.Path, nil)
}

func refreshDictionaryItemChildren(tx *gorm.DB, dictionaryID, parentID int64) error {
	var parent dictionaryItemModel
	if err := tx.First(&parent, "id = ? AND dictionary_id = ?", parentID, dictionaryID).Error; err != nil {
		return inframysql.MapReadError(err, "dictionary item", "find dictionary item")
	}
	var children []dictionaryItemModel
	if err := tx.Where("dictionary_id = ? AND parent_id = ?", dictionaryID, parentID).Find(&children).Error; err != nil {
		return apperr.WrapDatabase(err, "list dictionary item children")
	}
	for _, child := range children {
		child.Level = parent.Level + 1
		child.Path = childPath(parent)
		if err := tx.Save(&child).Error; err != nil {
			return apperr.WrapDatabase(err, "update dictionary item child path")
		}
		if err := refreshDictionaryItemChildren(tx, dictionaryID, child.ID); err != nil {
			return err
		}
	}
	return nil
}

func childPath(parent dictionaryItemModel) string {
	parentID := strconv.FormatInt(parent.ID, 10)
	if parent.Path == "" {
		return parentID
	}
	return parent.Path + "," + parentID
}

type versionModel struct {
	ID          int64     `gorm:"primaryKey"`
	Version     string    `gorm:"type:varchar(80);not null;uniqueIndex"`
	Name        string    `gorm:"type:varchar(120);not null"`
	Description string    `gorm:"type:text;not null"`
	Data        string    `gorm:"type:longtext"`
	PublishedAt time.Time `gorm:"not null;index"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (versionModel) TableName() string {
	return "settings_system_versions"
}

func versionModelFromDomain(version domain.SystemVersion, now time.Time) versionModel {
	return versionModel{
		ID:          version.ID,
		Version:     version.Version,
		Name:        version.Name,
		Description: version.Description,
		Data:        version.Data,
		PublishedAt: coalesceTime(version.PublishedAt, now),
		CreatedAt:   coalesceTime(version.CreatedAt, now),
		UpdatedAt:   coalesceTime(version.UpdatedAt, now),
	}
}

func (m versionModel) toDomain() (domain.SystemVersion, error) {
	return domain.RestoreSystemVersion(m.ID, m.Version, m.Name, m.Description, m.Data, m.PublishedAt, m.CreatedAt, m.UpdatedAt)
}

func coalesceTime(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}
