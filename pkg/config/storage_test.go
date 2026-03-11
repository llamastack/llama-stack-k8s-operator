package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

func TestApplyStorage_Nil(t *testing.T) {
	config := map[string]interface{}{
		"metadata_store": map[string]interface{}{"type": "sqlite", "db_path": "/data/original.db"},
	}

	ApplyStorage(nil, config)
	assert.Equal(t, "sqlite", config["metadata_store"].(map[string]interface{})["type"])
	assert.Equal(t, "/data/original.db", config["metadata_store"].(map[string]interface{})["db_path"])
}

func TestApplyStorage_KV_SQLite(t *testing.T) {
	config := map[string]interface{}{}

	ApplyStorage(&v1alpha2.StorageSpec{
		KV: &v1alpha2.KVStorageSpec{Type: "sqlite"},
	}, config)

	kv := config["metadata_store"].(map[string]interface{})
	assert.Equal(t, "sqlite", kv["type"])
	assert.Equal(t, "/data/kv_store.db", kv["db_path"])
}

func TestApplyStorage_KV_SQLiteDefault(t *testing.T) {
	config := map[string]interface{}{}

	ApplyStorage(&v1alpha2.StorageSpec{
		KV: &v1alpha2.KVStorageSpec{},
	}, config)

	kv := config["metadata_store"].(map[string]interface{})
	assert.Equal(t, "sqlite", kv["type"])
}

func TestApplyStorage_KV_Redis(t *testing.T) {
	config := map[string]interface{}{}

	ApplyStorage(&v1alpha2.StorageSpec{
		KV: &v1alpha2.KVStorageSpec{
			Type:     "redis",
			Endpoint: "redis://redis:6379",
			Password: &v1alpha2.SecretKeyRef{Name: "redis-creds", Key: "password"},
		},
	}, config)

	kv := config["metadata_store"].(map[string]interface{})
	assert.Equal(t, "redis", kv["type"])
	assert.Equal(t, "redis://redis:6379", kv["endpoint"])
	assert.Equal(t, "${env.LLSD_STORAGE_KV_PASSWORD}", kv["password"])
}

func TestApplyStorage_SQL_Sqlite(t *testing.T) {
	config := map[string]interface{}{}

	ApplyStorage(&v1alpha2.StorageSpec{
		SQL: &v1alpha2.SQLStorageSpec{Type: "sqlite"},
	}, config)

	sql := config["inference_store"].(map[string]interface{})
	assert.Equal(t, "sqlite", sql["type"])
	assert.Equal(t, "/data/inference_store.db", sql["db_path"])
}

func TestApplyStorage_SQL_Postgres(t *testing.T) {
	config := map[string]interface{}{}

	ApplyStorage(&v1alpha2.StorageSpec{
		SQL: &v1alpha2.SQLStorageSpec{
			Type:             "postgres",
			ConnectionString: &v1alpha2.SecretKeyRef{Name: "pg-creds", Key: "conn"},
		},
	}, config)

	sql := config["inference_store"].(map[string]interface{})
	assert.Equal(t, "postgres", sql["type"])
	assert.Equal(t, "${env.LLSD_STORAGE_SQL_CONNECTION_STRING}", sql["connection_string"])
}

func TestApplyStorage_IndependentSubsections(t *testing.T) {
	config := map[string]interface{}{
		"metadata_store":  map[string]interface{}{"type": "sqlite", "db_path": "/old/kv.db"},
		"inference_store": map[string]interface{}{"type": "sqlite", "db_path": "/old/sql.db"},
	}

	// Only override KV, SQL should remain unchanged
	ApplyStorage(&v1alpha2.StorageSpec{
		KV: &v1alpha2.KVStorageSpec{
			Type:     "redis",
			Endpoint: "redis://redis:6379",
		},
	}, config)

	kv := config["metadata_store"].(map[string]interface{})
	assert.Equal(t, "redis", kv["type"])

	sql := config["inference_store"].(map[string]interface{})
	assert.Equal(t, "sqlite", sql["type"])
	assert.Equal(t, "/old/sql.db", sql["db_path"])
}
