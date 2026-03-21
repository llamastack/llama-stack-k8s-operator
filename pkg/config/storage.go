package config

import (
	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

const storageTypeSQLite = "sqlite"

// ApplyStorage applies storage configuration from the CRD to the base config.
// Uses merge_by_subsection strategy: kv and sql are independently replaced if specified.
func ApplyStorage(storage *v1alpha2.StorageSpec, config map[string]interface{}) {
	if storage == nil {
		return
	}

	if storage.KV != nil {
		applyKVStorage(storage.KV, config)
	}

	if storage.SQL != nil {
		applySQLStorage(storage.SQL, config)
	}
}

func applyKVStorage(kv *v1alpha2.KVStorageSpec, config map[string]interface{}) {
	kvType := kv.Type
	if kvType == "" {
		kvType = storageTypeSQLite
	}

	kvConfig := map[string]interface{}{
		"type": kvType,
	}

	switch kvType {
	case "redis":
		if kv.Endpoint != "" {
			kvConfig["endpoint"] = kv.Endpoint
		}
		if kv.Password != nil {
			kvConfig["password"] = "${env.LLSD_STORAGE_KV_PASSWORD}"
		}
	case storageTypeSQLite:
		kvConfig["db_path"] = "/data/kv_store.db"
	}

	config["metadata_store"] = kvConfig
}

func applySQLStorage(sql *v1alpha2.SQLStorageSpec, config map[string]interface{}) {
	sqlType := sql.Type
	if sqlType == "" {
		sqlType = storageTypeSQLite
	}

	sqlConfig := map[string]interface{}{
		"type": sqlType,
	}

	switch sqlType {
	case "postgres":
		if sql.ConnectionString != nil {
			sqlConfig["connection_string"] = "${env.LLSD_STORAGE_SQL_CONNECTION_STRING}"
		}
	case storageTypeSQLite:
		sqlConfig["db_path"] = "/data/inference_store.db"
	}

	config["inference_store"] = sqlConfig
}
