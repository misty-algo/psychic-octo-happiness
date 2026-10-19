// Package mysql persists access roles, menus, permissions, and API metadata in MySQL.
package mysql

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/NSObjects/echo-admin/internal/modules/access/domain"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	inframysql "github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysql"
	"github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysqljson"
)

// Store persists access management data in MySQL.
type Store struct {
	db *gorm.DB
}

// NewStore migrates the MySQL access tables.
func NewStore(ctx context.Context, db *gorm.DB) (*Store, error) {
	if ctx == nil {
		return nil, errors.New("create access store: nil context")
	}
	if db == nil {
		return nil, errors.New("create access store: nil db")
	}
	store := &Store{db: db}
	if err := db.WithContext(ctx).AutoMigrate(&roleModel{}, &menuModel{}, &menuButtonModel{}, &permissionModel{}, &apiModel{}); err != nil {
		return nil, apperr.WrapDatabase(err, "migrate access tables")
	}
	return store, nil
}

// WithDB returns a store bound to db for transaction-scoped access operations.
func (s *Store) WithDB(db *gorm.DB) *Store {
	return &Store{db: db}
}

// FindRoleByID returns a role by id.
func (s *Store) FindRoleByID(ctx context.Context, id int64) (domain.Role, error) {
	if err := ctx.Err(); err != nil {
		return domain.Role{}, err
	}
	var model roleModel
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		return domain.Role{}, inframysql.MapReadError(err, "role", "find role")
	}
	return model.toDomain()
}

// FindRoleByCode returns a role by stable role code for bootstrapping.
func (s *Store) FindRoleByCode(ctx context.Context, code string) (domain.Role, error) {
	if err := ctx.Err(); err != nil {
		return domain.Role{}, err
	}
	var model roleModel
	err := s.db.WithContext(ctx).First(&model, "code = ?", code).Error
	if err != nil {
		return domain.Role{}, inframysql.MapReadError(err, "role", "find role by code")
	}
	return model.toDomain()
}

// ListAllRoles returns all roles ordered for scoped delegation checks.
func (s *Store) ListAllRoles(ctx context.Context) ([]domain.Role, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []roleModel
	err := s.db.WithContext(ctx).Order("id DESC").Find(&models).Error
	if err != nil {
		return nil, apperr.WrapDatabase(err, "list all roles")
	}
	roles := make([]domain.Role, 0, len(models))
	for _, model := range models {
		role, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// CreateRole inserts a role.
func (s *Store) CreateRole(ctx context.Context, role domain.Role) (domain.Role, error) {
	if err := ctx.Err(); err != nil {
		return domain.Role{}, err
	}
	model := roleModelFromDomain(role)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return domain.Role{}, inframysql.MapWriteError(err, "role code already exists", "create role")
	}
	return model.toDomain()
}

// UpdateRole replaces mutable role fields.
func (s *Store) UpdateRole(ctx context.Context, role domain.Role) (domain.Role, error) {
	if err := ctx.Err(); err != nil {
		return domain.Role{}, err
	}
	model := roleModelFromDomain(role)
	result := s.db.WithContext(ctx).Save(&model)
	if result.Error != nil {
		return domain.Role{}, inframysql.MapWriteError(result.Error, "role code already exists", "update role")
	}
	if result.RowsAffected == 0 {
		return domain.Role{}, apperr.NewNotFound("role")
	}
	return model.toDomain()
}

// DeleteRole removes a role row by id.
func (s *Store) DeleteRole(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Delete(&roleModel{}, "id = ?", id)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete role")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("role")
	}
	return nil
}

// ListAPIs returns API routes ordered by group and path.
func (s *Store) ListAPIs(ctx context.Context) ([]domain.API, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []apiModel
	err := s.db.WithContext(ctx).Order("api_group ASC, path ASC, method ASC").Find(&models).Error
	if err != nil {
		return nil, apperr.WrapDatabase(err, "list apis")
	}
	apis := make([]domain.API, 0, len(models))
	for _, model := range models {
		api, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		apis = append(apis, api)
	}
	return apis, nil
}

