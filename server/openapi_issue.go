package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/bytebase/bytebase/api"
	openAPIV1 "github.com/bytebase/bytebase/api/v1"
	"github.com/bytebase/bytebase/plugin/db"
)

func (s *Server) createIssueByOpenAPI(c echo.Context) error {
	ctx := c.Request().Context()

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to read request body").SetInternal(err)
	}

	create := &openAPIV1.IssueCreate{}
	if err := json.Unmarshal(body, create); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Malformed create instance request").SetInternal(err)
	}

	project, err := s.store.GetProjectByKey(ctx, create.ProjectKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to find project with key %s", create.ProjectKey)).SetInternal(err)
	}
	if project == nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Cannot found project with key %s", create.ProjectKey))
	}

	issueType := api.IssueDatabaseDataUpdate
	migrationList := []*api.MigrationDetail{}
	dbList, err := s.findProjectDatabases(ctx, project.ID, create.Database, create.Environment)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list database").SetInternal(err)
	}

	for _, database := range dbList {
		migrationList = append(migrationList, &api.MigrationDetail{
			DatabaseID:    database.ID,
			MigrationType: create.MigrationType,
			Statement:     create.Statement,
			SchemaVersion: create.SchemaVersion,
		})
	}

	if create.MigrationType == db.Migrate || create.MigrationType == db.Baseline {
		issueType = api.IssueDatabaseSchemaUpdate
	}

	createContext, err := json.Marshal(
		&api.MigrationContext{
			DetailList: migrationList,
		},
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to marshal update schema context").SetInternal(err)
	}

	issueCreate := &api.IssueCreate{
		CreatorID:             c.Get(getPrincipalIDContextKey()).(int),
		ProjectID:             project.ID,
		Name:                  create.Name,
		Type:                  issueType,
		Description:           create.Description,
		AssigneeID:            api.SystemBotID,
		AssigneeNeedAttention: true,
		CreateContext:         string(createContext),
	}

	if _, err := s.createIssue(ctx, issueCreate); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create the issue").SetInternal(err)
	}

	return c.String(http.StatusOK, "OK")
}
