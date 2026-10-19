// Package settingshttp adapts setting and dictionary HTTP requests to the settings usecase.
package settingshttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/NSObjects/echo-admin/internal/modules/audit/oprec"
	"github.com/NSObjects/echo-admin/internal/modules/settings/usecase"
	"github.com/NSObjects/echo-admin/internal/platform/apperr"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpreq"
	"github.com/NSObjects/echo-admin/internal/platform/server/httpresp"
)

// Handler adapts setting and dictionary HTTP requests to the settings usecase.
type Handler struct {
	usecase *usecase.Usecase
	audit   *oprec.Recorder
}

// New creates a settings HTTP handler.
func New(uc *usecase.Usecase, audit *oprec.Recorder) *Handler {
	return &Handler{usecase: uc, audit: audit}
}

// Register mounts setting and dictionary routes on group.
func Register(group *echo.Group, handler *Handler) {
	group.GET("/system/configs", handler.ListConfigs)
	group.PUT("/system/configs/:key", handler.UpsertConfig)
	group.DELETE("/system/configs/:key", handler.DeleteConfig)
	group.GET("/system/params", handler.ListParams)
	group.POST("/system/params", handler.CreateParam)
	group.POST("/system/params/batch-delete", handler.BatchDeleteParams)
	group.GET("/system/params/key/:key", handler.FindParamByKey)
	group.GET("/system/params/:id", handler.FindParam)
	group.PATCH("/system/params/:id", handler.UpdateParam)
	group.DELETE("/system/params/:id", handler.DeleteParam)
	group.GET("/dictionaries", handler.ListDictionaries)
	group.POST("/dictionaries", handler.CreateDictionary)
	group.GET("/dictionaries/export", handler.ExportDictionaries)
	group.POST("/dictionaries/import", handler.ImportDictionaries)
	group.PATCH("/dictionaries/:code", handler.UpdateDictionary)
	group.DELETE("/dictionaries/:code", handler.DeleteDictionary)
	group.POST("/dictionaries/:code/items", handler.AddDictionaryItem)
	group.PATCH("/dictionaries/:code/items/:item_id", handler.UpdateDictionaryItem)
	group.DELETE("/dictionaries/:code/items/:item_id", handler.DeleteDictionaryItem)
	group.GET("/system/versions", handler.ListVersions)
	group.POST("/system/versions", handler.CreateVersion)
	group.POST("/system/versions/export", handler.ExportVersion)
	group.POST("/system/versions/import", handler.ImportVersion)
	group.POST("/system/versions/batch-delete", handler.BatchDeleteVersions)
	group.GET("/system/versions/:id", handler.FindVersion)
	group.GET("/system/versions/:id/download", handler.DownloadVersion)
	group.PATCH("/system/versions/:id", handler.UpdateVersion)
	group.DELETE("/system/versions/:id", handler.DeleteVersion)
}

// ListConfigs returns system configs.
func (h *Handler) ListConfigs(c *echo.Context) error {
	configs, err := h.usecase.ListConfigs(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, configs)
}

// UpsertConfig creates or updates a system config.
func (h *Handler) UpsertConfig(c *echo.Context) error {
	var req configRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	key := c.Param("key")
	config, err := h.usecase.UpsertConfig(c.Request().Context(), usecase.ConfigInput{
		Key:    key,
		Name:   req.Name,
		Value:  req.Value,
		Public: req.Public,
	})
	err = h.audit.Record(c, "upsert", "config", config.Key, "updated config", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, config)
}

// DeleteConfig deletes a system config.
func (h *Handler) DeleteConfig(c *echo.Context) error {
	key := c.Param("key")
	err := h.usecase.DeleteConfig(c.Request().Context(), key)
	err = h.audit.Record(c, "delete", "config", key, "deleted config", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, deletedResponse{Code: key})
}

