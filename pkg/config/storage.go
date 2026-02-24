/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"errors"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

const (
	// kvRedisID is the KV storage identifier for substitutions.
	kvRedisID = "kv-redis"
	// sqlPostgresID is the SQL storage identifier for substitutions.
	sqlPostgresID = "sql-postgres"
	// storageTypeSQLite is the default storage type.
	storageTypeSQLite = "sqlite"
)

// ExpandStorage merges user storage spec over the base config's storage sections.
// When storage is nil, the base config's storage is preserved unchanged.
func ExpandStorage(spec *v1alpha2.StateStorageSpec, base *BaseConfig) (*BaseConfig, error) {
	if spec == nil {
		return base, nil
	}
	if base == nil {
		return nil, errors.New("failed to expand storage: base config is nil")
	}

	clone := cloneBaseConfig(base)
	substitutions := make(map[string]string)
	if spec.KV != nil && spec.KV.Password != nil {
		substitutions[kvRedisID+":password"] = "${env.LLSD_KV_REDIS_PASSWORD}"
	}
	if spec.SQL != nil && spec.SQL.ConnectionString != nil {
		substitutions[sqlPostgresID+":connectionString"] = "${env.LLSD_SQL_POSTGRES_CONNECTIONSTRING}"
	}

	applyKVStorage(clone, spec.KV, substitutions)
	applySQLStorage(clone, spec.SQL, substitutions)

	return clone, nil
}

func applyKVStorage(clone *BaseConfig, kv *v1alpha2.KVStorageSpec, substitutions map[string]string) {
	if kv == nil {
		return
	}
	kvMap := ExpandKVStorage(kv, substitutions)
	clone.MetadataStore = mergeStore(clone.MetadataStore, kvMap)
}

func applySQLStorage(clone *BaseConfig, sql *v1alpha2.SQLStorageSpec, substitutions map[string]string) {
	if sql == nil {
		return
	}
	sqlMap := ExpandSQLStorage(sql, substitutions)
	sqlStores := []*map[string]interface{}{
		&clone.InferenceStore,
		&clone.SafetyStore,
		&clone.VectorIOStore,
		&clone.ToolRuntimeStore,
		&clone.TelemetryStore,
		&clone.PostTrainingStore,
		&clone.ScoringStore,
		&clone.EvalStore,
		&clone.DatasetIOStore,
	}
	for _, store := range sqlStores {
		if *store != nil {
			*store = mergeStore(*store, sqlMap)
		} else {
			*store = copyMap(sqlMap)
		}
	}
}

// cloneBaseConfig returns a deep copy of the base config.
func cloneBaseConfig(base *BaseConfig) *BaseConfig {
	return &BaseConfig{
		Version:           base.Version,
		APIs:              append([]string{}, base.APIs...),
		Providers:         copyMap(base.Providers),
		RegisteredModels:  copySliceOfMaps(base.RegisteredModels),
		Shields:           copySliceOfMaps(base.Shields),
		ToolGroups:        copySliceOfMaps(base.ToolGroups),
		MetadataStore:     copyMap(base.MetadataStore),
		InferenceStore:    copyMap(base.InferenceStore),
		SafetyStore:       copyMap(base.SafetyStore),
		VectorIOStore:     copyMap(base.VectorIOStore),
		ToolRuntimeStore:  copyMap(base.ToolRuntimeStore),
		TelemetryStore:    copyMap(base.TelemetryStore),
		PostTrainingStore: copyMap(base.PostTrainingStore),
		ScoringStore:      copyMap(base.ScoringStore),
		EvalStore:         copyMap(base.EvalStore),
		DatasetIOStore:    copyMap(base.DatasetIOStore),
		Server:            copyMap(base.Server),
		ExternalProviders: copyMap(base.ExternalProviders),
	}
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func copySliceOfMaps(s []map[string]interface{}) []map[string]interface{} {
	if s == nil {
		return nil
	}
	out := make([]map[string]interface{}, len(s))
	for i, m := range s {
		out[i] = copyMap(m)
	}
	return out
}

// mergeStore merges override into base, with override taking precedence.
func mergeStore(base, override map[string]interface{}) map[string]interface{} {
	out := copyMap(base)
	if out == nil {
		out = make(map[string]interface{})
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

// ExpandKVStorage converts KVStorageSpec to the config.yaml metadata_store format.
// type: sqlite -> {type: sqlite, db_path: ...}
// type: redis -> {type: redis, host: endpoint, password: ${env.VAR}}
func ExpandKVStorage(kv *v1alpha2.KVStorageSpec, substitutions map[string]string) map[string]interface{} {
	if kv == nil {
		return nil
	}
	typ := kv.Type
	if typ == "" {
		typ = storageTypeSQLite
	}

	out := make(map[string]interface{})
	out["type"] = typ

	switch typ {
	case storageTypeSQLite:
		out["db_path"] = "${env.SQLITE_STORE_DIR:=~/.llama}/kvstore.db"
	case "redis":
		if kv.Endpoint != "" {
			out["host"] = kv.Endpoint
		}
		if kv.Password != nil {
			ident := kvRedisID + ":password"
			if sub, ok := substitutions[ident]; ok {
				out["password"] = sub
			} else {
				out["password"] = "${env.LLSD_KV_REDIS_PASSWORD}"
			}
		}
	}
	return out
}

// ExpandSQLStorage converts SQLStorageSpec to the config.yaml inference_store/etc format.
// type: sqlite -> {type: sqlite, db_path: ...}
// type: postgres -> {type: postgres, connection_string: ${env.VAR}}
func ExpandSQLStorage(sql *v1alpha2.SQLStorageSpec, substitutions map[string]string) map[string]interface{} {
	if sql == nil {
		return nil
	}
	typ := sql.Type
	if typ == "" {
		typ = storageTypeSQLite
	}

	out := make(map[string]interface{})
	out["type"] = typ

	switch typ {
	case storageTypeSQLite:
		out["db_path"] = "${env.SQLITE_STORE_DIR:=~/.llama}/sqlstore.db"
	case "postgres":
		if sql.ConnectionString != nil {
			ident := sqlPostgresID + ":connectionString"
			if sub, ok := substitutions[ident]; ok {
				out["connection_string"] = sub
			} else {
				out["connection_string"] = "${env.LLSD_SQL_POSTGRES_CONNECTIONSTRING}"
			}
		}
	}
	return out
}
