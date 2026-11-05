/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package sqldb

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pole-io/specification/source/go/api/v1/ai"
)

func TestNewMCPID_MatchesSchemaLength(t *testing.T) {
	id := newMCPID()

	assert.Len(t, id, 32)
	assert.False(t, strings.Contains(id, "-"))
}

func TestNormalizeMCPServerBackend_Service(t *testing.T) {
	server := &ai.MCPServer{
		BackendServiceNamespace: "default",
		BackendServiceName:      "order-service",
		BackendAddress:          "http://127.0.0.1:8080/mcp",
	}

	normalizeMCPServerBackend(server)

	assert.Equal(t, "service", server.BackendType)
	assert.Equal(t, "default", server.Namespace)
	assert.Equal(t, "order-service", server.Name)
	assert.Equal(t, "order-service", server.Reference)
	assert.Empty(t, server.BackendAddress)
	assert.NoError(t, validateMCPServerBackend(server))
}

func TestNormalizeMCPServerBackend_Address(t *testing.T) {
	server := &ai.MCPServer{
		Name:                    "external-mcp",
		Namespace:               "default",
		BackendType:             "address",
		BackendServiceNamespace: "default",
		BackendServiceName:      "order-service",
		BackendAddress:          "http://127.0.0.1:8080/mcp",
		Reference:               "http://127.0.0.1:8080/mcp",
	}

	normalizeMCPServerBackend(server)

	assert.Equal(t, "address", server.BackendType)
	assert.Empty(t, server.BackendServiceNamespace)
	assert.Empty(t, server.BackendServiceName)
	assert.Empty(t, server.Reference)
	assert.Equal(t, "http://127.0.0.1:8080/mcp", server.BackendAddress)
	assert.NoError(t, validateMCPServerBackend(server))
}

func TestValidateMCPServerBackend_MissingRequiredFields(t *testing.T) {
	assert.Error(t, validateMCPServerBackend(&ai.MCPServer{BackendType: "service", BackendServiceNamespace: "default"}))
	assert.Error(t, validateMCPServerBackend(&ai.MCPServer{BackendType: "address"}))
	assert.NoError(t, validateMCPServerBackend(&ai.MCPServer{}))
}

func TestCreateMCPServerToolRevivesStableRegistryTool(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer rawDB.Close()
	store := &mcpServerStore{master: &BaseDB{DB: rawDB}}
	tool := &ai.MCPServerTool{
		Id: "tool-id", McpServerId: "server-id", Name: "list_namespaces",
		Description: "list", InputSchema: `{"type":"object"}`,
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO mcp_server_tools(id, mcp_server_id, name, description, input_schema, output_schema, annotations, flag, ctime, mtime)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, sysdate(), sysdate())
		ON DUPLICATE KEY UPDATE mcp_server_id = VALUES(mcp_server_id), name = VALUES(name),
		description = VALUES(description), input_schema = VALUES(input_schema),
		output_schema = VALUES(output_schema), annotations = VALUES(annotations),
		flag = VALUES(flag), mtime = sysdate()`)).
		WithArgs("tool-id", "server-id", "list_namespaces", "list", `{"type":"object"}`, "", "", uint32(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.CreateMCPServerTool(tool))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMCPServerReturnsNilWhenRegistryProjectionDoesNotExist(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer rawDB.Close()

	store := &mcpServerStore{master: &BaseDB{DB: rawDB}}
	columns := []string{
		"id", "name", "namespace", "ports", "business", "department", "description",
		"revision", "flag", "reference", "protocol", "ctime", "mtime", "export_to",
		"backend_type", "backend_service_namespace", "backend_service_name", "backend_address",
	}

	mock.ExpectQuery("FROM mcp_server WHERE id =").
		WithArgs("missing-id").
		WillReturnRows(sqlmock.NewRows(columns))
	server, err := store.GetMCPServer("missing-id")
	require.NoError(t, err)
	require.Nil(t, server)

	mock.ExpectQuery("FROM mcp_server WHERE name = .*namespace = .*flag != 1").
		WithArgs("pole-control-plane", "pole-system").
		WillReturnRows(sqlmock.NewRows(columns))
	server, err = store.GetMCPServerByName("pole-control-plane", "pole-system")
	require.NoError(t, err)
	require.Nil(t, server)

	require.NoError(t, mock.ExpectationsWereMet())
}