// ListParams returns system parameters.
func (h *Handler) ListParams(c *echo.Context) error {
	input, err := paramListInput(c)
	if err != nil {
		return err
	}
	output, err := h.usecase.ListParams(c.Request().Context(), input)
	if err != nil {
		return err
	}
	return httpresp.Paginated(c, output.Items, output.Page, output.PageSize, output.Total)
}

// FindParam returns one system parameter by id.
func (h *Handler) FindParam(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system param")
	if err != nil {
		return err
	}
	param, err := h.usecase.FindParam(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return httpresp.OK(c, param)
}

// FindParamByKey returns one system parameter by key.
func (h *Handler) FindParamByKey(c *echo.Context) error {
	param, err := h.usecase.FindParamByKey(c.Request().Context(), c.Param("key"))
	if err != nil {
		return err
	}
	return httpresp.OK(c, param)
}

// CreateParam creates one system parameter.
func (h *Handler) CreateParam(c *echo.Context) error {
	var req paramRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	param, err := h.usecase.CreateParam(c.Request().Context(), paramInput(req))
	err = h.audit.Record(c, "create", "system_param", strconv.FormatInt(param.ID, 10), "created system param", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, param)
}

// UpdateParam updates one system parameter.
func (h *Handler) UpdateParam(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system param")
	if err != nil {
		return err
	}
	var req paramRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return bindErr
	}
	param, err := h.usecase.UpdateParam(c.Request().Context(), updateParamInput(id, req))
	err = h.audit.Record(c, "update", "system_param", strconv.FormatInt(param.ID, 10), "updated system param", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, param)
}

// DeleteParam deletes one system parameter.
func (h *Handler) DeleteParam(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system param")
	if err != nil {
		return err
	}
	err = h.usecase.DeleteParam(c.Request().Context(), id)
	err = h.audit.Record(c, "delete", "system_param", strconv.FormatInt(id, 10), "deleted system param", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, deletedIDResponse{ID: id})
}

// BatchDeleteParams deletes system parameters by id.
func (h *Handler) BatchDeleteParams(c *echo.Context) error {
	ids, err := httpreq.BindIDs(c)
	if err != nil {
		return err
	}
	err = h.usecase.DeleteParams(c.Request().Context(), ids)
	err = h.audit.Record(c, "delete", "system_param", "batch", "deleted system params", err)
	if err != nil {
		return err
	}
	return httpresp.DeletedIDs(c, ids)
}

// ListDictionaries returns dictionaries.
func (h *Handler) ListDictionaries(c *echo.Context) error {
	dictionaries, err := h.usecase.ListDictionaries(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, dictionaries)
}