// FindAPIByID returns an API route by id.
func (s *Store) FindAPIByID(ctx context.Context, id int64) (domain.API, error) {
	if err := ctx.Err(); err != nil {
		return domain.API{}, err
	}
	var model apiModel
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		return domain.API{}, inframysql.MapReadError(err, "api", "find api")
	}
	return model.toDomain()
}

// FindAPIByRoute returns one API route by normalized HTTP method and Echo path.
func (s *Store) FindAPIByRoute(ctx context.Context, method, path string) (domain.API, error) {
	if err := ctx.Err(); err != nil {
		return domain.API{}, err
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	var model apiModel
	err := s.db.WithContext(ctx).First(&model, "method = ? AND path = ?", method, path).Error
	if err != nil {
		return domain.API{}, inframysql.MapReadError(err, "api", "find api by route")
	}
	return model.toDomain()
}

// ListMenus returns menus ordered for display.
func (s *Store) ListMenus(ctx context.Context) ([]domain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var models []menuModel
	err := s.db.WithContext(ctx).
		Preload("Buttons", func(tx *gorm.DB) *gorm.DB { return tx.Order("id ASC") }).
		Order("sort ASC, id ASC").
		Find(&models).Error
	if err != nil {
		return nil, apperr.WrapDatabase(err, "list menus")
	}
	menus := make([]domain.Menu, 0, len(models))
	for _, model := range models {
		menu, err := model.toDomain()
		if err != nil {
			return nil, err
		}
		menus = append(menus, menu)
	}
	return menus, nil
}

// FindMenuByID returns a menu by id.
func (s *Store) FindMenuByID(ctx context.Context, id int64) (domain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return domain.Menu{}, err
	}
	var model menuModel
	err := s.db.WithContext(ctx).
		Preload("Buttons", func(tx *gorm.DB) *gorm.DB { return tx.Order("id ASC") }).
		First(&model, "id = ?", id).Error
	if err != nil {
		return domain.Menu{}, inframysql.MapReadError(err, "menu", "find menu")
	}
	return model.toDomain()
}

// CreateMenu inserts a menu.
func (s *Store) CreateMenu(ctx context.Context, menu domain.Menu) (domain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return domain.Menu{}, err
	}
	model := menuModelFromDomain(menu)
	var created domain.Menu
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Create(&model).Error; err != nil {
			return inframysql.MapWriteError(err, "menu path already exists", "create menu")
		}
		if err := replaceMenuButtons(ctx, tx, model.ID, menu.Buttons); err != nil {
			return err
		}
		loaded, err := loadMenu(ctx, tx, model.ID)
		if err != nil {
			return err
		}
		created = loaded
		return nil
	})
	if err != nil {
		return domain.Menu{}, err
	}
	return created, nil
}

// UpdateMenu replaces mutable menu fields.
func (s *Store) UpdateMenu(ctx context.Context, menu domain.Menu) (domain.Menu, error) {
	if err := ctx.Err(); err != nil {
		return domain.Menu{}, err
	}
	model := menuModelFromDomain(menu)
	var updated domain.Menu
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.WithContext(ctx).Save(&model)
		if result.Error != nil {
			return inframysql.MapWriteError(result.Error, "menu path already exists", "update menu")
		}
		if result.RowsAffected == 0 {
			return apperr.NewNotFound("menu")
		}
		if err := replaceMenuButtons(ctx, tx, model.ID, menu.Buttons); err != nil {
			return err
		}
		loaded, err := loadMenu(ctx, tx, model.ID)
		if err != nil {
			return err
		}
		updated = loaded
		return nil
	})
	if err != nil {
		return domain.Menu{}, err
	}
	return updated, nil
}

