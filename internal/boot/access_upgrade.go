package boot

import (
	"context"
	"errors"
	"fmt"

	accessmysql "github.com/NSObjects/echo-admin/internal/modules/access/adapters/mysql"
	"github.com/NSObjects/echo-admin/internal/platform/configs"
	inframysql "github.com/NSObjects/echo-admin/internal/platform/infrastructure/mysql"
)

// UpgradeAccessAuthorization runs the operator-controlled access catalog
// upgrade without starting the HTTP application.
func UpgradeAccessAuthorization(ctx context.Context, configPath string) (err error) {
	if ctx == nil {
		return errors.New("upgrade access authorization: nil context")
	}
	cfg, err := configs.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	resource, err := inframysql.Open(ctx, cfg.MySQL)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer func() {
		err = errors.Join(err, resource.Close())
	}()
	db := resource.DB()
	if db == nil {
		return errors.New("upgrade access authorization: mysql is disabled")
	}
	store, err := accessmysql.NewStore(ctx, db)
	if err != nil {
		return err
	}
	return store.UpgradeManagedAPIRouteCatalog(ctx)
}