// ExportDictionaries downloads all dictionaries as a JSON bundle.
func (h *Handler) ExportDictionaries(c *echo.Context) error {
	data, err := h.usecase.DictionaryBundleJSON(c.Request().Context())
	if err != nil {
		return err
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="dictionaries.json"`)
	return c.Blob(http.StatusOK, "application/json; charset=utf-8", data)
}

// ImportDictionaries imports dictionaries from a JSON bundle.
func (h *Handler) ImportDictionaries(c *echo.Context) error {
	var bundle usecase.DictionaryBundle
	if err := httpreq.BindAndValidate(c, &bundle); err != nil {
		return err
	}
	dictionaries, err := h.usecase.ImportDictionaries(c.Request().Context(), bundle)
	err = h.audit.Record(c, "import", "dictionary", "bundle", "imported dictionaries", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, dictionaries)
}

// CreateDictionary creates a dictionary.
func (h *Handler) CreateDictionary(c *echo.Context) error {
	var req dictionaryRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	dictionary, err := h.usecase.CreateDictionary(c.Request().Context(), usecase.DictionaryInput{
		Code: req.Code,
		Name: req.Name,
	})
	err = h.audit.Record(c, "create", "dictionary", dictionary.Code, "created dictionary", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, dictionary)
}

// UpdateDictionary updates a dictionary.
func (h *Handler) UpdateDictionary(c *echo.Context) error {
	var req updateDictionaryRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	dictionary, err := h.usecase.UpdateDictionary(c.Request().Context(), usecase.UpdateDictionaryInput{
		Code: c.Param("code"),
		Name: req.Name,
	})
	err = h.audit.Record(c, "update", "dictionary", dictionary.Code, "updated dictionary", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, dictionary)
}

// DeleteDictionary deletes a dictionary.
func (h *Handler) DeleteDictionary(c *echo.Context) error {
	code := c.Param("code")
	err := h.usecase.DeleteDictionary(c.Request().Context(), code)
	err = h.audit.Record(c, "delete", "dictionary", code, "deleted dictionary", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, deletedResponse{Code: code})
}

// AddDictionaryItem appends one dictionary item.
func (h *Handler) AddDictionaryItem(c *echo.Context) error {
	var req dictionaryItemRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	dictionary, err := h.usecase.AddDictionaryItem(c.Request().Context(), c.Param("code"), usecase.DictionaryItemInput{
		ParentID: req.ParentID,
		Label:    req.Label,
		Value:    req.Value,
		Extend:   req.Extend,
		Sort:     req.Sort,
		Active:   req.Active,
	})
	err = h.audit.Record(c, "create", "dictionary_item", c.Param("code"), "created dictionary item", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, dictionary)
}

// UpdateDictionaryItem updates one dictionary item.
func (h *Handler) UpdateDictionaryItem(c *echo.Context) error {
	itemID, err := httpreq.PathID(c, "item_id", "dictionary item")
	if err != nil {
		return err
	}
	var req dictionaryItemRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return bindErr
	}
	dictionary, err := h.usecase.UpdateDictionaryItem(c.Request().Context(), c.Param("code"), usecase.DictionaryItemInput{
		ID:       itemID,
		ParentID: req.ParentID,
		Label:    req.Label,
		Value:    req.Value,
		Extend:   req.Extend,
		Sort:     req.Sort,
		Active:   req.Active,
	})
	err = h.audit.Record(c, "update", "dictionary_item", strconv.FormatInt(itemID, 10), "updated dictionary item", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, dictionary)
}

// DeleteDictionaryItem deletes one dictionary item.
func (h *Handler) DeleteDictionaryItem(c *echo.Context) error {
	itemID, err := httpreq.PathID(c, "item_id", "dictionary item")
	if err != nil {
		return err
	}
	dictionary, err := h.usecase.DeleteDictionaryItem(c.Request().Context(), c.Param("code"), itemID)
	err = h.audit.Record(c, "delete", "dictionary_item", strconv.FormatInt(itemID, 10), "deleted dictionary item", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, dictionary)
}

// ListVersions returns release records.
func (h *Handler) ListVersions(c *echo.Context) error {
	versions, err := h.usecase.ListVersions(c.Request().Context())
	if err != nil {
		return err
	}
	return httpresp.OK(c, versions)
}

// FindVersion returns one release record.
func (h *Handler) FindVersion(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system version")
	if err != nil {
		return err
	}
	version, err := h.usecase.FindVersion(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return httpresp.OK(c, version)
}

// DownloadVersion returns one release record as a JSON attachment.
func (h *Handler) DownloadVersion(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system version")
	if err != nil {
		return err
	}
	data, version, err := h.usecase.VersionJSON(c.Request().Context(), id)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("version_%s.json", version.Version)
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK, "application/json; charset=utf-8", data)
}

// CreateVersion creates a release record.
func (h *Handler) CreateVersion(c *echo.Context) error {
	var req versionRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	version, err := h.usecase.CreateVersion(c.Request().Context(), usecase.VersionInput{
		Version:     req.Version,
		Name:        req.Name,
		Description: req.Description,
		PublishedAt: req.PublishedAt,
	})
	err = h.audit.Record(c, "create", "system_version", version.Version, "created system version", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, version)
}

// ExportVersion creates a portable version bundle from selected resources.
func (h *Handler) ExportVersion(c *echo.Context) error {
	var req exportVersionRequest
	if err := httpreq.BindAndValidate(c, &req); err != nil {
		return err
	}
	version, err := h.usecase.ExportVersion(c.Request().Context(), usecase.ExportVersionInput{
		Version:       req.Version,
		Name:          req.Name,
		Description:   req.Description,
		MenuIDs:       req.MenuIDs,
		DictionaryIDs: req.DictionaryIDs,
	})
	err = h.audit.Record(c, "export", "system_version", version.Version, "exported system version", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, version)
}

// ImportVersion imports a portable version bundle.
func (h *Handler) ImportVersion(c *echo.Context) error {
	bundle, err := decodeVersionBundle(c)
	if err != nil {
		return err
	}
	version, err := h.usecase.ImportVersion(c.Request().Context(), bundle)
	err = h.audit.Record(c, "import", "system_version", version.Version, "imported system version", err)
	if err != nil {
		return err
	}
	return httpresp.Created(c, version)
}

func decodeVersionBundle(c *echo.Context) (usecase.VersionBundle, error) {
	var bundle usecase.VersionBundle
	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return usecase.VersionBundle{}, apperr.WrapBadRequest(err, "invalid version bundle")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return usecase.VersionBundle{}, apperr.WrapBadRequest(err, "invalid version bundle")
	}
	if err := c.Validate(&bundle); err != nil {
		return usecase.VersionBundle{}, err
	}
	return bundle, nil
}

// UpdateVersion updates a release record.
func (h *Handler) UpdateVersion(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system version")
	if err != nil {
		return err
	}
	var req versionRequest
	if bindErr := httpreq.BindAndValidate(c, &req); bindErr != nil {
		return bindErr
	}
	version, err := h.usecase.UpdateVersion(c.Request().Context(), usecase.UpdateVersionInput{
		ID:          id,
		Version:     req.Version,
		Name:        req.Name,
		Description: req.Description,
		PublishedAt: req.PublishedAt,
	})
	err = h.audit.Record(c, "update", "system_version", strconv.FormatInt(version.ID, 10), "updated system version", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, version)
}

// DeleteVersion deletes a release record.
func (h *Handler) DeleteVersion(c *echo.Context) error {
	id, err := httpreq.PathID(c, "id", "system version")
	if err != nil {
		return err
	}
	err = h.usecase.DeleteVersion(c.Request().Context(), id)
	err = h.audit.Record(c, "delete", "system_version", strconv.FormatInt(id, 10), "deleted system version", err)
	if err != nil {
		return err
	}
	return httpresp.OK(c, deletedIDResponse{ID: id})
}

// BatchDeleteVersions deletes release records by id.
func (h *Handler) BatchDeleteVersions(c *echo.Context) error {
	ids, err := httpreq.BindIDs(c)
	if err != nil {
		return err
	}
	err = h.usecase.DeleteVersions(c.Request().Context(), ids)
	err = h.audit.Record(c, "delete", "system_version", "batch", "deleted system versions", err)
	if err != nil {
		return err
	}
	return httpresp.DeletedIDs(c, ids)
}

func paramListInput(c *echo.Context) (usecase.ParamListInput, error) {
	page, pageSize, err := httpreq.Pagination(c)
	if err != nil {
		return usecase.ParamListInput{}, err
	}
	return usecase.ParamListInput{
		Page:     page,
		PageSize: pageSize,
		Name:     c.QueryParam("name"),
		Key:      c.QueryParam("key"),
	}, nil
}

func paramInput(req paramRequest) usecase.ParamInput {
	return usecase.ParamInput{
		Name:  req.Name,
		Key:   req.Key,
		Value: req.Value,
		Desc:  req.Desc,
	}
}

func updateParamInput(id int64, req paramRequest) usecase.UpdateParamInput {
	return usecase.UpdateParamInput{
		ID:    id,
		Name:  req.Name,
		Key:   req.Key,
		Value: req.Value,
		Desc:  req.Desc,
	}
}

type deletedResponse struct {
	Code string `json:"code"`
}

type deletedIDResponse struct {
	ID int64 `json:"id"`
}