// DeleteMenu removes a menu row by id.
func (s *Store) DeleteMenu(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Delete(&menuModel{}, "id = ?", id)
	if result.Error != nil {
		return apperr.WrapDatabase(result.Error, "delete menu")
	}
	if result.RowsAffected == 0 {
		return apperr.NewNotFound("menu")
	}
	return nil
}

// InstallRootAuthorization creates the initial authorization catalog and root role.
func (s *Store) InstallRootAuthorization(ctx context.Context) (domain.Role, error) {
	// Root authority is one authorization baseline: the role must be created
	// with the permission, API, menu, and button catalog it grants.
	var root domain.Role
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seedStore := &Store{db: tx}
		if err := seedStore.seedPermissions(ctx); err != nil {
			return err
		}
		apiIDs, err := seedStore.seedAPIs(ctx)
		if err != nil {
			return err
		}
		menuIDs, buttonIDs, err := seedStore.seedMenus(ctx)
		if err != nil {
			return err
		}
		if err := seedStore.seedSuperAdminRole(ctx, menuIDs, apiIDs, buttonIDs); err != nil {
			return err
		}
		role, err := seedStore.FindRoleByCode(ctx, domain.RoleCodeSuperAdmin)
		if err != nil {
			return err
		}
		root = role
		return nil
	})
	if err != nil {
		return domain.Role{}, err
	}
	return root, nil
}

func (s *Store) seedPermissions(ctx context.Context) error {
	now := time.Now().UTC()
	for _, permission := range domain.PermissionCatalog() {
		if err := s.ensurePermission(ctx, permission, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensurePermission(ctx context.Context, permission domain.PermissionDefinition, now time.Time) error {
	var existing permissionModel
	err := s.db.WithContext(ctx).Where("token = ?", permission.Token).First(&existing).Error
	if err == nil {
		existing.Resource = permission.Resource
		existing.Action = permission.Action
		existing.Name = permission.Name
		if saveErr := s.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return apperr.WrapDatabase(saveErr, "update seed permission")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.WrapDatabase(err, "find seed permission")
	}
	model := permissionModel{
		Token:     permission.Token,
		Resource:  permission.Resource,
		Action:    permission.Action,
		Name:      permission.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if createErr := s.db.WithContext(ctx).Create(&model).Error; createErr != nil {
		return inframysql.MapWriteError(createErr, "permission token already exists", "create seed permission")
	}
	return nil
}

func (s *Store) seedAPIs(ctx context.Context) ([]int64, error) {
	now := time.Now().UTC()
	definitions := domain.ManagedAPIRouteCatalog()
	apiIDs := make([]int64, 0, len(definitions))
	for _, api := range definitions {
		id, err := s.ensureAPI(ctx, api, now)
		if err != nil {
			return nil, err
		}
		apiIDs = append(apiIDs, id)
	}
	return apiIDs, nil
}

func (s *Store) ensureAPI(ctx context.Context, definition domain.ManagedAPIRouteDefinition, now time.Time) (int64, error) {
	var existing apiModel
	err := s.db.WithContext(ctx).Where("method = ? AND path = ?", definition.Method, definition.Pattern).First(&existing).Error
	if err == nil {
		existing.Description = definition.Description
		existing.Group = definition.Group
		existing.Permission = definition.Permission
		if saveErr := s.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return 0, apperr.WrapDatabase(saveErr, "update seed api")
		}
		return existing.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, apperr.WrapDatabase(err, "find seed api")
	}
	model := apiModel{
		Method:      definition.Method,
		Path:        definition.Pattern,
		Description: definition.Description,
		Group:       definition.Group,
		Permission:  definition.Permission,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if createErr := s.db.WithContext(ctx).Create(&model).Error; createErr != nil {
		return 0, inframysql.MapWriteError(createErr, "api route already exists", "create seed api")
	}
	return model.ID, nil
}

func (s *Store) seedMenus(ctx context.Context) ([]int64, []int64, error) {
	catalog := domain.MenuCatalog()
	menuIDs := make([]int64, 0, len(catalog))
	buttonIDs := make([]int64, 0, len(catalog)*3)
	for _, seed := range catalog {
		id, ids, err := s.ensureMenu(ctx, seed)
		if err != nil {
			return nil, nil, err
		}
		menuIDs = append(menuIDs, id)
		buttonIDs = append(buttonIDs, ids...)
	}
	return menuIDs, buttonIDs, nil
}

func (s *Store) ensureMenu(ctx context.Context, seed domain.MenuSeed) (int64, []int64, error) {
	parentID, err := s.seedMenuParentID(ctx, seed)
	if err != nil {
		return 0, nil, err
	}
	var existing menuModel
	err = s.db.WithContext(ctx).Where("path = ?", seed.Path).First(&existing).Error
	if err == nil {
		existing.ParentID = parentID
		existing.Name = seed.Name
		existing.Icon = seed.Icon
		existing.Hidden = seed.Hidden
		existing.Component = seed.Component
		existing.ActiveName = seed.Meta.ActiveName
		existing.KeepAlive = seed.Meta.KeepAlive
		existing.DefaultMenu = seed.Meta.DefaultMenu
		existing.CloseTab = seed.Meta.CloseTab
		existing.TransitionType = seed.Meta.TransitionType
		existing.Permission = seed.Permission
		existing.Sort = seed.Sort
		existing.Active = true
		if saveErr := s.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return 0, nil, apperr.WrapDatabase(saveErr, "update seed menu")
		}
		buttonIDs, buttonErr := s.ensureSeedButtons(ctx, existing.ID, seed.Buttons)
		return existing.ID, buttonIDs, buttonErr
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil, apperr.WrapDatabase(err, "find seed menu")
	}
	now := time.Now().UTC()
	menu, err := domain.RestoreMenu(0, parentID, seed.Name, seed.Path, seed.Icon, seed.Hidden, seed.Component, seed.Meta, seed.Permission, seed.Sort, true, nil, now, now)
	if err != nil {
		return 0, nil, err
	}
	model := menuModelFromDomain(menu)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return 0, nil, apperr.WrapDatabase(err, "create seed menu")
	}
	buttonIDs, buttonErr := s.ensureSeedButtons(ctx, model.ID, seed.Buttons)
	return model.ID, buttonIDs, buttonErr
}

func (s *Store) seedMenuParentID(ctx context.Context, seed domain.MenuSeed) (int64, error) {
	if seed.ParentPath == "" {
		return 0, nil
	}
	var parent menuModel
	err := s.db.WithContext(ctx).Where("path = ?", seed.ParentPath).First(&parent).Error
	if err == nil {
		return parent.ID, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, apperr.Newf(apperr.ErrInternalServer, "seed menu parent %s is missing", seed.ParentPath)
	}
	return 0, apperr.WrapDatabase(err, "find seed menu parent")
}

func (s *Store) ensureSeedButtons(ctx context.Context, menuID int64, seeds []domain.MenuButtonSeed) ([]int64, error) {
	buttonIDs := make([]int64, 0, len(seeds))
	for _, seed := range seeds {
		id, err := s.ensureSeedButton(ctx, menuID, seed)
		if err != nil {
			return nil, err
		}
		buttonIDs = append(buttonIDs, id)
	}
	return buttonIDs, nil
}

func (s *Store) ensureSeedButton(ctx context.Context, menuID int64, seed domain.MenuButtonSeed) (int64, error) {
	var existing menuButtonModel
	err := s.db.WithContext(ctx).Where("menu_id = ? AND name = ?", menuID, seed.Name).First(&existing).Error
	if err == nil {
		existing.Description = seed.Description
		if saveErr := s.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return 0, apperr.WrapDatabase(saveErr, "update seed menu button")
		}
		return existing.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, apperr.WrapDatabase(err, "find seed menu button")
	}
	now := time.Now().UTC()
	button, err := domain.RestoreMenuButton(0, menuID, seed.Name, seed.Description, now, now)
	if err != nil {
		return 0, err
	}
	model := menuButtonModelFromDomain(button)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return 0, inframysql.MapWriteError(err, "menu button already exists", "create seed menu button")
	}
	return model.ID, nil
}

func (s *Store) seedSuperAdminRole(ctx context.Context, menuIDs, apiIDs, buttonIDs []int64) error {
	roleIDs, err := s.allRoleIDs(ctx)
	if err != nil {
		return err
	}
	// The Root Role shape is deployment-owned baseline data; this adapter only
	// resolves the persisted ID sets and applies the domain baseline.
	baseline, err := domain.NewRootRole(menuIDs, apiIDs, buttonIDs, roleIDs, time.Now().UTC())
	if err != nil {
		return err
	}
	var existing roleModel
	err = s.db.WithContext(ctx).Where("code = ?", domain.RoleCodeSuperAdmin).First(&existing).Error
	if err == nil {
		existing.ParentID = baseline.ParentID
		existing.Name = baseline.Name
		existing.Permissions = mysqljson.Strings(baseline.Permissions)
		existing.MenuIDs = mysqljson.Int64s(baseline.MenuIDs)
		existing.APIIDs = mysqljson.Int64s(baseline.APIIDs)
		existing.ButtonIDs = mysqljson.Int64s(baseline.ButtonIDs)
		existing.DataRoleIDs = mysqljson.Int64s(baseline.DataRoleIDs)
		existing.DefaultPath = baseline.DefaultPath
		existing.Active = baseline.Active
		if saveErr := s.db.WithContext(ctx).Save(&existing).Error; saveErr != nil {
			return apperr.WrapDatabase(saveErr, "update seed role")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.WrapDatabase(err, "find seed role")
	}
	model := roleModelFromDomain(baseline)
	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		return apperr.WrapDatabase(err, "create seed role")
	}
	// The row's own ID joins its data-role grants only after persistence
	// because the database assigns it.
	model.DataRoleIDs = mysqljson.Int64s(append(roleIDs, model.ID))
	if err := s.db.WithContext(ctx).Save(&model).Error; err != nil {
		return apperr.WrapDatabase(err, "update seed role data authority")
	}
	return nil
}

func (s *Store) allRoleIDs(ctx context.Context) ([]int64, error) {
	var roleIDs []int64
	if err := s.db.WithContext(ctx).Model(&roleModel{}).Pluck("id", &roleIDs).Error; err != nil {
		return nil, apperr.WrapDatabase(err, "list role ids")
	}
	return roleIDs, nil
}

type permissionModel struct {
	ID        int64     `gorm:"primaryKey"`
	Token     string    `gorm:"type:varchar(80);not null;uniqueIndex"`
	Resource  string    `gorm:"type:varchar(80);not null"`
	Action    string    `gorm:"type:varchar(40);not null"`
	Name      string    `gorm:"type:varchar(120);not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (permissionModel) TableName() string {
	return "access_permissions"
}

type apiModel struct {
	ID          int64     `gorm:"primaryKey"`
	Method      string    `gorm:"type:varchar(12);not null;uniqueIndex:idx_access_api_method_path"`
	Path        string    `gorm:"type:varchar(180);not null;uniqueIndex:idx_access_api_method_path"`
	Description string    `gorm:"type:varchar(120);not null"`
	Group       string    `gorm:"column:api_group;type:varchar(80);not null;index"`
	Permission  string    `gorm:"type:varchar(80);not null;index"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (apiModel) TableName() string {
	return "access_apis"
}

func (m apiModel) toDomain() (domain.API, error) {
	return domain.RestoreAPI(m.ID, m.Method, m.Path, m.Description, m.Group, m.Permission, m.CreatedAt, m.UpdatedAt)
}

type roleModel struct {
	ID          int64             `gorm:"primaryKey"`
	ParentID    int64             `gorm:"not null;index"`
	Code        string            `gorm:"type:varchar(64);not null;uniqueIndex"`
	Name        string            `gorm:"type:varchar(80);not null"`
	Permissions mysqljson.Strings `gorm:"type:json;not null"`
	MenuIDs     mysqljson.Int64s  `gorm:"type:json;not null"`
	APIIDs      mysqljson.Int64s  `gorm:"type:json;not null"`
	ButtonIDs   mysqljson.Int64s  `gorm:"type:json;not null"`
	DataRoleIDs mysqljson.Int64s  `gorm:"type:json"`
	DefaultPath string            `gorm:"type:varchar(160);not null"`
	Active      bool              `gorm:"not null"`
	CreatedAt   time.Time         `gorm:"not null"`
	UpdatedAt   time.Time         `gorm:"not null"`
}

func (roleModel) TableName() string {
	return "access_roles"
}

func roleModelFromDomain(role domain.Role) roleModel {
	return roleModel{
		ID:          role.ID,
		ParentID:    role.ParentID,
		Code:        role.Code,
		Name:        role.Name,
		Permissions: mysqljson.Strings(role.Permissions),
		MenuIDs:     mysqljson.Int64s(role.MenuIDs),
		APIIDs:      mysqljson.Int64s(role.APIIDs),
		ButtonIDs:   mysqljson.Int64s(role.ButtonIDs),
		DataRoleIDs: mysqljson.Int64s(role.DataRoleIDs),
		DefaultPath: role.DefaultPath,
		Active:      role.Active,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}

func (m roleModel) toDomain() (domain.Role, error) {
	return domain.RestoreRole(m.ID, m.ParentID, m.Code, m.Name, []string(m.Permissions), []int64(m.MenuIDs), []int64(m.APIIDs), []int64(m.ButtonIDs), []int64(m.DataRoleIDs), m.DefaultPath, m.Active, m.CreatedAt, m.UpdatedAt)
}

type menuModel struct {
	ID             int64             `gorm:"primaryKey"`
	ParentID       int64             `gorm:"not null"`
	Name           string            `gorm:"type:varchar(80);not null"`
	Path           string            `gorm:"type:varchar(160);not null;uniqueIndex"`
	Icon           string            `gorm:"type:varchar(80);not null"`
	Hidden         bool              `gorm:"not null"`
	Component      string            `gorm:"type:varchar(160);not null"`
	ActiveName     string            `gorm:"type:varchar(160);not null"`
	KeepAlive      bool              `gorm:"not null"`
	DefaultMenu    bool              `gorm:"not null"`
	CloseTab       bool              `gorm:"not null"`
	TransitionType string            `gorm:"type:varchar(80);not null"`
	Permission     string            `gorm:"type:varchar(80);not null"`
	Sort           int               `gorm:"not null"`
	Active         bool              `gorm:"not null"`
	Buttons        []menuButtonModel `gorm:"foreignKey:MenuID;constraint:OnDelete:CASCADE"`
	CreatedAt      time.Time         `gorm:"not null"`
	UpdatedAt      time.Time         `gorm:"not null"`
}

func (menuModel) TableName() string {
	return "access_menus"
}

func menuModelFromDomain(menu domain.Menu) menuModel {
	return menuModel{
		ID:             menu.ID,
		ParentID:       menu.ParentID,
		Name:           menu.Name,
		Path:           menu.Path,
		Icon:           menu.Icon,
		Hidden:         menu.Hidden,
		Component:      menu.Component,
		ActiveName:     menu.Meta.ActiveName,
		KeepAlive:      menu.Meta.KeepAlive,
		DefaultMenu:    menu.Meta.DefaultMenu,
		CloseTab:       menu.Meta.CloseTab,
		TransitionType: menu.Meta.TransitionType,
		Permission:     menu.Permission,
		Sort:           menu.Sort,
		Active:         menu.Active,
		CreatedAt:      menu.CreatedAt,
		UpdatedAt:      menu.UpdatedAt,
	}
}

func (m menuModel) toDomain() (domain.Menu, error) {
	buttons := make([]domain.MenuButton, 0, len(m.Buttons))
	for _, buttonModel := range m.Buttons {
		button, err := buttonModel.toDomain()
		if err != nil {
			return domain.Menu{}, err
		}
		buttons = append(buttons, button)
	}
	meta := domain.MenuMeta{
		ActiveName:     m.ActiveName,
		KeepAlive:      m.KeepAlive,
		DefaultMenu:    m.DefaultMenu,
		CloseTab:       m.CloseTab,
		TransitionType: m.TransitionType,
	}
	return domain.RestoreMenu(m.ID, m.ParentID, m.Name, m.Path, m.Icon, m.Hidden, m.Component, meta, m.Permission, m.Sort, m.Active, buttons, m.CreatedAt, m.UpdatedAt)
}

type menuButtonModel struct {
	ID          int64     `gorm:"primaryKey"`
	MenuID      int64     `gorm:"not null;uniqueIndex:idx_access_menu_button_name"`
	Name        string    `gorm:"type:varchar(80);not null;uniqueIndex:idx_access_menu_button_name"`
	Description string    `gorm:"type:varchar(120);not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (menuButtonModel) TableName() string {
	return "access_menu_buttons"
}

func menuButtonModelFromDomain(button domain.MenuButton) menuButtonModel {
	return menuButtonModel{
		ID:          button.ID,
		MenuID:      button.MenuID,
		Name:        button.Name,
		Description: button.Description,
		CreatedAt:   button.CreatedAt,
		UpdatedAt:   button.UpdatedAt,
	}
}

func (m menuButtonModel) toDomain() (domain.MenuButton, error) {
	return domain.RestoreMenuButton(m.ID, m.MenuID, m.Name, m.Description, m.CreatedAt, m.UpdatedAt)
}

func loadMenu(ctx context.Context, tx *gorm.DB, id int64) (domain.Menu, error) {
	var model menuModel
	err := tx.WithContext(ctx).
		Preload("Buttons", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		First(&model, "id = ?", id).Error
	if err != nil {
		return domain.Menu{}, inframysql.MapReadError(err, "menu", "load menu")
	}
	return model.toDomain()
}

func replaceMenuButtons(ctx context.Context, tx *gorm.DB, menuID int64, buttons []domain.MenuButton) error {
	var existing []menuButtonModel
	if err := tx.WithContext(ctx).Where("menu_id = ?", menuID).Find(&existing).Error; err != nil {
		return apperr.WrapDatabase(err, "list menu buttons")
	}
	byID := make(map[int64]menuButtonModel, len(existing))
	byName := make(map[string]menuButtonModel, len(existing))
	for _, button := range existing {
		byID[button.ID] = button
		byName[button.Name] = button
	}
	keptIDs := make([]int64, 0, len(buttons))
	for _, button := range buttons {
		model, ok := byID[button.ID]
		if !ok {
			model, ok = byName[button.Name]
		}
		if ok {
			model.Name = button.Name
			model.Description = button.Description
			if err := tx.WithContext(ctx).Save(&model).Error; err != nil {
				return inframysql.MapWriteError(err, "menu button already exists", "update menu button")
			}
			keptIDs = append(keptIDs, model.ID)
			continue
		}
		now := time.Now().UTC()
		created, err := domain.RestoreMenuButton(0, menuID, button.Name, button.Description, now, now)
		if err != nil {
			return err
		}
		model = menuButtonModelFromDomain(created)
		if err := tx.WithContext(ctx).Create(&model).Error; err != nil {
			return inframysql.MapWriteError(err, "menu button already exists", "create menu button")
		}
		keptIDs = append(keptIDs, model.ID)
	}
	deleteQuery := tx.WithContext(ctx).Where("menu_id = ?", menuID)
	if len(keptIDs) > 0 {
		deleteQuery = deleteQuery.Where("id NOT IN ?", keptIDs)
	}
	if err := deleteQuery.Delete(&menuButtonModel{}).Error; err != nil {
		return apperr.WrapDatabase(err, "delete removed menu buttons")
	}
	return nil
}
